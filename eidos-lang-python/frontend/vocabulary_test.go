// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/python/frontend"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// everyKind declares one of each kind of declaration that a module states,
// which the lowering tells apart by the kinds of the vocabulary: a struct,
// an interface, an enum, a function, a variable, a constant and an alias.
const everyKind = "from enum import Enum\nfrom typing import Final, Protocol\n\n\nclass S: ...\n\n\n" +
	"class I(Protocol): ...\n\n\nclass E(Enum):\n    A = 1\n\n\ndef f() -> None: ...\n\n\nv = 1\nc: Final = 2\n" +
	"type T = int\n"

// TestVocabulary pins the vocabulary of the grammar, which the frontend
// resolves once when it is built.
func TestVocabulary(t *testing.T) {
	t.Parallel()

	t.Run("newVocabulary", func(t *testing.T) {
		t.Parallel()

		t.Run("resolves every kind and field that the lowering reads from the Python grammar", func(t *testing.T) {
			t.Parallel()

			assert.NotPanics(t, func() { frontend.New(nil) }, "the grammar declares every name")
		})

		t.Run("lowers every kind of declaration to its kind", func(t *testing.T) {
			t.Parallel()

			decls := declsOf(t, everyKind)
			kinds := make([]symbol.Kind, 0, len(decls))
			for _, d := range decls {
				kinds = append(kinds, d.Kind())
			}
			assert.Equal(t, kinds, []symbol.Kind{
				symbol.KindStruct, symbol.KindInterface, symbol.KindEnum, symbol.KindFunction, symbol.KindVariable,
				symbol.KindConstant, symbol.KindAlias,
			}, "one declaration of each kind, in source order")
		})
	})
}
