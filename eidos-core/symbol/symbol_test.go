// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package symbol_test

import (
	"testing"

	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// declaration is the least a model kind answers, and stands in for
// one here so the interfaces are exercised without importing a
// model: the vocabulary sits below both.
type declaration struct {
	kind symbol.Kind
	pos  position.Pos
	docs []string
}

func (d *declaration) Kind() symbol.Kind        { return d.kind }
func (d *declaration) Position() position.Pos   { return d.pos }
func (d *declaration) Docs() []string           { return d.docs }
func (*declaration) FieldList() []symbol.Symbol { return nil }
func (*declaration) MethodList() []symbol.Symbol {
	return []symbol.Symbol{&declaration{kind: symbol.KindMethod}}
}
func (*declaration) EmbedList() []symbol.Symbol { return nil }
func (*declaration) TypeRef() symbol.Symbol     { return nil }

var (
	_ symbol.Symbol   = (*declaration)(nil)
	_ symbol.Membered = (*declaration)(nil)
	_ symbol.Typed    = (*declaration)(nil)
)

func TestSymbol(t *testing.T) {
	t.Parallel()

	t.Run("Symbol", func(t *testing.T) {
		t.Parallel()

		t.Run("answers a kind, a position and its documentation", func(t *testing.T) {
			t.Parallel()

			at := position.Pos{File: "svc/store.go", Line: 41, Col: 2}
			var subject symbol.Symbol = &declaration{
				kind: symbol.KindStruct,
				pos:  at,
				docs: []string{"Store is the persistence seam."},
			}
			if got := subject.Kind(); got != symbol.KindStruct {
				t.Fatalf("Kind() = %v, want KindStruct", got)
			}
			if got := subject.Position(); got != at {
				t.Fatalf("Position() = %v, want %v", got, at)
			}
			if got := subject.Docs(); len(got) != 1 {
				t.Fatalf("Docs() = %v, want one line", got)
			}
		})

		t.Run("a synthesized declaration answers the zero position", func(t *testing.T) {
			t.Parallel()

			var subject symbol.Symbol = &declaration{kind: symbol.KindStruct}
			if !subject.Position().IsZero() {
				t.Fatal("Position() is set, want the zero position")
			}
			if subject.Docs() != nil {
				t.Fatal("Docs() is set, want nil")
			}
		})
	})

	t.Run("Membered", func(t *testing.T) {
		t.Parallel()

		t.Run("answers member lists a caller can walk", func(t *testing.T) {
			t.Parallel()

			var subject symbol.Membered = &declaration{kind: symbol.KindStruct}
			if got := subject.MethodList(); len(got) != 1 {
				t.Fatalf("MethodList() = %v, want one member", got)
			}
			if got := subject.FieldList(); got != nil {
				t.Fatalf("FieldList() = %v, want nil for a kind carrying none", got)
			}
		})
	})

	t.Run("Typed", func(t *testing.T) {
		t.Parallel()

		t.Run("answers nil when the source states no type", func(t *testing.T) {
			t.Parallel()

			var subject symbol.Typed = &declaration{kind: symbol.KindField}
			if got := subject.TypeRef(); got != nil {
				t.Fatalf("TypeRef() = %v, want nil", got)
			}
		})
	})
}
