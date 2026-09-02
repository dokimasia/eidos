// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/rust/spell"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The convention is what the settle spells every declared name
// through, so each mapping is pinned byte for byte.
func TestName(t *testing.T) {
	t.Parallel()

	t.Run("types take Pascal and callables take snake", func(t *testing.T) {
		t.Parallel()

		got, err := spell.Name(symbol.KindInvalid, symbol.KindStruct,
			symbol.VisibilityUnknown, "httpRow")
		assert.NoError(t, err, "a struct name spells")
		assert.Equal(t, got, "HTTPRow", "Pascal, the initialism kept whole")

		got, err = spell.Name(symbol.KindStruct, symbol.KindMethod,
			symbol.VisibilityInternal, "FetchRow")
		assert.NoError(t, err, "a method spells whatever its scope")
		assert.Equal(t, got, "fetch_row", "snake, because pub carries the scope")

		got, err = spell.Name(symbol.KindFunction, symbol.KindParam,
			symbol.VisibilityUnknown, "rowCount")
		assert.NoError(t, err, "a parameter spells")
		assert.Equal(t, got, "row_count", "snake too")
	})

	t.Run("escapes keywords raw and refuses what raw cannot carry", func(t *testing.T) {
		t.Parallel()

		got, err := spell.Name(symbol.KindStruct, symbol.KindField,
			symbol.VisibilityPublic, "type")
		assert.NoError(t, err, "a keyword field takes the raw form")
		assert.Equal(t, got, "r#type", "spelled r#type")

		_, err = spell.Name(symbol.KindStruct, symbol.KindField,
			symbol.VisibilityPublic, "self")
		assert.HasError(t, err, "rustc rejects r#self, so the spelling refuses")

		_, err = spell.Name(symbol.KindStruct, symbol.KindField,
			symbol.VisibilityPublic, "content-type")
		assert.HasError(t, err, "a convention must not respell a wire name")
	})

	t.Run("constants scream and type parameters stand", func(t *testing.T) {
		t.Parallel()

		got, err := spell.Name(symbol.KindInvalid, symbol.KindConstant,
			symbol.VisibilityUnknown, "maxRows")
		assert.NoError(t, err, "a constant spells")
		assert.Equal(t, got, "MAX_ROWS", "screaming snake")

		got, err = spell.Name(symbol.KindInvalid, symbol.KindSumVariant,
			symbol.VisibilityUnknown, "rowOpen")
		assert.NoError(t, err, "a variant spells")
		assert.Equal(t, got, "RowOpen", "Pascal")

		got, err = spell.Name(symbol.KindStruct, symbol.KindTypeParam,
			symbol.VisibilityUnknown, "N")
		assert.NoError(t, err, "a type parameter spells")
		assert.Equal(t, got, "N", "as itself, so a const parameter stands")
	})
}
