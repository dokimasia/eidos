// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package textfmt_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/textfmt"
)

// normalizeAllocs is the normalized text, allocated at the input's
// length.
const normalizeAllocs = 1

// Normalize removes the whitespace a template leaves. Every other byte
// passes through unchanged.
func TestNormalize(t *testing.T) {
	t.Parallel()

	t.Run("Normalize", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want string
		}{
			{name: "strips trailing whitespace", give: "a; \t\nb;\n", want: "a;\nb;\n"},
			{name: "collapses a run of blank lines to one", give: "a;\n\n\n\nb;\n", want: "a;\n\nb;\n"},
			{name: "drops leading blank lines", give: "\n\nclass A {}\n", want: "class A {}\n"},
			{name: "ends the text with one newline", give: "class A {}\n\n\n", want: "class A {}\n"},
			{name: "keeps interior content", give: "  indented\ntext  here\n", want: "  indented\ntext  here\n"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := textfmt.Normalize([]byte(tt.give))
				assert.NoError(t, err, "the text normalizes")
				assert.Equal(t, string(got), tt.want, "the normalized text")
			})
		}

		t.Run("returns an empty input as it is", func(t *testing.T) {
			t.Parallel()

			got, err := textfmt.Normalize(nil)
			assert.NoError(t, err, "nothing normalizes")
			assert.Empty(t, got, "to nothing")
		})
	})
}

// Normalize allocates its output once. The ordinary run, which runs no
// benchmark, checks that ceiling here.
func TestNormalizeAllocs(t *testing.T) {
	src := renderedText()
	var got []byte
	assert.MaxAllocs(t, func() { got, _ = textfmt.Normalize(src) }, normalizeAllocs,
		"Normalize allocates the output")
	assert.Equal(t, string(got), "a;\n\nb;\nclass A {}\n", "Normalize normalizes the text")
}

// BenchmarkNormalize measures the normalization every rendered file
// passes through once.
func BenchmarkNormalize(b *testing.B) {
	b.Run("Normalize", func(b *testing.B) {
		src := renderedText()
		c := bench.Start(b).MaxAllocs(normalizeAllocs)
		defer c.End()
		var got []byte
		for c.Loop() {
			got, _ = textfmt.Normalize(src)
		}
		assert.Equal(b, string(got), "a;\n\nb;\nclass A {}\n", "Normalize normalizes the text")
	})
}

// renderedText returns a template's output with trailing whitespace,
// a run of blank lines and trailing blank lines.
func renderedText() []byte {
	return []byte("a; \t\n\n\n\nb;\nclass A {}\n\n\n")
}
