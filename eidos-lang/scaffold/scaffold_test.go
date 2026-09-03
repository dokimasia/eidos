// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package scaffold_test

import (
	"strings"
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

// The expression spelling is shared by four targets, so what it
// writes and what it refuses are contract for all of them.
func TestExpr(t *testing.T) {
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
				var b strings.Builder
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
				var b strings.Builder
				assert.HasError(t, scaffold.Expr(&b, tt.expr, &scripted{}), tt.name)
			})
		}
	})
}
