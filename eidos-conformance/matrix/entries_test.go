// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package matrix_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/conformance/matrix"
	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/lang/rust"
	typescript "go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

func TestEntries(t *testing.T) {
	t.Parallel()

	t.Run("Entries", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the satellites in the order Go, TypeScript, Java, Rust and protobuf", func(t *testing.T) {
			t.Parallel()

			var langs []symbol.Lang
			for _, e := range matrix.Entries() {
				langs = append(langs, e.Corpus.Frontend.Lang())
			}
			assert.Equal(t, langs, []symbol.Lang{golang.Lang, typescript.Lang, java.Lang, rust.Lang, protobuf.Lang},
				"the entries follow the order of the satellites")
		})
		t.Run("returns a backend of the language's target for each satellite but protobuf", func(t *testing.T) {
			t.Parallel()

			var targets []plugin.Target
			for _, e := range matrix.Entries() {
				var target plugin.Target
				if e.Backend != nil {
					target = e.Backend.Target()
				}
				targets = append(targets, target)
			}
			assert.Equal(t, targets, []plugin.Target{
				plugin.Target(golang.Lang), plugin.Target(typescript.Lang), plugin.Target(java.Lang),
				plugin.Target(rust.Lang), "",
			}, "the target of each backend has the spelling of its language, and protobuf does not have a backend")
		})
		t.Run("returns corpora without a tree", func(t *testing.T) {
			t.Parallel()

			for _, e := range matrix.Entries() {
				lang := string(e.Corpus.Frontend.Lang())
				expect.Nil(t, e.Corpus.Sources, "the corpus of "+lang+" does not have a tree")
			}
		})
	})
}
