// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/go/backend"
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

// Every spelling is pinned byte for byte: scaffold output is
// spliced into generated bodies, so a drift here rewrites files.
func TestScaffold(t *testing.T) {
	t.Parallel()

	t.Run("spells the vocabulary as Go", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			stmt emit.Stmt
			want string
		}{
			{
				name: "a bare return",
				stmt: emit.Stmt{Kind: emit.StmtReturn},
				want: "\treturn\n",
			},
			{
				name: "a return carrying a value",
				stmt: emit.Stmt{Kind: emit.StmtReturn, Value: callOf("load", "ctx")},
				want: "\treturn load(ctx)\n",
			},
			{
				name: "a declaring assignment binds with colon-equals",
				stmt: emit.Stmt{
					Kind: emit.StmtAssign, Names: []string{"res", "err"},
					Value: callOf("next"), Declare: true,
				},
				want: "\tres, err := next()\n",
			},
			{
				name: "a plain assignment binds with equals",
				stmt: emit.Stmt{
					Kind: emit.StmtAssign, Names: []string{"total"}, Value: nameOf("sum"),
				},
				want: "\ttotal = sum\n",
			},
			{
				name: "an expression statement is the call alone",
				stmt: emit.Stmt{Kind: emit.StmtExpr, Value: callOf("audit", "ctx")},
				want: "\taudit(ctx)\n",
			},
			{
				name: "a guard is the nil comparison with its body indented",
				stmt: emit.Stmt{
					Kind: emit.StmtGuard, Name: "err",
					Then: []emit.Stmt{{Kind: emit.StmtReturn, Value: nameOf("err")}},
				},
				want: "\tif err != nil {\n\t\treturn err\n\t}\n",
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

	t.Run("refuses what Go cannot say", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			stmt emit.Stmt
		}{
			{"an assignment binding no name", emit.Stmt{Kind: emit.StmtAssign, Value: nameOf("v")}},
			{"a guard naming nothing", emit.Stmt{Kind: emit.StmtGuard}},
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
