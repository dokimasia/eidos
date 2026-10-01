// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/java/frontend"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// everyType declares one of each type declaration kind the lowering
// walks by id, so a vocabulary entry that names the wrong kind lowers
// less.
const everyType = "public class C {}\npublic interface I {}\npublic enum E {\n    V\n}\n" +
	"public record R() {}\npublic @interface N {}\n"

// The vocabulary resolves the grammar's kinds once, so construction and
// the lowering of every type declaration kind through it are pinned.
func TestVocabulary(t *testing.T) {
	t.Parallel()

	t.Run("newVocabulary", func(t *testing.T) {
		t.Parallel()

		t.Run("resolves every kind and field the lowering reads from the Java grammar", func(t *testing.T) {
			t.Parallel()

			assert.NotPanics(t, func() { frontend.New(nil) }, "the grammar declares every name")
		})

		t.Run("lowers every type declaration kind to its declaration kind", func(t *testing.T) {
			t.Parallel()

			var kinds []symbol.Kind
			for _, d := range declsOf(t, everyType) {
				kinds = append(kinds, d.Kind())
			}
			assert.Equal(t, kinds, []symbol.Kind{
				symbol.KindStruct, symbol.KindInterface, symbol.KindEnum, symbol.KindStruct, symbol.KindInterface,
			}, "one declaration per type, in source order")
		})
	})
}
