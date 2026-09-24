// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rulestest

import (
	"testing"

	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/rules"
)

// Budget is the ceiling a benchmark checks a language against.
type Budget struct {
	// MaxAllocs is the allocation ceiling per iteration, pinned
	// from a profiled run with headroom. A zero ceiling refuses.
	MaxAllocs uint64
}

// BenchRules drives the four hot projections over the setup's
// graph, one iteration projecting every callable, every type's
// members, every reference's shape and every field's samples, and
// fails when the allocations per iteration exceed the budget. The
// fixture builds once outside the loop, and every iteration binds a
// fresh view.
func BenchRules(b *testing.B, setup Setup, budget Budget) {
	b.Helper()

	if budget.MaxAllocs == 0 {
		b.Fatal("the budget states no ceiling")
	}
	r, f := setup(b)
	if f == nil || f.Graph == nil {
		b.Fatal("the setup has no graph")
	}
	subs := subjects(f.Graph)
	refs := graphReferences(f.Graph)
	if len(subs) == 0 || len(refs) == 0 {
		b.Fatal("the corpus contains nothing to project")
	}

	c := bench.Start(b).MaxAllocs(budget.MaxAllocs)
	defer c.End()
	for c.Loop() {
		view, _, _ := viewOf(b, f)
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
