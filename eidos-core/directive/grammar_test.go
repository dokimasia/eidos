// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package directive_test

import (
	"cmp"
	"encoding/binary"
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"
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

// The mark that opens a directive in the cases of Cut, as the mark of a
// carrier opens one. The block also has the payload that the allocation
// checks and the benchmarks cut, and the offset of the argument depth in
// the payload that Join folds from continued.
const (
	mark          = "+gen:"
	markedPayload = "index fields=[a, b, c] depth=3 " + mark + "mock"
	locatedOffset = len("index fields=[a, b, c] ")
)

// The properties of Cut and Locate that [TestGrammar] states.
const (
	cutsAtMarks = "Cut must cut a payload before its first positional argument that opens with the mark, " +
		"and the part before the cut must parse to the arguments before that argument"
	locatesBytes = "Locate must map each byte of a joined payload that is not a blank to the same byte of its line"
)

// opensMark reports whether the rest of a payload opens with the mark, as
// the owner of a carrier tests the marks of its brand.
var opensMark = func(rest string) bool { return strings.HasPrefix(rest, mark) }

// The arguments that the payloads of the Cut property draw, and the lines
// that the Locate property draws. The arguments have a plain value, keyed
// values, a mark, and marks inside a quoted value, a list and a keyed
// value. Some lines continue, and some open with blanks.
var (
	cutArguments = []string{
		"plain", "k=v", `k="a b"`, `k="x \" +gen:y"`, "k=[a, +gen:b]", "[c, d]", mark + "mock",
		`"+gen:quoted"`, "k=+gen:v",
	}
	locateLines = []string{`stub \`, `  tag=test \`, "\tx=1", "a b", `c] \`, `\`, "  \\", "end"}
)

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

	t.Run("Bare", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give directive.Name
			want directive.Name
		}{
			{name: "returns the part after the prefix of a prefixed name", give: "mockgen:stub", want: "stub"},
			{name: "returns a bare plugin name whole", give: "stub", want: "stub"},
			{name: "returns a kernel name whole", give: directive.KernelSkip, want: directive.KernelSkip},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.Bare(), tt.want, "the bare part is the part after the colon")
			})
		}
	})

	t.Run("Valid", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give directive.Name
			want bool
		}{
			{name: "reports true for a bare name", give: "stub", want: true},
			{name: "reports true for a prefixed name", give: "mockgen:stub", want: true},
			{name: "reports true for a name with hyphens", give: "k8s:deepcopy-gen", want: true},
			{name: "reports false for the empty name", give: "", want: false},
			{name: "reports false for a name with a dollar sign", give: "$stub", want: false},
			{name: "reports false for an empty plugin", give: ":stub", want: false},
			{name: "reports false for an empty name after the plugin", give: "mockgen:", want: false},
			{name: "reports false for a name with two colons", give: "a:b:c", want: false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.Valid(), tt.want, "the verdict follows the grammar's identifier")
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

	t.Run("Cut", func(t *testing.T) {
		t.Parallel()

		cuts := []struct {
			name       string
			give       string
			wantBefore string
			wantAfter  string
		}{
			{
				name:       "cuts before the first argument that opens a directive",
				give:       "stub tag=test +gen:mock",
				wantBefore: "stub tag=test", wantAfter: "+gen:mock",
			},
			{
				name:       "cuts before the first of two arguments that open a directive",
				give:       "stub +gen:mock +gen:audit",
				wantBefore: "stub", wantAfter: "+gen:mock +gen:audit",
			},
			{
				name:       "drops the blanks before the argument",
				give:       "stub \t +gen:mock",
				wantBefore: "stub", wantAfter: "+gen:mock",
			},
			{
				name:       "cuts before an argument after a list",
				give:       "index fields=[a, b] +gen:mock",
				wantBefore: "index fields=[a, b]", wantAfter: "+gen:mock",
			},
			{
				name:       "cuts before an argument after a quoted value",
				give:       `doc text="a b" +gen:mock`,
				wantBefore: `doc text="a b"`, wantAfter: "+gen:mock",
			},
			{
				name:       "cuts before an argument after a stray closing bracket",
				give:       "stub ] +gen:mock",
				wantBefore: "stub ]", wantAfter: "+gen:mock",
			},
		}
		for _, tt := range cuts {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				before, after, found := directive.Cut(tt.give, opensMark)
				assert.True(t, found, "the payload has an argument that opens a directive")
				expect.Equal(t, before, tt.wantBefore, "the part before the cut ends before the blanks")
				expect.Equal(t, after, tt.wantAfter, "the part after the cut opens with the argument")
			})
		}

		wholes := []struct {
			name string
			give string
		}{
			{name: "reports false for a mark inside a quoted value", give: `doc text="a +gen:mock"`},
			{name: "reports false for a mark after an escaped quote", give: `doc text="a \" +gen:mock"`},
			{name: "reports false for a mark inside a list", give: "index fields=[a, +gen:mock]"},
			{name: "reports false for a keyed value that opens with a mark", give: "stub tag=+gen:mock"},
			{name: "reports false for a mark inside an unterminated quoted value", give: `doc text="a +gen:mock`},
			{name: "reports false for a mark that opens the name", give: "+gen:mock"},
			{name: "reports false for a mark that opens the name after blanks", give: "  +gen:mock"},
			{name: "reports false for a payload without a mark", give: "stub tag=test"},
			{name: "reports false for a payload with trailing blanks", give: "stub tag=test  "},
		}
		for _, tt := range wholes {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				before, after, found := directive.Cut(tt.give, opensMark)
				assert.False(t, found, "the payload has no argument that opens a directive")
				expect.Equal(t, before, tt.give, "the part before the cut is the payload")
				expect.Empty(t, after, "the part after the cut is empty")
			})
		}

		t.Run("cuts each drawn payload before its first positional mark", func(t *testing.T) {
			t.Parallel()

			prop.ForAll(t, cutsAtMarks, cutsBeforeMarks)
		})
	})

	t.Run("Locate", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			lines    []string
			offset   int
			wantLine int
			wantAt   int
		}{
			{
				name:  "returns the offset in a payload of one line",
				lines: []string{"stub tag=test"}, offset: 5, wantLine: 0, wantAt: 5,
			},
			{
				name:  "returns the line and the offset of a byte of a continuation line",
				lines: []string{`stub \`, "tag=test"}, offset: 5, wantLine: 1, wantAt: 0,
			},
			{
				name:  "counts the blanks that the join drops from a continuation line",
				lines: []string{`stub \`, " \ttag=test"}, offset: 5, wantLine: 1, wantAt: 2,
			},
			{
				name:  "returns a byte of the third of three lines",
				lines: continued, offset: locatedOffset, wantLine: 2, wantAt: len("c] "),
			},
			{
				name:  "returns the end of the earlier line for the space between two lines",
				lines: []string{`stub \`, "tag=test"}, offset: 4, wantLine: 0, wantAt: 4,
			},
			{
				name:  "returns the end of the last line for an offset past the payload",
				lines: []string{"stub"}, offset: 9, wantLine: 0, wantAt: 4,
			},
			{
				name:  "returns the first line for no lines",
				lines: nil, offset: 3, wantLine: 0, wantAt: 0,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				line, at := directive.Locate(tt.lines, tt.offset)
				expect.Equal(t, line, tt.wantLine, "the line contains the byte")
				expect.Equal(t, at, tt.wantAt, "the offset is the byte's in its line")
			})
		}

		t.Run("maps each byte of drawn lines that is not a blank to its line", func(t *testing.T) {
			t.Parallel()

			prop.ForAll(t, locatesBytes, locatesEachByte)
		})
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
		assert.Matches(c, arg.Key, "^[^ \t=\"\\[\\],]*$",
			"a key must be an identifier, never grammar punctuation")
	}
}

// cutsBeforeMarks checks [cutsAtMarks] on a payload of drawn arguments.
// The parse of the whole payload states where each argument starts, so the
// check compares the cut with the first positional argument that opens
// with the mark.
func cutsBeforeMarks(c *prop.Case) {
	args := c.Draw(prop.List(prop.SampledFrom(cutArguments...), prop.MaxSize(6)), "arguments")
	payload := strings.Join(append([]string{"stub"}, args...), " ")
	whole, err := directive.Parse(payload)
	assert.NoError(c, err, "a payload of the drawn arguments must parse")
	before, after, found := directive.Cut(payload, opensMark)
	at := slices.IndexFunc(whole.Args, func(arg directive.RawArg) bool {
		return arg.Key == "" && !arg.Value.Quoted && strings.HasPrefix(arg.Value.Text, mark)
	})
	if at < 0 {
		assert.False(c, found, "Cut must not cut a payload without a positional mark")
		assert.Equal(c, before, payload, "Cut must return the whole payload without a positional mark")
		return
	}
	assert.True(c, found, "Cut must cut a payload with a positional mark")
	assert.Equal(c, len(payload)-len(after), whole.Args[at].Col, "Cut must cut at the first positional mark")
	head, err := directive.Parse(before)
	assert.NoError(c, err, "the part before the cut must parse")
	assert.Equal(c, head.Args, whole.Args[:at], "the part before the cut must have the arguments before the mark",
		assert.EquateEmpty())
}

// locatesEachByte checks [locatesBytes] on drawn lines. A blank of the
// joined payload can be the space between two lines, so the check reads
// every other byte.
func locatesEachByte(c *prop.Case) {
	lines := c.Draw(prop.List(prop.SampledFrom(locateLines...), prop.MaxSize(6)), "lines")
	joined := directive.Join(lines)
	for offset := range len(joined) {
		if joined[offset] == ' ' || joined[offset] == '\t' {
			continue
		}
		line, at := directive.Locate(lines, offset)
		assert.Equal(c, lines[line][at], joined[offset], "the byte of the joined payload must be the byte of its line")
	}
}

// A name's plugin reads without allocating, Parse allocates the
// arguments and each list once, and the joined payload of continued
// lines is one allocation. Cut and Locate do not allocate. Each count
// keeps the first error of its calls, which cmp.Or returns without
// allocating. The check runs alone, because the count includes every
// goroutine's allocations.
func TestGrammarAllocs(t *testing.T) {
	name := directive.Name("mockgen:stub")
	var plugin string
	assert.MaxAllocs(t, func() { plugin = name.Plugin() }, 0, "Plugin allocates nothing")
	assert.Equal(t, plugin, "mockgen", "Plugin returns the prefix")
	var bare directive.Name
	assert.MaxAllocs(t, func() { bare = name.Bare() }, 0, "Bare allocates nothing")
	assert.Equal(t, bare, directive.Name("stub"), "Bare returns the part after the prefix")
	var valid bool
	assert.MaxAllocs(t, func() { valid = name.Valid() }, 0, "Valid allocates nothing")
	assert.True(t, valid, "Valid accepts a prefixed name")
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
	var found bool
	assert.MaxAllocs(t, func() { _, _, found = directive.Cut(markedPayload, opensMark) }, 0, "Cut allocates nothing")
	assert.True(t, found, "Cut finds the mark")
	var line int
	assert.MaxAllocs(t, func() { line, _ = directive.Locate(continued, locatedOffset) }, 0, "Locate allocates nothing")
	assert.Equal(t, line, 2, "Locate returns the third line")
}

// BenchmarkGrammar measures a name's plugin, parsing, which runs once
// per carrier across every loaded file, and the join of continued lines.
// It also measures the cut of a payload before a mark, which runs once
// per carrier line, and the location of a byte of a joined payload.
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

	b.Run("Name.Bare", func(b *testing.B) {
		name := directive.Name("mockgen:stub")
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got directive.Name
		for c.Loop() {
			got = name.Bare()
		}
		assert.Equal(b, got, directive.Name("stub"), "Bare returns the part after the prefix")
	})

	b.Run("Name.Valid", func(b *testing.B) {
		name := directive.Name("mockgen:stub")
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got bool
		for c.Loop() {
			got = name.Valid()
		}
		assert.True(b, got, "Valid accepts a prefixed name")
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

	b.Run("Cut", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			_, got, _ = directive.Cut(markedPayload, opensMark)
		}
		assert.Equal(b, got, mark+"mock", "Cut cuts before the mark")
	})

	b.Run("Locate", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got int
		for c.Loop() {
			_, got = directive.Locate(continued, locatedOffset)
		}
		assert.Equal(b, got, len("c] "), "Locate returns the offset of depth in its line")
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
