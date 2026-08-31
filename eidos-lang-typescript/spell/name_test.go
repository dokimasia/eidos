// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/lang-typescript/spell"
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
			symbol.VisibilityPublic, "FetchRow")
		assert.NoError(t, err, "a method spells")
		assert.Equal(t, got, "fetchRow", "camel")

		got, err = spell.Name(symbol.KindStruct, symbol.KindField,
			symbol.VisibilityPrivate, "RowKey")
		assert.NoError(t, err, "a private field spells")
		assert.Equal(t, got, "rowKey", "camel, because the keyword carries the scope")
	})

	t.Run("constants scream and type parameters stand", func(t *testing.T) {
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
}
