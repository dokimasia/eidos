// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protobuf_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// syntaxAllocs is the comment syntax: its list of line openers and its
// list of block forms.
const syntaxAllocs = 2

// A composition addresses this satellite through the identity, the
// extension and the comment forms. Each is pinned, because a change
// breaks compositions outside this module.
func TestLang(t *testing.T) {
	t.Parallel()

	t.Run("Lang", func(t *testing.T) {
		t.Parallel()

		t.Run("spells protobuf", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, string(protobuf.Lang), "protobuf", "the language every identity names")
		})
	})

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		t.Run("spells protobuf", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, string(protobuf.Name), "protobuf", "the identity findings report under")
		})
	})

	t.Run("Extension", func(t *testing.T) {
		t.Parallel()

		t.Run("spells .proto", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, protobuf.Extension, ".proto", "the suffix every schema file ends in")
		})
	})

	t.Run("Version", func(t *testing.T) {
		t.Parallel()

		t.Run("spells a version", func(t *testing.T) {
			t.Parallel()

			assert.NotEqual(t, protobuf.Version, "", "the version every unit key folds")
		})
	})

	t.Run("Syntax", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the line comment", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, protobuf.Syntax().Line, []string{"//"}, "protobuf's line comment")
		})

		t.Run("returns the block comment with its star gutter", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, protobuf.Syntax().Blocks, []plugin.CommentBlock{{Open: "/*", Close: "*/", Gutter: "*"}},
				"the block form C writes, continuation lines opening with a star")
		})

		t.Run("reports directives", func(t *testing.T) {
			t.Parallel()

			assert.True(t, protobuf.Syntax().Directives, "the directive convention every language shares")
		})
	})
}

// Syntax allocates its two lists, so no caller shares another's. The
// ordinary run, which runs no benchmark, checks that ceiling here.
func TestLangAllocs(t *testing.T) {
	var got plugin.CommentSyntax
	assert.MaxAllocs(t, func() { got = protobuf.Syntax() }, syntaxAllocs, "Syntax allocates its two lists")
	assert.Length(t, got.Line, 1, "Syntax returns the line comment")
}

// BenchmarkLang measures the comment syntax a composition reads once
// per frontend.
func BenchmarkLang(b *testing.B) {
	b.Run("Syntax", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(syntaxAllocs)
		defer c.End()
		var got plugin.CommentSyntax
		for c.Loop() {
			got = protobuf.Syntax()
		}
		assert.Length(b, got.Blocks, 1, "Syntax returns the block comment")
	})
}
