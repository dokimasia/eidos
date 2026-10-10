// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/workspace"
)

// A plan's status is what a caller branches on and a report prints,
// so its spelling is contract.
func TestReport(t *testing.T) {
	t.Parallel()

	t.Run("PlanStatus", func(t *testing.T) {
		t.Parallel()

		t.Run("String", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give workspace.PlanStatus
				want string
			}{
				{name: "returns committed for PlanCommitted", give: workspace.PlanCommitted, want: "committed"},
				{name: "returns failed for PlanFailed", give: workspace.PlanFailed, want: "failed"},
				{name: "returns cancelled for PlanCancelled", give: workspace.PlanCancelled, want: "cancelled"},
				{name: "returns prepared for PlanPrepared", give: workspace.PlanPrepared, want: "prepared"},
				{name: "returns skipped for PlanSkipped", give: workspace.PlanSkipped, want: "skipped"},
				{name: "returns the number of the zero status", give: workspace.PlanStatus(0), want: "PlanStatus(0)"},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, tt.give.String(), tt.want, "the spelling is pinned")
				})
			}
		})
	})
}

// A declared status spells without allocating in the ordinary run,
// which runs no benchmark.
func TestReportAllocs(t *testing.T) {
	status := workspace.PlanCommitted
	var got string
	assert.MaxAllocs(t, func() { got = status.String() }, 0,
		"PlanStatus.String allocates nothing for a declared status")
	assert.Equal(t, got, "committed", "PlanStatus.String spells PlanCommitted")
}

// BenchmarkReport measures the spelling of a plan's status, which a
// report prints once per plan.
func BenchmarkReport(b *testing.B) {
	b.Run("PlanStatus", func(b *testing.B) {
		b.Run("String", func(b *testing.B) {
			status := workspace.PlanCommitted
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var got string
			for c.Loop() {
				got = status.String()
			}
			assert.Equal(b, got, "committed", "PlanStatus.String spells PlanCommitted")
		})
	})
}
