// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package scaffold_test

import (
	"bytes"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/scaffold"
	"go.dokimi.dev/eidos/sdk/emit"
)

// name returns a name expression.
func name(n string) emit.Expr { return emit.Expr{Kind: emit.ExprName, Name: n} }

// call returns an application of fn to args.
func call(fn emit.Expr, args ...emit.Expr) emit.Expr {
	return emit.Expr{Kind: emit.ExprCall, Fn: &fn, Args: args}
}

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

// The statement printer and the expression writer are shared by
// four targets, so what they write and what they refuse are
// contract for all of them.
func TestScaffold(t *testing.T) {
	t.Parallel()

	t.Run("Scaffold", func(t *testing.T) {
		t.Parallel()

		guard := emit.Stmt{
			Kind: emit.StmtGuard, Name: "err",
			Then: []emit.Stmt{{Kind: emit.StmtReturn, Value: name("err")}},
		}
		tests := []struct {
			name    string
			grammar scaffold.Grammar
			give    emit.Stmt
			want    string
		}{
			{name: "writes a bare return", grammar: bare, give: emit.Stmt{Kind: emit.StmtReturn}, want: "\treturn\n"},
			{
				name: "writes a return with its value", grammar: ended,
				give: emit.Stmt{Kind: emit.StmtReturn, Value: call(name("load"), name("ctx"))},
				want: "  return load(ctx);\n",
			},
			{
				name: "writes a declaring assignment through the grammar's operator", grammar: bare,
				give: emit.Stmt{
					Kind: emit.StmtAssign, Names: []string{"res", "err"},
					Value: call(name("next")), Declare: true,
				},
				want: "\tres, err := next()\n",
			},
			{
				name: "writes a declaring assignment of a tuple after the grammar's keyword", grammar: ended,
				give: emit.Stmt{
					Kind: emit.StmtAssign, Names: []string{"res", "err"},
					Value: call(name("next")), Declare: true,
				},
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
				name: "writes a guard as a block that indents its actions", grammar: ended, give: guard,
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
				give: emit.Stmt{
					Kind: emit.StmtGuard, Name: "err",
					Then: []emit.Stmt{{Kind: emit.StmtReturn, Value: name("err")}},
				},
				want: "throws its failures",
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
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := scaffold.Scaffold(tt.grammar, tt.give, &scripted{})
				assert.HasError(t, err, "the grammar cannot state the statement")
				assert.Contains(t, err.Error(), tt.want, "the refusal names what is missing")
			})
		}

		t.Run("writes nothing for a statement it refuses", func(t *testing.T) {
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
	})
}
