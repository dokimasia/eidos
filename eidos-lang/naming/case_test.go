// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package naming_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/naming"
)

// style is one conversion case: the input, what it spells, and
// the Caser it spells through where the default is not the point.
type style struct {
	name  string
	caser *naming.Caser
	in    string
	want  string
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

// inputs returns every string of up to n runes drawn from alphabet,
// the empty string included.
func inputs(alphabet []string, n int) []string {
	out, level := []string{""}, []string{""}
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

// The styles are what a satellite spells its names through, so
// each one's rules are contract: how a word is cased, where a
// separator goes, and which acronyms keep their case through a round
// trip.
func TestCase(t *testing.T) {
	t.Parallel()

	t.Run("returns an input already in the style unchanged", func(t *testing.T) {
		t.Parallel()

		// BenchmarkCase and the settle ceilings that consume it check
		// the zero-allocation half of the contract, because
		// AllocsPerRun refuses to run beside parallel tests.
		styled := []struct {
			name string
			fn   func(string) string
			in   string
		}{
			{"Pascal", naming.Pascal, "HTTPRow"},
			{"Camel", naming.Camel, "httpRow"},
			{"Snake", naming.Snake, "row_count"},
			{"ScreamingSnake", naming.ScreamingSnake, "MAX_ROWS"},
			{"Kebab", naming.Kebab, "row-count"},
		}
		for _, c := range styled {
			assert.Equal(t, c.fn(c.in), c.in,
				c.name+" keeps an already-styled input whole")
		}

		assert.Equal(t, naming.Camel("HTTPRow"), "httpRow",
			"a near miss still converts")
		assert.Equal(t, naming.Snake("rowCount"), "row_count",
			"through the builder")
		assert.Equal(t, naming.Snake("row__count"), "row_count",
			"and a doubled separator never passes as styled")
	})

	t.Run("Pascal", func(t *testing.T) {
		t.Parallel()
		runStyles(t, (*naming.Caser).Pascal, []style{
			{name: "empty input spells nothing", in: "", want: ""},
			{name: "snake becomes pascal", in: "hello_world", want: "HelloWorld"},
			{name: "camel becomes pascal", in: "helloWorld", want: "HelloWorld"},
			{name: "pascal is unchanged", in: "HelloWorld", want: "HelloWorld"},
			{name: "a recognised initialism is upper-cased", in: "url_path", want: "URLPath"},
			{
				name:  "an unrecognised one is title-cased",
				caser: naming.New(), in: "url_path", want: "UrlPath",
			},
			{name: "an acronym run inside the name keeps its case", in: "HTTPServer", want: "HTTPServer"},
			{
				name:  "an acronym run the caser does not know keeps its case",
				caser: naming.New(), in: "IOReader", want: "IOReader",
			},
			{name: "a word after a digit keeps its capital", in: "Base64Encode", want: "Base64Encode"},
			{name: "a trailing initialism keeps its case", in: "user_id", want: "UserID"},
			{
				name:  "an all-upper input keeps its case without being an initialism",
				caser: naming.New(), in: "FOO", want: "FOO",
			},
			{name: "an upper-case identifier keeps its separators", in: "STATUS_ACTIVE", want: "STATUS_ACTIVE"},
			{name: "an upper-case identifier keeps its digits", in: "SHA256", want: "SHA256"},
		})
	})

	t.Run("Camel", func(t *testing.T) {
		t.Parallel()
		runStyles(t, (*naming.Caser).Camel, []style{
			{name: "empty input spells nothing", in: "", want: ""},
			{name: "snake becomes camel", in: "hello_world", want: "helloWorld"},
			{name: "pascal becomes camel", in: "HelloWorld", want: "helloWorld"},
			{name: "the first word lower-cases even as an initialism", in: "URL_path", want: "urlPath"},
			{name: "a trailing initialism keeps its case", in: "user_id", want: "userID"},
			{
				name:  "an acronym run after the first word keeps its case",
				caser: naming.New(), in: "parseIOReader", want: "parseIOReader",
			},
			{
				name: "an input without a lower-case letter title-cases its later words",
				in:   "STATUS_ACTIVE", want: "statusActive",
			},
			{
				name: "and the already-styled check agrees with the conversion on it",
				in:   "1ABC", want: "1Abc",
			},
		})
	})

	t.Run("Snake", func(t *testing.T) {
		t.Parallel()
		runStyles(t, (*naming.Caser).Snake, []style{
			{name: "empty input spells nothing", in: "", want: ""},
			{name: "pascal becomes snake", in: "HelloWorld", want: "hello_world"},
			{name: "an acronym run lower-cases whole", in: "HTTPServer", want: "http_server"},
			{name: "snake is unchanged", in: "hello_world", want: "hello_world"},
		})
	})

	t.Run("ScreamingSnake", func(t *testing.T) {
		t.Parallel()
		runStyles(t, (*naming.Caser).ScreamingSnake, []style{
			{name: "empty input spells nothing", in: "", want: ""},
			{name: "pascal becomes screaming snake", in: "HelloWorld", want: "HELLO_WORLD"},
			{name: "an acronym run is kept upper-case", in: "HTTPServer", want: "HTTP_SERVER"},
		})
	})

	t.Run("Kebab", func(t *testing.T) {
		t.Parallel()
		runStyles(t, (*naming.Caser).Kebab, []style{
			{name: "empty input spells nothing", in: "", want: ""},
			{name: "pascal becomes kebab", in: "HelloWorld", want: "hello-world"},
			{name: "snake becomes kebab", in: "hello_world", want: "hello-world"},
		})
	})

	t.Run("the package functions spell through the default Caser", func(t *testing.T) {
		t.Parallel()

		const in = "user_id_fetcher"
		c := naming.Default()
		tests := []struct {
			name  string
			got   string
			want  string
			style string
		}{
			{"Pascal", naming.Pascal(in), c.Pascal(in), "UserIDFetcher"},
			{"Camel", naming.Camel(in), c.Camel(in), "userIDFetcher"},
			{"Snake", naming.Snake(in), c.Snake(in), "user_id_fetcher"},
			{"ScreamingSnake", naming.ScreamingSnake(in), c.ScreamingSnake(in), "USER_ID_FETCHER"},
			{"Kebab", naming.Kebab(in), c.Kebab(in), "user-id-fetcher"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, tt.got, tt.want, "the shorthand takes the default Caser")
				assert.Equal(t, tt.got, tt.style, "and spells what the style names")
			})
		}
		assert.Equal(t, naming.Words(in), c.Words(in), "Words takes it too")
	})

	t.Run("every style is idempotent over ASCII letters and separators", func(t *testing.T) {
		t.Parallel()

		styles := map[string]func(string) string{
			"Pascal": naming.Pascal, "Camel": naming.Camel, "Snake": naming.Snake,
			"ScreamingSnake": naming.ScreamingSnake, "Kebab": naming.Kebab,
		}
		for _, in := range inputs([]string{"a", "d", "A", "D", "_", "-"}, 5) {
			for name, convert := range styles {
				once := convert(in)
				if again := convert(once); again != once {
					t.Fatalf("%s(%q) = %q, and converting that again gives %q", name, in, once, again)
				}
			}
		}
	})

	t.Run("the second conversion is a fixed point", func(t *testing.T) {
		t.Parallel()

		// A digit or a non-ASCII rune can move a boundary the second
		// pass reads differently, but the third never moves again, so
		// the divergence is bounded at one step.
		for _, in := range []string{"aA1a", "aÉ", "ßa"} {
			t.Run(in, func(t *testing.T) {
				t.Parallel()
				once := naming.Pascal(in)
				assert.Equal(t, naming.Pascal(naming.Pascal(once)), naming.Pascal(once),
					"the third pass spells what the second did")
			})
		}
	})
}

// BenchmarkCase measures the conversions a render calls per
// declaration: splitting is the shared cost, and each style adds
// one Builder over the words it splits into.
func BenchmarkCase(b *testing.B) {
	const in = "HTTPServerConfig_v2"

	b.Run("Words", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			naming.Words(in)
		}
	})
	b.Run("Pascal", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			naming.Pascal(in)
		}
	})
	b.Run("Snake", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			naming.Snake(in)
		}
	})
}
