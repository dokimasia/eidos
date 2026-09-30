// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package emit_test

import (
	"strconv"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/emit"
)

// The body fixture's names: the owner slots it declares, the
// template a reference names, the text a verbatim body has, and the
// callables whose bodies go through the codec.
const (
	checksSlot   = "checks"
	cleanupSlot  = "cleanup"
	methodRef    = "method1"
	verbatimText = "return nil"
	methodName   = "Do"
	functionName = "Handler"
)

// delegate returns the one-statement scaffold the body tests
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
// The handles Declare returns and the one question Form returns
// are both contract.
func TestBody(t *testing.T) {
	t.Parallel()

	t.Run("Declare", func(t *testing.T) {
		t.Parallel()

		t.Run("adds slots in declaration order", func(t *testing.T) {
			t.Parallel()

			var b emit.Body
			b.Declare(checksSlot)
			b.Declare(cleanupSlot)
			assert.Length(t, b.Slots, 2, "both slots arrived")
			assert.Equal(t, b.Slots[0].Name, checksSlot, "the first declaration comes first")
			assert.Equal(t, b.Slots[1].Name, cleanupSlot, "the order is not name order")
		})

		t.Run("returns the existing slot for a declared name", func(t *testing.T) {
			t.Parallel()

			var b emit.Body
			first := b.Declare(checksSlot)
			first.Append(delegate())
			again := b.Declare(checksSlot)
			assert.Equal(t, again.Len(), 1, "the second declaration returns the one slot")
			assert.Length(t, b.Slots, 1, "the second declaration adds nothing")
		})

		t.Run("returns a handle that survives following declarations", func(t *testing.T) {
			t.Parallel()

			var b emit.Body
			held := b.Declare("first")
			for i := range 16 {
				b.Declare("slot-" + strconv.Itoa(i))
			}
			held.Append(delegate())
			got, declared := b.Slot("first")
			assert.True(t, declared, "the slot is still declared")
			assert.Equal(t, got.Len(), 1, "an early handle still points at its slot after the list grew")
		})

		standard := []struct {
			name string
			slot string
		}{
			{name: "panics on the prologue's name", slot: emit.SlotPrologue},
			{name: "panics on the epilogue's name", slot: emit.SlotEpilogue},
		}
		for _, tt := range standard {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var b emit.Body
				assert.Panics(t, func() { b.Declare(tt.slot) }, "the standard pair has the name")
			})
		}
	})

	t.Run("Slot", func(t *testing.T) {
		t.Parallel()

		t.Run("returns false for a name the owner never declared", func(t *testing.T) {
			t.Parallel()

			var b emit.Body
			b.Declare(checksSlot)
			_, declared := b.Slot("invented")
			assert.False(t, declared, "a contribution into an invented extension point fails here")
		})
	})

	t.Run("Form", func(t *testing.T) {
		t.Parallel()

		var touched emit.Body
		touched.Prologue.Append(delegate())
		forms := []struct {
			name string
			body emit.Body
			want emit.Form
		}{
			{name: "returns the default form for the zero body", want: emit.FormDefault},
			{
				name: "returns the default form for a body with slot content alone",
				body: touched, want: emit.FormDefault,
			},
			{
				name: "returns the scaffolding form for statements",
				body: emit.Body{Stmts: []emit.Stmt{delegate()}}, want: emit.FormStmts,
			},
			{
				name: "returns the template form for a reference",
				body: emit.Body{Ref: &emit.TemplateRef{Name: methodRef}}, want: emit.FormTemplate,
			},
			{
				name: "returns the verbatim form for text",
				body: emit.Body{Verbatim: verbatimText}, want: emit.FormVerbatim,
			},
		}
		for _, tt := range forms {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				form, err := tt.body.Form()
				assert.NoError(t, err, "one form is valid")
				assert.Equal(t, form, tt.want, "the form is the content's")
			})
		}

		conflicts := []struct {
			name  string
			body  emit.Body
			names []string
		}{
			{
				name:  "returns an error naming scaffolding beside verbatim text",
				body:  emit.Body{Stmts: []emit.Stmt{delegate()}, Verbatim: verbatimText},
				names: []string{"scaffolding", "verbatim"},
			},
			{
				name:  "returns an error naming scaffolding beside a reference",
				body:  emit.Body{Stmts: []emit.Stmt{delegate()}, Ref: &emit.TemplateRef{Name: methodRef}},
				names: []string{"scaffolding", "template"},
			},
			{
				name:  "returns an error naming a reference beside verbatim text",
				body:  emit.Body{Ref: &emit.TemplateRef{Name: methodRef}, Verbatim: verbatimText},
				names: []string{"template", "verbatim"},
			},
			{
				name: "returns an error naming all three forms at once",
				body: emit.Body{
					Stmts:    []emit.Stmt{delegate()},
					Ref:      &emit.TemplateRef{Name: methodRef},
					Verbatim: verbatimText,
				},
				names: []string{"scaffolding", "template", "verbatim"},
			},
		}
		for _, tt := range conflicts {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				form, err := tt.body.Form()
				assert.HasError(t, err, "content is exactly one form")
				assert.Equal(t, form, emit.FormDefault, "a body with two claims has none")
				assert.ContainsInOrder(t, err.Error(), tt.names,
					"the error names every form the body has, in field order")
			})
		}
	})

	t.Run("IsZero", func(t *testing.T) {
		t.Parallel()

		var touched emit.Body
		touched.Epilogue.Append(delegate())
		tests := []struct {
			name string
			body emit.Body
			want bool
		}{
			{name: "reports true for the zero body", want: true},
			{name: "reports false for a touched slot", body: touched},
			{name: "reports false for content", body: emit.Body{Verbatim: "x"}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.body.IsZero(), tt.want, "emptiness covers every field")
			})
		}
	})

	t.Run("EncodeJSON", func(t *testing.T) {
		t.Parallel()

		t.Run("omits an untouched body", func(t *testing.T) {
			t.Parallel()

			encoded, err := emit.EncodeJSON(&emit.Method{Name: methodName})
			assert.NoError(t, err, "the method encodes")
			assert.NotContains(t, string(encoded), `"body"`, "an empty body is not encoded")
		})

		t.Run("encodes a function's body whole", func(t *testing.T) {
			t.Parallel()

			f := &emit.Function{Name: functionName}
			f.Body.Verbatim = verbatimText
			encoded, err := emit.EncodeJSON(f)
			assert.NoError(t, err, "the bodied function encodes")
			decoded, err := emit.DecodeJSON(encoded)
			assert.NoError(t, err, "the encoding decodes")
			got, held := decoded.(*emit.Function)
			assert.True(t, held, "the decoded value is a function")
			assert.Equal(t, got.Body.Verbatim, verbatimText, "the body arrives whole")
		})

		t.Run("encodes a method's body whole", func(t *testing.T) {
			t.Parallel()

			m := &emit.Method{Name: methodName}
			m.Body.Stmts = []emit.Stmt{delegate()}
			encoded, err := emit.EncodeJSON(m)
			assert.NoError(t, err, "the bodied method encodes")
			decoded, err := emit.DecodeJSON(encoded)
			assert.NoError(t, err, "the encoding decodes")
			got, held := decoded.(*emit.Method)
			assert.True(t, held, "the decoded value is a method")
			assert.Length(t, got.Body.Stmts, 1, "the scaffolding arrives whole")
		})

		t.Run("round-trips a body with every part", func(t *testing.T) {
			t.Parallel()

			m := &emit.Method{Name: methodName}
			m.Body.Prologue.Append(delegate())
			m.Body.Declare(checksSlot).Append(emit.Stmt{
				Kind:    emit.StmtAssign,
				Names:   []string{"res", "err"},
				Value:   emit.Expr{Kind: emit.ExprName, Name: "impl.Do"},
				Declare: true,
			})
			m.Body.Ref = &emit.TemplateRef{Name: methodRef, Data: map[string]any{"n": "x"}}

			first, err := emit.EncodeJSON(m)
			assert.NoError(t, err, "the method encodes")
			decoded, err := emit.DecodeJSON(first)
			assert.NoError(t, err, "the encoding decodes")
			second, err := emit.EncodeJSON(decoded)
			assert.NoError(t, err, "the decoded method encodes again")
			assert.Equal(t, string(second), string(first), "the round trip returns the same bytes")
		})
	})
}

// BenchmarkBody measures the per-callable operations the render
// pass runs once per body, and the codec the conformance checks
// run per encoded declaration.
func BenchmarkBody(b *testing.B) {
	b.Run("the form question", func(b *testing.B) {
		b.ReportAllocs()
		var body emit.Body
		body.Stmts = []emit.Stmt{delegate()}
		for b.Loop() {
			form, err := body.Form()
			if err != nil || form != emit.FormStmts {
				b.Fatal("the scaffold body returns its form")
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
				b.Fatal("the scaffold body has content")
			}
		}
	})

	b.Run("encode a bodied method", func(b *testing.B) {
		b.ReportAllocs()
		m := &emit.Method{Name: methodName}
		m.Body.Prologue.Append(delegate())
		m.Body.Stmts = []emit.Stmt{delegate()}
		for b.Loop() {
			if _, err := emit.EncodeJSON(m); err != nil {
				b.Fatalf("EncodeJSON: unexpected error: %v", err)
			}
		}
	})
}
