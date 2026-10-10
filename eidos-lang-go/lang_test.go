// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// syntaxAllocs is the comment syntax: its list of line openers and its
// list of block forms.
const syntaxAllocs = 2

// A composition selects this satellite through the identity, the
// extension and the comment forms. Each is pinned, because a change
// breaks compositions outside this module.
func TestLang(t *testing.T) {
	t.Parallel()

	t.Run("Target", func(t *testing.T) {
		t.Parallel()

		t.Run("spells golang", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, golang.Target, "golang", "the target a plan resolves")
		})
	})

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		t.Run("spells golang", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, golang.Name, "golang", "the identity findings report under")
		})
	})

	t.Run("Extension", func(t *testing.T) {
		t.Parallel()

		t.Run("spells .go", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, golang.Extension, ".go", "the suffix of every file")
		})
	})

	t.Run("Syntax", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the line opener", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, golang.Syntax().Line, []string{"//"}, "Go's one line opener")
		})

		t.Run("returns the block form", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, golang.Syntax().Blocks, []plugin.CommentBlock{{Open: "/*", Close: "*/"}},
				"Go's one block form")
		})

		t.Run("reports directives", func(t *testing.T) {
			t.Parallel()

			assert.True(t, golang.Syntax().Directives, "go:build is Go's own directive convention")
		})
	})
}

// Syntax allocates its two lists, so no caller shares another's. The
// ordinary run, which runs no benchmark, checks that ceiling here.
func TestLangAllocs(t *testing.T) {
	var got plugin.CommentSyntax
	assert.MaxAllocs(t, func() { got = golang.Syntax() }, syntaxAllocs, "Syntax allocates its two lists")
	assert.Length(t, got.Line, 1, "Syntax returns the line opener")
}

// BenchmarkLang measures the comment syntax a composition reads once
// per frontend and backend.
func BenchmarkLang(b *testing.B) {
	b.Run("Syntax", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(syntaxAllocs)
		defer c.End()
		var got plugin.CommentSyntax
		for c.Loop() {
			got = golang.Syntax()
		}
		assert.Length(b, got.Blocks, 1, "Syntax returns the block form")
	})
}
