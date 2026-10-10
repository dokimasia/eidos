// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The imports the pruning cases write: one an exported signature
// names, one a function body calls, one behind an alias no reference
// names, and one a dot import brings in.
const (
	signatureImport = "a.test/sig"
	bodyImport      = "b.test/body"
	aliasedImport   = "c.test/aliased"
	dotImport       = "d.test/dot"
)

// Signature-only loading follows what exported signatures reference,
// so which imports a signature-only file keeps is pinned.
func TestPrune(t *testing.T) {
	t.Parallel()

	t.Run("pruneImports", func(t *testing.T) {
		t.Parallel()

		t.Run("keeps only the imports an exported signature names", func(t *testing.T) {
			t.Parallel()

			src := "package p\n\nimport (\n\t\"" + signatureImport + "\"\n\t\"" + bodyImport + "\"\n)\n\n" +
				"// F takes a sig.T.\nfunc F(v sig.T) {}\n\nfunc g() { body.Do() }\n"
			assert.Equal(t, importsAt(t, plugin.DepthSignatures, src), []string{signatureImport},
				"a body's call and an unexported function are not the package's API")
		})

		t.Run("keeps every import at full depth", func(t *testing.T) {
			t.Parallel()

			src := "package p\n\nimport (\n\t\"" + signatureImport + "\"\n\t\"" + bodyImport + "\"\n)\n\n" +
				"// F takes a sig.T.\nfunc F(v sig.T) {}\n\nfunc g() { body.Do() }\n"
			assert.Equal(t, importsAt(t, plugin.DepthFull, src), []string{signatureImport, bodyImport},
				"a full load records the file as written")
		})

		t.Run("keeps every unaliased import where a qualified reference names no import", func(t *testing.T) {
			t.Parallel()

			src := "package p\n\nimport (\n\t\"" + signatureImport + "\"\n\tal \"" + aliasedImport + "\"\n)\n\n" +
				"// F takes a type whose qualifier no import path assumes.\nfunc F(v other.T) {}\n\nvar _ = al.X\n"
			assert.Equal(t, importsAt(t, plugin.DepthSignatures, src), []string{signatureImport},
				"a package clause can declare a name its path does not assume, and an alias binds its own")
		})

		t.Run("keeps a dot import", func(t *testing.T) {
			t.Parallel()

			src := "package p\n\nimport . \"" + dotImport + "\"\n\n// F takes a dot-imported T.\nfunc F(v T) {}\n"
			assert.Equal(t, importsAt(t, plugin.DepthSignatures, src), []string{dotImport},
				"a bare exported name can come from the dot import")
		})

		t.Run("keeps no import of a file that retains no declaration", func(t *testing.T) {
			t.Parallel()

			src := "package p\n\nimport (\n\t\"" + signatureImport + "\"\n\t. \"" + dotImport + "\"\n)\n\n" +
				"func f(v sig.T) T { return v }\n"
			assert.Empty(t, importsAt(t, plugin.DepthSignatures, src),
				"the unexported function is left out, and a dot import serves no retained name")
		})

		t.Run("keeps no import of a file the build excludes", func(t *testing.T) {
			t.Parallel()

			src := "//go:build ignore\n\npackage p\n\nimport \"" + signatureImport + "\"\n\n" +
				"// F takes a sig.T.\nfunc F(v sig.T) {}\n"
			assert.Empty(t, importsAt(t, plugin.DepthSignatures, src), "an excluded file declares nothing")
		})
	})
}

// importPaths returns the paths of a file's import records, in source
// order.
func importPaths(file *node.File) []string {
	out := make([]string, 0, len(file.Imports))
	for _, imp := range file.Imports {
		out = append(out, imp.Path)
	}
	return out
}

// importsAt parses one file at a depth and returns its import paths.
func importsAt(tb testing.TB, depth plugin.Depth, src string) []string {
	tb.Helper()

	gb, _ := parsedFindings(tb, nil, depth, src)
	return importPaths(onlyFile(tb, gb))
}
