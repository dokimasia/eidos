// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/plugin"
)

// One CommentSyntax value per satellite serves both kits, so its
// shape has to cover the landscape: line forms with a canonical
// first, and block forms with their gutters.
func TestCommentSyntax(t *testing.T) {
	t.Parallel()

	golike := plugin.CommentSyntax{
		Line: []string{"//"},
		Blocks: []plugin.CommentBlock{
			{Open: "/*", Close: "*/"},
			{Open: "/**", Close: "*/", Gutter: "*"},
		},
	}
	assert.Equal(t, golike.Line[0], "//", "the first line form is canonical")
	assert.Length(t, golike.Blocks, 2,
		"doc blocks with gutters sit beside the plain form")
}
