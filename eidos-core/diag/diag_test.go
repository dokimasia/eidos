// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package diag_test

import (
	"testing"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/position"
)

// A diagnostic is the record every layer reports through. Its fields
// and the origins the kernel reports under are both API.
func TestDiag(t *testing.T) {
	t.Parallel()

	t.Run("PluginID", func(t *testing.T) {
		t.Parallel()

		phases := []struct {
			name string
			id   diag.PluginID
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
				if string(tt.id) != tt.want {
					t.Fatalf("%s = %q, want %q", tt.name, tt.id, tt.want)
				}
			})
		}

		t.Run("the kernel phases stay distinct", func(t *testing.T) {
			t.Parallel()

			seen := make(map[diag.PluginID]string, len(phases))
			for _, tt := range phases {
				if first, taken := seen[tt.id]; taken {
					t.Fatalf("%s and %s both spell %q: a consumer filtering by "+
						"origin cannot tell them apart", first, tt.name, tt.id)
				}
				seen[tt.id] = tt.name
			}
		})
	})

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		var d diag.Diag
		if d.Severity != diag.SeverityError {
			t.Fatalf("zero Diag: Severity = %v, want %v", d.Severity, diag.SeverityError)
		}
		if !d.Code.IsZero() {
			t.Fatalf("zero Diag: Code = %v, want the zero code", d.Code)
		}
		if !d.Pos.IsZero() {
			t.Fatalf("zero Diag: Pos = %v, want the zero position, so that a "+
				"finding reported without one is detectable", d.Pos)
		}
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
			if got := d.Related; len(got) != 2 || got[0] != twin || got[1] != export {
				t.Fatalf("Related = %v, want [%v %v]", got, twin, export)
			}
		})
	})
}
