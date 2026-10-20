// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/backend"
)

// The allocations of a docstring: the docstring, and the two escapes of a
// line with a backslash and a triple quote.
const (
	docstringAllocs        = 1
	escapedDocstringAllocs = 1 + 2
)

// The lines of the docstrings that the allocation test and the benchmark
// write: a text of one line, a text of three lines, and a text that needs
// escapes.
var (
	oneLineDoc  = []string{"Row is one record."}
	moreLineDoc = []string{"Row is one record.", "", "It has a key."}
	escapedDoc  = []string{`Writes """ and \.`}
)

// TestDocstring pins the docstrings that Docstring writes, each in the form
// that ruff's formatter leaves as it is.
func TestDocstring(t *testing.T) {
	t.Parallel()

	t.Run("Docstring", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			give       []string
			giveIndent string
			want       string
		}{
			{name: "returns nothing for no lines"},
			{name: "returns nothing for lines that are all blank", give: []string{"", "  "}},
			{
				name: "returns a text of one line between triple quotes on one line",
				give: oneLineDoc, want: "\"\"\"Row is one record.\"\"\"\n",
			},
			{
				name: "returns the docstring at the indentation",
				give: oneLineDoc, giveIndent: python.Indent, want: "    \"\"\"Row is one record.\"\"\"\n",
			},
			{
				name: "returns the closing quotes of a text of more lines on a line of their own",
				give: moreLineDoc, giveIndent: python.Indent,
				want: "    \"\"\"Row is one record.\n\n    It has a key.\n    \"\"\"\n",
			},
			{
				name: "returns the lines without the indentation that they share",
				give: []string{"Summary.", "    indented", "      deeper"},
				want: "\"\"\"Summary.\nindented\n  deeper\n\"\"\"\n",
			},
			{
				name: "returns the indentation of a tab as the columns to its next stop",
				give: []string{"Summary.", "\tdeeper", "    less"},
				want: "\"\"\"Summary.\n    deeper\nless\n\"\"\"\n",
			},
			{
				name: "returns the lines without their trailing whitespace",
				give: []string{"Summary.   ", "detail  "},
				want: "\"\"\"Summary.\ndetail\n\"\"\"\n",
			},
			{
				name: "returns the first line without its leading whitespace",
				give: []string{"  leading."}, want: "\"\"\"leading.\"\"\"\n",
			},
			{
				name: "returns the text without the blank lines at its end",
				give: []string{"Summary.", "", " "}, want: "\"\"\"Summary.\"\"\"\n",
			},
			{
				name: "returns a raw string for a text with a backslash",
				give: []string{`Matches \d.`}, want: "r\"\"\"Matches \\d.\"\"\"\n",
			},
			{
				name: "returns escapes for a text with a backslash and a triple quote",
				give: escapedDoc, want: "\"\"\"Writes \\\"\\\"\\\" and \\\\.\"\"\"\n",
			},
			{
				name: "returns escapes for a text that ends with a backslash",
				give: []string{`Ends with \`}, want: "\"\"\"Ends with \\\\\"\"\"\n",
			},
			{
				name: "returns a space before the closing quotes of a text of one line that ends with a quote",
				give: []string{`Say "hi"`}, want: "\"\"\"Say \"hi\" \"\"\"\n",
			},
			{
				name: "returns a space after the opening quotes of a text that starts with a quote",
				give: []string{`"Quoted" first.`}, want: "\"\"\" \"Quoted\" first.\"\"\"\n",
			},
			{
				name: "returns no space before closing quotes on a line of their own",
				give: []string{"Summary.", `Say "hi"`}, want: "\"\"\"Summary.\nSay \"hi\"\n\"\"\"\n",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, backend.Docstring(tt.give, tt.giveIndent), tt.want, "the docstring")
			})
		}
	})
}

// Docstring allocates the docstring, and the escapes of a line that needs
// them. The ordinary run, which runs no benchmark, checks those ceilings
// here.
func TestDocstringAllocs(t *testing.T) {
	checkAllocs(t, docstringCalls())
}

// BenchmarkDocstring measures the docstrings that a class, a callable and a
// field write.
func BenchmarkDocstring(b *testing.B) {
	benchCalls(b, docstringCalls())
}

// docstringCalls returns the calls of Docstring for a text of one line, for
// a text of three lines, and for a text that needs escapes.
func docstringCalls() []allocCall {
	var got string
	return []allocCall{
		{
			name: "Docstring", caseName: "one line", allocs: docstringAllocs,
			call: func() { got = backend.Docstring(oneLineDoc, python.Indent) },
			check: func(tb assert.TB) {
				assert.Equal(tb, got, "    \"\"\"Row is one record.\"\"\"\n", "Docstring writes the line")
			},
		},
		{
			name: "Docstring", caseName: "three lines", allocs: docstringAllocs,
			call: func() { got = backend.Docstring(moreLineDoc, python.Indent) },
			check: func(tb assert.TB) {
				assert.Equal(tb, got, "    \"\"\"Row is one record.\n\n    It has a key.\n    \"\"\"\n",
					"Docstring writes the lines")
			},
		},
		{
			name: "Docstring", caseName: "escapes", allocs: escapedDocstringAllocs,
			call: func() { got = backend.Docstring(escapedDoc, "") },
			check: func(tb assert.TB) {
				assert.Equal(tb, got, "\"\"\"Writes \\\"\\\"\\\" and \\\\.\"\"\"\n", "Docstring escapes the line")
			},
		},
	}
}
