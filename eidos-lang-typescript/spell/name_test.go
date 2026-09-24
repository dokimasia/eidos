// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/typescript/spell"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The convention is what the settle spells every declared name
// through, so each mapping is pinned byte for byte.
func TestName(t *testing.T) {
	t.Parallel()

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		t.Run("types take Pascal and members take camel", func(t *testing.T) {
			t.Parallel()

			got, err := spell.Name(symbol.KindInvalid, symbol.KindStruct,
				symbol.VisibilityUnknown, "httpRow")
			assert.NoError(t, err, "a class name spells")
			assert.Equal(t, got, "HTTPRow", "Pascal, the initialism kept whole")

			got, err = spell.Name(symbol.KindStruct, symbol.KindMethod,
				symbol.VisibilityPublic, "FetchRow")
			assert.NoError(t, err, "a method spells")
			assert.Equal(t, got, "fetchRow", "camel")

			got, err = spell.Name(symbol.KindStruct, symbol.KindField,
				symbol.VisibilityPrivate, "RowKey")
			assert.NoError(t, err, "a private field spells")
			assert.Equal(t, got, "rowKey", "camel, because the keyword states the scope")
		})

		t.Run("constants take screaming snake and type parameters keep theirs", func(t *testing.T) {
			t.Parallel()

			got, err := spell.Name(symbol.KindInvalid, symbol.KindConstant,
				symbol.VisibilityUnknown, "maxRows")
			assert.NoError(t, err, "a constant spells")
			assert.Equal(t, got, "MAX_ROWS", "screaming snake")

			got, err = spell.Name(symbol.KindStruct, symbol.KindTypeParam,
				symbol.VisibilityUnknown, "T")
			assert.NoError(t, err, "a type parameter spells")
			assert.Equal(t, got, "T", "as itself")

			got, err = spell.Name(symbol.KindInvalid, symbol.KindEnumVariant,
				symbol.VisibilityUnknown, "rowOpen")
			assert.NoError(t, err, "an enum member spells")
			assert.Equal(t, got, "RowOpen", "Pascal")
		})

		t.Run("passes a non-identifier to the backend where it quotes or refuses it", func(t *testing.T) {
			t.Parallel()

			for _, kind := range []symbol.Kind{
				symbol.KindField, symbol.KindMethod, symbol.KindEnumVariant, symbol.KindParam,
			} {
				got, err := spell.Name(symbol.KindStruct, kind, symbol.VisibilityUnknown, "content-type")
				assert.NoError(t, err, "a non-identifier passes on a "+kind.String())
				assert.Equal(t, got, "content-type", "unchanged")
			}
		})

		t.Run("refuses a non-identifier where no quoted form exists", func(t *testing.T) {
			t.Parallel()

			for _, kind := range []symbol.Kind{
				symbol.KindStruct, symbol.KindInterface, symbol.KindFunction,
				symbol.KindAlias, symbol.KindConstant, symbol.KindVariable,
			} {
				_, err := spell.Name(symbol.KindInvalid, kind, symbol.VisibilityUnknown, "content-type")
				assert.HasError(t, err, "a "+kind.String()+" name has no quoted form")
			}
		})

		t.Run("refuses a reserved word on every kind", func(t *testing.T) {
			t.Parallel()

			_, err := spell.Name(symbol.KindInvalid, symbol.KindFunction,
				symbol.VisibilityUnknown, "delete")
			assert.HasError(t, err, "a function named delete does not parse")
			_, err = spell.Name(symbol.KindInvalid, symbol.KindVariable,
				symbol.VisibilityUnknown, "yield")
			assert.HasError(t, err, "and strict mode reserves yield")
		})

		t.Run("refuses a binding-only reserved word on a binding alone", func(t *testing.T) {
			t.Parallel()

			for _, word := range []string{"await", "eval", "arguments"} {
				_, err := spell.Name(symbol.KindInvalid, symbol.KindVariable,
					symbol.VisibilityUnknown, word)
				assert.HasError(t, err, "a module binds no "+word)

				got, err := spell.Name(symbol.KindStruct, symbol.KindField,
					symbol.VisibilityUnknown, word)
				assert.NoError(t, err, "a member key takes "+word)
				assert.Equal(t, got, word, "unchanged")
			}
		})

		t.Run("refuses a predefined type name on a type declaration", func(t *testing.T) {
			t.Parallel()

			_, err := spell.Name(symbol.KindStruct, symbol.KindTypeParam,
				symbol.VisibilityUnknown, "string")
			assert.HasError(t, err, "a type parameter named string is refused")

			got, err := spell.Name(symbol.KindStruct, symbol.KindField,
				symbol.VisibilityUnknown, "string")
			assert.NoError(t, err, "a field named string spells")
			assert.Equal(t, got, "string", "because a property name is no type name")
		})
	})

	t.Run("IsIdentifier", func(t *testing.T) {
		t.Parallel()

		for _, name := range []string{"a", "_x", "$", "row1", "Row_$2"} {
			assert.True(t, spell.IsIdentifier(name), name+" spells bare")
		}
		for _, name := range []string{"", "1row", "content-type", "é", "a b"} {
			assert.False(t, spell.IsIdentifier(name), "\""+name+"\" does not spell bare")
		}
	})
}
