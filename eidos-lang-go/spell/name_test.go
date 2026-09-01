// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/go/spell"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The convention is what the settle spells every declared name
// through, so each mapping is pinned byte for byte.
func TestName(t *testing.T) {
	t.Parallel()

	t.Run("visibility becomes case", func(t *testing.T) {
		t.Parallel()

		got, err := spell.Name(symbol.KindInvalid, symbol.KindStruct,
			symbol.VisibilityPublic, "httpRow")
		assert.NoError(t, err, "a public name spells")
		assert.Equal(t, got, "HTTPRow",
			"exported through Pascal, the initialism kept whole")

		got, err = spell.Name(symbol.KindInvalid, symbol.KindFunction,
			symbol.VisibilityUnknown, "fetchRow")
		assert.NoError(t, err, "an unstated scope spells")
		assert.Equal(t, got, "FetchRow", "as exported, because a generated API is consumed")

		got, err = spell.Name(symbol.KindInvalid, symbol.KindConstant,
			symbol.VisibilityPackage, "MaxRows")
		assert.NoError(t, err, "a package scope spells")
		assert.Equal(t, got, "maxRows", "unexported through camel")

		_, err = spell.Name(symbol.KindInvalid, symbol.KindStruct,
			symbol.VisibilityProtected, "row")
		assert.HasError(t, err, "a scope no case carries refuses")
	})

	t.Run("parameters and type parameters keep their own forms", func(t *testing.T) {
		t.Parallel()

		got, err := spell.Name(symbol.KindFunction, symbol.KindParam,
			symbol.VisibilityUnknown, "RowCount")
		assert.NoError(t, err, "a parameter spells")
		assert.Equal(t, got, "rowCount", "camel whatever the scope")

		got, err = spell.Name(symbol.KindFunction, symbol.KindReturn,
			symbol.VisibilityUnknown, "errOut")
		assert.NoError(t, err, "a named result spells")
		assert.Equal(t, got, "errOut", "camel too")

		got, err = spell.Name(symbol.KindStruct, symbol.KindTypeParam,
			symbol.VisibilityUnknown, "T")
		assert.NoError(t, err, "a type parameter spells")
		assert.Equal(t, got, "T", "as itself")
	})

	t.Run("members fold their own visibility", func(t *testing.T) {
		t.Parallel()

		got, err := spell.Name(symbol.KindStruct, symbol.KindField,
			symbol.VisibilityPackage, "rowKey")
		assert.NoError(t, err, "a package-scoped field spells")
		assert.Equal(t, got, "rowKey", "unexported")

		got, err = spell.Name(symbol.KindStruct, symbol.KindMethod,
			symbol.VisibilityPublic, "fetch")
		assert.NoError(t, err, "a public method spells")
		assert.Equal(t, got, "Fetch", "exported")
	})
}
