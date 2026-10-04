// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package treesitter_test

import (
	"context"
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/treesitter"
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

// A tree's nodes are C memory the runtime allocates, and its errors are
// nodes. Its root, its error walk and its release are pinned over
// parsed sources.
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

		t.Run("yields every node the parser marks erroneous in document order", func(t *testing.T) {
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

		t.Run("releases nothing on a second call", func(t *testing.T) {
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

		t.Run("releases nothing for the zero Tree", func(t *testing.T) {
			t.Parallel()

			var tree treesitter.Tree
			tree.Close()
			assert.True(t, tree.Root().IsZero(), "the zero Tree is a closed one")
		})
	})
}

// A tree's root, its error walk and its release allocate nothing on the
// Go heap. The ordinary run, which runs no benchmark, checks that here.
func TestTreeZeroAlloc(t *testing.T) {
	checkAllocs(t, treeCalls(t))
	trees := make([]*treesitter.Tree, 0, allocRuns)
	for range allocRuns {
		trees = append(trees, parse(t, cleanJava))
	}
	closed := 0
	closing := func() {
		trees[closed].Close()
		closed++
	}
	assert.MaxAllocs(t, closing, 0, "Close allocates nothing")
	assert.Equal(t, closed, allocRuns, "Close released every tree")
	assert.True(t, trees[allocRuns-1].Root().IsZero(), "Close releases the tree")
}

// BenchmarkTree measures the root and error walk a frontend reads once
// per file, and the release of the file's tree.
func BenchmarkTree(b *testing.B) {
	benchCalls(b, treeCalls(b))

	b.Run("Close", func(b *testing.B) {
		ctx, src := context.Background(), []byte(cleanJava)
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var (
			tree *treesitter.Tree
			err  error
		)
		for c.Loop() {
			c.Excluding(func() { tree, err = java.Grammar.Parse(ctx, javaFile, src) })
			tree.Close()
		}
		assert.NoError(b, err, "the source parses")
		assert.True(b, tree.Root().IsZero(), "Close releases the tree")
	})
}

// treeCalls returns a call of the root and of the error walk of the
// broken source's tree.
func treeCalls(tb testing.TB) []allocCall {
	tb.Helper()

	tree := parse(tb, brokenJava)
	var (
		root treesitter.Node
		errs int
	)
	return []allocCall{
		{
			name: "Root", call: func() { root = tree.Root() },
			check: func(tb assert.TB) {
				assert.Equal(tb, root.Kind(), java.Grammar.Kind(programKind), "Root returns the program")
			},
		},
		{
			name: "Errors", call: func() {
				errs = 0
				for range tree.Errors() {
					errs++
				}
			},
			check: func(tb assert.TB) { assert.Equal(tb, errs, 2, "Errors yields the ERROR and the MISSING node") },
		},
	}
}
