// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package naming_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/naming"
)

// Splitting is the primitive every style is built on, so its
// boundary rules are the contract: where a word starts, which
// runes are consumed, and what a digit does.
func TestWords(t *testing.T) {
	t.Parallel()

	t.Run("Caser.Words", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			in   string
			want []string
		}{
			{name: "returns nil for an empty input", in: "", want: nil},
			{name: "returns nil for separators alone", in: "___---...", want: nil},
			{name: "returns one lower-case word whole", in: "hello", want: []string{"hello"}},
			{
				name: "splits camel case at a lower-to-upper transition",
				in:   "helloWorld",
				want: []string{"hello", "World"},
			},
			{
				name: "splits Pascal case at a lower-to-upper transition",
				in:   "HelloWorld",
				want: []string{"Hello", "World"},
			},
			{
				name: "splits an acronym run before the word that follows it",
				in:   "HTTPServer", want: []string{"HTTP", "Server"},
			},
			{name: "returns a trailing acronym whole", in: "userID", want: []string{"user", "ID"}},
			{name: "returns an all-upper identifier whole", in: "HTTP", want: []string{"HTTP"}},
			{name: "splits at an underscore", in: "hello_world", want: []string{"hello", "world"}},
			{name: "splits at a hyphen", in: "hello-world", want: []string{"hello", "world"}},
			{name: "splits at a dot", in: "hello.world", want: []string{"hello", "world"}},
			{name: "splits at a space", in: "hello world", want: []string{"hello", "world"}},
			{name: "splits at a tab", in: "hello\tworld", want: []string{"hello", "world"}},
			{name: "splits at a slash", in: "hello/world", want: []string{"hello", "world"}},
			{name: "collapses repeated separators", in: "__hello-_-world__", want: []string{"hello", "world"}},
			{name: "joins a digit to the word before it", in: "Version2", want: []string{"Version2"}},
			{
				name: "splits at an upper-case rune after a digit",
				in:   "Int64Value", want: []string{"Int64", "Value"},
			},
			{
				name: "splits at an upper-case rune after an acronym's digit",
				in:   "HTTP2Server", want: []string{"HTTP2", "Server"},
			},
			{
				name: "applies every boundary rule in one input",
				in:   "URLPath_v2 helloWorld", want: []string{"URL", "Path", "v2", "hello", "World"},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, naming.Default().Words(tt.in), tt.want, "the words of the input")
			})
		}

		t.Run("returns the words a Caser without initialisms returns", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, naming.New().Words("URLPath"), naming.Default().Words("URLPath"),
				"the splitter reads case and separators, never the initialism set")
		})

		t.Run("returns a word with U+FFFD in place of an invalid byte", func(t *testing.T) {
			t.Parallel()

			got := naming.Default().Words("a\xbeb")
			assert.Length(t, got, 1, "an invalid byte starts no word")
			assert.Equal(t, got[0], "a�b", "the word is a copy in valid UTF-8")
		})
	})
}

// A split allocates its slice once for an ASCII input in the ordinary
// run, which runs no benchmark.
func TestWordsAllocs(t *testing.T) {
	caser := naming.Default()
	var words []string
	assert.MaxAllocs(t, func() { words = caser.Words(mixedInput) }, wordsAllocs,
		"Words allocates the slice it returns, once")
	assert.Length(t, words, mixedWords, "the input splits into its words")
}

// BenchmarkWords measures the split every style runs over its input.
func BenchmarkWords(b *testing.B) {
	b.Run("Caser.Words", func(b *testing.B) {
		caser := naming.Default()
		c := bench.Start(b).MaxAllocs(wordsAllocs)
		defer c.End()
		var words []string
		for c.Loop() {
			words = caser.Words(mixedInput)
		}
		assert.Length(b, words, mixedWords, "the input splits into its words")
	})
}
