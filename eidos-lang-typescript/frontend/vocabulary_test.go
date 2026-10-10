// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// everyForm declares one of each declaration kind the lowering walks by
// id, so a vocabulary resolved from the wrong grammar lowers less.
const everyForm = "export class A { m(): void {} }\nexport interface I { p: string }\n" +
	"export enum E { V }\nexport type T = string[];\nexport function f(): void {}\nexport const c = 1;\n"

// The TypeScript and TSX grammars number their kinds apart, so each has
// its own vocabulary, and the lowering of one source through both is
// pinned to agree.
func TestVocabulary(t *testing.T) {
	t.Parallel()

	t.Run("newVocabulary", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers one source to the same declarations through both grammars", func(t *testing.T) {
			t.Parallel()

			kinds := func(path, pkg string) []string {
				gb, _ := parsedTree(t, fstest.MapFS{path: {Data: []byte(everyForm)}}, path, plugin.DepthFull)
				decls := fileIn(t, gb, pkg).Decls
				out := make([]string, 0, len(decls))
				for _, d := range decls {
					out = append(out, d.Kind().String())
				}
				return out
			}
			ts := kinds(aFile, aPackage)
			assert.Length(t, ts, 6, "every form lowers")
			assert.Equal(t, kinds(tsxFile, tsxPackage), ts, "the TSX vocabulary lowers what the TypeScript one does")
		})

		t.Run("lowers a class's members through the TSX vocabulary", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, fstest.MapFS{tsxFile: {Data: []byte(everyForm)}}, tsxFile, plugin.DepthFull)
			st := named[*node.Struct](t, fileIn(t, gb, tsxPackage).Decls, "A")
			assert.Length(t, st.Methods, 1, "the method's kind resolves in the TSX grammar")
		})
	})
}
