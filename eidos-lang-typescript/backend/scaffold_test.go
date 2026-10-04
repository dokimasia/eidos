// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/typescript/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
)

// The statements spell a function a return calls with its argument, a
// result a call binds with its failure, the function that returns
// them, and a string a return writes.
const (
	loadName    = "load"
	contextName = "ctx"
	resultName  = "res"
	errorName   = "err"
	nextName    = "next"
	literalText = "hi"
)

// The allocations of a statement.
const (
	// scaffoldAllocs is one statement: the buffer it writes into.
	scaffoldAllocs = 1
	// guardAllocs is a guard: the buffer, and the truthiness check.
	guardAllocs = scaffoldAllocs + 1
	// tupleAllocs is an assignment of two names: the buffer, the
	// comma-joined names, and their array pattern.
	tupleAllocs = scaffoldAllocs + 1 + 1
	// stringAllocs is a return of a string: the buffer, and the quoted
	// string.
	stringAllocs = scaffoldAllocs + 1
)

// Every spelling is pinned byte for byte: scaffold output is
// spliced into generated bodies, so a drift here rewrites files.
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
				want: "  return;\n",
			},
			{
				name: "writes a return of a value",
				give: emit.Stmt{Kind: emit.StmtReturn, Value: callOf(loadName, contextName)},
				want: "  return load(ctx);\n",
			},
			{
				name: "writes a declaring assignment of two names as a destructuring const",
				give: emit.Stmt{
					Kind: emit.StmtAssign, Names: []string{resultName, errorName},
					Value: callOf(nextName), Declare: true,
				},
				want: "  const [res, err] = next();\n",
			},
			{
				name: "writes a plain assignment with equals",
				give: emit.Stmt{
					Kind: emit.StmtAssign, Names: []string{"total"}, Value: nameOf("sum"),
				},
				want: "  total = sum;\n",
			},
			{
				name: "writes an expression statement as the call alone",
				give: emit.Stmt{Kind: emit.StmtExpr, Value: callOf("audit", contextName)},
				want: "  audit(ctx);\n",
			},
			{
				name: "writes a guard as the truthiness check around its body",
				give: emit.Stmt{
					Kind: emit.StmtGuard, Name: errorName,
					Then: []emit.Stmt{{Kind: emit.StmtReturn, Value: nameOf(errorName)}},
				},
				want: "  if (err) {\n    return err;\n  }\n",
			},
		}
		for _, tt := range spells {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.Scaffold(tt.give, nil)
				assert.NoError(t, err, "the statement spells")
				assert.Equal(t, string(got), tt.want, "the TypeScript spelling")
			})
		}

		refusals := []struct {
			name string
			give emit.Stmt
		}{
			{
				name: "returns an error for an assignment binding no name",
				give: emit.Stmt{Kind: emit.StmtAssign, Value: nameOf("v")},
			},
			{name: "returns an error for a guard naming nothing", give: emit.Stmt{Kind: emit.StmtGuard}},
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
				assert.HasError(t, err, "TypeScript cannot state the statement")
			})
		}
	})
}

// A statement allocates the buffer it writes into. A guard, an
// assignment of more than one name and a string allocate the text the
// grammar spells. The ordinary run, which runs no benchmark, checks
// those ceilings here.
func TestScaffoldAllocs(t *testing.T) {
	checkAllocs(t, scaffoldCalls())
}

// BenchmarkScaffold measures the spelling of one statement of a
// generated body.
func BenchmarkScaffold(b *testing.B) {
	benchCalls(b, scaffoldCalls())
}

// scaffoldCalls returns a call of Scaffold over a return of a call, a
// guard, a declaring assignment of two names, and a return of a string.
func scaffoldCalls() []allocCall {
	stmt := emit.Stmt{Kind: emit.StmtReturn, Value: callOf(loadName, contextName)}
	guard := emit.Stmt{
		Kind: emit.StmtGuard, Name: errorName, Then: []emit.Stmt{{Kind: emit.StmtReturn, Value: nameOf(errorName)}},
	}
	tuple := emit.Stmt{
		Kind: emit.StmtAssign, Names: []string{resultName, errorName}, Value: callOf(nextName), Declare: true,
	}
	text := emit.Stmt{Kind: emit.StmtReturn, Value: emit.ValueExpr(emit.Literal(emit.LiteralString, literalText))}
	var set render.ImportSet
	var (
		out []byte
		err error
	)
	spells := func(want string) func(tb assert.TB) {
		return func(tb assert.TB) {
			assert.NoError(tb, err, "Scaffold spells the statement")
			assert.Equal(tb, string(out), want, "Scaffold writes the TypeScript spelling")
		}
	}
	return []allocCall{
		{
			name: "Scaffold", allocs: scaffoldAllocs,
			call: func() { out, err = backend.Scaffold(stmt, &set) }, check: spells("  return load(ctx);\n"),
		},
		{
			name:   "Scaffold/a guard",
			allocs: guardAllocs,
			call:   func() { out, err = backend.Scaffold(guard, &set) },
			check:  spells("  if (err) {\n    return err;\n  }\n"),
		},
		{
			name: "Scaffold/an assignment of two names", allocs: tupleAllocs,
			call: func() { out, err = backend.Scaffold(tuple, &set) }, check: spells("  const [res, err] = next();\n"),
		},
		{
			name: "Scaffold/a return of a string", allocs: stringAllocs,
			call: func() { out, err = backend.Scaffold(text, &set) }, check: spells("  return 'hi';\n"),
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
