// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package emit_test

import (
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
)

// The scaffolding vocabulary is deliberately small and its
// spellings are API: faults and lint findings name statement and
// expression kinds, and the codec encodes every form whole.
func TestScaffold(t *testing.T) {
	t.Parallel()

	t.Run("StmtKind.String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give emit.StmtKind
			want string
		}{
			{name: "returns return for a return", give: emit.StmtReturn, want: "return"},
			{name: "returns assign for an assignment", give: emit.StmtAssign, want: "assign"},
			{name: "returns expr for an expression statement", give: emit.StmtExpr, want: "expr"},
			{name: "returns guard for a guard", give: emit.StmtGuard, want: "guard"},
			{name: "returns the number of a kind nothing declares", give: emit.StmtKind(9), want: "9"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.String(), tt.want, "the spelling a finding names")
			})
		}
	})

	t.Run("ExprKind.String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give emit.ExprKind
			want string
		}{
			{name: "returns name for a name", give: emit.ExprName, want: "name"},
			{name: "returns call for an application", give: emit.ExprCall, want: "call"},
			{name: "returns value for a value", give: emit.ExprValue, want: "value"},
			{name: "returns the number of a kind nothing declares", give: emit.ExprKind(9), want: "9"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.String(), tt.want, "the spelling a finding names")
			})
		}
	})

	t.Run("Form.String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give emit.Form
			want string
		}{
			{name: "returns default for the default form", give: emit.FormDefault, want: "default"},
			{name: "returns scaffolding for statements", give: emit.FormStmts, want: "scaffolding"},
			{name: "returns template for a template reference", give: emit.FormTemplate, want: "template"},
			{name: "returns verbatim for verbatim text", give: emit.FormVerbatim, want: "verbatim"},
			{name: "returns the number of a form nothing declares", give: emit.Form(9), want: "9"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.String(), tt.want, "the spelling the lint check names")
			})
		}
	})

	t.Run("json.Marshal", func(t *testing.T) {
		t.Parallel()

		t.Run("omits the value of a bare return", func(t *testing.T) {
			t.Parallel()

			encoded, err := json.Marshal(emit.Stmt{Kind: emit.StmtReturn})
			assert.NoError(t, err, "the bare return encodes")
			assert.NotContains(t, string(encoded), "value",
				"the zero expression is absence, not an empty object")
		})
	})

	t.Run("json.Unmarshal", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a guard with the actions it encoded", func(t *testing.T) {
			t.Parallel()

			guard := emit.Stmt{
				Kind: emit.StmtGuard,
				Name: "err",
				Then: []emit.Stmt{{
					Kind:  emit.StmtReturn,
					Value: emit.Expr{Kind: emit.ExprName, Name: "err"},
				}},
			}
			encoded, err := json.Marshal(guard)
			assert.NoError(t, err, "the guard encodes")
			var decoded emit.Stmt
			assert.NoError(t, json.Unmarshal(encoded, &decoded), "the guard decodes")
			assert.Equal(t, decoded, guard, "the round trip returns the value")
		})
	})
}
