// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package textfmt_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/textfmt"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The comment frames serve every backend, so each shape and its
// emptiness are pinned once.
func TestComment(t *testing.T) {
	t.Parallel()

	t.Run("LineDocs/one marker per line behind the prefix", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, textfmt.LineDocs([]string{"A.", "B."}, "// ", "  "),
			"  // A.\n  // B.\n", "the line form")
		assert.Equal(t, textfmt.LineDocs(nil, "// "), "", "nothing for no lines")
	})

	t.Run("LineDocs/a line break inside a line starts a commented line", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, textfmt.LineDocs([]string{"A.\nB.\r\nC."}, "// "),
			"// A.\n// B.\n// C.\n", "every part is behind a marker")
	})

	t.Run("BlockDocs/opener, gutter, closer", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, textfmt.BlockDocs([]string{"A."}, "/**", " * ", " */"),
			"/**\n * A.\n */\n", "the block form")
		assert.Equal(t, textfmt.BlockDocs(nil, "/**", " * ", " */"), "",
			"nothing for no lines")
	})

	t.Run("BlockDocs/a closer or a line break inside a line keeps the comment open", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, textfmt.BlockDocs([]string{"ends */ here", "A.\nB."}, "/**", " * ", " */"),
			"/**\n * ends *\\/ here\n * A.\n * B.\n */\n",
			"the closer is escaped and every part is guttered")
	})

	t.Run("Inline/one line, delimiters escaped", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, textfmt.Inline(""), "", "nothing for no text")
		assert.Equal(t, textfmt.Inline("the id"), " /* the id */", "the trailing block form")
		assert.Equal(t, textfmt.Inline("a */ b /* c\nd"), ` /* a *\/ b /\* c d */`,
			"a closer, an opener and a line break are kept inside the comment")
	})

	t.Run("Marked/names and arguments in each language's frame", func(t *testing.T) {
		t.Parallel()

		as := symbol.Annotations{
			{Name: "Override"},
			{Name: "derive", Args: []string{"Debug", "Clone"}},
		}
		assert.Equal(t, textfmt.Marked(as, "@", ""),
			"@Override\n@derive(Debug, Clone)\n", "the at form")
		assert.Equal(t, textfmt.Marked(as, "#[", "]"),
			"#[Override]\n#[derive(Debug, Clone)]\n", "the attribute form")
	})
}
