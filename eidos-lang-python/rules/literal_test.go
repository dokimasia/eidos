// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/python"
	pythonrules "go.dokimi.dev/eidos/lang/python/rules"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/rules"
)

// The literals that the allocation test and the benchmark scan: a decimal
// integer, a negative integer, a float, a string without an escape and a
// name.
const (
	decimalText  = "42"
	negativeText = "-7"
	floatText    = "1.5"
	stringText   = `"abc"`
	nameText     = "Color.RED"
)

// The ceilings of the scan of one literal: a decimal integer, a string
// without an escape and a name allocate nothing, and a negative integer and
// a float allocate their decimal text.
const (
	scanAllocs = 0
	textAllocs = 1
)

// TestLiteral pins the grammar of the literals of Python that the rules
// read, through the literal of its own kind that LiteralFor returns for a
// value without a type.
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
			{name: "reads a negative integer", give: negativeText, want: emit.Number(emit.LiteralInt, negativeText, 0)},
			{name: "reads a sign apart from its integer", give: "- 7", want: emit.Number(emit.LiteralInt, "-7", 0)},
			{name: "reads a plus sign", give: "+5", want: emit.Number(emit.LiteralInt, "5", 0)},
			{name: "reads a negative zero as zero", give: "-0", want: emit.Number(emit.LiteralInt, "0", 0)},
			{name: "reads a hexadecimal integer", give: "0x1F", want: emit.Number(emit.LiteralInt, "31", 0)},
			{name: "reads a literal between blanks", give: " 42 ", want: emit.Number(emit.LiteralInt, decimalText, 0)},
			{name: "reads a float", give: floatText, want: emit.Number(emit.LiteralFloat, floatText, 64)},
			{name: "reads a negative float", give: "-2.5", want: emit.Number(emit.LiteralFloat, "-2.5", 64)},
			{name: "reads a string", give: stringText, want: emit.Literal(emit.LiteralString, "abc")},
			{name: "reads a string with an escape", give: `'a\tb'`, want: emit.Literal(emit.LiteralString, "a\tb")},
			{name: "reads a raw string", give: `r"\d"`, want: emit.Literal(emit.LiteralString, `\d`)},
			{name: "reads True", give: "True", want: emit.Literal(emit.LiteralBool, "true")},
			{name: "reads False", give: "False", want: emit.Literal(emit.LiteralBool, "false")},
			{name: "reads None", give: "None", want: emit.Literal(emit.LiteralNil, "")},
			{name: "reads bytes as Python text", give: `b"a"`, want: emit.Raw(python.Lang, `b"a"`)},
			{name: "reads raw bytes as Python text", give: `Rb'\d'`, want: emit.Raw(python.Lang, `Rb'\d'`)},
			{name: "reads raw bytes with the bytes letter first", give: `bR"x"`, want: emit.Raw(python.Lang, `bR"x"`)},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, read := pythonrules.Rules{}.LiteralFor(rules.Scope{}, nil, tt.give, rules.View{})
				assert.True(t, read, "the text is a literal")
				assert.Equal(t, got, tt.want, "the literal of its own kind")
			})
		}

		refusals := []struct {
			name string
			give string
		}{
			{name: "refuses a name, which only a type resolves", give: nameText},
			{name: "refuses a sign before a truth value", give: "-True"},
			{name: "refuses a sign before a name", give: "-x"},
			{name: "refuses an f-string", give: `f"x"`},
			{name: "refuses bytes beyond ASCII", give: "b\"é\""},
			{name: "refuses bytes with two bytes letters", give: `bb"x"`},
			{name: "refuses bytes with a long prefix", give: `bRr"x"`},
			{name: "refuses bytes with the letter u", give: `bu"x"`},
			{name: "refuses bytes without a closing quote", give: `b"x`},
			{name: "refuses an expression", give: "1 + 2"},
			{name: "refuses a dotted name with an empty name", give: "a..b"},
			{name: "refuses the empty text", give: ""},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, read := pythonrules.Rules{}.LiteralFor(rules.Scope{}, nil, tt.give, rules.View{})
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
	r := pythonrules.Rules{}
	tests := []struct {
		caseName string
		give     string
		allocs   uint64
		want     bool
	}{
		{caseName: "a decimal integer", give: decimalText, allocs: scanAllocs, want: true},
		{caseName: "a negative integer", give: negativeText, allocs: textAllocs, want: true},
		{caseName: "a float", give: floatText, allocs: textAllocs, want: true},
		{caseName: "a string", give: stringText, allocs: scanAllocs, want: true},
		{caseName: "a name", give: nameText, allocs: scanAllocs},
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
