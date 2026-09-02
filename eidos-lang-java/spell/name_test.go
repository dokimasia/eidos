// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/java/spell"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The convention is what the settle spells every declared name
// through, so each mapping is pinned byte for byte.
func TestName(t *testing.T) {
	t.Parallel()

	t.Run("types take Pascal and members take camel", func(t *testing.T) {
		t.Parallel()

		got, err := spell.Name(symbol.KindInvalid, symbol.KindStruct,
			symbol.VisibilityUnknown, "httpRow")
		assert.NoError(t, err, "a class name spells")
		assert.Equal(t, got, "HTTPRow", "Pascal, the initialism kept whole")

		got, err = spell.Name(symbol.KindStruct, symbol.KindMethod,
			symbol.VisibilityProtected, "FetchRow")
		assert.NoError(t, err, "a method spells whatever its scope")
		assert.Equal(t, got, "fetchRow", "camel, because the keyword carries the scope")

		got, err = spell.Name(symbol.KindStruct, symbol.KindField,
			symbol.VisibilityPublic, "rowKey")
		assert.NoError(t, err, "a class field spells")
		assert.Equal(t, got, "rowKey", "camel")
	})

	t.Run("refuses a wire name and a reserved landing", func(t *testing.T) {
		t.Parallel()

		_, err := spell.Name(symbol.KindStruct, symbol.KindField,
			symbol.VisibilityPublic, "content-type")
		assert.HasError(t, err, "a convention must not respell a wire name")
		assert.Contains(t, err.Error(), "content-type", "naming it")

		_, err = spell.Name(symbol.KindStruct, symbol.KindField,
			symbol.VisibilityPublic, "class")
		assert.HasError(t, err, "Java grants no escape for a reserved word")
		assert.Contains(t, err.Error(), "reserved", "saying why")
	})

	t.Run("the host turns an interface field into a constant", func(t *testing.T) {
		t.Parallel()

		got, err := spell.Name(symbol.KindInterface, symbol.KindField,
			symbol.VisibilityUnknown, "maxRows")
		assert.NoError(t, err, "an interface field spells")
		assert.Equal(t, got, "MAX_ROWS",
			"screaming snake, because Java reads it as a constant")

		got, err = spell.Name(symbol.KindInvalid, symbol.KindEnumVariant,
			symbol.VisibilityUnknown, "rowOpen")
		assert.NoError(t, err, "an enum constant spells")
		assert.Equal(t, got, "ROW_OPEN", "screaming snake too")

		got, err = spell.Name(symbol.KindStruct, symbol.KindTypeParam,
			symbol.VisibilityUnknown, "T")
		assert.NoError(t, err, "a type parameter spells")
		assert.Equal(t, got, "T", "as itself")
	})
}
