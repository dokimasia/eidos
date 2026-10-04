// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package scaffold_test

import (
	"bytes"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/scaffold"
	"go.dokimi.dev/eidos/sdk/emit"
)

// The allocations of a statement whose values allocate nothing.
const (
	// statementAllocs is a statement's buffer.
	statementAllocs = 1
	// tupleAllocs is an assignment of two names: the buffer and the
	// comma-joined names passed to Tuple.
	tupleAllocs = statementAllocs + 1
	// guardAllocs is a guard with one action: the buffer and the
	// condition the fixture grammar's Guard concatenates.
	guardAllocs = statementAllocs + 1
)

// bare, ended and throwing are the fixture grammars. bare ends a
// statement at its line break and declares by operator, ended ends
// every statement with a semicolon and destructures, and throwing
// throws its failures.
var (
	bare = scaffold.Grammar{
		Lang:      "bare",
		Indent:    "\t",
		DeclareOp: " := ",
		Tuple:     func(names string) string { return names },
		Guard:     func(name string) string { return "if " + name + " != nil" },
	}
	ended = scaffold.Grammar{
		Lang:    "ended",
		Indent:  "  ",
		End:     ";",
		Declare: "let ",
		Tuple:   func(names string) string { return "(" + names + ")" },
		Guard:   func(name string) string { return "if (" + name + ")" },
	}
	throwing = scaffold.Grammar{Lang: "throwing", Indent: "    ", End: ";", Declare: "var "}
)

// The Go, Java, Rust and TypeScript backends write their statements
// through Scaffold and Expr. What each writes and what each refuses is
// pinned for all four.
func TestScaffold(t *testing.T) {
	t.Parallel()

	t.Run("Scaffold", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			grammar scaffold.Grammar
			give    emit.Stmt
			want    string
		}{
			{name: "writes a bare return", grammar: bare, give: emit.Stmt{Kind: emit.StmtReturn}, want: "\treturn\n"},
			{
				name: "writes a return with its value", grammar: ended,
				give: returnLoad(),
				want: "  return load(ctx);\n",
			},
			{
				name: "writes a declaring assignment through the grammar's operator", grammar: bare,
				give: assignNext(),
				want: "\tres, err := next()\n",
			},
			{
				name: "writes a declaring assignment of a tuple after the grammar's keyword", grammar: ended,
				give: assignNext(),
				want: "  let (res, err) = next();\n",
			},
			{
				name: "writes a plain assignment with equals", grammar: ended,
				give: emit.Stmt{Kind: emit.StmtAssign, Names: []string{"total"}, Value: name("sum")},
				want: "  total = sum;\n",
			},
			{
				name: "writes an expression statement as the call alone", grammar: throwing,
				give: emit.Stmt{Kind: emit.StmtExpr, Value: call(name("audit"), name("ctx"))},
				want: "    audit(ctx);\n",
			},
			{
				name: "writes a guard as a block that indents its actions", grammar: ended, give: guardErr(),
				want: "  if (err) {\n    return err;\n  }\n",
			},
			{
				name: "writes nothing for a bare guard where failures throw", grammar: throwing,
				give: emit.Stmt{Kind: emit.StmtGuard, Name: "err"}, want: "",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := scaffold.Scaffold(tt.grammar, tt.give, &scripted{})
				assert.NoError(t, err, "the fixture spells")
				assert.Equal(t, string(got), tt.want, "the statement")
			})
		}

		refusals := []struct {
			name    string
			grammar scaffold.Grammar
			give    emit.Stmt
			want    string
		}{
			{
				name: "returns an error for an assignment that binds no name", grammar: bare,
				give: emit.Stmt{Kind: emit.StmtAssign, Value: name("v")}, want: "binds no name",
			},
			{
				name: "returns an error for two names where the grammar binds one per assignment", grammar: throwing,
				give: emit.Stmt{Kind: emit.StmtAssign, Names: []string{"res", "err"}, Value: name("v")},
				want: "binds 2 names",
			},
			{
				name: "returns an error for a guard with actions where failures throw", grammar: throwing,
				give: guardErr(), want: "throws its failures",
			},
			{
				name: "returns an error for a guard that names nothing", grammar: bare,
				give: emit.Stmt{Kind: emit.StmtGuard}, want: "names no value",
			},
			{
				name: "returns an error for a statement kind nothing declares", grammar: bare,
				give: emit.Stmt{}, want: "no spelling",
			},
			{
				name: "returns an error for a broken action inside a guard", grammar: bare,
				give: emit.Stmt{Kind: emit.StmtGuard, Name: "err", Then: []emit.Stmt{{}}}, want: "no spelling",
			},
			{
				name: "returns an error for a broken expression inside a statement", grammar: ended,
				give: emit.Stmt{Kind: emit.StmtExpr, Value: name("")}, want: "spells nothing",
			},
			{
				name: "returns an error for a broken value inside a return", grammar: ended,
				give: emit.Stmt{Kind: emit.StmtReturn, Value: name("")}, want: "spells nothing",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := scaffold.Scaffold(tt.grammar, tt.give, &scripted{})
				assert.HasError(t, err, "the grammar cannot state the statement")
				assert.Contains(t, err.Error(), tt.want, "the refusal names what is missing")
			})
		}

		t.Run("returns nil bytes for a statement kind nothing declares", func(t *testing.T) {
			t.Parallel()

			got, _ := scaffold.Scaffold(bare, emit.Stmt{}, &scripted{})
			assert.Nil(t, got, "no bytes for the refused statement")
		})
	})

	t.Run("Expr", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give emit.Expr
			want string
		}{
			{name: "writes a name as itself", give: name("next"), want: "next"},
			{name: "writes a dotted name as itself", give: name("s.inner"), want: "s.inner"},
			{name: "writes a call without arguments with empty parentheses", give: call(name("next")), want: "next()"},
			{
				name: "writes arguments joined with commas",
				give: call(name("load"), name("ctx"), name("key")), want: "load(ctx, key)",
			},
			{name: "writes a call of a call", give: call(call(name("pick")), name("x")), want: "pick()(x)"},
			{name: "writes a value expression as its value", give: emit.ValueExpr(integer("42")), want: "42"},
			{
				name: "writes a value expression as a call's argument",
				give: call(name("want"), emit.ValueExpr(integer("42"))), want: "want(42)",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var b bytes.Buffer
				assert.NoError(t, scaffold.Expr(&b, tt.give, &scripted{}), "the fixture spells")
				assert.Equal(t, b.String(), tt.want, "the expression")
			})
		}

		refusals := []struct {
			name string
			give emit.Expr
		}{
			{name: "returns an error for an empty name", give: name("")},
			{name: "returns an error for a call that applies nothing", give: emit.Expr{Kind: emit.ExprCall}},
			{name: "returns an error for a kind nothing declares", give: emit.Expr{}},
			{name: "returns an error for a broken argument inside a call", give: call(name("f"), name(""))},
			{name: "returns an error for a broken function inside a call", give: call(name(""))},
			{name: "returns an error for a value expression without a value", give: emit.Expr{Kind: emit.ExprValue}},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var b bytes.Buffer
				assert.HasError(t, scaffold.Expr(&b, tt.give, &scripted{}), "no target spells it")
			})
		}

		t.Run("returns an error for a value expression without a target", func(t *testing.T) {
			t.Parallel()

			var b bytes.Buffer
			assert.HasError(t, scaffold.Expr(&b, emit.ValueExpr(integer("42")), nil), "nothing spells the value")
		})

		t.Run("returns the error the target returns for a value", func(t *testing.T) {
			t.Parallel()

			var b bytes.Buffer
			raw := emit.ValueExpr(emit.Literal(emit.LiteralRaw, "Row{}"))
			err := scaffold.Expr(&b, raw, &scripted{refuseRaw: true})
			assert.HasError(t, err, "the target refuses the value")
			assert.Contains(t, err.Error(), "another language", "the target's refusal passes through")
		})
	})
}

// Scaffold allocates each statement's buffer, and Expr writes into the
// caller's. The ordinary run, which runs no benchmark, checks those
// ceilings here.
func TestScaffoldAllocs(t *testing.T) {
	ret, assign, check := returnLoad(), assignNext(), guardErr()
	expr := call(name("load"), name("ctx"), emit.ValueExpr(integer("42")))
	var (
		got []byte
		err error
		buf bytes.Buffer
	)
	assert.MaxAllocs(t, func() { got, err = scaffold.Scaffold(ended, ret, passthrough{}) }, statementAllocs,
		"Scaffold allocates the statement's buffer")
	assert.NoError(t, err, "Scaffold spells the return")
	assert.Equal(t, string(got), "  return load(ctx);\n", "Scaffold writes the return")
	assert.MaxAllocs(t, func() { got, err = scaffold.Scaffold(bare, assign, passthrough{}) }, tupleAllocs,
		"Scaffold allocates the joined names of a tuple")
	assert.NoError(t, err, "Scaffold spells the assignment")
	assert.MaxAllocs(t, func() { got, err = scaffold.Scaffold(ended, check, passthrough{}) }, guardAllocs,
		"Scaffold allocates the buffer beside the grammar's guard")
	assert.NoError(t, err, "Scaffold spells the guard")
	assert.Contains(t, string(got), "    return err;\n", "Scaffold indents the action two levels")
	buf.Grow(len("load(ctx, 42)"))
	assert.MaxAllocs(t, func() { buf.Reset(); err = scaffold.Expr(&buf, expr, passthrough{}) }, 0,
		"Expr allocates nothing in a grown buffer")
	assert.NoError(t, err, "Expr spells the call")
	assert.Equal(t, buf.String(), "load(ctx, 42)", "Expr writes the call")
}

// BenchmarkScaffold measures each statement form a backend scaffolds
// once per scaffolded declaration, and the expression writer under it.
func BenchmarkScaffold(b *testing.B) {
	statements := []struct {
		name    string
		grammar scaffold.Grammar
		give    emit.Stmt
		allocs  uint64
	}{
		{name: "Scaffold", grammar: ended, give: returnLoad(), allocs: statementAllocs},
		{name: "Scaffold/an assignment of two names", grammar: bare, give: assignNext(), allocs: tupleAllocs},
		{name: "Scaffold/a guard", grammar: ended, give: guardErr(), allocs: guardAllocs},
	}
	for _, tt := range statements {
		b.Run(tt.name, func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(tt.allocs)
			defer c.End()
			var (
				got []byte
				err error
			)
			for c.Loop() {
				got, err = scaffold.Scaffold(tt.grammar, tt.give, passthrough{})
			}
			assert.NoError(b, err, "Scaffold spells the statement")
			assert.NotEmpty(b, got, "Scaffold writes the statement")
		})
	}

	b.Run("Expr", func(b *testing.B) {
		expr := call(name("load"), name("ctx"), emit.ValueExpr(integer("42")))
		var buf bytes.Buffer
		buf.Grow(len("load(ctx, 42)"))
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var err error
		for c.Loop() {
			buf.Reset()
			err = scaffold.Expr(&buf, expr, passthrough{})
		}
		assert.NoError(b, err, "Expr spells the call")
		assert.Equal(b, buf.String(), "load(ctx, 42)", "Expr writes the call")
	})
}

// name returns a name expression.
func name(n string) emit.Expr { return emit.Expr{Kind: emit.ExprName, Name: n} }

// call returns an application of fn to args.
func call(fn emit.Expr, args ...emit.Expr) emit.Expr {
	return emit.Expr{Kind: emit.ExprCall, Fn: &fn, Args: args}
}

// returnLoad returns the statement return load(ctx).
func returnLoad() emit.Stmt {
	return emit.Stmt{Kind: emit.StmtReturn, Value: call(name("load"), name("ctx"))}
}

// assignNext returns the declaring assignment of res and err from
// next().
func assignNext() emit.Stmt {
	return emit.Stmt{Kind: emit.StmtAssign, Names: []string{"res", "err"}, Value: call(name("next")), Declare: true}
}

// guardErr returns a guard on err whose one action returns err.
func guardErr() emit.Stmt {
	return emit.Stmt{
		Kind: emit.StmtGuard, Name: "err",
		Then: []emit.Stmt{{Kind: emit.StmtReturn, Value: name("err")}},
	}
}
