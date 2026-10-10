// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rulestest

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
)

// Budget is the ceiling a benchmark checks a language against.
type Budget struct {
	// MaxAllocs is the most allocations one projection pass may make.
	// Zero states no ceiling, and [BenchRules] refuses it.
	MaxAllocs uint64
}

// BenchRules drives the four hot projections over the setup's
// graph, one iteration projecting every callable, every type's
// members, every reference's shape and every field's samples, and
// fails when the allocations per iteration exceed the budget. The
// fixture builds once outside the loop, and every iteration binds a
// fresh view. A budget without a ceiling, a setup without a graph,
// and a corpus without a subject or a reference stop the benchmark.
//
// One warm-up iteration runs before the contract counts, so the
// count excludes what a process builds on the first projection: the
// runtime's type-assertion caches and the lazily built state of the
// language's rules.
func BenchRules(b *testing.B, setup Setup, budget Budget) {
	b.Helper()

	assert.NotEqual(b, budget.MaxAllocs, 0, "the budget states a ceiling")
	r, f := setup(b)
	viewOf(b, f)
	subs := subjects(f.Graph)
	refs := graphReferences(f.Graph)
	assert.NotEmpty(b, subs, "the corpus contains a subject to project")
	assert.NotEmpty(b, refs, "the corpus contains a reference to fold")

	c := bench.Start(b).Warmup(1).MaxAllocs(budget.MaxAllocs)
	defer c.End()
	for c.Loop() {
		view, _ := viewOf(b, f)
		bd := rules.NewBound(r, view, nil)
		for _, ref := range refs {
			bd.TypeOf(ref)
		}
		for _, s := range subs {
			bd.CallableOf(s)
			bd.MembersOf(s)
			if field, is := s.(*node.Field); is {
				bd.SamplesOf(field.ID, field.Type, field.Name)
			}
		}
	}
}
