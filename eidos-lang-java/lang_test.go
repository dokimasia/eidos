// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package java_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/java"
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

		t.Run("spells java", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, java.Target, "java", "the target a plan resolves")
		})
	})

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		t.Run("spells java", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, java.Name, "java", "the identity findings report under")
		})
	})

	t.Run("Extension", func(t *testing.T) {
		t.Parallel()

		t.Run("spells .java", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, java.Extension, ".java", "the suffix of every file")
		})
	})

	t.Run("CodePrefix", func(t *testing.T) {
		t.Parallel()

		t.Run("spells JAVA", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, java.CodePrefix, "JAVA", "the prefix of every diagnostic code")
		})
	})

	t.Run("Syntax", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the line opener", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, java.Syntax().Line, []string{"//"}, "Java's one line opener")
		})

		t.Run("returns the plain block form before the Javadoc form", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, java.Syntax().Blocks, []plugin.CommentBlock{
				{Open: "/*", Close: "*/"},
				{Open: "/**", Close: "*/", Gutter: "*"},
			}, "the plain block, and the Javadoc block with its star gutter")
		})
	})
}

// Syntax allocates its two lists, so no caller shares another's. The
// ordinary run, which runs no benchmark, checks that ceiling here.
func TestLangAllocs(t *testing.T) {
	var got plugin.CommentSyntax
	assert.MaxAllocs(t, func() { got = java.Syntax() }, syntaxAllocs, "Syntax allocates its two lists")
	assert.Length(t, got.Blocks, 2, "Syntax returns both block forms")
}

// BenchmarkLang measures the comment syntax a composition reads once
// per frontend and backend.
func BenchmarkLang(b *testing.B) {
	b.Run("Syntax", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(syntaxAllocs)
		defer c.End()
		var got plugin.CommentSyntax
		for c.Loop() {
			got = java.Syntax()
		}
		assert.Length(b, got.Blocks, 2, "Syntax returns both block forms")
	})
}
