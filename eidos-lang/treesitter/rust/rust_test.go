// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rust_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/treesitter/rust"
)

// The source the grammar parses, and the name it loads under.
const (
	rustFile   = "src/lib.rs"
	rustSource = "/// A row.\npub struct Row {\n    id: u64,\n}\n\nimpl Row {\n    pub fn id(&self) -> u64 { self.id }\n}\n"
	rustName   = "rust"
)

// The package loads one grammar, so the grammar is pinned as the one
// that parses Rust.
func TestRust(t *testing.T) {
	t.Parallel()

	t.Run("Grammar", func(t *testing.T) {
		t.Parallel()

		t.Run("parses a struct and its impl block without an error", func(t *testing.T) {
			t.Parallel()

			tree, err := rust.Grammar.Parse(t.Context(), rustFile, []byte(rustSource))
			assert.NoError(t, err, "the source parses")
			defer tree.Close()
			assert.Empty(t, slices.Collect(tree.Errors()), "the source fits the grammar")
		})

		t.Run("loads under the language's name", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rust.Grammar.Name(), rustName, "the name a frontend reports")
		})
	})
}
