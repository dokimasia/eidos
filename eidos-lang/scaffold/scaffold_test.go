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

// name and call are the fixture spellings.
func name(n string) emit.Expr { return emit.Expr{Kind: emit.ExprName, Name: n} }

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

		t.Run("writes every statement through the grammar", func(t *testing.T) {
			t.Parallel()

			guard := emit.Stmt{
				Kind: emit.StmtGuard, Name: "err",
				Then: []emit.Stmt{{Kind: emit.StmtReturn, Value: name("err")}},
			}
			tests := []struct {
				name    string
				grammar scaffold.Grammar
				stmt    emit.Stmt
				want    string
			}{
				{"a bare return", bare, emit.Stmt{Kind: emit.StmtReturn}, "\treturn\n"},
				{
					"a return carrying a value, ended", ended,
					emit.Stmt{Kind: emit.StmtReturn, Value: call(name("load"), name("ctx"))},
					"  return load(ctx);\n",
				},
				{
					"a declaring assignment by operator", bare,
					emit.Stmt{
						Kind: emit.StmtAssign, Names: []string{"res", "err"},
						Value: call(name("next")), Declare: true,
					},
					"\tres, err := next()\n",
				},
				{
					"a declaring assignment by keyword, destructured", ended,
					emit.Stmt{
						Kind: emit.StmtAssign, Names: []string{"res", "err"},
						Value: call(name("next")), Declare: true,
					},
					"  let (res, err) = next();\n",
				},
				{
					"a plain assignment binds with equals", ended,
					emit.Stmt{Kind: emit.StmtAssign, Names: []string{"total"}, Value: name("sum")},
					"  total = sum;\n",
				},
				{
					"an expression statement is the call alone", throwing,
					emit.Stmt{Kind: emit.StmtExpr, Value: call(name("audit"), name("ctx"))},
					"    audit(ctx);\n",
				},
				{
					"a guard opens a block and indents its actions", ended, guard,
					"  if (err) {\n    return err;\n  }\n",
				},
				{
					"a bare guard spells nothing where failures throw", throwing,
					emit.Stmt{Kind: emit.StmtGuard, Name: "err"},
					"",
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()
					got, err := scaffold.Scaffold(tt.grammar, tt.stmt, &scripted{})
					assert.NoError(t, err, "the fixture spells")
					assert.Equal(t, string(got), tt.want, tt.name)
				})
			}
		})

		t.Run("refuses what the grammar cannot say", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name    string
				grammar scaffold.Grammar
				stmt    emit.Stmt
				mention string
			}{
				{
					"an assignment binding no name", bare,
					emit.Stmt{Kind: emit.StmtAssign, Value: name("v")},
					"binds no name",
				},
				{
					"several names where the grammar binds one per assignment", throwing,
					emit.Stmt{Kind: emit.StmtAssign, Names: []string{"res", "err"}, Value: name("v")},
					"binds 2 names",
				},
				{
					"a guard carrying actions where failures throw", throwing,
					emit.Stmt{
						Kind: emit.StmtGuard, Name: "err",
						Then: []emit.Stmt{{Kind: emit.StmtReturn, Value: name("err")}},
					},
					"throws its failures",
				},
				{"a guard naming nothing", bare, emit.Stmt{Kind: emit.StmtGuard}, "names no value"},
				{"a statement kind nothing declares", bare, emit.Stmt{}, "no spelling"},
				{
					"a broken action inside a guard", bare,
					emit.Stmt{Kind: emit.StmtGuard, Name: "err", Then: []emit.Stmt{{}}},
					"no spelling",
				},
				{
					"a broken expression inside", ended,
					emit.Stmt{Kind: emit.StmtExpr, Value: name("")},
					"spells nothing",
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()
					got, err := scaffold.Scaffold(tt.grammar, tt.stmt, &scripted{})
					assert.HasError(t, err, tt.name)
					assert.Contains(t, err.Error(), tt.mention, "the refusal names what is missing")
					assert.Nil(t, got, "and nothing is written for the statement")
				})
			}
		})
	})

	t.Run("Expr", func(t *testing.T) {
		t.Parallel()

		t.Run("spells names and calls", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				expr emit.Expr
				want string
			}{
				{"a name spells itself", name("next"), "next"},
				{"a dotted name spells itself too", name("s.inner"), "s.inner"},
				{"a bare call closes empty", call(name("next")), "next()"},
				{
					"arguments join with commas",
					call(name("load"), name("ctx"), name("key")), "load(ctx, key)",
				},
				{
					"a call may apply a call",
					call(call(name("pick")), name("x")), "pick()(x)",
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()
					var b bytes.Buffer
					assert.NoError(t, scaffold.Expr(&b, tt.expr, &scripted{}), "the fixture spells")
					assert.Equal(t, b.String(), tt.want, tt.name)
				})
			}
		})

		t.Run("refuses what no target can spell", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				expr emit.Expr
			}{
				{"an empty name", name("")},
				{"a call applying nothing", emit.Expr{Kind: emit.ExprCall}},
				{"a kind nothing declares", emit.Expr{}},
				{"a broken argument inside a call", call(name("f"), name(""))},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()
					var b bytes.Buffer
					assert.HasError(t, scaffold.Expr(&b, tt.expr, &scripted{}), tt.name)
				})
			}
		})
	})
}
