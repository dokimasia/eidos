// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package naming_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/naming"
)

// Splitting is the primitive every style is built on, so its
// boundary rules are the contract: where a word starts, which
// runes are consumed, and what a digit does.
func TestWords(t *testing.T) {
	t.Parallel()

	t.Run("splits at the boundaries it declares", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			in   string
			want []string
		}{
			{"empty input carries no word", "", nil},
			{"separators alone carry no word", "___---...", nil},
			{"one lower-case word", "hello", []string{"hello"}},
			{"camel case breaks at lower to upper", "helloWorld", []string{"hello", "World"}},
			{"pascal case breaks the same way", "HelloWorld", []string{"Hello", "World"}},
			{"an acronym before a word breaks after the run", "HTTPServer", []string{"HTTP", "Server"}},
			{"a trailing acronym stays whole", "userID", []string{"user", "ID"}},
			{"an all-upper identifier stays whole", "HTTP", []string{"HTTP"}},
			{"underscore separates", "hello_world", []string{"hello", "world"}},
			{"hyphen separates", "hello-world", []string{"hello", "world"}},
			{"dot separates", "hello.world", []string{"hello", "world"}},
			{"space separates", "hello world", []string{"hello", "world"}},
			{"tab separates", "hello\tworld", []string{"hello", "world"}},
			{"slash separates", "hello/world", []string{"hello", "world"}},
			{"repeated separators collapse", "__hello-_-world__", []string{"hello", "world"}},
			{"a digit joins the word before it", "Version2", []string{"Version2"}},
			{"an upper-case rune after a digit starts a word", "Int64Value", []string{"Int64", "Value"}},
			{"after an acronym's digit too", "HTTP2Server", []string{"HTTP2", "Server"}},
			{
				"every rule at once", "URLPath_v2 helloWorld",
				[]string{"URL", "Path", "v2", "hello", "World"},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, naming.Words(tt.in), tt.want, tt.name)
			})
		}
	})

	t.Run("splits by structure, never by initialism", func(t *testing.T) {
		t.Parallel()

		bare := naming.New()
		assert.Equal(t, bare.Words("URLPath"), naming.Default().Words("URLPath"),
			"the splitter reads case and separators, so a Caser recognising "+
				"no initialism splits identically")
	})

	t.Run("keeps the words of an invalid encoding", func(t *testing.T) {
		t.Parallel()

		got := naming.Words("a\xbeb")
		assert.Length(t, got, 1, "an invalid byte starts no word")
		assert.Equal(t, got[0], "a�b",
			"and is replaced, so the word owns its bytes rather than "+
				"carrying a byte no decoder accepts")
	})
}
