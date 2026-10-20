// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/rust"
	rustrules "go.dokimi.dev/eidos/lang/rust/rules"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The literals that the allocation test and the benchmark scan: a decimal
// integer, a float, a string without an escape, a truth value and a path.
const (
	decimalText = "42"
	floatText   = "1.5"
	stringText  = `"abc"`
	boolText    = "true"
	pathText    = "Color::Red"
)

// The ceilings of the scan of one literal through LiteralFor without a type:
// an integer below 100, a string without an escape, a truth value and a path
// allocate nothing, and a float allocates its decimal text.
const (
	scanAllocs  = 0
	floatAllocs = 1
)

// minusSignText is a minus sign without a number.
const minusSignText = "-"

// TestLiteral pins the grammar of the literals of Rust that the rules read,
// through the literal of its own kind that LiteralFor returns for a value
// without a type.
func TestLiteral(t *testing.T) {
	t.Parallel()

	t.Run("scanLiteral", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want emit.Value
		}{
			{name: "reads a decimal integer", give: decimalText, want: emit.Number(emit.LiteralInt, decimalText, 0)},
			{name: "reads a negative integer", give: "-7", want: emit.Number(emit.LiteralInt, "-7", 0)},
			{name: "reads a sign apart from its integer", give: "- 7", want: emit.Number(emit.LiteralInt, "-7", 0)},
			{name: "reads a negative zero as zero", give: "-0", want: emit.Number(emit.LiteralInt, "0", 0)},
			{name: "reads a hexadecimal integer", give: "0x1F", want: emit.Number(emit.LiteralInt, "31", 0)},
			{name: "reads an octal integer", give: "0o17", want: emit.Number(emit.LiteralInt, "15", 0)},
			{name: "reads a binary integer", give: "0b101", want: emit.Number(emit.LiteralInt, "5", 0)},
			{name: "reads an integer with underscores", give: "1_000", want: emit.Number(emit.LiteralInt, "1000", 0)},
			{
				name: "reads a decimal integer with leading zeros",
				give: "007",
				want: emit.Number(emit.LiteralInt, "7", 0),
			},
			{
				name: "reads an integer with a type suffix",
				give: "42u8",
				want: emit.Number(emit.LiteralInt, decimalText, 0),
			},
			{
				name: "reads a hexadecimal integer with a type suffix",
				give: "0xffu8",
				want: emit.Number(emit.LiteralInt, "255", 0),
			},
			{
				name: "reads an integer wider than 64 bits", give: "340282366920938463463374607431768211455",
				want: emit.Number(emit.LiteralInt, "340282366920938463463374607431768211455", 0),
			},
			{name: "reads a literal between blanks", give: " 42 ", want: emit.Number(emit.LiteralInt, decimalText, 0)},
			{name: "reads a float", give: floatText, want: emit.Number(emit.LiteralFloat, floatText, 0)},
			{name: "reads a negative float", give: "-2.5", want: emit.Number(emit.LiteralFloat, "-2.5", 0)},
			{name: "reads a float with an exponent", give: "2.5e3", want: emit.Number(emit.LiteralFloat, "2500", 0)},
			{
				name: "reads a float with a negative exponent",
				give: "1E-7",
				want: emit.Number(emit.LiteralFloat, "1e-7", 0),
			},
			{name: "reads a float without a fraction", give: "1.", want: emit.Number(emit.LiteralFloat, "1", 0)},
			{name: "reads a float with a type suffix", give: "1.0f32", want: emit.Number(emit.LiteralFloat, "1", 0)},
			{
				name: "reads an integer with a float suffix as a float",
				give: "5f64",
				want: emit.Number(emit.LiteralFloat, "5", 0),
			},
			{name: "reads true", give: boolText, want: emit.Literal(emit.LiteralBool, boolText)},
			{name: "reads false", give: "false", want: emit.Literal(emit.LiteralBool, "false")},
			{name: "reads a character as Rust text", give: "'a'", want: emit.Raw(rust.Lang, "'a'")},
			{name: "reads a character of two bytes", give: "'é'", want: emit.Raw(rust.Lang, "'é'")},
			{name: "reads a character escape", give: `'\n'`, want: emit.Raw(rust.Lang, `'\n'`)},
			{name: "reads a quote inside a character", give: `'"'`, want: emit.Raw(rust.Lang, `'"'`)},
			{
				name: "reads a Unicode escape of a character",
				give: `'\u{1F600}'`,
				want: emit.Raw(rust.Lang, `'\u{1F600}'`),
			},
			{name: "reads a string", give: stringText, want: emit.Literal(emit.LiteralString, "abc")},
			{name: "reads a string with an escape", give: `"a\tb\\"`, want: emit.Literal(emit.LiteralString, "a\tb\\")},
			{name: "reads a string with a quote escape", give: `"a\"b"`, want: emit.Literal(emit.LiteralString, `a"b`)},
			{name: "reads a string with an ASCII escape", give: `"\x41"`, want: emit.Literal(emit.LiteralString, "A")},
			{
				name: "reads a string with a Unicode escape",
				give: `"\u{48}i"`,
				want: emit.Literal(emit.LiteralString, "Hi"),
			},
			{
				name: "reads a string with an underscore in a Unicode escape", give: `"\u{1_F600}"`,
				want: emit.Literal(emit.LiteralString, "\U0001F600"),
			},
			{
				name: "reads a string with a continued line", give: "\"a\\\n    b\"",
				want: emit.Literal(emit.LiteralString, "ab"),
			},
			{name: "reads a raw string", give: `r"\d"`, want: emit.Literal(emit.LiteralString, `\d`)},
			{name: "reads a raw string with hashes", give: `r#"a"b"#`, want: emit.Literal(emit.LiteralString, `a"b`)},
			{name: "reads a byte string as Rust text", give: `b"ab"`, want: emit.Raw(rust.Lang, `b"ab"`)},
			{name: "reads a byte string with an escape", give: `b"\xff\""`, want: emit.Raw(rust.Lang, `b"\xff\""`)},
			{name: "reads a raw byte string as Rust text", give: `br"\d"`, want: emit.Raw(rust.Lang, `br"\d"`)},
			{
				name: "reads the unit value",
				give: "()",
				want: emit.Composite(&emit.TypeRef{Spelling: "()", Form: symbol.FormTuple}),
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, read := rustrules.Rules{}.LiteralFor(rules.Scope{}, nil, tt.give, rules.View{})
				assert.True(t, read, "the text is a literal")
				assert.Equal(t, got, tt.want, "the literal of its own kind")
			})
		}

		refusals := []struct {
			name string
			give string
		}{
			{name: "refuses a path, which only a type resolves", give: pathText},
			{name: "refuses Some, which only a type resolves", give: "Some(1)"},
			{name: "refuses Some without its closing parenthesis", give: "Some(1"},
			{name: "refuses a sign before a truth value", give: "-true"},
			{name: "refuses a sign before a name", give: "-x"},
			{name: "refuses a sign alone", give: minusSignText},
			{name: "refuses a float whose point an exponent follows", give: "1.e5"},
			{name: "refuses a float without a leading digit", give: ".5"},
			{name: "refuses a float with an exponent without digits", give: "1e"},
			{name: "refuses a float with two points", give: "1.2.3"},
			{name: "refuses a prefix without digits", give: "0x"},
			{name: "refuses a hexadecimal prefix with no hexadecimal digits", give: "0xZZ"},
			{name: "refuses a binary literal with a digit of another base", give: "0b102"},
			{name: "refuses a sign after a prefix", give: "0x-1"},
			{name: "refuses an integer wider than 64 bits with a letter", give: "99999999999999999999999z"},
			{name: "refuses a float beyond the range of a double", give: "1e400"},
			{name: "refuses a string with a quote inside", give: `"a"b"`},
			{name: "refuses a string with an escape and a quote inside", give: `"a\tb"c"`},
			{name: "refuses a raw string without its quote", give: `r#x`},
			{name: "refuses a string without its closing quote", give: `"abc`},
			{name: "refuses an unknown escape", give: `"\q"`},
			{name: "refuses an escape at the end of a string", give: `"a\"`},
			{name: "refuses an ASCII escape above the ASCII range", give: `"\x80"`},
			{name: "refuses an ASCII escape without its digits", give: `"\x4"`},
			{name: "refuses a Unicode escape of a surrogate", give: `"\u{D800}"`},
			{name: "refuses a Unicode escape beyond the range of Unicode", give: `"\u{110000}"`},
			{name: "refuses a Unicode escape without digits", give: `"\u{}"`},
			{name: "refuses a Unicode escape of seven digits", give: `"\u{1000000}"`},
			{name: "refuses a Unicode escape without braces", give: `"\u0041"`},
			{name: "refuses a character literal of two characters", give: "'ab'"},
			{name: "refuses an empty character literal", give: "''"},
			{name: "refuses a quote as a character", give: "'''"},
			{name: "refuses a character literal without its closing quote", give: "'a"},
			{name: "refuses a character escape of two characters", give: `'\u{41}b'`},
			{name: "refuses a raw string with an early end", give: `r#"a"#"#`},
			{name: "refuses a raw string with a quote and no hashes", give: `r"a"b"`},
			{name: "refuses a raw string without its closing quote", give: `r"a`},
			{name: "refuses a raw string without its hashes", give: `r#"a"`},
			{name: "refuses a byte string beyond ASCII", give: "b\"é\""},
			{name: "refuses a byte string without its closing quote", give: `b"a`},
			{name: "refuses a byte string with an escaped closing quote", give: `b"a\"`},
			{name: "refuses a byte string with a quote inside", give: `b"a"b"`},
			{name: "refuses a raw byte string without its closing quote", give: `br"a`},
			{name: "refuses an expression", give: "1 + 2"},
			{name: "refuses a path with an empty segment", give: "a::::b"},
			{name: "refuses the empty text", give: ""},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, read := rustrules.Rules{}.LiteralFor(rules.Scope{}, nil, tt.give, rules.View{})
				assert.False(t, read, "the text is no literal of a value")
			})
		}
	})
}

// TestLiteralAllocs checks the ceilings of the scan of a literal in the
// ordinary run, which does not run benchmarks.
func TestLiteralAllocs(t *testing.T) {
	checkAllocs(t, literalCalls())
}

// BenchmarkLiteral measures the scan of each kind of literal under its
// ceiling.
func BenchmarkLiteral(b *testing.B) {
	benchCalls(b, literalCalls())
}

// literalCalls returns a scan of each kind of literal through LiteralFor
// without a type.
func literalCalls() []allocCall {
	r := rustrules.Rules{}
	tests := []struct {
		caseName string
		give     string
		allocs   uint64
		want     bool
	}{
		{caseName: "a decimal integer", give: decimalText, allocs: scanAllocs, want: true},
		{caseName: "a float", give: floatText, allocs: floatAllocs, want: true},
		{caseName: "a string", give: stringText, allocs: scanAllocs, want: true},
		{caseName: "a truth value", give: boolText, allocs: scanAllocs, want: true},
		{caseName: "a path", give: pathText, allocs: scanAllocs},
	}
	calls := make([]allocCall, 0, len(tests))
	for _, tt := range tests {
		var read bool
		calls = append(calls, allocCall{
			name: "scanLiteral", caseName: tt.caseName, allocs: tt.allocs,
			call: func() { _, read = r.LiteralFor(rules.Scope{}, nil, tt.give, rules.View{}) },
			check: func(tb assert.TB) {
				assert.Equal(tb, read, tt.want, "LiteralFor reads "+tt.caseName+" as a literal of a value")
			},
		})
	}
	return calls
}
