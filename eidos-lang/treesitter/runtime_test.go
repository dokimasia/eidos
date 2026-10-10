// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package treesitter_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/lang/treesitter/java"
)

// semicolon is punctuation every pinned grammar spells, which no named
// kind is.
const semicolon = ";"

// The layer resolves kinds through the runtime's own lookup. Every
// grammar's resolution is pinned against the binding's wrapper of that
// lookup.
func TestRuntime(t *testing.T) {
	t.Parallel()

	t.Run("publicKind", func(t *testing.T) {
		t.Parallel()

		t.Run("resolves every named kind of every grammar to the id the runtime reports", func(t *testing.T) {
			t.Parallel()

			for _, p := range allPinned() {
				for id := range uint16(p.language.NodeKindCount()) {
					name := p.language.NodeKindForId(id)
					// IdForNodeKind leaks its C copy of the name at the pinned
					// runtime, which a test process affords and the layer does
					// not.
					want := treesitter.Kind(p.language.IdForNodeKind(name, true))
					if want == 0 {
						continue // punctuation or a keyword, which no named kind spells
					}
					assert.Equal(t, p.grammar.Kind(name), want, "the runtime's own lookup of a named kind")
				}
			}
		})

		t.Run("resolves no kind for the name of punctuation", func(t *testing.T) {
			t.Parallel()

			got := assert.Panics(t, func() { java.Grammar.Kind(semicolon) }, "a kind no named node has")
			assert.Contains(t, got, semicolon, "the panic names the kind")
		})
	})
}
