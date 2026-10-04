// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package textfmt_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/textfmt"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The comment frames the cases write in: a line marker, a block's
// opener, gutter and closer, and an indentation prefix.
const (
	lineMarker = "// "
	docOpener  = "/**"
	docGutter  = " * "
	docCloser  = " */"
	indent     = "\t"
)

// commentAllocs is a comment written into one buffer sized to it.
const commentAllocs = 1

// The Go, Java, Rust and TypeScript backends write their comments
// through these four forms. Each form is pinned byte for byte, its
// empty input included.
func TestComment(t *testing.T) {
	t.Parallel()

	t.Run("LineDocs", func(t *testing.T) {
		t.Parallel()

		t.Run("writes one marker per line behind the prefix", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, textfmt.LineDocs([]string{"A.", "B."}, lineMarker, "  "),
				"  // A.\n  // B.\n", "the line form")
		})

		t.Run("writes a commented line for each part of a line with a line break", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, textfmt.LineDocs([]string{"A.\nB.\r\nC."}, lineMarker),
				"// A.\n// B.\n// C.\n", "every part is behind a marker")
		})

		t.Run("writes nothing for no lines", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, textfmt.LineDocs(nil, lineMarker), "", "no lines, no comment")
		})
	})

	t.Run("BlockDocs", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the block form", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, textfmt.BlockDocs([]string{"A."}, docOpener, docGutter, docCloser),
				"/**\n * A.\n */\n", "the block form")
		})

		t.Run("escapes a closer inside a line", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, textfmt.BlockDocs([]string{"ends */ here"}, docOpener, docGutter, docCloser),
				"/**\n * ends *\\/ here\n */\n", "the closer cannot end the comment early")
		})

		t.Run("writes a guttered line for each part of a line with a line break", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, textfmt.BlockDocs([]string{"A.\nB."}, docOpener, docGutter, docCloser),
				"/**\n * A.\n * B.\n */\n", "every part is guttered")
		})

		t.Run("writes nothing for no lines", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, textfmt.BlockDocs(nil, docOpener, docGutter, docCloser), "", "no lines, no comment")
		})
	})

	t.Run("Inline", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want string
		}{
			{name: "writes nothing for no text", give: "", want: ""},
			{name: "writes the trailing block form", give: "the id", want: " /* the id */"},
			{name: "escapes a closer", give: "a */ b", want: ` /* a *\/ b */`},
			{name: "escapes an opener", give: "a /* b", want: ` /* a /\* b */`},
			{name: "folds a line break onto one line", give: "c\nd", want: " /* c d */"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, textfmt.Inline(tt.give), tt.want, "the inline comment")
			})
		}
	})

	t.Run("Marked", func(t *testing.T) {
		t.Parallel()

		t.Run("writes each marker in the at form", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, textfmt.Marked(markers(), "@", ""),
				"@Override\n@derive(Debug, Clone)\n", "the at form")
		})

		t.Run("writes each marker in the attribute form", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, textfmt.Marked(markers(), "#[", "]"),
				"#[Override]\n#[derive(Debug, Clone)]\n", "the attribute form")
		})

		t.Run("writes each marker behind the prefix", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, textfmt.Marked(markers()[:1], "@", "", indent), "\t@Override\n",
				"the prefix opens the line")
		})
	})
}

// Each comment form allocates one buffer sized to the comment. The
// ordinary run, which runs no benchmark, checks that ceiling here.
func TestCommentAllocs(t *testing.T) {
	lines, as := docLines(), markers()
	var got string
	assert.MaxAllocs(t, func() { got = textfmt.LineDocs(lines, lineMarker, indent) }, commentAllocs,
		"LineDocs allocates the comment")
	assert.Contains(t, got, "\t// A.\n", "LineDocs writes the lines")
	assert.MaxAllocs(t, func() { got = textfmt.BlockDocs(lines, docOpener, docGutter, docCloser, indent) },
		commentAllocs, "BlockDocs allocates the comment")
	assert.Contains(t, got, "\t * A.\n", "BlockDocs writes the lines")
	assert.MaxAllocs(t, func() { got = textfmt.Inline("the id") }, commentAllocs, "Inline allocates the comment")
	assert.Equal(t, got, " /* the id */", "Inline writes the comment")
	assert.MaxAllocs(t, func() { got = textfmt.Marked(as, "#[", "]", indent) }, commentAllocs,
		"Marked allocates the markers")
	assert.Contains(t, got, "\t#[derive(Debug, Clone)]\n", "Marked writes the arguments")
}

// BenchmarkComment measures each comment form a backend writes once per
// documented declaration.
func BenchmarkComment(b *testing.B) {
	lines, as := docLines(), markers()
	forms := []struct {
		name  string
		write func() string
		want  string
	}{
		{
			name:  "LineDocs",
			write: func() string { return textfmt.LineDocs(lines, lineMarker, indent) },
			want:  "\t// A.\n",
		},
		{
			name:  "BlockDocs",
			write: func() string { return textfmt.BlockDocs(lines, docOpener, docGutter, docCloser, indent) },
			want:  "\t * A.\n",
		},
		{
			name:  "Inline",
			write: func() string { return textfmt.Inline("the id") },
			want:  "the id",
		},
		{
			name:  "Marked",
			write: func() string { return textfmt.Marked(as, "#[", "]", indent) },
			want:  "(Debug, Clone)",
		},
	}
	for _, tt := range forms {
		b.Run(tt.name, func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(commentAllocs)
			defer c.End()
			var got string
			for c.Loop() {
				got = tt.write()
			}
			assert.Contains(b, got, tt.want, "the comment is written")
		})
	}
}

// docLines returns three lines of documentation, the last with a line
// break inside it.
func docLines() []string {
	return []string{"Row is one record of the store.", "Its columns follow.", "A.\nB."}
}

// markers returns a marker without arguments and one with two.
func markers() symbol.Annotations {
	return symbol.Annotations{
		{Name: "Override"},
		{Name: "derive", Args: []string{"Debug", "Clone"}},
	}
}
