// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package emit_test

import (
	"encoding/json"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

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

	t.Run("ValueExpr", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a value expression over the value", func(t *testing.T) {
			t.Parallel()

			got := emit.ValueExpr(forty())
			assert.Equal(t, got.Kind, emit.ExprValue, "the expression is a value")
			assert.Equal(t, *got.Val, forty(), "over the value given")
		})
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

// The kinds spell without allocating, and a value expression allocates
// the value it points at, in the ordinary run, which runs no benchmark.
func TestScaffoldAllocs(t *testing.T) {
	stmt, expr := emit.StmtGuard, emit.ExprCall
	assert.MaxAllocs(t, func() {
		if stmt.String() != "guard" {
			t.Fatal("String spelled another statement kind")
		}
	}, 0, "StmtKind.String allocates nothing for a declared kind")
	assert.MaxAllocs(t, func() {
		if expr.String() != "call" {
			t.Fatal("String spelled another expression kind")
		}
	}, 0, "ExprKind.String allocates nothing for a declared kind")
	value := forty()
	var got emit.Expr
	assert.MaxAllocs(t, func() { got = emit.ValueExpr(value) }, 1, "ValueExpr allocates the value it points at")
	assert.Equal(t, *got.Val, value, "ValueExpr points at the value given")
}

// BenchmarkScaffold measures the kinds' spellings, which faults and
// lint findings name, and a value expression's construction.
func BenchmarkScaffold(b *testing.B) {
	b.Run("StmtKind.String", func(b *testing.B) {
		stmt := emit.StmtGuard
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = stmt.String()
		}
		assert.Equal(b, got, "guard", "String spells StmtGuard")
	})

	b.Run("ExprKind.String", func(b *testing.B) {
		expr := emit.ExprCall
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = expr.String()
		}
		assert.Equal(b, got, "call", "String spells ExprCall")
	})

	b.Run("ValueExpr", func(b *testing.B) {
		value := forty()
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		var got emit.Expr
		for c.Loop() {
			got = emit.ValueExpr(value)
		}
		assert.Equal(b, *got.Val, value, "ValueExpr points at the value given")
	})
}
