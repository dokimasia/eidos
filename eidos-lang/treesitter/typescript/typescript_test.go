// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/lang/treesitter/typescript"
)

// The sources the two grammars disagree on: a type assertion in angle
// brackets, which TypeScript parses and TSX reads as an element, and a
// JSX element, which TSX parses. An interface fits both.
const (
	tsFile      = "src/a.ts"
	tsxFile     = "src/a.tsx"
	assertion   = "const a = <number>b;\n"
	element     = "const a = <div className=\"x\" />;\n"
	tsName      = "typescript"
	tsxName     = "tsx"
	interfaceTS = "export interface User {\n  id: string;\n}\n"
)

// The package loads two grammars from one binding, so each is pinned
// by the source only it parses.
func TestTypeScript(t *testing.T) {
	t.Parallel()

	t.Run("TypeScript", func(t *testing.T) {
		t.Parallel()

		t.Run("parses an interface without an error", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, errorsOf(t, typescript.TypeScript, tsFile, interfaceTS), 0, "the source fits the grammar")
		})

		t.Run("parses a type assertion in angle brackets without an error", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, errorsOf(t, typescript.TypeScript, tsFile, assertion), 0, "the .ts form of a cast")
		})

		t.Run("loads under the language's name", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, typescript.TypeScript.Name(), tsName, "the name a frontend reports")
		})
	})

	t.Run("TSX", func(t *testing.T) {
		t.Parallel()

		t.Run("parses a JSX element without an error", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, errorsOf(t, typescript.TSX, tsxFile, element), 0, "an element is an expression in .tsx")
		})

		t.Run("reports an error for a type assertion in angle brackets", func(t *testing.T) {
			t.Parallel()

			assert.NotEqual(t, errorsOf(t, typescript.TSX, tsxFile, assertion), 0,
				"the angle brackets open an element in .tsx")
		})

		t.Run("loads under its own name", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, typescript.TSX.Name(), tsxName, "the name a frontend reports")
		})
	})
}

// errorsOf parses a source with a grammar and returns how many errors
// the tree reports.
func errorsOf(tb testing.TB, g *treesitter.Grammar, file, src string) int {
	tb.Helper()

	tree, err := g.Parse(tb.Context(), file, []byte(src))
	assert.NoError(tb, err, "the source parses")
	defer tree.Close()
	return len(slices.Collect(tree.Errors()))
}
