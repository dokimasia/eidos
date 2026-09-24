// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rulestest

import (
	"slices"
	"sync"
	"testing"

	"go.dokimi.dev/assert"

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
// recorded. A fixture without a graph fails the check and reports
// false.
func viewOf(tb assert.TB, f *Fixture) (rules.View, *store.ReadSet, bool) {
	tb.Helper()

	if f == nil || f.Graph == nil {
		tb.Errorf("the setup has no graph")
		return rules.View{}, nil, false
	}
	reads := store.NewReadSet()
	reader, err := f.Graph.Reader(reads, nil)
	assert.NoError(tb, err, "the sealed graph hands out a reader")
	return rules.View{Decls: reader, Facts: f.Facts, Reads: reads, Kernel: f.Keys}, reads, true
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

// AssertDeterministic checks that two bounds over one view return
// equal values from every projection. Each bound memoises its own
// fold, so the second pass derives every value again.
func AssertDeterministic(tb assert.TB, setup Setup) {
	tb.Helper()

	r, f := setup(tb)
	view, _, ok := viewOf(tb, f)
	if !ok {
		return
	}
	one := project(rules.NewBound(r, view, nil), f.Graph)
	two := project(rules.NewBound(r, view, nil), f.Graph)
	assert.Equal(tb, two, one, "two bounds over one view return equal values")
}

// AssertTotal checks that the fold returns a shape for every
// reference and that the mapping reports false for every
// non-callable kind, without panicking.
func AssertTotal(tb assert.TB, setup Setup) {
	tb.Helper()

	r, f := setup(tb)
	view, _, ok := viewOf(tb, f)
	if !ok {
		return
	}
	b := rules.NewBound(r, view, nil)
	for _, ref := range graphReferences(f.Graph) {
		s := b.TypeOf(ref)
		if ref.Form != symbol.FormNamed && len(ref.Elems) > 0 {
			assert.True(tb, len(s.Elems) > 0 || s.Form == symbol.FormBytes,
				"a structural reference folds its children")
		}
		if ref.Form == symbol.FormNamed {
			assert.False(tb, s.Form == symbol.FormNamed,
				"a named reference folds to a leaf, never to the name itself")
		}
	}
	for _, s := range subjects(f.Graph) {
		_, is := b.CallableOf(s)
		callable := s.Kind() == symbol.KindFunction || s.Kind() == symbol.KindMethod
		assert.Equal(tb, is, callable, "the mapping accepts callables and refuses the rest")
	}
	assert.Equal(tb, b.TypeOf(nil).Form, symbol.FormOpaque, "a nil reference folds to opaque")
}

// AssertRefusesWithReason checks that every refused sample has a
// refusal other than none and that every gap names a reason.
func AssertRefusesWithReason(tb assert.TB, setup Setup) {
	tb.Helper()

	r, f := setup(tb)
	view, _, ok := viewOf(tb, f)
	if !ok {
		return
	}
	b := rules.NewBound(r, view, nil)
	for _, s := range subjects(f.Graph) {
		if field, is := s.(*node.Field); is {
			sample, alternate := b.SamplesOf(field.ID, field.Type, field.Name)
			for _, half := range []rules.Sample{sample, alternate} {
				if !half.OK() {
					assert.False(tb, half.Refusal == rules.RefusedNone,
						"a sample without a value names its refusal")
				}
			}
		}
		if set, is := b.MembersOf(s); is {
			for _, gap := range set.Gaps {
				assert.False(tb, gap.Reason == 0, "a gap names its reason")
			}
		}
	}
	unknown := &node.TypeRef{Spelling: "rulestest.Unknown"}
	sample, _ := b.SamplesOf(symbol.Identity{}, unknown, "unknown")
	assert.False(tb, sample.OK(), "a spelling the language cannot reason about refuses")
	assert.False(tb, sample.Refusal == rules.RefusedNone, "with a reason")
}

// AssertDistinctSamples checks that the two halves of every derived
// pair differ, compared field by field at every depth.
func AssertDistinctSamples(tb assert.TB, setup Setup) {
	tb.Helper()

	r, f := setup(tb)
	view, _, ok := viewOf(tb, f)
	if !ok {
		return
	}
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
			assert.NotEqual(tb, alternate.Value, sample.Value,
				"the two halves differ, or a check comparing against one passes whenever the subject has it",
				assert.EquateEmpty())
		}
	}
	assert.True(tb, derived > 0, "the fixture derives at least one pair, or the check proves nothing")
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
	view, _, ok := viewOf(tb, f)
	generics, capable := r.(rules.GenericsRules)
	if !ok || !capable {
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
			assert.True(tb, w != nil && w.Spelling != "",
				"a derived witness is a reference with a spelling, or no backend can write the instantiation")
			witnesses = append(witnesses, w)
		}
		if len(params) == 0 || len(witnesses) != len(params) {
			continue
		}
		for _, ref := range references(s) {
			before := rules.EmitRef(ref)
			got := generics.Substitute(ref, params, witnesses)
			assert.Equal(tb, rules.EmitRef(ref), before,
				"Substitute copies what it rewrites and leaves the reference it is handed unchanged")
			at := slices.IndexFunc(params, func(p *node.TypeParam) bool {
				return p != nil && ref.Target == p.ID
			})
			if at < 0 || ref.Form != symbol.FormNamed || len(ref.Args) > 0 {
				continue
			}
			rewritten++
			assert.True(tb, got != nil && got.Spelling == witnesses[at].Spelling,
				"a reference to a type parameter becomes the parameter's witness")
		}
	}
	if generic > 0 {
		assert.True(tb, rewritten > 0,
			"the fixture declares type parameters and references none whose witnesses derive, "+
				"so the check proves nothing")
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
	if _, _, ok := viewOf(tb, f); !ok {
		return
	}
	driven := 0
	for _, s := range subjects(f.Graph) {
		field, is := s.(*node.Field)
		if !is || field.Type == nil || field.Type.Target.IsZero() ||
			field.Type.Target.Kind == symbol.KindTypeParam {
			continue
		}
		driven++
		view, reads, _ := viewOf(tb, f)
		r.SamplesOf(field.Type, field.Name, view)
		assert.True(tb, slices.Contains(slices.Collect(reads.Identities()), field.Type.Target),
			"a sample of a named type reads the declaration through the view handed to it, "+
				"or a change there re-runs nothing")
	}
	assert.True(tb, driven > 0, "the fixture has a field typed by a declaration, or the check proves nothing")
}

// AssertConcurrent checks that the projections, run from parallel
// goroutines each over its own view, return the serial values.
func AssertConcurrent(tb assert.TB, setup Setup) {
	tb.Helper()

	r, f := setup(tb)
	serial, _, ok := viewOf(tb, f)
	if !ok {
		return
	}
	want := project(rules.NewBound(r, serial, nil), f.Graph)

	const workers = 8
	views := make([]rules.View, workers)
	for i := range workers {
		views[i], _, _ = viewOf(tb, f)
	}
	got := make([]pass, workers)
	var wg sync.WaitGroup
	for i := range workers {
		wg.Go(func() {
			got[i] = project(rules.NewBound(r, views[i], nil), f.Graph)
		})
	}
	wg.Wait()
	for i := range workers {
		assert.Equal(tb, got[i], want, "a parallel pass returns the serial values")
	}
}
