// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/java/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
)

// literalText is the string the allocation case returns.
const literalText = "hi"

// The allocations of a statement.
const (
	// scaffoldAllocs is one statement: the buffer it writes into.
	scaffoldAllocs = 1
	// stringAllocs is a return of a string: the buffer, and the quoted
	// string.
	stringAllocs = scaffoldAllocs + 1
)

// Every spelling is pinned byte for byte, and so are the two
// refusals that keep Java's failure model honest: nothing
// destructures, and a thrown failure needs no guard.
func TestScaffold(t *testing.T) {
	t.Parallel()

	t.Run("Scaffold", func(t *testing.T) {
		t.Parallel()

		spells := []struct {
			name string
			give emit.Stmt
			want string
		}{
			{
				name: "writes a bare return",
				give: emit.Stmt{Kind: emit.StmtReturn},
				want: "    return;\n",
			},
			{
				name: "writes a return of a value",
				give: emit.Stmt{Kind: emit.StmtReturn, Value: callOf("load", "ctx")},
				want: "    return load(ctx);\n",
			},
			{
				name: "writes a declaring assignment with var",
				give: emit.Stmt{
					Kind: emit.StmtAssign, Names: []string{"res"},
					Value: callOf("next"), Declare: true,
				},
				want: "    var res = next();\n",
			},
			{
				name: "writes a plain assignment with equals",
				give: emit.Stmt{
					Kind: emit.StmtAssign, Names: []string{"total"}, Value: nameOf("sum"),
				},
				want: "    total = sum;\n",
			},
			{
				name: "writes an expression statement as the call alone",
				give: emit.Stmt{Kind: emit.StmtExpr, Value: callOf("audit", "ctx")},
				want: "    audit(ctx);\n",
			},
			{
				name: "writes nothing for a bare guard because a thrown failure propagates",
				give: emit.Stmt{Kind: emit.StmtGuard, Name: "err"},
				want: "",
			},
		}
		for _, tt := range spells {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.Scaffold(tt.give, nil)
				assert.NoError(t, err, "the statement spells")
				assert.Equal(t, string(got), tt.want, "the Java spelling")
			})
		}

		refusals := []struct {
			name string
			give emit.Stmt
		}{
			{
				name: "returns an error for an assignment binding two names because nothing destructures",
				give: emit.Stmt{
					Kind: emit.StmtAssign, Names: []string{"res", "err"},
					Value: callOf("next"), Declare: true,
				},
			},
			{
				name: "returns an error for a guard with actions because a failure throws",
				give: emit.Stmt{
					Kind: emit.StmtGuard, Name: "err",
					Then: []emit.Stmt{{Kind: emit.StmtReturn, Value: nameOf("err")}},
				},
			},
			{
				name: "returns an error for an assignment binding no name",
				give: emit.Stmt{Kind: emit.StmtAssign, Value: nameOf("v")},
			},
			{name: "returns an error for a statement kind nothing declares", give: emit.Stmt{}},
			{
				name: "returns an error for a broken expression inside",
				give: emit.Stmt{Kind: emit.StmtExpr, Value: nameOf("")},
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := backend.Scaffold(tt.give, nil)
				assert.HasError(t, err, "Java cannot state the statement")
			})
		}
	})
}

// A statement allocates the buffer it writes into. The ordinary run,
// which runs no benchmark, checks that ceiling here.
func TestScaffoldAllocs(t *testing.T) {
	checkAllocs(t, scaffoldCalls())
}

// BenchmarkScaffold measures the spelling of one statement of a
// generated body.
func BenchmarkScaffold(b *testing.B) {
	benchCalls(b, scaffoldCalls())
}

// scaffoldCalls returns a call of Scaffold over a return of a call, and
// over a return of a string.
func scaffoldCalls() []allocCall {
	stmt := emit.Stmt{Kind: emit.StmtReturn, Value: callOf("load", "ctx")}
	text := emit.Stmt{Kind: emit.StmtReturn, Value: emit.ValueExpr(emit.Literal(emit.LiteralString, literalText))}
	var set render.ImportSet
	var (
		out []byte
		err error
	)
	spells := func(want string) func(tb assert.TB) {
		return func(tb assert.TB) {
			assert.NoError(tb, err, "Scaffold spells the statement")
			assert.Equal(tb, string(out), want, "Scaffold writes the Java spelling")
		}
	}
	return []allocCall{
		{
			name: "Scaffold", caseName: "a return of a call", allocs: scaffoldAllocs,
			call: func() { out, err = backend.Scaffold(stmt, &set) }, check: spells("    return load(ctx);\n"),
		},
		{
			name: "Scaffold", caseName: "a return of a string", allocs: stringAllocs,
			call: func() { out, err = backend.Scaffold(text, &set) }, check: spells("    return \"hi\";\n"),
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
