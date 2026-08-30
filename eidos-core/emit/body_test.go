// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package emit_test

import (
	"encoding/json"
	"strconv"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
)

// delegate answers the one-statement scaffold the body tests
// reuse: return f(ctx).
func delegate() emit.Stmt {
	return emit.Stmt{
		Kind: emit.StmtReturn,
		Value: emit.Expr{
			Kind: emit.ExprCall,
			Fn:   &emit.Expr{Kind: emit.ExprName, Name: "f"},
			Args: []emit.Expr{{Kind: emit.ExprName, Name: "ctx"}},
		},
	}
}

// A body is the composition point rendering reads: the standard
// slots, the owner's declared ones, and exactly one content form.
// The handles Declare answers and the one question Form answers
// are both contract.
func TestBody(t *testing.T) {
	t.Parallel()

	t.Run("Declare", func(t *testing.T) {
		t.Parallel()

		t.Run("adds slots in declaration order", func(t *testing.T) {
			t.Parallel()

			var b emit.Body
			b.Declare("checks")
			b.Declare("cleanup")
			assert.Length(t, b.Slots, 2, "both slots landed")
			assert.Equal(t, b.Slots[0].Name, "checks", "in declaration order")
			assert.Equal(t, b.Slots[1].Name, "cleanup", "not name order")
		})

		t.Run("answers the existing slot for a declared name", func(t *testing.T) {
			t.Parallel()

			var b emit.Body
			first := b.Declare("checks")
			first.Append(delegate())
			again := b.Declare("checks")
			assert.Equal(t, again.Len(), 1,
				"declaring twice answers the one slot, holding what it held")
			assert.Length(t, b.Slots, 2-1, "and adds nothing")
		})

		t.Run("handles survive following declarations", func(t *testing.T) {
			t.Parallel()

			var b emit.Body
			held := b.Declare("first")
			for i := range 16 {
				b.Declare("slot-" + strconv.Itoa(i))
			}
			held.Append(delegate())
			got, declared := b.Slot("first")
			assert.True(t, declared, "the slot is still declared")
			assert.Equal(t, got.Len(), 1,
				"an early handle still reaches its slot after the list grew")
		})
	})

	t.Run("Slot", func(t *testing.T) {
		t.Parallel()

		t.Run("answers false for a name the owner never declared", func(t *testing.T) {
			t.Parallel()

			var b emit.Body
			b.Declare("checks")
			_, declared := b.Slot("invented")
			assert.False(t, declared,
				"a contribution into an invented extension point fails here")
		})
	})

	t.Run("Form", func(t *testing.T) {
		t.Parallel()

		t.Run("answers one form per content", func(t *testing.T) {
			t.Parallel()

			var touched emit.Body
			touched.Prologue.Append(delegate())
			tests := []struct {
				name string
				body emit.Body
				want emit.Form
			}{
				{name: "the zero body is the default", want: emit.FormDefault},
				{name: "slots alone stay the default", body: touched, want: emit.FormDefault},
				{
					name: "scaffolding",
					body: emit.Body{Stmts: []emit.Stmt{delegate()}},
					want: emit.FormStmts,
				},
				{
					name: "a template claim",
					body: emit.Body{Ref: &emit.TemplateRef{Name: "method1"}},
					want: emit.FormTemplate,
				},
				{
					name: "verbatim text",
					body: emit.Body{Verbatim: "return nil"},
					want: emit.FormVerbatim,
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					form, err := tt.body.Form()
					assert.NoError(t, err, "one form is lawful")
					assert.Equal(t, form, tt.want, "and named")
				})
			}
		})

		t.Run("refuses two forms naming both", func(t *testing.T) {
			t.Parallel()

			b := emit.Body{
				Stmts:    []emit.Stmt{delegate()},
				Verbatim: "return nil",
			}
			_, err := b.Form()
			assert.HasError(t, err, "content is exactly one form")
			assert.Contains(t, err.Error(), "scaffolding", "naming one")
			assert.Contains(t, err.Error(), "verbatim", "and the other")
		})
	})

	t.Run("IsZero", func(t *testing.T) {
		t.Parallel()

		var b emit.Body
		assert.True(t, b.IsZero(), "the zero body holds nothing")
		b.Epilogue.Append(delegate())
		assert.False(t, b.IsZero(), "a touched slot counts")
		assert.False(t, emit.Body{Verbatim: "x"}.IsZero(), "content counts")
	})

	t.Run("rides the callable kinds", func(t *testing.T) {
		t.Parallel()

		f := &emit.Function{Name: "Handler"}
		f.Body.Verbatim = "return nil"
		m := &emit.Method{Name: "Do"}
		m.Body.Stmts = []emit.Stmt{delegate()}

		encoded, err := emit.EncodeJSON(f)
		assert.NoError(t, err, "the bodied function encodes")
		decoded, err := emit.DecodeJSON(encoded)
		assert.NoError(t, err, "and decodes")
		got, held := decoded.(*emit.Function)
		assert.True(t, held, "as its own kind")
		assert.Equal(t, got.Body.Verbatim, "return nil", "carrying the body whole")

		encoded, err = emit.EncodeJSON(m)
		assert.NoError(t, err, "the bodied method encodes")
		decoded, err = emit.DecodeJSON(encoded)
		assert.NoError(t, err, "and decodes")
		gotM, held := decoded.(*emit.Method)
		assert.True(t, held, "as its own kind")
		assert.Length(t, gotM.Body.Stmts, 1, "carrying the scaffolding whole")
	})

	t.Run("codec", func(t *testing.T) {
		t.Parallel()

		t.Run("an untouched body encodes empty", func(t *testing.T) {
			t.Parallel()

			encoded, err := json.Marshal(emit.Body{})
			assert.NoError(t, err, "the zero body encodes")
			assert.Equal(t, string(encoded), "{}", "and carries nothing")
		})

		t.Run("round-trips through JSON", func(t *testing.T) {
			t.Parallel()

			var b emit.Body
			b.Prologue.Append(delegate())
			b.Declare("checks").Append(emit.Stmt{
				Kind:    emit.StmtAssign,
				Names:   []string{"res", "err"},
				Value:   emit.Expr{Kind: emit.ExprName, Name: "impl.Do"},
				Declare: true,
			})
			b.Ref = &emit.TemplateRef{Name: "method1", Data: map[string]any{"n": "x"}}

			first, err := json.Marshal(b)
			assert.NoError(t, err, "the body encodes")
			var decoded emit.Body
			assert.NoError(t, json.Unmarshal(first, &decoded), "and decodes")
			second, err := json.Marshal(decoded)
			assert.NoError(t, err, "and encodes again")
			assert.Equal(t, string(second), string(first),
				"the round trip answers the same bytes")
		})
	})
}

// BenchmarkBody prices the per-callable operations the render pass
// pays once per body, and the codec the conformance rungs pay per
// encoded declaration.
func BenchmarkBody(b *testing.B) {
	b.Run("the form question", func(b *testing.B) {
		b.ReportAllocs()
		var body emit.Body
		body.Stmts = []emit.Stmt{delegate()}
		for b.Loop() {
			form, err := body.Form()
			if err != nil || form != emit.FormStmts {
				b.Fatal("the scaffold body answers its form")
			}
		}
	})

	b.Run("build the delegate scaffold", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			var body emit.Body
			body.Prologue.Append(delegate())
			body.Stmts = []emit.Stmt{delegate()}
			if body.IsZero() {
				b.Fatal("the scaffold body holds content")
			}
		}
	})

	b.Run("encode a bodied method", func(b *testing.B) {
		b.ReportAllocs()
		m := &emit.Method{Name: "Do"}
		m.Body.Prologue.Append(delegate())
		m.Body.Stmts = []emit.Stmt{delegate()}
		for b.Loop() {
			if _, err := emit.EncodeJSON(m); err != nil {
				b.Fatalf("EncodeJSON: unexpected error: %v", err)
			}
		}
	})
}
