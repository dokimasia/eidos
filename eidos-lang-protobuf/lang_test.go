// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protobuf_test

import (
	"testing"

	"go.dokimi.dev/assert"

	protobuf "go.dokimi.dev/eidos/lang/protobuf"
)

// The identity, the extension and the comment forms are what the
// rest of the system spells to address this satellite, so each is
// pinned: a drift here breaks compositions, not this module.
func TestLang(t *testing.T) {
	t.Parallel()

	t.Run("pins the boundary spellings", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, string(protobuf.Lang), "protobuf", "the language every identity names")
		assert.Equal(t, string(protobuf.Name), "protobuf", "the identity findings report under")
		assert.Equal(t, protobuf.Extension, ".proto", "the suffix every schema file ends in")
		assert.NotEqual(t, protobuf.Version, "", "and a version every unit key folds")
	})

	t.Run("declares the comment forms whole", func(t *testing.T) {
		t.Parallel()

		s := protobuf.Syntax()
		assert.Equal(t, s.Line, []string{"//"}, "protobuf's line comment")
		assert.Length(t, s.Blocks, 1, "and its block comment")
		assert.Equal(t, s.Blocks[0].Open, "/*", "opening as C does")
		assert.Equal(t, s.Blocks[0].Close, "*/", "and closing the same way")
		assert.Equal(t, s.Blocks[0].Gutter, "*", "with the star a continuation line opens with")
		assert.True(t, s.Directives, "with the directive convention every language shares")
	})
}
