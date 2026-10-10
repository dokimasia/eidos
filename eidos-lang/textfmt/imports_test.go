// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package textfmt_test

import (
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/textfmt"
	"go.dokimi.dev/eidos/sdk/render"
)

// The allocations of an import block.
const (
	// importAllocs is a block of at most eight entries: its buffer.
	importAllocs = 1
	// wideImportAllocs is a block of more than eight entries: its buffer
	// and the three allocations of the set of written statements.
	wideImportAllocs = importAllocs + 3
	// longImportAllocs is a block with one statement longer than the
	// stack buffer it is spelled into: its buffer and the spelling.
	longImportAllocs = importAllocs + 1
)

// longSegments is how often the long path repeats its segment, which
// makes its statement longer than 256 bytes.
const longSegments = 40

// Java and Rust render every file's imports through ImportLines. The
// block is pinned byte for byte.
func TestImports(t *testing.T) {
	t.Parallel()

	t.Run("ImportLines", func(t *testing.T) {
		t.Parallel()

		t.Run("writes nothing for no entries", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, textfmt.ImportLines(nil, "use", "::"), "", "no entries, no block")
		})

		t.Run("writes one statement per entry in the given order", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, textfmt.ImportLines(javaImports(), "import", "."),
				"import example.rows;\nimport example.rows.Row;\nimport java.util.List;\n\n",
				"each path spelled with the separator, the bound name behind it, a blank line after")
		})

		t.Run("writes a statement two entries spell alike once", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			set.AddNamed("crate/rows", "Row")
			set.Add("crate/rows/Row")
			assert.Equal(t, textfmt.ImportLines(set.Entries(), "use", "::"),
				"use crate::rows::Row;\n\n",
				"a bare path and a named entry spelling one statement render it once")
		})

		t.Run("writes the bound name of a renamed item behind as", func(t *testing.T) {
			t.Parallel()

			var set render.ImportSet
			set.BindItem("crate/rows", "Row", false)
			set.BindItem("crate/legacy", "Row", false)
			assert.Equal(t, textfmt.ImportLines(set.Entries(), "use", "::"),
				"use crate::legacy::Row as Row2;\nuse crate::rows::Row;\n\n",
				"the second item binds under the suffixed name")
		})

		t.Run("writes a statement longer than 256 bytes", func(t *testing.T) {
			t.Parallel()

			got := textfmt.ImportLines(longImport(), "import", ".")
			assert.Equal(t, got, "import "+strings.Repeat("segment.", longSegments)+"Row;\n\n",
				"the whole path behind the keyword")
		})
	})
}

// ImportLines allocates the block's buffer, and the set of written
// statements above eight entries. The ordinary run, which runs no
// benchmark, checks those ceilings here.
func TestImportsAllocs(t *testing.T) {
	entries, wide, long := javaImports(), wideImports(), longImport()
	var got string
	assert.MaxAllocs(t, func() { got = textfmt.ImportLines(entries, "import", ".") }, importAllocs,
		"ImportLines allocates the block")
	assert.Contains(t, got, "import java.util.List;\n", "ImportLines writes the statements")
	assert.MaxAllocs(t, func() { got = textfmt.ImportLines(wide, "import", ".") }, wideImportAllocs,
		"ImportLines allocates the set of written statements above eight entries")
	assert.Contains(t, got, "import java.util.Set;\n", "ImportLines writes the ninth statement")
	assert.MaxAllocs(t, func() { got = textfmt.ImportLines(long, "import", ".") }, longImportAllocs,
		"ImportLines allocates the spelling of a statement longer than 256 bytes")
	assert.Contains(t, got, "segment.Row;\n", "ImportLines writes the long statement")
}

// BenchmarkImports measures the import block a Java file renders once.
func BenchmarkImports(b *testing.B) {
	b.Run("ImportLines", func(b *testing.B) {
		entries := javaImports()
		c := bench.Start(b).MaxAllocs(importAllocs)
		defer c.End()
		var got string
		for c.Loop() {
			got = textfmt.ImportLines(entries, "import", ".")
		}
		assert.Contains(b, got, "import java.util.List;\n", "ImportLines writes the statements")
	})
}

// javaImports returns the entries of a Java file importing List, the
// rows package and its Row.
func javaImports() []render.Entry {
	var set render.ImportSet
	set.AddNamed("java/util", "List")
	set.Add("example/rows")
	set.AddNamed("example/rows", "Row")
	return set.Entries()
}

// wideImports returns the entries of a Java file importing nine types
// of java.util.
func wideImports() []render.Entry {
	var set render.ImportSet
	for _, name := range []string{"ArrayList", "Collection", "Deque", "HashMap", "Iterator", "List", "Map", "Queue", "Set"} {
		set.AddNamed("java/util", name)
	}
	return set.Entries()
}

// longImport returns the entry of a Java import whose statement is
// longer than 256 bytes.
func longImport() []render.Entry {
	var set render.ImportSet
	set.AddNamed(strings.TrimSuffix(strings.Repeat("segment/", longSegments), "/"), "Row")
	return set.Entries()
}
