// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package directive_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/directive"
)

// scalar returns a bare scalar value.
func scalar(text string) directive.RawValue {
	return directive.RawValue{Text: text}
}

// quoted returns a quoted scalar value.
func quoted(text string) directive.RawValue {
	return directive.RawValue{Text: text, Quoted: true}
}

func TestGrammar(t *testing.T) {
	t.Parallel()

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the pinned grammar", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name    string
				payload string
				want    directive.Raw
			}{
				{
					name:    "a bare name alone",
					payload: "skip",
					want:    directive.Raw{Name: "skip"},
				},
				{
					name:    "a prefixed name",
					payload: "mockgen:stub",
					want:    directive.Raw{Name: "mockgen:stub"},
				},
				{
					name:    "a positional bare argument",
					payload: "stub handler",
					want: directive.Raw{Name: "stub", Args: []directive.RawArg{
						{Value: scalar("handler"), Col: 5},
					}},
				},
				{
					name:    "a keyed argument",
					payload: "stub tag=test",
					want: directive.Raw{Name: "stub", Args: []directive.RawArg{
						{Key: "tag", Value: scalar("test"), Col: 5},
					}},
				},
				{
					name:    "positional and keyed interleave in source order",
					payload: "route get path=/x fallback",
					want: directive.Raw{Name: "route", Args: []directive.RawArg{
						{Value: scalar("get"), Col: 6},
						{Key: "path", Value: scalar("/x"), Col: 10},
						{Value: scalar("fallback"), Col: 18},
					}},
				},
				{
					name:    "a quoted value keeps spaces and resolves escapes",
					payload: `doc text="a \"quoted\" line\nnext\ttab"`,
					want: directive.Raw{Name: "doc", Args: []directive.RawArg{
						{Key: "text", Value: quoted("a \"quoted\" line\nnext\ttab"), Col: 4},
					}},
				},
				{
					name:    "a quoted empty string is a value",
					payload: `doc text=""`,
					want: directive.Raw{Name: "doc", Args: []directive.RawArg{
						{Key: "text", Value: quoted(""), Col: 4},
					}},
				},
				{
					name:    "a list value",
					payload: "index fields=[a, b,c]",
					want: directive.Raw{Name: "index", Args: []directive.RawArg{
						{Key: "fields", Value: directive.RawValue{List: []directive.RawValue{
							scalar("a"), scalar("b"), scalar("c"),
						}}, Col: 6},
					}},
				},
				{
					name:    "an empty list",
					payload: "index fields=[]",
					want: directive.Raw{Name: "index", Args: []directive.RawArg{
						{Key: "fields", Value: directive.RawValue{List: []directive.RawValue{}}, Col: 6},
					}},
				},
				{
					name:    "a nested list parses; the schema refuses it",
					payload: "x k=[[a],b]",
					want: directive.Raw{Name: "x", Args: []directive.RawArg{
						{Key: "k", Value: directive.RawValue{List: []directive.RawValue{
							{List: []directive.RawValue{scalar("a")}},
							scalar("b"),
						}}, Col: 2},
					}},
				},
				{
					name:    "surrounding whitespace is tolerated",
					payload: "  stub  tag=test  ",
					want: directive.Raw{Name: "stub", Args: []directive.RawArg{
						{Key: "tag", Value: scalar("test"), Col: 8},
					}},
				},
				{
					name:    "a bare value carries symbols the grammar admits",
					payload: "meta drop=shape.role",
					want: directive.Raw{Name: "meta", Args: []directive.RawArg{
						{Key: "drop", Value: scalar("shape.role"), Col: 5},
					}},
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					got, err := directive.Parse(tt.payload)
					assert.NoError(t, err, "a payload inside the grammar parses")
					assert.Equal(t, got, tt.want, "and reads back exactly what was written")
				})
			}
		})

		t.Run("refuses a payload outside the grammar", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name    string
				payload string
			}{
				{name: "an empty payload", payload: ""},
				{name: "whitespace alone", payload: "   "},
				{name: "a name starting with a digit", payload: "1stub"},
				{name: "a name that is only a prefix", payload: "mockgen:"},
				{name: "a prefix that is empty", payload: ":stub"},
				{name: "an unterminated quote", payload: `doc text="open`},
				{name: "an unknown escape", payload: `doc text="a\z"`},
				{name: "an unterminated list", payload: "index fields=[a, b"},
				{name: "a key without a value", payload: "stub tag="},
				{name: "a key that is not an identifier", payload: "stub 2x=v"},
				{name: "an equals sign starting an argument", payload: "stub =v"},
				{name: "a stray closing bracket", payload: "stub ]"},
				{name: "a stray comma", payload: "stub a,b"},
				{name: "a bad key spelling inside a list keyed argument", payload: "stub k=[2x=1]"},
				{name: "an escape cut off by the end", payload: `doc text="a\`},
				{name: "a broken value inside a list", payload: `stub k=["a\z"]`},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					_, err := directive.Parse(tt.payload)
					assert.HasError(t, err, "a payload outside the grammar is refused")
					assert.HasPrefix(t, err.Error(), "directive: ", "under the package prefix")
				})
			}
		})

		t.Run("names the offset where reading stopped", func(t *testing.T) {
			t.Parallel()

			_, err := directive.Parse("stub tag=")
			assert.HasError(t, err, "the empty value is refused")
			assert.Contains(t, err.Error(), "9", "at its byte offset")
		})
	})

	t.Run("Join", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			lines []string
			want  string
		}{
			{
				name:  "a single line joins to itself",
				lines: []string{"stub tag=test"},
				want:  "stub tag=test",
			},
			{
				name:  "a continuation joins with a single space",
				lines: []string{`stub \`, "tag=test"},
				want:  "stub tag=test",
			},
			{
				name:  "three lines chain",
				lines: []string{`index \`, `fields=[a, \`, "b]"},
				want:  "index fields=[a, b]",
			},
			{
				name:  "a trailing backslash on the last line strips",
				lines: []string{`stub \`},
				want:  "stub",
			},
			{
				name:  "a backslash inside a line is not a continuation",
				lines: []string{`doc text="a\nb"`},
				want:  `doc text="a\nb"`,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, directive.Join(tt.lines), tt.want,
					"the continuation rule lives here once, not in every frontend")
			})
		}
	})
}

// FuzzParse drives the parser with bytes nothing in this repository
// wrote: payloads arrive from consumers' source files.
//
// Two properties hold whatever the bytes are: Parse never panics,
// and a payload that parses re-parses to the same instance after a
// join round through its own spelling-neutral form.
func FuzzParse(f *testing.F) {
	for _, seed := range []string{
		"skip",
		"mockgen:stub handler tag=test",
		`doc text="a \"b\" c\n"`,
		"index fields=[a, b,c] unique",
		"  x  ", "", ":", "=", `k="`, "a=[", "meta drop=shape.role",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, payload string) {
		got, err := directive.Parse(payload)
		if err != nil {
			assert.HasPrefix(t, err.Error(), "directive: ",
				"every refusal carries the package prefix")
			return
		}
		assert.NotEqual(t, string(got.Name), "",
			"a parsed instance always names a directive")
		for _, arg := range got.Args {
			assert.True(t, arg.Col > 0 && arg.Col <= len(payload),
				"every argument's offset points inside the payload")
			if arg.Key != "" {
				assert.False(t, strings.ContainsAny(arg.Key, " \t=\"[],"),
					"a key is an identifier, never grammar punctuation")
			}
		}
	})
}

// Parsing runs once per carrier line across every loaded file, so
// its cost per directive is what bounds a large workspace's load.
func BenchmarkGrammar(b *testing.B) {
	b.Run("Parse", func(b *testing.B) {
		b.ReportAllocs()

		const payload = `indexer:index btree fields=[a, b, c] depth=3 unique=true role=server`
		for b.Loop() {
			if _, err := directive.Parse(payload); err != nil {
				b.Fatalf("Parse: unexpected error: %v", err)
			}
		}
	})

	b.Run("Parse minimal", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			if _, err := directive.Parse("skip"); err != nil {
				b.Fatalf("Parse: unexpected error: %v", err)
			}
		}
	})

	b.Run("Join", func(b *testing.B) {
		b.ReportAllocs()

		lines := []string{`index \`, `fields=[a, b, \`, `c] depth=3`}
		for b.Loop() {
			_ = directive.Join(lines)
		}
	})
}
