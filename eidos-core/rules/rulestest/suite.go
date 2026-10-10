// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rulestest

import (
	"slices"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/history"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// Fixture is what a language brings to the suite: a sealed graph
// its frontend loaded, the facts the run stamped on it, and the
// kernel's keys those facts registered under.
type Fixture struct {
	// Graph is the sealed graph the projections read.
	Graph *store.Graph
	// Facts is the run's fact store, and Keys the kernel's handles;
	// a fixture without facts reads no authored values.
	Facts *meta.Facts
	Keys  meta.KernelKeys
}

// Setup builds the rules under test and their fixture, fresh per
// check.
type Setup func(tb assert.TB) (rules.SourceRules, *Fixture)

// RunRulesSuite checks a language's rules against the projection
// contract: equal values from two bounds over one view, a total fold
// and mapping, refusals and gaps with reasons, distinct samples,
// witnesses a backend can spell and substitute, sample reads
// recorded on the view the language is handed, and the serial
// values under concurrency.
func RunRulesSuite(t *testing.T, setup Setup) {
	t.Helper()

	t.Run("returns equal values from two bounds", func(t *testing.T) {
		t.Parallel()
		AssertDeterministic(t, setup)
	})
	t.Run("folds and maps totally", func(t *testing.T) {
		t.Parallel()
		AssertTotal(t, setup)
	})
	t.Run("refuses with a reason", func(t *testing.T) {
		t.Parallel()
		AssertRefusesWithReason(t, setup)
	})
	t.Run("derives distinct samples", func(t *testing.T) {
		t.Parallel()
		AssertDistinctSamples(t, setup)
	})
	t.Run("derives and substitutes witnesses", func(t *testing.T) {
		t.Parallel()
		AssertWitnesses(t, setup)
	})
	t.Run("records its reads", func(t *testing.T) {
		t.Parallel()
		AssertRecorded(t, setup)
	})
	t.Run("returns the serial values under concurrency", func(t *testing.T) {
		t.Parallel()
		AssertConcurrent(t, setup)
	})
}

// viewOf mints a view over the fixture's graph with a fresh read
// set, and returns the set so a check can read what the view
// recorded. A fixture without a graph stops the check.
func viewOf(tb assert.TB, f *Fixture) (rules.View, *store.ReadSet) {
	tb.Helper()

	assert.NotNil(tb, f, "the setup returns a fixture")
	assert.NotNil(tb, f.Graph, "the fixture has a graph")
	reads := store.NewReadSet()
	reader, err := f.Graph.Reader(reads, nil)
	assert.NoError(tb, err, "the sealed graph hands out a reader")
	return rules.View{Decls: reader, Facts: f.Facts, Reads: reads, Kernel: f.Keys}, reads
}

// subjects returns every declaration in the graph, in the store's
// own order.
func subjects(g *store.Graph) []symbol.Symbol {
	var out []symbol.Symbol
	for pkg := range g.Packages() {
		for decl := range node.Declarations(pkg) {
			out = append(out, decl)
		}
	}
	return out
}

// references returns every type reference inside a symbol, the
// children and the arguments of each included.
func references(s symbol.Symbol) []*node.TypeRef {
	var out []*node.TypeRef
	node.Walk(s, func(x symbol.Symbol) bool {
		if ref, is := x.(*node.TypeRef); is {
			out = append(out, ref)
		}
		return true
	})
	return out
}

// graphReferences returns every type reference in the graph.
func graphReferences(g *store.Graph) []*node.TypeRef {
	var out []*node.TypeRef
	for pkg := range g.Packages() {
		out = append(out, references(pkg)...)
	}
	return out
}

// pass is one run of every projection over the graph, as comparable
// values.
type pass struct {
	shapes    []rules.TypeShape
	callables []rules.Callable
	members   []rules.MemberSet
	samples   [][2]rules.Sample
}

// project runs every projection once.
func project(b rules.Bound, g *store.Graph) pass {
	var p pass
	for _, ref := range graphReferences(g) {
		p.shapes = append(p.shapes, b.TypeOf(ref))
	}
	for _, s := range subjects(g) {
		if c, is := b.CallableOf(s); is {
			p.callables = append(p.callables, c)
		}
		if m, is := b.MembersOf(s); is {
			p.members = append(p.members, m)
		}
		if f, is := s.(*node.Field); is {
			sample, alternate := b.SamplesOf(f.ID, f.Type, f.Name)
			p.samples = append(p.samples, [2]rules.Sample{sample, alternate})
		}
	}
	return p
}

// AssertDeterministic checks that bounds over one view return equal
// values from every projection, one bound per call of
// [assert.Deterministic]. Each bound memoises its own fold, so every
// pass derives every value again.
func AssertDeterministic(tb assert.TB, setup Setup) {
	tb.Helper()

	r, f := setup(tb)
	view, _ := viewOf(tb, f)
	assert.Deterministic(tb, func(v rules.View) (pass, error) {
		return project(rules.NewBound(r, v, nil), f.Graph), nil
	}, view, "bounds over one view return equal values")
}

// AssertTotal checks that the fold returns a shape for every
// reference and that the mapping reports false for every
// non-callable kind, without panicking. Each reference and each
// subject reports on its own.
func AssertTotal(tb assert.TB, setup Setup) {
	tb.Helper()

	r, f := setup(tb)
	view, _ := viewOf(tb, f)
	b := rules.NewBound(r, view, nil)
	for _, ref := range graphReferences(f.Graph) {
		s := b.TypeOf(ref)
		at := ref.Spelling + " at " + ref.Pos.String()
		if ref.Form != symbol.FormNamed && len(ref.Elems) > 0 && s.Form != symbol.FormBytes {
			expect.NotEmpty(tb, s.Elems, "a structural reference folds its children, unless it folds to bytes: "+at)
		}
		if ref.Form == symbol.FormNamed {
			expect.NotEqual(tb, s.Form, symbol.FormNamed,
				"a named reference folds to a leaf, never to the name itself: "+at)
		}
	}
	for _, s := range subjects(f.Graph) {
		_, is := b.CallableOf(s)
		callable := s.Kind() == symbol.KindFunction || s.Kind() == symbol.KindMethod
		expect.Equal(tb, is, callable, "the mapping accepts callables and refuses the rest: "+
			s.Kind().String()+" at "+s.Position().String())
	}
	expect.Equal(tb, b.TypeOf(nil).Form, symbol.FormOpaque, "a nil reference folds to opaque")
}

// AssertRefusesWithReason checks that every refused sample has a
// refusal other than none and that every gap names a reason. Each
// sample and each gap reports on its own.
func AssertRefusesWithReason(tb assert.TB, setup Setup) {
	tb.Helper()

	r, f := setup(tb)
	view, _ := viewOf(tb, f)
	b := rules.NewBound(r, view, nil)
	for _, s := range subjects(f.Graph) {
		if field, is := s.(*node.Field); is {
			sample, alternate := b.SamplesOf(field.ID, field.Type, field.Name)
			if !sample.OK() {
				expect.NotEqual(tb, sample.Refusal, rules.RefusedNone,
					"a sample without a value names its refusal: "+field.ID.String())
			}
			if !alternate.OK() {
				expect.NotEqual(tb, alternate.Refusal, rules.RefusedNone,
					"an alternate without a value names its refusal: "+field.ID.String())
			}
		}
		if set, is := b.MembersOf(s); is {
			for _, gap := range set.Gaps {
				expect.NotEqual(tb, gap.Reason, 0, "a gap names its reason: "+gap.Host.String())
			}
		}
	}
	unknown := &node.TypeRef{Spelling: "rulestest.Unknown"}
	sample, _ := b.SamplesOf(symbol.Identity{}, unknown, "unknown")
	expect.False(tb, sample.OK(), "a spelling the language cannot reason about refuses")
	expect.NotEqual(tb, sample.Refusal, rules.RefusedNone,
		"a spelling the language cannot reason about names its refusal")
}

// AssertDistinctSamples checks that the two halves of every derived
// pair differ, compared field by field at every depth. Each pair
// reports on its own.
func AssertDistinctSamples(tb assert.TB, setup Setup) {
	tb.Helper()

	r, f := setup(tb)
	view, _ := viewOf(tb, f)
	b := rules.NewBound(r, view, nil)
	derived := 0
	for _, s := range subjects(f.Graph) {
		field, is := s.(*node.Field)
		if !is {
			continue
		}
		sample, alternate := b.SamplesOf(field.ID, field.Type, field.Name)
		if sample.OK() && alternate.OK() {
			derived++
			expect.NotEqual(tb, alternate.Value, sample.Value,
				"the two halves differ, or a check comparing against one passes whenever the subject has it: "+
					field.ID.String(), assert.EquateEmpty())
		}
	}
	assert.NotEqual(tb, derived, 0, "the fixture derives at least one pair, or the check proves nothing")
}

// AssertWitnesses checks a language's generics capability over every
// generic declaration in the fixture. Derive returns a reference
// with a spelling wherever it reports a witness, because a backend
// writes the instantiation from the spelling. Substitute rewrites
// every reference that targets a parameter into the parameter's
// witness and leaves the reference it is handed unchanged. A
// language without the capability, and a fixture declaring no type
// parameter, have nothing to check. A fixture that declares type
// parameters references one whose witnesses derive, or the check
// fails as proving nothing.
func AssertWitnesses(tb assert.TB, setup Setup) {
	tb.Helper()

	r, f := setup(tb)
	view, _ := viewOf(tb, f)
	generics, capable := r.(rules.GenericsRules)
	if !capable {
		return
	}
	generic, rewritten := 0, 0
	for _, s := range subjects(f.Graph) {
		params := typeParamsOf(s)
		if len(params) > 0 {
			generic++
		}
		witnesses := make([]*node.TypeRef, 0, len(params))
		for _, p := range params {
			w, derived := generics.Derive(p, view)
			if !derived {
				break
			}
			assert.NotNil(tb, w, "a derived witness is a reference: "+p.ID.String())
			expect.NotEqual(tb, w.Spelling, "",
				"a derived witness has a spelling, or no backend can write the instantiation: "+p.ID.String())
			witnesses = append(witnesses, w)
		}
		if len(params) == 0 || len(witnesses) != len(params) {
			continue
		}
		for _, ref := range references(s) {
			where := ref.Spelling + " at " + ref.Pos.String()
			at := slices.IndexFunc(params, func(p *node.TypeParam) bool {
				return p != nil && ref.Target == p.ID
			})
			direct := at >= 0 && ref.Form == symbol.FormNamed && len(ref.Args) == 0
			var got *node.TypeRef
			expect.Pure(tb, func() *emit.TypeRef { return rules.EmitRef(ref) }, func() {
				got = generics.Substitute(ref, params, witnesses)
			}, "Substitute copies what it rewrites and leaves the reference it is handed unchanged: "+where)
			if !direct {
				continue
			}
			rewritten++
			assert.NotNil(tb, got, "Substitute returns a reference: "+where)
			expect.Equal(tb, got.Spelling, witnesses[at].Spelling,
				"a reference to a type parameter becomes the parameter's witness: "+where)
		}
	}
	if generic > 0 {
		assert.NotEqual(tb, rewritten, 0, "the fixture declares type parameters and references one whose "+
			"witnesses derive, or the check proves nothing")
	}
}

// typeParamsOf reads a declaration's type parameters.
func typeParamsOf(s symbol.Symbol) []*node.TypeParam {
	switch d := s.(type) {
	case *node.Struct:
		return d.TypeParams
	case *node.Interface:
		return d.TypeParams
	case *node.Alias:
		return d.TypeParams
	case *node.Sum:
		return d.TypeParams
	case *node.Function:
		return d.TypeParams
	case *node.Method:
		return d.TypeParams
	default:
		return nil
	}
}

// AssertRecorded checks that a language's samples read through the
// view they are handed. A sample of a reference that targets a
// declaration reads the declaration, so the read set the view
// records into contains the target, and a change to the declaration
// re-runs every generator that wrote the sample. The check calls the
// language's own SamplesOf with a fresh view per field, so no read
// the kernel's walks make counts toward it.
func AssertRecorded(tb assert.TB, setup Setup) {
	tb.Helper()

	r, f := setup(tb)
	viewOf(tb, f)
	driven := 0
	for _, s := range subjects(f.Graph) {
		field, is := s.(*node.Field)
		if !is || field.Type == nil || field.Type.Target.IsZero() ||
			field.Type.Target.Kind == symbol.KindTypeParam {

			continue
		}
		driven++
		view, reads := viewOf(tb, f)
		r.SamplesOf(field.Type, field.Name, view)
		expect.Contains(tb, slices.Collect(reads.Identities()), field.Type.Target,
			"a sample of a named type reads the declaration through the view handed to it, "+
				"or a change there re-runs nothing: "+field.ID.String())
	}
	assert.NotEqual(tb, driven, 0, "the fixture has a field typed by a declaration, or the check proves nothing")
}

// concurrentPasses is how many passes [AssertConcurrent] runs at once.
const concurrentPasses = 8

// AssertConcurrent checks that the projections, run by
// [go.dokimi.dev/assert/history.Concurrently] on goroutines released
// together, each over its own view, return the serial values. Each
// pass reports on its own, and one that does not finish within a
// minute fails.
func AssertConcurrent(tb assert.TB, setup Setup) {
	tb.Helper()

	r, f := setup(tb)
	serial, _ := viewOf(tb, f)
	want := project(rules.NewBound(r, serial, nil), f.Graph)

	views := make([]rules.View, concurrentPasses)
	for i := range views {
		views[i], _ = viewOf(tb, f)
	}
	outcomes := history.Concurrently(concurrentPasses, time.Minute, func(client int) (any, error) {
		return project(rules.NewBound(r, views[client], nil), f.Graph), nil
	})
	for _, o := range outcomes {
		expect.True(tb, o.Finished, "a parallel pass finishes")
		expect.Equal(tb, o.Output, any(want), "a parallel pass returns the serial values")
	}
}
