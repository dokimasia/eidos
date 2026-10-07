// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package directive_test

import (
	"cmp"
	"encoding/binary"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/prop"

	"go.dokimi.dev/eidos/core/directive"
)

// The payloads the allocation checks and the benchmarks parse: one with
// five arguments, one a list, and one with a bare name alone.
const (
	fullPayload = `indexer:index btree fields=[a, b, c] depth=3 unique=true role=server`
	barePayload = "skip"
)

// parsesSafely is the property [FuzzParse] and its ForAll twin in
// [TestGrammar] state.
const parsesSafely = "Parse must return a prefixed error, or an instance that names a directive " +
	"and positions every argument inside the payload under an identifier key"

// continued is a payload written over three carrier lines.
var continued = []string{`index \`, `fields=[a, b, \`, `c] depth=3`}

func TestGrammar(t *testing.T) {
	t.Parallel()

	t.Run("Plugin", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give directive.Name
			want string
		}{
			{name: "returns the prefix of a prefixed name", give: "mockgen:stub", want: "mockgen"},
			{name: "returns the empty string for a bare plugin name", give: "stub", want: ""},
			{name: "returns the empty string for a kernel name", give: directive.KernelSkip, want: ""},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.Plugin(), tt.want, "the prefix is the part before the colon")
			})
		}
	})

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		reads := []struct {
			name    string
			payload string
			want    directive.Raw
		}{
			{
				name:    "reads a bare name alone",
				payload: "skip",
				want:    directive.Raw{Name: "skip"},
			},
			{
				name:    "reads a prefixed name",
				payload: "mockgen:stub",
				want:    directive.Raw{Name: "mockgen:stub"},
			},
			{
				name:    "reads a positional bare argument",
				payload: "stub handler",
				want: directive.Raw{Name: "stub", Args: []directive.RawArg{
					{Value: scalar("handler"), Col: 5},
				}},
			},
			{
				name:    "reads a keyed argument",
				payload: "stub tag=test",
				want: directive.Raw{Name: "stub", Args: []directive.RawArg{
					{Key: "tag", Value: scalar("test"), Col: 5},
				}},
			},
			{
				name:    "reads mixed arguments in source order",
				payload: "route get path=/x fallback",
				want: directive.Raw{Name: "route", Args: []directive.RawArg{
					{Value: scalar("get"), Col: 6},
					{Key: "path", Value: scalar("/x"), Col: 10},
					{Value: scalar("fallback"), Col: 18},
				}},
			},
			{
				name:    "reads a quoted value with its escapes resolved",
				payload: `doc text="a \"quoted\" line\nnext\ttab"`,
				want: directive.Raw{Name: "doc", Args: []directive.RawArg{
					{Key: "text", Value: quoted("a \"quoted\" line\nnext\ttab"), Col: 4},
				}},
			},
			{
				name:    "reads a quoted value without escapes",
				payload: `doc text="plain words"`,
				want: directive.Raw{Name: "doc", Args: []directive.RawArg{
					{Key: "text", Value: quoted("plain words"), Col: 4},
				}},
			},
			{
				name:    "reads a quoted value whose escape follows plain text",
				payload: `doc text="plain \"words\""`,
				want: directive.Raw{Name: "doc", Args: []directive.RawArg{
					{Key: "text", Value: quoted(`plain "words"`), Col: 4},
				}},
			},
			{
				name:    "reads a quoted empty string as a value",
				payload: `doc text=""`,
				want: directive.Raw{Name: "doc", Args: []directive.RawArg{
					{Key: "text", Value: quoted(""), Col: 4},
				}},
			},
			{
				name:    "reads more arguments than the parser collects on the stack",
				payload: "x a b c d e f g h i j",
				want: directive.Raw{Name: "x", Args: []directive.RawArg{
					{Value: scalar("a"), Col: 2},
					{Value: scalar("b"), Col: 4},
					{Value: scalar("c"), Col: 6},
					{Value: scalar("d"), Col: 8},
					{Value: scalar("e"), Col: 10},
					{Value: scalar("f"), Col: 12},
					{Value: scalar("g"), Col: 14},
					{Value: scalar("h"), Col: 16},
					{Value: scalar("i"), Col: 18},
					{Value: scalar("j"), Col: 20},
				}},
			},
			{
				name:    "reads a list longer than the parser collects on the stack",
				payload: "x k=[a,b,c,d,e,f,g,h,i,j]",
				want: directive.Raw{Name: "x", Args: []directive.RawArg{
					{Key: "k", Value: directive.RawValue{List: []directive.RawValue{
						scalar("a"), scalar("b"), scalar("c"), scalar("d"), scalar("e"),
						scalar("f"), scalar("g"), scalar("h"), scalar("i"), scalar("j"),
					}}, Col: 2},
				}},
			},
			{
				name:    "reads a list value",
				payload: "index fields=[a, b,c]",
				want: directive.Raw{Name: "index", Args: []directive.RawArg{
					{Key: "fields", Value: directive.RawValue{List: []directive.RawValue{
						scalar("a"), scalar("b"), scalar("c"),
					}}, Col: 6},
				}},
			},
			{
				name:    "reads an empty list",
				payload: "index fields=[]",
				want: directive.Raw{Name: "index", Args: []directive.RawArg{
					{Key: "fields", Value: directive.RawValue{List: []directive.RawValue{}}, Col: 6},
				}},
			},
			{
				name:    "reads a nested list",
				payload: "x k=[[a],b]",
				want: directive.Raw{Name: "x", Args: []directive.RawArg{
					{Key: "k", Value: directive.RawValue{List: []directive.RawValue{
						{List: []directive.RawValue{scalar("a")}},
						scalar("b"),
					}}, Col: 2},
				}},
			},
			{
				name:    "ignores surrounding whitespace",
				payload: "  stub  tag=test  ",
				want: directive.Raw{Name: "stub", Args: []directive.RawArg{
					{Key: "tag", Value: scalar("test"), Col: 8},
				}},
			},
			{
				name:    "reads a bare value with dots",
				payload: "meta drop=shape.role",
				want: directive.Raw{Name: "meta", Args: []directive.RawArg{
					{Key: "drop", Value: scalar("shape.role"), Col: 5},
				}},
			},
		}
		for _, tt := range reads {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := directive.Parse(tt.payload)
				assert.NoError(t, err, "the payload parses")
				assert.Equal(t, got, tt.want, "the instance is what the payload wrote")
			})
		}

		refusals := []struct {
			name    string
			payload string
		}{
			{name: "returns an error for an empty payload", payload: ""},
			{name: "returns an error for whitespace alone", payload: "   "},
			{name: "returns an error for a name starting with a digit", payload: "1stub"},
			{name: "returns an error for a prefix without a name", payload: "mockgen:"},
			{name: "returns an error for a name with an empty prefix", payload: ":stub"},
			{name: "returns an error for an unterminated quote", payload: `doc text="open`},
			{name: "returns an error for an unknown escape", payload: `doc text="a\z"`},
			{name: "returns an error for an unterminated list", payload: "index fields=[a, b"},
			{name: "returns an error for a key without a value", payload: "stub tag="},
			{name: "returns an error for a key that is not an identifier", payload: "stub 2x=v"},
			{name: "returns an error for an argument starting with an equals sign", payload: "stub =v"},
			{name: "returns an error for a stray closing bracket", payload: "stub ]"},
			{name: "returns an error for a stray comma", payload: "stub a,b"},
			{name: "returns an error for a bad key spelling inside a list", payload: "stub k=[2x=1]"},
			{name: "returns an error for an escape cut off by the end", payload: `doc text="a\`},
			{name: "returns an error for a broken value inside a list", payload: `stub k=["a\z"]`},
			{name: "returns an error for an argument glued to a quoted value", payload: `stub tag="t"btree`},
			{name: "returns an error for an argument glued to a list", payload: "stub k=[a]b"},
			{name: "returns an error for a name with a non-identifier byte", payload: "stüb"},
			{name: "returns an error for a name with a second prefix", payload: "mockgen:stub:x"},
			{name: "returns an error for whitespace after a list opens", payload: "index fields=[ a]"},
			{name: "returns an error for whitespace before a list closes", payload: "index fields=[a ]"},
			{name: "returns an error for a list of whitespace alone", payload: "index fields=[ ]"},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := directive.Parse(tt.payload)
				assert.HasError(t, err, "the payload does not parse")
				assert.HasPrefix(t, err.Error(), "directive: ", "the error has the package prefix")
			})
		}

		t.Run("returns an error naming the offset where reading stopped", func(t *testing.T) {
			t.Parallel()

			_, err := directive.Parse("stub tag=")
			assert.HasError(t, err, "the empty value does not parse")
			assert.Contains(t, err.Error(), "9", "the error names the byte offset")
		})

		t.Run("returns an error naming an unknown escape by its character", func(t *testing.T) {
			t.Parallel()

			_, err := directive.Parse(`doc text="\é"`)
			assert.HasError(t, err, "the escape does not parse")
			assert.Contains(t, err.Error(), `\é`, "the error names the escape the author wrote")
		})

		t.Run("returns an error or an instance with positioned identifier keys for any payload", func(t *testing.T) {
			t.Parallel()

			prop.ForAll(t, parsesSafely, parsesPositioned)
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
				name:  "returns a single line unchanged",
				lines: []string{"stub tag=test"},
				want:  "stub tag=test",
			},
			{
				name:  "joins a continuation with a single space",
				lines: []string{`stub \`, "tag=test"},
				want:  "stub tag=test",
			},
			{
				name:  "joins three lines in order",
				lines: []string{`index \`, `fields=[a, \`, "b]"},
				want:  "index fields=[a, b]",
			},
			{
				name:  "strips a trailing backslash on the last line",
				lines: []string{`stub \`},
				want:  "stub",
			},
			{
				name:  "keeps a backslash inside a line",
				lines: []string{`doc text="a\nb"`},
				want:  `doc text="a\nb"`,
			},
			{
				name:  "folds a continued line's leading blanks into the one space",
				lines: []string{`doc text="hello \`, "  \tworld\""},
				want:  `doc text="hello world"`,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, directive.Join(tt.lines), tt.want, "the joined payload is pinned")
			})
		}
	})
}

// FuzzParse checks [parsesSafely] on payloads nothing in this
// repository wrote: payloads arrive from consumers' source files. Each
// seed is the choices of one case: a two-byte little-endian length,
// then the bytes of the payload.
func FuzzParse(f *testing.F) {
	for _, seed := range []string{
		"skip",
		"mockgen:stub handler tag=test",
		`doc text="a \"b\" c\n"`,
		"index fields=[a, b,c] unique",
		"  x  ", "", ":", "=", `k="`, "a=[", "meta drop=shape.role",
	} {
		f.Add(append(binary.LittleEndian.AppendUint16(nil, uint16(len(seed))), seed...))
	}

	prop.Fuzz(f, parsesSafely, parsesPositioned)
}

// parsesPositioned checks [parsesSafely] on a payload that the case
// draws as bytes.
func parsesPositioned(c *prop.Case) {
	payload := string(c.Draw(prop.Bytes(), "payload"))
	got, err := directive.Parse(payload)
	if err != nil {
		assert.HasPrefix(c, err.Error(), "directive: ", "every error must have the package prefix")
		return
	}
	assert.NotEqual(c, string(got.Name), "", "a parsed instance must name a directive")
	for _, arg := range got.Args {
		assert.InRange(c, arg.Col, 1, float64(len(payload)),
			"every argument's offset must point inside the payload")
		assert.Matches(c, string(arg.Key), "^[^ \t=\"\\[\\],]*$",
			"a key must be an identifier, never grammar punctuation")
	}
}

// A name's plugin reads without allocating, Parse allocates the
// arguments and each list once, and the joined payload of continued
// lines is one allocation. Each count keeps the first error of its
// calls, which cmp.Or returns without allocating. The check runs alone,
// because the count includes every goroutine's allocations.
func TestGrammarAllocs(t *testing.T) {
	name := directive.Name("mockgen:stub")
	var plugin string
	assert.MaxAllocs(t, func() { plugin = name.Plugin() }, 0, "Plugin allocates nothing")
	assert.Equal(t, plugin, "mockgen", "Plugin returns the prefix")
	var err error
	for _, tt := range []struct {
		payload string
		allocs  uint64
		msg     string
	}{
		{payload: fullPayload, allocs: 2, msg: "Parse allocates the arguments and the list's elements"},
		{payload: barePayload, allocs: 0, msg: "Parse allocates nothing for a bare name"},
		{
			payload: `doc text="plain words"`, allocs: 1,
			msg: "Parse allocates the arguments and no quoted value without an escape",
		},
	} {
		assert.MaxAllocs(t, func() {
			_, perr := directive.Parse(tt.payload)
			err = cmp.Or(err, perr)
		}, tt.allocs, tt.msg)
		assert.NoError(t, err, "the payload parses: "+tt.payload)
	}
	var joined string
	assert.MaxAllocs(t, func() { joined = directive.Join(continued) }, 1, "Join allocates the joined payload")
	assert.NotEmpty(t, joined, "Join returns the joined payload")
	assert.MaxAllocs(t, func() { joined = directive.Join(continued[2:]) }, 0,
		"Join allocates nothing for one line")
	assert.NotEmpty(t, joined, "Join returns the one line")
}

// BenchmarkGrammar measures a name's plugin, parsing, which runs once
// per carrier line across every loaded file, and the join of continued
// lines.
func BenchmarkGrammar(b *testing.B) {
	b.Run("Name.Plugin", func(b *testing.B) {
		name := directive.Name("mockgen:stub")
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = name.Plugin()
		}
		assert.Equal(b, got, "mockgen", "Plugin returns the prefix")
	})

	b.Run("Parse", func(b *testing.B) {
		b.Run("five arguments with a list", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(2)
			defer c.End()
			var (
				got directive.Raw
				err error
			)
			for c.Loop() {
				got, err = directive.Parse(fullPayload)
			}
			assert.NoError(b, err, "the payload parses")
			assert.Length(b, got.Args, 5, "with its five arguments")
		})

		b.Run("a bare name", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var (
				got directive.Raw
				err error
			)
			for c.Loop() {
				got, err = directive.Parse(barePayload)
			}
			assert.NoError(b, err, "the payload parses")
			assert.Equal(b, got.Name, directive.Name(barePayload), "to the bare name")
		})
	})

	b.Run("Join", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		var got string
		for c.Loop() {
			got = directive.Join(continued)
		}
		assert.Equal(b, got, "index fields=[a, b, c] depth=3", "Join folds the three lines")
	})
}

// scalar returns a bare scalar value.
func scalar(text string) directive.RawValue {
	return directive.RawValue{Text: text}
}

// quoted returns a quoted scalar value.
func quoted(text string) directive.RawValue {
	return directive.RawValue{Text: text, Quoted: true}
}
