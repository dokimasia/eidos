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

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// Fixture is the input that a language passes to the suite. It contains a
// sealed graph that the frontend of the language loaded, the facts that
// the run stamped on it, the keys of the kernel under which those facts
// registered, and the findings of the load.
type Fixture struct {
	// Graph is the sealed graph that the projections read.
	Graph *store.Graph
	// Facts is the fact store of the run, and Keys contains the handles of
	// the kernel. A fixture without facts reads no authored values.
	Facts *meta.Facts
	Keys  meta.KernelKeys
	// Findings are the findings of the load in the order of
	// [diag.Sink.All]. [Loaded] fails on an Error, so its findings are
	// Warnings and Infos. The suite does not read them.
	Findings []diag.Diag
}

// Setup builds the rules under test and their fixture. The suite calls it
// once for each check.
type Setup func(tb assert.TB) (rules.SourceRules, *Fixture)

// RunRulesSuite checks the rules of a language against the projection
// contract. The contract covers these properties:
//   - Two bounds over one view return equal values.
//   - The fold and the mapping are total.
//   - Each refusal and each gap has a reason.
//   - The two halves of a pair of samples differ.
//   - A backend can spell and substitute each derived witness.
//   - The reads of a sample record on the view that the language receives.
//   - Concurrent passes return the serial values.
func RunRulesSuite(t *testing.T, setup Setup) {
	t.Helper()

	t.Run("returns equal values from two bounds", func(t *testing.T) {
		t.Parallel()
		AssertDeterministic(t, setup)
	})
	t.Run("returns a projection for every input", func(t *testing.T) {
		t.Parallel()
		AssertTotal(t, setup)
	})
	t.Run("returns a reason with each refusal", func(t *testing.T) {
		t.Parallel()
		AssertRefusesWithReason(t, setup)
	})
	t.Run("derives distinct samples", func(t *testing.T) {
		t.Parallel()
		AssertDistinctSamples(t, setup)
	})
	t.Run("derives witnesses that a backend can write", func(t *testing.T) {
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

// viewOf returns a view over the graph of the fixture with a fresh read
// set, and returns the set, so that a check can read what the view
// recorded. A fixture without a graph stops the check.
func viewOf(tb assert.TB, f *Fixture) (rules.View, *store.ReadSet) {
	tb.Helper()

	assert.NotNil(tb, f, "the setup returns a fixture")
	assert.NotNil(tb, f.Graph, "the fixture has a graph")
	reads := store.NewReadSet()
	reader, err := f.Graph.Reader(reads, nil)
	assert.NoError(tb, err, "the sealed graph returns a reader")
	return rules.NewView(reader, f.Facts, reads, f.Keys), reads
}

// subjects returns every declaration in the graph, in the order of the
// store.
func subjects(g *store.Graph) []symbol.Symbol {
	var out []symbol.Symbol
	for pkg := range g.Packages() {
		for decl := range node.Declarations(pkg) {
			out = append(out, decl)
		}
	}
	return out
}

// references returns every type reference inside a symbol, including the
// children and the arguments of each reference.
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

// pass contains the comparable values of one run of every projection
// over the graph.
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
// values from every projection. Each call of [assert.Deterministic]
// creates one bound. Each bound memoises its own fold, so every pass
// derives every value again.
func AssertDeterministic(tb assert.TB, setup Setup) {
	tb.Helper()

	r, f := setup(tb)
	view, _ := viewOf(tb, f)
	assert.Deterministic(tb, func(v rules.View) (pass, error) {
		return project(rules.NewBound(r, v, nil), f.Graph), nil
	}, view, "bounds over one view return equal values")
}

// AssertTotal checks that the fold returns a shape for every reference
// and that the mapping reports false for every kind that is not callable,
// without a panic. Each reference and each subject reports on its own.
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
			expect.NotEqual(tb, s.Form, symbol.FormNamed, "a named reference folds to a leaf and not to its name: "+at)
		}
	}
	for _, s := range subjects(f.Graph) {
		_, is := b.CallableOf(s)
		callable := s.Kind() == symbol.KindFunction || s.Kind() == symbol.KindMethod
		expect.Equal(tb, is, callable, "the mapping reports true for a callable and false for every other kind: "+
			s.Kind().String()+" at "+s.Position().String())
	}
	expect.Equal(tb, b.TypeOf(nil).Form, symbol.FormOpaque, "a nil reference folds to opaque")
}

// AssertRefusesWithReason checks that every sample without a value has a
// refusal other than none, and that every gap has a reason. Each sample
// and each gap reports on its own.
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
					"a sample without a value has a refusal: "+field.ID.String())
			}
			if !alternate.OK() {
				expect.NotEqual(tb, alternate.Refusal, rules.RefusedNone,
					"an alternate without a value has a refusal: "+field.ID.String())
			}
		}
		if set, is := b.MembersOf(s); is {
			for _, gap := range set.Gaps {
				expect.NotEqual(tb, gap.Reason, 0, "a gap has a reason: "+gap.Host.String())
			}
		}
	}
	unknown := &node.TypeRef{Spelling: "rulestest.Unknown"}
	sample, _ := b.SamplesOf(symbol.Identity{}, unknown, "unknown")
	expect.False(tb, sample.OK(), "a sample of a spelling that the language cannot reason about has no value")
	expect.NotEqual(tb, sample.Refusal, rules.RefusedNone,
		"a sample of a spelling that the language cannot reason about has a refusal")
}

// AssertDistinctSamples checks that the two halves of every derived pair
// differ, compared field by field at every depth. Each pair reports on
// its own. A check that compares against one half of a pair that does not
// differ passes whenever the subject has that value.
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
				"the two halves of a derived pair differ: "+field.ID.String(), assert.EquateEmpty())
		}
	}
	assert.NotEqual(tb, derived, 0, "the fixture derives at least one pair")
}

// AssertWitnesses checks the generics capability of a language over
// every generic declaration in the fixture. Derive returns a reference
// with a spelling wherever it reports a witness, because a backend
// writes the instantiation from the spelling. [rules.Substitute]
// rewrites every reference that targets a parameter into the witness of
// the parameter, and leaves the reference that it receives unchanged, so
// the check also proves that the frontend targets a type parameter in
// scope. A language without the capability, and a fixture that declares
// no type parameter, have nothing to check. A fixture that declares type
// parameters must reference one whose witnesses derive. Otherwise the
// check fails, because it proves nothing.
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
			expect.NotEqual(tb, w.Spelling, "", "a derived witness has a spelling: "+p.ID.String())
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
				got = rules.Substitute(ref, params, witnesses)
			}, "Substitute leaves the reference that it receives unchanged: "+where)
			if !direct {
				continue
			}
			rewritten++
			assert.NotNil(tb, got, "Substitute returns a reference: "+where)
			expect.Equal(tb, got.Spelling, witnesses[at].Spelling,
				"Substitute rewrites a reference to a type parameter into the witness of the parameter: "+where)
		}
	}
	if generic > 0 {
		assert.NotEqual(tb, rewritten, 0, "the fixture references a type parameter whose witnesses derive")
	}
}

// typeParamsOf returns the type parameters of a declaration.
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

// AssertRecorded checks that the samples of a language read through the
// view that they receive. A sample of a reference that targets a
// declaration reads the declaration, so the read set of the view
// contains the target, and a change to the declaration runs every
// generator that wrote the sample again. The check calls the
// DeriveSamples of the language with a fresh view per field, so no read
// of the walks of the kernel counts toward it.
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
		r.DeriveSamples(field.Type, field.Name, view)
		expect.Contains(tb, slices.Collect(reads.Identities()), field.Type.Target,
			"a sample of a named type reads the declaration through the view that it receives: "+field.ID.String())
	}
	assert.NotEqual(tb, driven, 0, "the fixture has a field whose type is a declaration")
}

// concurrentPasses is the number of passes that [AssertConcurrent] runs
// at once.
const concurrentPasses = 8

// AssertConcurrent checks that concurrent passes of the projections
// return the serial values. [go.dokimi.dev/assert/history.Concurrently]
// runs the passes on goroutines that start together, each over its own
// view. Each pass reports on its own, and a pass that does not finish
// within a minute fails.
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
