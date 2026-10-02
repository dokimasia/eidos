// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/workspace"
)

// A plan's status is what a caller branches on and a report prints,
// so its spelling is contract.
func TestReport(t *testing.T) {
	t.Parallel()

	t.Run("PlanStatus.String", func(t *testing.T) {
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
			{name: "returns the number of the zero status", give: workspace.PlanStatus(0), want: "PlanStatus(0)"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.String(), tt.want, "the spelling is pinned")
			})
		}
	})
}
