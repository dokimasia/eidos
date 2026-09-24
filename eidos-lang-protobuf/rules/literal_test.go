// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	protorules "go.dokimi.dev/eidos/lang/protobuf/rules"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/rules"
)

// untyped lifts a literal against no type, so each lifts as its own
// kind.
func untyped(text string) (emit.Value, bool) {
	return protorules.New().LiteralFor(nil, nil, text, rules.View{})
}

// A literal reads in protoc's grammar, not Go's, so every form
// protobuf writes and every form only Go writes are pinned.
func TestLiteral(t *testing.T) {
	t.Parallel()

	t.Run("LiteralFor", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the truth values", func(t *testing.T) {
			t.Parallel()

			v, ok := untyped(" true ")
			assert.True(t, ok, "true is a literal, whitespace trimmed")
			assert.Equal(t, v, emit.Literal(emit.LiteralBool, "true"), "a truth value")
			v, _ = untyped("false")
			assert.Equal(t, v.Text, "false", "and so is false")
		})

		t.Run("reads a string in protoc's escape grammar", func(t *testing.T) {
			t.Parallel()

			for _, tc := range []struct {
				text, want, why string
			}{
				{`"hi"`, "hi", "a double-quoted string"},
				{`'hi'`, "hi", "a single-quoted one, which protobuf admits"},
				{`"a" 'b'`, "ab", "adjacent strings concatenated"},
				{`"\a\b\f\n\r\t\v\\\'\"\?"`, "\a\b\f\n\r\t\v\\'\"?", "every named escape, the question mark included"},
				{`"\x41\X42\x7"`, "AB\x07", "one or two hexadecimal digits"},
				{`"\101\0"`, "A\x00", "one to three octal digits"},
				{`"é\U0001F600"`, "é😀", "four and eight hexadecimal digits of a code point"},
			} {
				v, ok := untyped(tc.text)
				assert.True(t, ok, tc.why+" reads")
				assert.Equal(t, v, emit.Literal(emit.LiteralString, tc.want), tc.why)
			}
		})

		t.Run("refuses a string outside the grammar", func(t *testing.T) {
			t.Parallel()

			for _, tc := range []struct {
				text, why string
			}{
				{`"\q"`, "an escape protoc does not define"},
				{`"\x"`, "a hexadecimal escape with no digit"},
				{`"\400"`, "an octal escape above one byte"},
				{`"\u12"`, "a short code point"},
				{`"\U00110000"`, "a code point past Unicode"},
				{`"open`, "an unterminated string"},
				{`"a"b`, "text after the closing quote"},
				{"\"line\nbreak\"", "a string spanning two lines"},
			} {
				_, ok := untyped(tc.text)
				assert.False(t, ok, tc.why+" is no literal")
			}
		})

		t.Run("reads a number in every base protobuf writes", func(t *testing.T) {
			t.Parallel()

			for _, tc := range []struct {
				text string
				want emit.Value
				why  string
			}{
				{"42", emit.Literal(emit.LiteralInt, "42"), "a decimal integer"},
				{"-7", emit.Literal(emit.LiteralInt, "-7"), "a negative one"},
				{"- 7", emit.Literal(emit.LiteralInt, "-7"), "with the sign apart, which the grammar admits"},
				{"0x10", emit.Literal(emit.LiteralInt, "16"), "a hexadecimal integer, written back in decimal"},
				{"017", emit.Literal(emit.LiteralInt, "15"), "an octal integer"},
				{"2.5", emit.Number(emit.LiteralFloat, "2.5", 0), "a float"},
				{".5", emit.Number(emit.LiteralFloat, "0.5", 0), "a float opening with its point"},
				{"5.", emit.Number(emit.LiteralFloat, "5", 0), "a float closing with its point"},
				{"1e21", emit.Number(emit.LiteralFloat, "1e+21", 0), "an exponent in canonical text"},
			} {
				v, ok := untyped(tc.text)
				assert.True(t, ok, tc.why+" reads")
				assert.Equal(t, v, tc.want, tc.why)
			}
		})

		t.Run("refuses a number only Go writes", func(t *testing.T) {
			t.Parallel()

			for _, tc := range []struct {
				text, why string
			}{
				{"1_000", "a digit separator"},
				{"0b101", "a binary integer"},
				{"0o7", "an octal integer with Go's prefix"},
				{"08", "an octal integer with a decimal digit"},
				{"0x", "a hexadecimal prefix alone"},
				{"-", "a sign alone"},
				{"--1", "two signs"},
			} {
				_, ok := untyped(tc.text)
				assert.False(t, ok, tc.why+" is no literal")
			}
		})

		t.Run("reads an identifier only against an enum", func(t *testing.T) {
			t.Parallel()

			for _, text := range []string{"COLOUR_RED", "_x1", "inf", "nan"} {
				_, ok := untyped(text)
				assert.False(t, ok, text+" has no kind of its own")
			}
			for _, text := range []string{"", "1bad", "{ }"} {
				_, ok := untyped(text)
				assert.False(t, ok, "and "+text+" is no literal at all")
			}
		})
	})
}
