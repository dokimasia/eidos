// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package naming_test

import (
	"strconv"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/naming"
)

// The inputs the allocation contracts convert: an identifier with an
// acronym run, a case boundary, a separator and a digit, which no style
// spells already, and the words it splits into.
const (
	mixedInput = "HTTPServerConfig_v2"
	mixedWords = 4
)

// The ceilings of [BenchmarkCase].
const (
	// wordsAllocs is the words slice, which the estimate sizes once for
	// an ASCII input.
	wordsAllocs = 1
	// styleAllocs is a conversion's result, which the builder sizes once
	// to the input's length.
	styleAllocs = 1
)

// caseStyles is every style of the package with an input the style
// spells already and the style's spelling of [mixedInput].
var caseStyles = []struct {
	name   string
	fn     func(string) string
	styled string
	mixed  string
}{
	{name: "Pascal", fn: naming.Pascal, styled: "HTTPRow", mixed: "HTTPServerConfigV2"},
	{name: "Camel", fn: naming.Camel, styled: "httpRow", mixed: "httpServerConfigV2"},
	{name: "Snake", fn: naming.Snake, styled: "row_count", mixed: "http_server_config_v2"},
	{name: "ScreamingSnake", fn: naming.ScreamingSnake, styled: "MAX_ROWS", mixed: "HTTP_SERVER_CONFIG_V2"},
	{name: "Kebab", fn: naming.Kebab, styled: "row-count", mixed: "http-server-config-v2"},
}

// style is one conversion case: the input, what it spells, and
// the Caser it spells through where the default is not the point.
type style struct {
	name  string
	caser *naming.Caser
	in    string
	want  string
}

// Every satellite spells its names through these styles. Each style's
// rules are pinned: how a word is cased, where a separator goes, and
// which acronyms keep their case through a round trip.
func TestCase(t *testing.T) {
	t.Parallel()

	t.Run("Pascal", func(t *testing.T) {
		t.Parallel()

		runStyles(t, (*naming.Caser).Pascal, []style{
			{name: "returns an empty string for an empty input", in: "", want: ""},
			{name: "converts a snake-case input", in: "hello_world", want: "HelloWorld"},
			{name: "converts a camel-case input", in: "helloWorld", want: "HelloWorld"},
			{name: "returns a Pascal-case input unchanged", in: "HelloWorld", want: "HelloWorld"},
			{name: "upper-cases a recognised initialism", in: "url_path", want: "URLPath"},
			{
				name:  "title-cases an initialism the caser does not know",
				caser: naming.New(), in: "url_path", want: "UrlPath",
			},
			{name: "keeps the case of an acronym run inside the name", in: "HTTPServer", want: "HTTPServer"},
			{
				name:  "keeps the case of an acronym run the caser does not know",
				caser: naming.New(), in: "IOReader", want: "IOReader",
			},
			{name: "keeps the capital of a word after a digit", in: "Base64Encode", want: "Base64Encode"},
			{name: "keeps the case of a trailing initialism", in: "user_id", want: "UserID"},
			{
				name:  "keeps the case of an all-upper input that is no initialism",
				caser: naming.New(), in: "FOO", want: "FOO",
			},
			{name: "keeps the separators of an upper-case identifier", in: "STATUS_ACTIVE", want: "STATUS_ACTIVE"},
			{name: "keeps the digits of an upper-case identifier", in: "SHA256", want: "SHA256"},
			{name: "upper-cases an initialism written in lower case", in: "http_server", want: "HTTPServer"},
			{name: "upper-cases an initialism written in mixed case", in: "Http_server", want: "HTTPServer"},
			{
				name: "upper-cases a word whose upper-case form is an initialism",
				in:   "ıd_token", want: "IDToken",
			},
		})
		runShared(t, naming.Pascal, (*naming.Caser).Pascal, "HTTPRow", "UserIDFetcher")

		// A digit or a non-ASCII rune can move a boundary the second
		// pass reads differently, but the third never moves again, so
		// the divergence is bounded at one step.
		for _, in := range []string{"aA1a", "aÉ", "ßa"} {
			t.Run("returns from a third conversion of "+in+" what the second returns", func(t *testing.T) {
				t.Parallel()

				once := naming.Pascal(in)
				assert.Equal(t, naming.Pascal(naming.Pascal(once)), naming.Pascal(once),
					"the third pass spells what the second did")
			})
		}
	})

	t.Run("Camel", func(t *testing.T) {
		t.Parallel()

		runStyles(t, (*naming.Caser).Camel, []style{
			{name: "returns an empty string for an empty input", in: "", want: ""},
			{name: "converts a snake-case input", in: "hello_world", want: "helloWorld"},
			{name: "converts a Pascal-case input", in: "HelloWorld", want: "helloWorld"},
			{name: "converts a Pascal-case input that opens with an acronym run", in: "HTTPRow", want: "httpRow"},
			{name: "lower-cases a first word that is an initialism", in: "URL_path", want: "urlPath"},
			{name: "keeps the case of a trailing initialism", in: "user_id", want: "userID"},
			{
				name:  "keeps the case of an acronym run after the first word",
				caser: naming.New(), in: "parseIOReader", want: "parseIOReader",
			},
			{
				name: "title-cases the later words of an input without a lower-case letter",
				in:   "STATUS_ACTIVE", want: "statusActive",
			},
			{
				name: "converts an input without a lower-case letter that opens with a digit",
				in:   "1ABC", want: "1Abc",
			},
		})
		runShared(t, naming.Camel, (*naming.Caser).Camel, "httpRow", "userIDFetcher")
	})

	t.Run("Snake", func(t *testing.T) {
		t.Parallel()

		runStyles(t, (*naming.Caser).Snake, []style{
			{name: "returns an empty string for an empty input", in: "", want: ""},
			{name: "converts a Pascal-case input", in: "HelloWorld", want: "hello_world"},
			{name: "converts a camel-case input", in: "rowCount", want: "row_count"},
			{name: "lower-cases an acronym run whole", in: "HTTPServer", want: "http_server"},
			{name: "returns a snake-case input unchanged", in: "hello_world", want: "hello_world"},
			{name: "joins the words of a doubled separator with one", in: "row__count", want: "row_count"},
			{name: "writes the replacement rune for a byte of invalid UTF-8", in: "\xbe", want: "�"},
		})
		runShared(t, naming.Snake, (*naming.Caser).Snake, "row_count", "user_id_fetcher")
	})

	t.Run("ScreamingSnake", func(t *testing.T) {
		t.Parallel()

		runStyles(t, (*naming.Caser).ScreamingSnake, []style{
			{name: "returns an empty string for an empty input", in: "", want: ""},
			{name: "converts a Pascal-case input", in: "HelloWorld", want: "HELLO_WORLD"},
			{name: "keeps an acronym run upper-case", in: "HTTPServer", want: "HTTP_SERVER"},
		})
		runShared(t, naming.ScreamingSnake, (*naming.Caser).ScreamingSnake, "MAX_ROWS", "USER_ID_FETCHER")
	})

	t.Run("Kebab", func(t *testing.T) {
		t.Parallel()

		runStyles(t, (*naming.Caser).Kebab, []style{
			{name: "returns an empty string for an empty input", in: "", want: ""},
			{name: "converts a Pascal-case input", in: "HelloWorld", want: "hello-world"},
			{name: "converts a snake-case input", in: "hello_world", want: "hello-world"},
		})
		runShared(t, naming.Kebab, (*naming.Caser).Kebab, "row-count", "user-id-fetcher")
	})

	t.Run("Words", func(t *testing.T) {
		t.Parallel()

		t.Run("returns what the default Caser returns", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, naming.Words(mixedInput), naming.Default().Words(mixedInput),
				"the shorthand takes the default Caser")
		})
	})
}

// Each conversion allocates its result alone, and nothing for an input
// already in its style. Words allocates its slice once. The ordinary
// run, which runs no benchmark, checks those ceilings here.
func TestCaseAllocs(t *testing.T) {
	var words []string
	assert.MaxAllocs(t, func() { words = naming.Words(mixedInput) }, wordsAllocs,
		"Words allocates the slice it returns, once")
	assert.Length(t, words, mixedWords, "the input splits into its words")

	for _, s := range caseStyles {
		var got string
		converts := s.name + " allocates its result alone"
		assert.MaxAllocs(t, func() { got = s.fn(mixedInput) }, styleAllocs, converts)
		assert.Equal(t, got, s.mixed, s.name+" spells the input in its style")
		keeps := s.name + " returns an input already in its style without allocating"
		assert.MaxAllocs(t, func() { got = s.fn(s.styled) }, 0, keeps)
		assert.Equal(t, got, s.styled, s.name+" returns the styled input itself")
	}
}

// BenchmarkCase measures the conversions a render calls per
// declaration: each splits its input and writes the words into one
// Builder, and an input already in the style passes through whole.
func BenchmarkCase(b *testing.B) {
	b.Run("Words", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(wordsAllocs)
		defer c.End()
		var words []string
		for c.Loop() {
			words = naming.Words(mixedInput)
		}
		assert.Length(b, words, mixedWords, "the input splits into its words")
	})

	for _, s := range caseStyles {
		b.Run(s.name, func(b *testing.B) {
			b.Run("an input in mixed styles", func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(styleAllocs)
				defer c.End()
				var got string
				for c.Loop() {
					got = s.fn(mixedInput)
				}
				assert.Equal(b, got, s.mixed, "the conversion spells the input in its style")
			})

			b.Run("an input already in the style", func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(0)
				defer c.End()
				var got string
				for c.Loop() {
					got = s.fn(s.styled)
				}
				assert.Equal(b, got, s.styled, "the conversion returns the input itself")
			})
		})
	}
}

// runStyles drives one style over its cases.
func runStyles(t *testing.T, convert func(*naming.Caser, string) string, cases []style) {
	t.Helper()

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := tt.caser
			if c == nil {
				c = naming.Default()
			}
			assert.Equal(t, convert(c, tt.in), tt.want, tt.name)
		})
	}
}

// runShared drives the cases every style shares: an input already in
// the style returns whole, the package function spells through the
// default Caser, and a conversion of a conversion changes nothing over
// every short string of ASCII letters and separators.
func runShared(
	t *testing.T, fn func(string) string, method func(*naming.Caser, string) string, styled, fetcher string,
) {
	t.Helper()

	t.Run("returns an input already in the style unchanged", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, fn(styled), styled, "the style keeps an already-styled input whole")
	})

	t.Run("returns what the default Caser returns", func(t *testing.T) {
		t.Parallel()

		const in = "user_id_fetcher"
		assert.Equal(t, fn(in), method(naming.Default(), in), "the shorthand takes the default Caser")
		assert.Equal(t, fn(in), fetcher, "and spells what the style names")
	})

	t.Run("returns its own output unchanged for inputs of letters with separators", func(t *testing.T) {
		t.Parallel()

		for _, in := range inputs([]string{"a", "d", "A", "D", "_", "-"}, 5) {
			once := fn(in)
			assert.Equal(t, fn(once), once, "a conversion of the conversion of "+strconv.Quote(in)+" changes nothing")
		}
	})
}

// inputs returns every string of up to n runes drawn from alphabet,
// the empty string included.
func inputs(alphabet []string, n int) []string {
	total, width := 1, 1
	for range n {
		width *= len(alphabet)
		total += width
	}
	out, level := make([]string, 1, total), []string{""}
	for range n {
		next := make([]string, 0, len(level)*len(alphabet))
		for _, prefix := range level {
			for _, r := range alphabet {
				next = append(next, prefix+r)
			}
		}
		out, level = append(out, next...), next
	}
	return out
}
