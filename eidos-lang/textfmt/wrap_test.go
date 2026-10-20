// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package textfmt_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/textfmt"
)

// The sentence of a builder's documentation that the backends wrap, and
// the four lines that a wrap at the width of the documentation returns.
const builderSentence = "ConfigurationRegistryBuilder constructs [ConfigurationRegistry] values. " +
	"Start with [NewConfigurationRegistryBuilder] or [NewConfigurationRegistryBuilderFrom], " +
	"set what you care about, and call [ConfigurationRegistryBuilder.Build]."

// builderLines is the wrap of builderSentence at [textfmt.DocsWidth].
var builderLines = []string{
	"ConfigurationRegistryBuilder constructs [ConfigurationRegistry] values.",
	"Start with [NewConfigurationRegistryBuilder] or",
	"[NewConfigurationRegistryBuilderFrom], set what you care about, and call",
	"[ConfigurationRegistryBuilder.Build].",
}

// The allocations of a wrap.
const (
	// singleSpacedAllocs is a paragraph whose words one space separates:
	// the list of lines, because each line is a part of the text.
	singleSpacedAllocs = 1
	// spacedLineAllocs is each line whose words other spaces separate,
	// which the wrap joins into a line of its own.
	spacedLineAllocs = 1
	// joinedLines is the number of lines of builderLines with two or more
	// words, which the wrap joins where two spaces separate the words of
	// the text.
	joinedLines = 3
)

// The documentation of every backend is wrapped here, so the rules of the
// wrap and of each markup are pinned once.
func TestWrap(t *testing.T) {
	t.Parallel()

	t.Run("Wrap", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			give      string
			giveWidth int
			want      []string
		}{
			{name: "returns nil for empty text", give: "", giveWidth: 10},
			{name: "returns nil for text of spaces and line breaks", give: " \n\t\n ", giveWidth: 10},
			{
				name: "keeps a short paragraph on one line",
				give: "a short line", giveWidth: 20,
				want: []string{"a short line"},
			},
			{
				name: "wraps a paragraph before the word that passes the width",
				give: "one two three four five", giveWidth: 9,
				want: []string{"one two", "three", "four five"},
			},
			{
				name: "fills a line to the width",
				give: "abcd efgh ij", giveWidth: 9,
				want: []string{"abcd efgh", "ij"},
			},
			{
				name: "writes a word longer than the width on a line of its own",
				give: "a [VeryLongReference.Name] b", giveWidth: 8,
				want: []string{"a", "[VeryLongReference.Name]", "b"},
			},
			{
				name: "writes a run of spaces, tabs and line breaks as one space",
				give: "one  two\tthree\nfour", giveWidth: 40,
				want: []string{"one two three four"},
			},
			{
				name: "keeps a no-break space inside its word",
				give: "a\U000000a0b  c", giveWidth: 40,
				want: []string{"a\U000000a0b c"},
			},
			{
				name: "separates two paragraphs with an empty line",
				give: "first paragraph\n\nsecond paragraph", giveWidth: 40,
				want: []string{"first paragraph", "", "second paragraph"},
			},
			{
				name: "separates two paragraphs with one empty line for two blank lines",
				give: "first\n\n \n\nsecond", giveWidth: 40,
				want: []string{"first", "", "second"},
			},
			{
				name: "drops the blank lines around the paragraphs",
				give: "\n\nonly\n\n", giveWidth: 40,
				want: []string{"only"},
			},
			{
				name: "counts the width in runes",
				give: "ééé èèè ëëë", giveWidth: 7,
				want: []string{"ééé èèè", "ëëë"},
			},
			{
				name: "wraps nothing at a width below one",
				give: "one two three", giveWidth: 0,
				want: []string{"one two three"},
			},
			{
				name: "writes the sentence of a builder in four lines at the width of the documentation",
				give: builderSentence, giveWidth: textfmt.DocsWidth,
				want: builderLines,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, textfmt.Wrap(tt.give, tt.giveWidth, textfmt.MarkupNone), tt.want,
					"the wrap returns the lines of the text")
			})
		}

		t.Run("starts a line with a word that starts with @ without markup", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, textfmt.Wrap("see the @param tag", 8, textfmt.MarkupNone),
				[]string{"see the", "@param", "tag"}, "a Go comment reads @ as text")
		})

		t.Run("keeps a block tag on the line before it under MarkupTags", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, textfmt.Wrap("see the @param tag", 8, textfmt.MarkupTags),
				[]string{"see the @param", "tag"}, "a block tag at the start of a line would begin a tag")
		})

		t.Run("keeps a word that starts with @ at the start of a paragraph", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, textfmt.Wrap("@deprecated use the builder", 40, textfmt.MarkupTags),
				[]string{"@deprecated use the builder"}, "the author's own tag opens the paragraph")
		})

		openers := []struct {
			name string
			give string
		}{
			{name: "keeps an ATX heading marker on the line before it", give: "#"},
			{name: "keeps an ATX heading marker of six signs on the line before it", give: "######"},
			{name: "keeps a block quote on the line before it", give: ">quoted"},
			{name: "keeps a plus sign on the line before it", give: "+"},
			{name: "keeps an asterisk on the line before it", give: "*"},
			{name: "keeps a dash on the line before it", give: "-"},
			{name: "keeps an ordinal with a period on the line before it", give: "1."},
			{name: "keeps an ordinal of nine digits with a parenthesis on the line before it", give: "123456789)"},
			{name: "keeps a run of dashes on the line before it", give: "---"},
			{name: "keeps a run of equals signs on the line before it", give: "==="},
			{name: "keeps a run of three asterisks on the line before it", give: "***"},
			{name: "keeps a run of three underscores on the line before it", give: "___"},
			{name: "keeps a fence of backticks on the line before it", give: "```go"},
			{name: "keeps a fence of tildes on the line before it", give: "~~~"},
			{name: "keeps an HTML tag on the line before it", give: "<div>"},
		}
		for _, tt := range openers {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, textfmt.Wrap("see "+tt.give+" next", 3, textfmt.MarkupMarkdown),
					[]string{"see " + tt.give, "next"}, "the word would open a block at the start of a line")
			})
		}

		texts := []struct {
			name string
			give string
		}{
			{name: "starts a line with a heading marker of seven signs", give: "#######"},
			{name: "starts a line with a word in bold", give: "**bold**"},
			{name: "starts a line with a decimal number", give: "1.5"},
			{name: "starts a line with an ordinal of ten digits", give: "1234567890."},
			{name: "starts a line with a flag", give: "-v"},
			{name: "starts a line with a run of two asterisks", give: "**"},
		}
		for _, tt := range texts {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, textfmt.Wrap("see "+tt.give, 3, textfmt.MarkupMarkdown),
					[]string{"see", tt.give}, "CommonMark reads the word as text")
			})
		}

		t.Run("keeps a block tag on the line before it under both markups", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, textfmt.Wrap("see @param - x", 3, textfmt.MarkupTags|textfmt.MarkupMarkdown),
				[]string{"see @param -", "x"}, "TSDoc reads tags and Markdown")
		})
	})
}

// A paragraph whose words one space separates allocates the list of its
// lines, and a line whose words other spaces separate allocates the line.
// The ordinary run, which runs no benchmark, checks those ceilings here.
func TestWrapAllocs(t *testing.T) {
	spaced := strings.ReplaceAll(builderSentence, " ", "  ")
	var lines []string
	assert.MaxAllocs(t, func() { lines = textfmt.Wrap(builderSentence, textfmt.DocsWidth, textfmt.MarkupNone) },
		singleSpacedAllocs, "Wrap allocates the list of lines of a single-spaced paragraph")
	assert.Equal(t, lines, builderLines, "Wrap wraps the sentence of the builder")
	assert.MaxAllocs(t, func() { lines = textfmt.Wrap(spaced, textfmt.DocsWidth, textfmt.MarkupNone) },
		singleSpacedAllocs+joinedLines*spacedLineAllocs, "Wrap allocates each line that it joins")
	assert.Equal(t, lines, builderLines, "Wrap writes one space between two words")
}

// BenchmarkWrap measures the wrap of the sentence of a builder, single
// spaced and double spaced.
func BenchmarkWrap(b *testing.B) {
	b.Run("Wrap", func(b *testing.B) {
		inputs := []struct {
			name   string
			give   string
			allocs uint64
		}{
			{name: "a single-spaced paragraph", give: builderSentence, allocs: singleSpacedAllocs},
			{
				name: "a double-spaced paragraph", give: strings.ReplaceAll(builderSentence, " ", "  "),
				allocs: singleSpacedAllocs + joinedLines*spacedLineAllocs,
			},
		}
		for _, tt := range inputs {
			b.Run(tt.name, func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var lines []string
				for c.Loop() {
					lines = textfmt.Wrap(tt.give, textfmt.DocsWidth, textfmt.MarkupNone)
				}
				assert.Equal(b, lines, builderLines, "Wrap wraps the sentence of the builder")
			})
		}
	})
}
