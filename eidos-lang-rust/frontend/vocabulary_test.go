// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/rust/frontend"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// everyItem declares one of each item kind the lowering walks by id, so
// a vocabulary entry that names the wrong kind lowers less.
const everyItem = "pub struct A;\npub union U {\n    a: u8,\n}\npub enum E {\n    V,\n}\n" +
	"pub enum S {\n    V(u8),\n}\npub trait T {}\npub fn f() {}\npub const C: u8 = 1;\n" +
	"pub static V: u8 = 1;\npub type Y = u8;\n"

// The vocabulary resolves the grammar's kinds once, so construction and
// the lowering of every item kind through it are pinned.
func TestVocabulary(t *testing.T) {
	t.Parallel()

	t.Run("newVocabulary", func(t *testing.T) {
		t.Parallel()

		t.Run("resolves every kind and field the lowering reads from the Rust grammar", func(t *testing.T) {
			t.Parallel()

			assert.NotPanics(t, func() { frontend.New(nil) }, "the grammar declares every name")
		})

		t.Run("lowers every item kind to its declaration kind", func(t *testing.T) {
			t.Parallel()

			decls := declsOf(t, everyItem)
			kinds := make([]symbol.Kind, 0, len(decls))
			for _, d := range decls {
				kinds = append(kinds, d.Kind())
			}
			assert.Equal(t, kinds, []symbol.Kind{
				symbol.KindStruct, symbol.KindStruct, symbol.KindEnum, symbol.KindSum, symbol.KindInterface,
				symbol.KindFunction, symbol.KindConstant, symbol.KindVariable, symbol.KindAlias,
			}, "one declaration per item, in source order")
		})
	})
}
