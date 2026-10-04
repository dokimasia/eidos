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

// The names the allocation cases spell: the result a call binds, and
// the function that returns it.
const (
	resultName = "res"
	nextName   = "next"
)

// The allocations of a statement.
const (
	// scaffoldAllocs is one statement: the buffer it writes into.
	scaffoldAllocs = 1
	// guardAllocs is a guard: the buffer, and the nil comparison.
	guardAllocs = scaffoldAllocs + 1
	// tupleAllocs is an assignment of two names: the buffer, and the
	// comma-joined names.
	tupleAllocs = scaffoldAllocs + 1
)

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

// A statement allocates the buffer it writes into. A guard and an
// assignment of more than one name allocate the text the grammar
// spells. The ordinary run, which runs no benchmark, checks those
// ceilings here.
func TestScaffoldAllocs(t *testing.T) {
	checkAllocs(t, scaffoldCalls())
}

// BenchmarkScaffold measures the spelling of one statement of a
// generated body.
func BenchmarkScaffold(b *testing.B) {
	benchCalls(b, scaffoldCalls())
}

// scaffoldCalls returns a call of Scaffold over a return of a call, a
// guard, and a declaring assignment of two names.
func scaffoldCalls() []allocCall {
	stmt := emit.Stmt{Kind: emit.StmtReturn, Value: callOf(loadName, contextName)}
	guard := emit.Stmt{
		Kind: emit.StmtGuard, Name: errorName, Then: []emit.Stmt{{Kind: emit.StmtReturn, Value: nameOf(errorName)}},
	}
	tuple := emit.Stmt{
		Kind: emit.StmtAssign, Names: []string{resultName, errorName}, Value: callOf(nextName), Declare: true,
	}
	var set render.ImportSet
	var (
		out []byte
		err error
	)
	spells := func(want string) func(tb assert.TB) {
		return func(tb assert.TB) {
			assert.NoError(tb, err, "Scaffold spells the statement")
			assert.Equal(tb, string(out), want, "Scaffold writes the Go spelling")
		}
	}
	return []allocCall{
		{
			name: "Scaffold", allocs: scaffoldAllocs,
			call: func() { out, err = backend.Scaffold(stmt, &set) }, check: spells("\treturn load(ctx)\n"),
		},
		{
			name:   "Scaffold/a guard",
			allocs: guardAllocs,
			call:   func() { out, err = backend.Scaffold(guard, &set) },
			check:  spells("\tif err != nil {\n\t\treturn err\n\t}\n"),
		},
		{
			name: "Scaffold/an assignment of two names", allocs: tupleAllocs,
			call: func() { out, err = backend.Scaffold(tuple, &set) }, check: spells("\tres, err := next()\n"),
		},
	}
}

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
