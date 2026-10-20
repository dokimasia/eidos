// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// TestScaffold pins the statements of the neutral vocabulary in Python: the
// body of a function and the body of a method one level deeper.
func TestScaffold(t *testing.T) {
	t.Parallel()

	t.Run("Scaffold", func(t *testing.T) {
		t.Parallel()

		audit := symbol.Identity{
			Lang: python.Lang, Package: "audit", Owner: "Audit", Name: "record", Kind: symbol.KindMethod,
		}
		// The settle rewrites the statements of a body in place, so each case
		// builds its own.
		tests := []struct {
			name string
			give []symbol.Symbol
			want string
		}{
			{
				name: "writes the statements of a function at one level of indentation",
				give: []symbol.Symbol{&emit.Function{Name: "load", Body: emit.Body{Stmts: []emit.Stmt{
					{
						Kind: emit.StmtAssign, Declare: true, Names: []string{"state"},
						Value: emit.Expr{Kind: emit.ExprCall, Fn: &emit.Expr{Kind: emit.ExprName, Name: "begin"}},
					},
					{
						Kind: emit.StmtAssign, Names: []string{"a", "b"},
						Value: emit.Expr{Kind: emit.ExprCall, Fn: &emit.Expr{Kind: emit.ExprName, Name: "pair"}},
					},
					{Kind: emit.StmtExpr, Value: emit.Expr{
						Kind: emit.ExprCall, Fn: &emit.Expr{Kind: emit.ExprName, Name: "commit"},
						Args: []emit.Expr{{Kind: emit.ExprName, Name: "state"}},
					}},
					{Kind: emit.StmtReturn, Value: emit.Expr{Kind: emit.ExprName, Name: "a"}},
				}}}},
				want: "def load() -> None:\n    state = begin()\n    a, b = pair()\n    commit(state)\n    return a\n",
			},
			{
				name: "writes a call of a declaration of another module with its import",
				give: []symbol.Symbol{&emit.Function{Name: "load", Body: emit.Body{Stmts: []emit.Stmt{
					{Kind: emit.StmtExpr, Value: emit.Expr{
						Kind: emit.ExprCall, Fn: &emit.Expr{Kind: emit.ExprRef, Ref: &audit},
						Args: []emit.Expr{{Kind: emit.ExprName, Name: "state"}},
					}},
					{Kind: emit.StmtReturn},
				}}}},
				want: "from audit import Audit\n\n\ndef load() -> None:\n    Audit.record(state)\n    return\n",
			},
			{
				name: "writes nothing for a guard without statements, because Python raises its failures",
				give: []symbol.Symbol{&emit.Function{Name: "load", Body: emit.Body{Stmts: []emit.Stmt{
					{Kind: emit.StmtGuard, Name: "err"},
				}}}},
				want: "def load() -> None: ...\n",
			},
			{
				name: "writes the statements of a method one level inside its class",
				give: []symbol.Symbol{&emit.Struct{Name: rowName, Methods: emit.SlotOf(&emit.Method{
					Name: "load", Body: emit.Body{Stmts: []emit.Stmt{{
						Kind: emit.StmtAssign, Declare: true, Names: []string{"state"},
						Value: emit.Expr{Kind: emit.ExprCall, Fn: &emit.Expr{Kind: emit.ExprName, Name: "begin"}},
					}}},
				})}},
				want: "class Row:\n    def load(self) -> None:\n        state = begin()\n",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, diags := rendered(t, backend.New(), versionPolicy(t, python.Version312), tt.give...)
				assert.Empty(t, diags, "the render reports nothing")
				assert.Equal(t, got, tt.want, "the file")
			})
		}

		// Each case renders after a class Box whose parameter T declares the
		// type variable T, so a parameter T with a bound takes the type
		// variable T2 in Python 3.11. Its statement constructs a Box of the
		// parameter. The settle rewrites the declarations, so each case
		// builds its own.
		renames := []struct {
			name string
			give symbol.Symbol
			want string
		}{
			{
				name: "writes a type parameter of a function by the name of its type variable",
				give: &emit.Function{
					Name:       "load",
					TypeParams: []*emit.TypeParam{{Name: "T", Bounds: []*emit.TypeRef{{Spelling: "Codec"}}}},
					Body: emit.Body{Stmts: []emit.Stmt{{Kind: emit.StmtReturn, Value: emit.ValueExpr(
						emit.Construction(&emit.TypeRef{Spelling: "Box", Args: []*emit.TypeRef{{Spelling: "T"}}}),
					)}}},
				},
				want: "def load() -> None:\n    return Box[T2]()\n",
			},
			{
				name: "writes a type parameter of the class of a method by the name of its type variable",
				give: &emit.Struct{
					Name:       "Pack",
					TypeParams: []*emit.TypeParam{{Name: "T", Bounds: []*emit.TypeRef{{Spelling: "Codec"}}}},
					Methods: emit.SlotOf(&emit.Method{Name: "box", Body: emit.Body{Stmts: []emit.Stmt{{
						Kind: emit.StmtReturn, Value: emit.ValueExpr(
							emit.Construction(&emit.TypeRef{Spelling: "Box", Args: []*emit.TypeRef{{Spelling: "T"}}}),
						),
					}}}}),
				},
				want: "    def box(self) -> None:\n        return Box[T2]()\n",
			},
			{
				name: "writes a type parameter of a method that hides the parameter of its class by its own variable",
				give: &emit.Struct{
					Name:       "Pack",
					TypeParams: []*emit.TypeParam{{Name: "T", Bounds: []*emit.TypeRef{{Spelling: "Codec"}}}},
					Methods: emit.SlotOf(&emit.Method{
						Name: "box", TypeParams: []*emit.TypeParam{{Name: "T"}},
						Body: emit.Body{Stmts: []emit.Stmt{{Kind: emit.StmtReturn, Value: emit.ValueExpr(
							emit.Construction(&emit.TypeRef{Spelling: "Box", Args: []*emit.TypeRef{{Spelling: "T"}}}),
						)}}},
					}),
				},
				want: "    def box(self) -> None:\n        return Box[T]()\n",
			},
		}
		for _, tt := range renames {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, diags := rendered(t, backend.New(), versionPolicy(t, python.Version311),
					&emit.Struct{Name: "Box", TypeParams: []*emit.TypeParam{{Name: "T"}}}, tt.give)
				assert.Empty(t, diags, "the render reports nothing")
				assert.Contains(t, got, tt.want, "the statement spells the type variable of the signature")
			})
		}

		t.Run("writes a type parameter of the syntax of PEP 695 by its own name", func(t *testing.T) {
			t.Parallel()

			got, diags := rendered(t, backend.New(), versionPolicy(t, python.Version312), &emit.Function{
				Name: "load", TypeParams: []*emit.TypeParam{{Name: "T"}},
				Body: emit.Body{Stmts: []emit.Stmt{{Kind: emit.StmtReturn, Value: emit.ValueExpr(
					emit.Construction(&emit.TypeRef{Spelling: "Box", Args: []*emit.TypeRef{{Spelling: "T"}}}),
				)}}},
			})
			assert.Empty(t, diags, "the render reports nothing")
			assert.Equal(t, got, "def load[T]() -> None:\n    return Box[T]()\n", "the file without a type variable")
		})

		t.Run("reports a type parameter of a body that Python refuses", func(t *testing.T) {
			t.Parallel()

			_, diags := rendered(t, backend.New(), versionPolicy(t, python.Version311),
				&emit.Function{
					Name:       "load",
					TypeParams: []*emit.TypeParam{{Name: "*Ts", Bounds: []*emit.TypeRef{{Spelling: "int"}}}},
					Body: emit.Body{Stmts: []emit.Stmt{{Kind: emit.StmtReturn, Value: emit.ValueExpr(
						emit.Construction(&emit.TypeRef{Spelling: "Box", Args: []*emit.TypeRef{{Spelling: "*Ts"}}}),
					)}}},
				},
				&emit.Constant{Name: "LIMIT", Value: "8"},
			)
			assert.Length(t, diags, 1, "the render reports the body")
			assert.Contains(t, diags[0].Msg,
				"python: a type variable tuple and a parameter specification take no bound, and *Ts states one",
				"the report names the type parameter")
		})

		t.Run("reports a guard with statements, because Python raises its failures", func(t *testing.T) {
			t.Parallel()

			got, diags := rendered(t, backend.New(), versionPolicy(t, python.Version312),
				&emit.Function{Name: "load", Body: emit.Body{Stmts: []emit.Stmt{{
					Kind: emit.StmtGuard, Name: "err", Then: []emit.Stmt{{Kind: emit.StmtReturn}},
				}}}},
				&emit.Constant{Name: "LIMIT", Value: "8"},
			)
			assert.Length(t, diags, 1, "the render reports the guard")
			assert.Contains(t, diags[0].Msg,
				"python: a guard states 1 statements, and python throws its failures, so no value is left to test",
				"the report names the guard")
			assert.NotContains(t, got, "def load", "the render skips the function")
		})
	})
}
