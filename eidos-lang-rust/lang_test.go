// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rust_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// syntaxAllocs is the comment syntax: its list of line openers and its
// list of block forms.
const syntaxAllocs = 2

// A composition addresses this satellite by its identity, its extension
// and its comment forms. Each is pinned, because a change breaks
// compositions outside this module.
func TestLang(t *testing.T) {
	t.Parallel()

	t.Run("Target", func(t *testing.T) {
		t.Parallel()

		t.Run("spells rust", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rust.Target, "rust", "the target a plan resolves")
		})
	})

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		t.Run("spells rust", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rust.Name, "rust", "the identity findings report under")
		})
	})

	t.Run("Extension", func(t *testing.T) {
		t.Parallel()

		t.Run("spells .rs", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rust.Extension, ".rs", "the suffix of every file")
		})
	})

	t.Run("CodePrefix", func(t *testing.T) {
		t.Parallel()

		t.Run("spells RUST", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rust.CodePrefix, "RUST", "the prefix of every diagnostic code")
		})
	})

	t.Run("Syntax", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the three line forms with the plain one first", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rust.Syntax().Line, []string{"//", "///", "//!"},
				"the plain form is canonical, then the outer and the inner doc line")
		})

		t.Run("returns the plain block form before the two doc block forms", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, rust.Syntax().Blocks, []plugin.CommentBlock{
				{Open: "/*", Close: "*/"},
				{Open: "/**", Close: "*/", Gutter: "*"},
				{Open: "/*!", Close: "*/", Gutter: "*"},
			}, "the plain block, then the outer and the inner doc block with their star gutter")
		})
	})
}

// Syntax allocates its two lists, so no caller shares another's. The
// ordinary run, which runs no benchmark, checks that ceiling here.
func TestLangAllocs(t *testing.T) {
	var got plugin.CommentSyntax
	assert.MaxAllocs(t, func() { got = rust.Syntax() }, syntaxAllocs, "Syntax allocates its two lists")
	assert.Length(t, got.Line, 3, "Syntax returns the three line forms")
}

// BenchmarkLang measures the comment syntax a composition reads once
// per frontend and backend.
func BenchmarkLang(b *testing.B) {
	b.Run("Syntax", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(syntaxAllocs)
		defer c.End()
		var got plugin.CommentSyntax
		for c.Loop() {
			got = rust.Syntax()
		}
		assert.Length(b, got.Blocks, 3, "Syntax returns the three block forms")
	})
}
