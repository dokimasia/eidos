// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/java/backend"
	"go.dokimi.dev/eidos/sdk/emit"
)

// nameOf and callOf are the fixture expression spellings.
func nameOf(n string) emit.Expr { return emit.Expr{Kind: emit.ExprName, Name: n} }

func callOf(fn string, args ...string) emit.Expr {
	f := nameOf(fn)
	e := emit.Expr{Kind: emit.ExprCall, Fn: &f}
	for _, a := range args {
		e.Args = append(e.Args, nameOf(a))
	}
	return e
}

// Every spelling is pinned byte for byte, and so are the two
// refusals that make Java's failure model honest: nothing
// destructures, and a thrown failure needs no guard.
func TestScaffold(t *testing.T) {
	t.Parallel()

	t.Run("spells the vocabulary as Java", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			stmt emit.Stmt
			want string
		}{
			{
				name: "a bare return",
				stmt: emit.Stmt{Kind: emit.StmtReturn},
				want: "    return;\n",
			},
			{
				name: "a return carrying a value",
				stmt: emit.Stmt{Kind: emit.StmtReturn, Value: callOf("load", "ctx")},
				want: "    return load(ctx);\n",
			},
			{
				name: "a declaring assignment binds with var",
				stmt: emit.Stmt{
					Kind: emit.StmtAssign, Names: []string{"res"},
					Value: callOf("next"), Declare: true,
				},
				want: "    var res = next();\n",
			},
			{
				name: "a plain assignment binds with equals",
				stmt: emit.Stmt{
					Kind: emit.StmtAssign, Names: []string{"total"}, Value: nameOf("sum"),
				},
				want: "    total = sum;\n",
			},
			{
				name: "an expression statement is the call alone",
				stmt: emit.Stmt{Kind: emit.StmtExpr, Value: callOf("audit", "ctx")},
				want: "    audit(ctx);\n",
			},
			{
				name: "a bare guard spells nothing, because a thrown failure propagates",
				stmt: emit.Stmt{Kind: emit.StmtGuard, Name: "err"},
				want: "",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				got, err := backend.Scaffold(tt.stmt, nil)
				assert.NoError(t, err, "the fixture spells")
				assert.Equal(t, string(got), tt.want, tt.name)
			})
		}
	})

	t.Run("refuses what Java cannot say", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			stmt emit.Stmt
		}{
			{
				"an assignment binding two names, because nothing destructures",
				emit.Stmt{
					Kind: emit.StmtAssign, Names: []string{"res", "err"},
					Value: callOf("next"), Declare: true,
				},
			},
			{
				"a guard carrying actions, because failures throw and the " +
					"actions would drop",
				emit.Stmt{
					Kind: emit.StmtGuard, Name: "err",
					Then: []emit.Stmt{{Kind: emit.StmtReturn, Value: nameOf("err")}},
				},
			},
			{"an assignment binding no name", emit.Stmt{Kind: emit.StmtAssign, Value: nameOf("v")}},
			{"a statement kind nothing declares", emit.Stmt{}},
			{"a broken expression inside", emit.Stmt{Kind: emit.StmtExpr, Value: nameOf("")}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				_, err := backend.Scaffold(tt.stmt, nil)
				assert.HasError(t, err, tt.name)
			})
		}
	})
}
