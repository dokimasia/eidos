// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/go/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
)

// nameOf returns the expression naming n.
func nameOf(n string) emit.Expr { return emit.Expr{Kind: emit.ExprName, Name: n} }

// callOf returns the expression calling fn with the named arguments.
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

	t.Run("Scaffold", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			give    emit.Stmt
			want    string
			wantErr bool
		}{
			{
				name: "writes a bare return",
				give: emit.Stmt{Kind: emit.StmtReturn},
				want: "\treturn\n",
			},
			{
				name: "writes a return of a value",
				give: emit.Stmt{Kind: emit.StmtReturn, Value: callOf("load", "ctx")},
				want: "\treturn load(ctx)\n",
			},
			{
				name: "writes a declaring assignment with colon-equals",
				give: emit.Stmt{
					Kind: emit.StmtAssign, Names: []string{"res", "err"},
					Value: callOf("next"), Declare: true,
				},
				want: "\tres, err := next()\n",
			},
			{
				name: "writes a plain assignment with equals",
				give: emit.Stmt{
					Kind: emit.StmtAssign, Names: []string{"total"}, Value: nameOf("sum"),
				},
				want: "\ttotal = sum\n",
			},
			{
				name: "writes an expression statement as the call alone",
				give: emit.Stmt{Kind: emit.StmtExpr, Value: callOf("audit", "ctx")},
				want: "\taudit(ctx)\n",
			},
			{
				name: "writes a guard as the nil comparison around its body",
				give: emit.Stmt{
					Kind: emit.StmtGuard, Name: "err",
					Then: []emit.Stmt{{Kind: emit.StmtReturn, Value: nameOf("err")}},
				},
				want: "\tif err != nil {\n\t\treturn err\n\t}\n",
			},
			{
				name:    "returns an error for an assignment binding no name",
				give:    emit.Stmt{Kind: emit.StmtAssign, Value: nameOf("v")},
				wantErr: true,
			},
			{
				name:    "returns an error for a guard naming nothing",
				give:    emit.Stmt{Kind: emit.StmtGuard},
				wantErr: true,
			},
			{
				name:    "returns an error for a statement kind nothing declares",
				give:    emit.Stmt{},
				wantErr: true,
			},
			{
				name:    "returns an error for a broken expression inside",
				give:    emit.Stmt{Kind: emit.StmtExpr, Value: nameOf("")},
				wantErr: true,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.Scaffold(tt.give, &render.ImportSet{})
				if tt.wantErr {
					assert.HasError(t, err, "Go cannot state the statement")
					return
				}
				assert.NoError(t, err, "the statement spells")
				assert.Equal(t, string(got), tt.want, "the Go spelling")
			})
		}
	})
}
