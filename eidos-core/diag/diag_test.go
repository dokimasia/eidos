// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package diag_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/position"
)

// A diagnostic is the record every layer reports through. Its fields
// and the origins the kernel reports under are both API.
func TestDiag(t *testing.T) {
	t.Parallel()

	t.Run("Origin", func(t *testing.T) {
		t.Parallel()

		phases := []struct {
			name string
			id   diag.Origin
			want string
		}{
			{name: "build", id: diag.PhaseBuild, want: "build"},
			{name: "load", id: diag.PhaseLoad, want: "load"},
			{name: "link", id: diag.PhaseLink, want: "link"},
			{name: "freeze", id: diag.PhaseFreeze, want: "freeze"},
			{name: "annotate", id: diag.PhaseAnnotate, want: "annotate"},
			{name: "generate", id: diag.PhaseGenerate, want: "generate"},
			{name: "render", id: diag.PhaseRender, want: "render"},
			{name: "close", id: diag.PhaseClose, want: "close"},
		}
		for _, tt := range phases {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, string(tt.id), tt.want,
					"a kernel phase reports under its own name")
			})
		}

		t.Run("the kernel phases stay distinct", func(t *testing.T) {
			t.Parallel()

			seen := make(map[diag.Origin]struct{}, len(phases))
			for _, tt := range phases {
				seen[tt.id] = struct{}{}
			}
			assert.Length(t, seen, len(phases),
				"a consumer filtering by origin can tell every phase apart")
		})
	})

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		var d diag.Diag
		assert.Equal(t, d.Severity, diag.SeverityError,
			"a zero Diag does not downgrade itself")
		assert.True(t, d.Code.IsZero(), "a zero Diag names no code")
		assert.True(t, d.Pos.IsZero(),
			"a zero Diag carries no position, so a finding reported without one is detectable")
	})

	t.Run("Related", func(t *testing.T) {
		t.Parallel()

		t.Run("carries the secondary positions in order", func(t *testing.T) {
			t.Parallel()

			twin := position.Pos{File: "svc/store.go", Line: 41, Col: 2}
			export := position.Pos{File: "svc/api.go", Line: 12, Col: 1}
			d := diag.Diag{
				Code:     diag.Code{Prefix: diag.KernelPrefix, Number: 1},
				Severity: diag.SeverityError,
				Pos:      position.Pos{File: "svc/store.go", Line: 7, Col: 6},
				Msg:      "Store is declared twice",
				Origin:   diag.PhaseLink,
				Related:  []position.Pos{twin, export},
			}
			assert.Equal(t, d.Related, []position.Pos{twin, export},
				"Related carries the secondary positions in order")
		})
	})
}
