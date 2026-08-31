// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package emit_test

import (
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
)

// The scaffolding vocabulary is deliberately small and its
// spellings are API: faults and lint findings name statement and
// expression kinds, and the codec carries every form whole.
func TestScaffold(t *testing.T) {
	t.Parallel()

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		t.Run("statement kinds spell their names", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				kind emit.StmtKind
				want string
			}{
				{kind: emit.StmtReturn, want: "return"},
				{kind: emit.StmtAssign, want: "assign"},
				{kind: emit.StmtExpr, want: "expr"},
				{kind: emit.StmtGuard, want: "guard"},
				{kind: emit.StmtKind(9), want: "9"},
			}
			for _, tt := range tests {
				t.Run(tt.want, func(t *testing.T) {
					t.Parallel()
					assert.Equal(t, tt.kind.String(), tt.want,
						"the spelling is what a finding carries")
				})
			}
		})

		t.Run("expression kinds spell their names", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, emit.ExprName.String(), "name", "the local spelling")
			assert.Equal(t, emit.ExprCall.String(), "call", "the application")
			assert.Equal(t, emit.ExprKind(9).String(), "9",
				"a kind nothing declares answers its number")
		})

		t.Run("forms spell their names", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				form emit.Form
				want string
			}{
				{form: emit.FormDefault, want: "default"},
				{form: emit.FormStmts, want: "scaffolding"},
				{form: emit.FormTemplate, want: "template"},
				{form: emit.FormVerbatim, want: "verbatim"},
				{form: emit.Form(9), want: "9"},
			}
			for _, tt := range tests {
				t.Run(tt.want, func(t *testing.T) {
					t.Parallel()
					assert.Equal(t, tt.form.String(), tt.want,
						"the spelling is what the lint check names")
				})
			}
		})
	})

	t.Run("codec", func(t *testing.T) {
		t.Parallel()

		t.Run("a bare return carries no value", func(t *testing.T) {
			t.Parallel()

			encoded, err := json.Marshal(emit.Stmt{Kind: emit.StmtReturn})
			assert.NoError(t, err, "the bare return encodes")
			assert.NotContains(t, string(encoded), "value",
				"the zero expression is absence, not an empty object")
		})

		t.Run("a guard round-trips its consequence", func(t *testing.T) {
			t.Parallel()

			guard := emit.Stmt{
				Kind: emit.StmtGuard,
				Name: "err",
				Then: []emit.Stmt{{
					Kind:  emit.StmtReturn,
					Value: emit.Expr{Kind: emit.ExprName, Name: "err"},
				}},
			}
			first, err := json.Marshal(guard)
			assert.NoError(t, err, "the guard encodes")
			var decoded emit.Stmt
			assert.NoError(t, json.Unmarshal(first, &decoded), "and decodes")
			assert.Equal(t, decoded, guard, "the round trip answers the value")
		})
	})
}
