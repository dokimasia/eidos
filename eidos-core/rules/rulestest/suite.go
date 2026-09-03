// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rulestest

import (
	"sync"
	"testing"

	"go.dokimi.dev/assert"

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

// RunRulesSuite holds a language's rules to the projection
// contract: deterministic answers, a total fold and mapping,
// refusals and gaps with reasons, distinct samples, whole witness
// lists, recorded reads, and equal answers under concurrency.
func RunRulesSuite(t *testing.T, setup Setup) {
	t.Helper()

	t.Run("answers deterministically", func(t *testing.T) {
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
	t.Run("witnesses whole or not at all", func(t *testing.T) {
		t.Parallel()
		AssertWitnessesWhole(t, setup)
	})
	t.Run("records its reads", func(t *testing.T) {
		t.Parallel()
		AssertRecorded(t, setup)
	})
	t.Run("holds under concurrency", func(t *testing.T) {
		t.Parallel()
		AssertConcurrent(t, setup)
	})
}

// bound mints a bound over the fixture with a fresh read set, and
// returns the set so a check can read what was recorded.
func bound(tb assert.TB, r rules.SourceRules, f *Fixture) (rules.Bound, *store.ReadSet) {
	tb.Helper()

	if f == nil || f.Graph == nil {
		tb.Errorf("the setup carries no graph")
		return rules.NewBound(r, rules.View{}, nil), nil
	}
	reads := store.NewReadSet()
	reader, err := f.Graph.Reader(reads, nil)
	assert.NoError(tb, err, "the sealed graph hands out a reader")
	view := rules.View{Decls: reader, Facts: f.Facts, Reads: reads, Kernel: f.Keys}
	return rules.NewBound(r, view, nil), reads
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

// references returns every type reference in the graph.
func references(g *store.Graph) []*node.TypeRef {
	var out []*node.TypeRef
	for pkg := range g.Packages() {
		node.Walk(pkg, func(s symbol.Symbol) bool {
			if ref, is := s.(*node.TypeRef); is {
				out = append(out, ref)
			}
			return true
		})
	}
	return out
}

// answers is one pass of every projection over the graph, as
// comparable values.
type answers struct {
	shapes    []rules.TypeShape
	callables []rules.Callable
	members   []rules.MemberSet
	samples   [][2]rules.Sample
}

// project runs every projection once.
func project(b rules.Bound, g *store.Graph) answers {
	var a answers
	for _, ref := range references(g) {
		a.shapes = append(a.shapes, b.TypeOf(ref))
	}
	for _, s := range subjects(g) {
		if c, is := b.CallableOf(s); is {
			a.callables = append(a.callables, c)
		}
		if m, is := b.MembersOf(s); is {
			a.members = append(a.members, m)
		}
		if f, is := s.(*node.Field); is {
			sample, alternate := b.SamplesOf(f.ID, f.Type, f.Name)
			a.samples = append(a.samples, [2]rules.Sample{sample, alternate})
		}
	}
	return a
}

// AssertDeterministic holds every projection to returning equal
// values on two calls with one view over one graph.
func AssertDeterministic(tb assert.TB, setup Setup) {
	tb.Helper()

	r, f := setup(tb)
	b, _ := bound(tb, r, f)
	one := project(b, f.Graph)
	two := project(b, f.Graph)
	assert.Equal(tb, two, one, "two passes over one view return one answer")
}

// AssertTotal holds the fold to returning a shape for every
// reference and the mapping to reporting false for every
// non-callable kind, without panicking.
func AssertTotal(tb assert.TB, setup Setup) {
	tb.Helper()

	r, f := setup(tb)
	b, _ := bound(tb, r, f)
	for _, ref := range references(f.Graph) {
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

// AssertRefusesWithReason holds every refused sample to carrying a
// refusal other than none, and every gap to naming a reason.
func AssertRefusesWithReason(tb assert.TB, setup Setup) {
	tb.Helper()

	r, f := setup(tb)
	b, _ := bound(tb, r, f)
	for _, s := range subjects(f.Graph) {
		if field, is := s.(*node.Field); is {
			sample, alternate := b.SamplesOf(field.ID, field.Type, field.Name)
			for _, half := range []rules.Sample{sample, alternate} {
				if !half.OK() {
					assert.False(tb, half.Refusal == rules.RefusedNone,
						"a sample without a value carries its refusal")
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

// AssertDistinctSamples holds the two halves of every derived pair
// to differing.
func AssertDistinctSamples(tb assert.TB, setup Setup) {
	tb.Helper()

	r, f := setup(tb)
	b, _ := bound(tb, r, f)
	derived := 0
	for _, s := range subjects(f.Graph) {
		field, is := s.(*node.Field)
		if !is {
			continue
		}
		sample, alternate := b.SamplesOf(field.ID, field.Type, field.Name)
		if sample.OK() && alternate.OK() {
			derived++
			assert.False(tb, sameValue(sample.Value, alternate.Value),
				"the two halves differ, or a check comparing against one passes whenever the subject held it")
		}
	}
	assert.True(tb, derived > 0, "the fixture derives at least one pair, or the check proves nothing")
}

// sameValue reports whether two values spell alike: the same kind,
// literal and text, and the same arguments and fields one level
// down, which is as deep as a derived pair differs.
func sameValue(a, b emit.Value) bool {
	if a.Kind != b.Kind || a.Literal != b.Literal || a.Text != b.Text {
		return false
	}
	if len(a.Args) != len(b.Args) || len(a.Fields) != len(b.Fields) {
		return false
	}
	for i := range a.Args {
		if a.Args[i].Text != b.Args[i].Text {
			return false
		}
	}
	for i := range a.Fields {
		if a.Fields[i].Value.Text != b.Fields[i].Value.Text {
			return false
		}
	}
	if (a.Inner == nil) != (b.Inner == nil) {
		return false
	}
	return a.Inner == nil || sameValue(*a.Inner, *b.Inner)
}

// AssertWitnessesWhole holds every witness list to being nil or one
// entry per parameter.
func AssertWitnessesWhole(tb assert.TB, setup Setup) {
	tb.Helper()

	r, f := setup(tb)
	b, _ := bound(tb, r, f)
	for _, s := range subjects(f.Graph) {
		params := typeParamsOf(s)
		if len(params) == 0 {
			continue
		}
		got := b.Witnesses(params)
		assert.True(tb, got == nil || len(got) == len(params),
			"a witness list is whole or absent, because an entry point instantiates every parameter at once")
	}
	assert.Length(tb, b.Witnesses(nil), 0, "no parameters take no witnesses")
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

// AssertRecorded holds a member walk over a subject to recording
// at least one identity on the view it was handed.
func AssertRecorded(tb assert.TB, setup Setup) {
	tb.Helper()

	r, f := setup(tb)
	b, reads := bound(tb, r, f)
	if reads == nil {
		return
	}
	walked := 0
	for _, s := range subjects(f.Graph) {
		if set, is := b.MembersOf(s); is && len(set.Members) > 0 {
			walked++
		}
		if ref := firstTargeted(s); ref != nil {
			b.TypeOf(ref)
		}
	}
	assert.True(tb, walked > 0, "the fixture holds a type with members, or the check proves nothing")
	recorded := 0
	for range reads.Identities() {
		recorded++
	}
	assert.True(tb, recorded > 0,
		"a projection that read a declaration records it, or a change there re-runs nothing")
}

// firstTargeted returns the first reference in a declaration that
// resolved, and nil where none did.
func firstTargeted(s symbol.Symbol) *node.TypeRef {
	var out *node.TypeRef
	node.Walk(s, func(x symbol.Symbol) bool {
		if ref, is := x.(*node.TypeRef); is && !ref.Target.IsZero() && out == nil {
			out = ref
		}
		return out == nil
	})
	return out
}

// AssertConcurrent holds the projections from parallel goroutines,
// each over its own view, to returning what the serial pass
// returned.
func AssertConcurrent(tb assert.TB, setup Setup) {
	tb.Helper()

	r, f := setup(tb)
	serial, _ := bound(tb, r, f)
	want := project(serial, f.Graph)

	const workers = 8
	got := make([]answers, workers)
	var wg sync.WaitGroup
	for i := range workers {
		wg.Go(func() {
			b, _ := bound(tb, r, f)
			got[i] = project(b, f.Graph)
		})
	}
	wg.Wait()
	for i := range workers {
		assert.Equal(tb, got[i], want, "a parallel pass returns the serial answer")
	}
}
