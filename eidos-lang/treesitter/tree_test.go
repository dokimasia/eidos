// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package treesitter_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/treesitter/java"
	"go.dokimi.dev/eidos/sdk/position"
)

// The positions of the broken source's two errors: the ERROR node that
// opens line 2's field, and the semicolon the parser inserted after
// line 3's initializer, which spans no source.
var (
	errorAt   = position.Pos{File: javaFile, Line: 2, Col: 3}
	missingAt = position.Pos{File: javaFile, Line: 3, Col: 25}
)

// A tree is C memory the runtime allocates, and it reports its errors
// as nodes, so its root, its error walk and its release are pinned
// over parsed sources.
func TestTree(t *testing.T) {
	t.Parallel()

	g := java.Grammar

	t.Run("Root", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the file's root node", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, parse(t, cleanJava).Root().Kind(), g.Kind(programKind), "the grammar's root kind")
		})

		t.Run("returns the zero Node for a closed tree", func(t *testing.T) {
			t.Parallel()

			tree := parse(t, cleanJava)
			tree.Close()
			assert.True(t, tree.Root().IsZero(), "a closed tree has no root")
		})
	})

	t.Run("Errors", func(t *testing.T) {
		t.Parallel()

		t.Run("yields nothing for a clean tree", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, slices.Collect(parse(t, cleanJava).Errors()), "the source fits the grammar")
		})

		t.Run("yields the ERROR and MISSING nodes in document order", func(t *testing.T) {
			t.Parallel()

			var at []position.Pos
			for n := range parse(t, brokenJava).Errors() {
				at = append(at, n.Pos())
			}
			assert.Equal(t, at, []position.Pos{errorAt, missingAt}, "the field's ERROR, then the missing semicolon")
		})

		t.Run("stops when the consumer stops", func(t *testing.T) {
			t.Parallel()

			seen := 0
			for n := range parse(t, brokenJava).Errors() {
				seen++
				if n.IsError() {
					break
				}
			}
			assert.Equal(t, seen, 1, "the walk ends at the first break")
		})

		t.Run("yields nothing for a closed tree", func(t *testing.T) {
			t.Parallel()

			tree := parse(t, brokenJava)
			tree.Close()
			assert.Empty(t, slices.Collect(tree.Errors()), "a closed tree has no nodes")
		})
	})

	t.Run("Close", func(t *testing.T) {
		t.Parallel()

		t.Run("is safe to call twice", func(t *testing.T) {
			t.Parallel()

			tree := parse(t, cleanJava)
			tree.Close()
			tree.Close()
			assert.True(t, tree.Root().IsZero(), "the second call finds the tree released")
		})

		t.Run("makes every node of the tree absent", func(t *testing.T) {
			t.Parallel()

			tree := parse(t, cleanJava)
			class := first(tree.Root(), g.Kind(classKind))
			tree.Close()
			assert.True(t, class.IsZero(), "a node of a closed tree is absent")
			assert.Equal(t, class.Kind(), 0, "and returns zero values")
		})
	})
}
