// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package symbol_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// declaration is the least a model kind returns, and stands in for
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

		t.Run("returns a kind, a position and its documentation", func(t *testing.T) {
			t.Parallel()

			at := position.Pos{File: "svc/store.go", Line: 41, Col: 2}
			var subject symbol.Symbol = &declaration{
				kind: symbol.KindStruct,
				pos:  at,
				docs: []string{"Store is the persistence seam."},
			}
			assert.Equal(t, subject.Kind(), symbol.KindStruct,
				"Kind returns what the declaration is")
			assert.Equal(t, subject.Position(), at,
				"Position returns where it was written")
			assert.Length(t, subject.Docs(), 1,
				"Docs returns the documentation it carries")
		})

		t.Run("a synthesized declaration returns the zero position", func(t *testing.T) {
			t.Parallel()

			var subject symbol.Symbol = &declaration{kind: symbol.KindStruct}
			assert.True(t, subject.Position().IsZero(),
				"a synthesized declaration returns the zero position")
			assert.Nil(t, subject.Docs(),
				"and carries no documentation")
		})
	})

	t.Run("Membered", func(t *testing.T) {
		t.Parallel()

		t.Run("returns member lists a caller can walk", func(t *testing.T) {
			t.Parallel()

			var subject symbol.Membered = &declaration{kind: symbol.KindStruct}
			assert.Length(t, subject.MethodList(), 1,
				"MethodList returns the members the kind carries")
			assert.Nil(t, subject.FieldList(),
				"a member list the kind does not carry returns nil")
		})
	})

	t.Run("Typed", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nil when the source states no type", func(t *testing.T) {
			t.Parallel()

			var subject symbol.Typed = &declaration{kind: symbol.KindField}
			assert.Nil(t, subject.TypeRef(),
				"TypeRef returns nil when the source states no type")
		})
	})
}
