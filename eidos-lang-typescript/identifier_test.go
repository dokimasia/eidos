// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/typescript"
)

// The characters that the identifier cases test in both positions.
const (
	letterOutsideASCII = '\U000000e9'
	letterNumber       = '\U0000216b'
	combiningMark      = '\U00000301'
	spacingMark        = '\U00000903'
	connector          = '\U0000203f'
	middleDot          = '\U000000b7'
	nonJoiner          = '\U0000200c'
	patternLetter      = '\U00002e2f'
	lineBreak          = '\U00002028'
)

// TestIdentifier checks the characters that can start and continue an
// identifier of TypeScript. Node's parser implements ECMAScript's grammar
// of identifiers, and its verdict on each character is the expected
// value of the case.
func TestIdentifier(t *testing.T) {
	t.Parallel()

	t.Run("IsIdentifierStart", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give rune
			want bool
		}{
			{name: "reports true for an ASCII letter", give: 'a', want: true},
			{name: "reports true for a dollar", give: '$', want: true},
			{name: "reports true for an underscore", give: '_', want: true},
			{name: "reports true for a letter outside ASCII", give: letterOutsideASCII, want: true},
			{name: "reports true for a title-case letter", give: '\U000001c5', want: true},
			{name: "reports true for a modifier letter", give: '\U000002b0', want: true},
			{name: "reports true for an ideograph", give: '\U00004e2d', want: true},
			{name: "reports true for a letter number", give: letterNumber, want: true},
			{name: "reports true for a character of Other_ID_Start", give: '\U00002118', want: true},
			{name: "reports false for a digit", give: '1'},
			{name: "reports false for a combining mark", give: combiningMark},
			{name: "reports false for a spacing mark", give: spacingMark},
			{name: "reports false for a connector other than the underscore", give: connector},
			{name: "reports false for a character of Other_ID_Continue", give: middleDot},
			{name: "reports false for the zero-width non-joiner", give: nonJoiner},
			{name: "reports false for a modifier letter of Pattern_Syntax", give: patternLetter},
			{name: "reports false for a hyphen", give: '-'},
			{name: "reports false for a space", give: ' '},
			{name: "reports false for a line separator", give: lineBreak},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, typescript.IsIdentifierStart(tt.give), tt.want,
					"IsIdentifierStart returns the verdict of node's parser")
			})
		}
	})

	t.Run("IsIdentifierPart", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give rune
			want bool
		}{
			{name: "reports true for an ASCII letter", give: 'a', want: true},
			{name: "reports true for a dollar", give: '$', want: true},
			{name: "reports true for an underscore", give: '_', want: true},
			{name: "reports true for a letter number", give: letterNumber, want: true},
			{name: "reports true for a digit", give: '1', want: true},
			{name: "reports true for a combining mark", give: combiningMark, want: true},
			{name: "reports true for a spacing mark", give: spacingMark, want: true},
			{name: "reports true for a connector other than the underscore", give: connector, want: true},
			{name: "reports true for a character of Other_ID_Continue", give: middleDot, want: true},
			{name: "reports true for the zero-width non-joiner", give: nonJoiner, want: true},
			{name: "reports true for the zero-width joiner", give: '\U0000200d', want: true},
			{name: "reports false for a modifier letter of Pattern_Syntax", give: patternLetter},
			{name: "reports false for a hyphen", give: '-'},
			{name: "reports false for a space", give: ' '},
			{name: "reports false for a line separator", give: lineBreak},
			{name: "reports false for an at sign", give: '@'},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, typescript.IsIdentifierPart(tt.give), tt.want,
					"IsIdentifierPart returns the verdict of node's parser")
			})
		}
	})
}

// TestIdentifierAllocs checks that both predicates do not allocate. The
// ordinary test run does not run benchmarks, so this test checks the
// ceilings.
func TestIdentifierAllocs(t *testing.T) {
	var start, part bool
	expect.MaxAllocs(t, func() { start = typescript.IsIdentifierStart(letterOutsideASCII) }, 0,
		"IsIdentifierStart allocates nothing")
	expect.MaxAllocs(t, func() { part = typescript.IsIdentifierPart(combiningMark) }, 0,
		"IsIdentifierPart allocates nothing")
	expect.True(t, start, "a letter outside ASCII starts an identifier")
	expect.True(t, part, "a combining mark continues an identifier")
}

// BenchmarkIdentifier measures each predicate on a character outside
// ASCII, which the predicates look up in the Unicode tables. The spelling
// of a name calls them for every character of the name.
func BenchmarkIdentifier(b *testing.B) {
	b.Run("IsIdentifierStart", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var start bool
		for c.Loop() {
			start = typescript.IsIdentifierStart(letterOutsideASCII)
		}
		assert.True(b, start, "a letter outside ASCII starts an identifier")
	})

	b.Run("IsIdentifierPart", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var part bool
		for c.Loop() {
			part = typescript.IsIdentifierPart(combiningMark)
		}
		assert.True(b, part, "a combining mark continues an identifier")
	})
}
