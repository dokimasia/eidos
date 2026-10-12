// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package treesitter_test

import (
	"iter"
	"slices"
	"strconv"
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

// longBrokenJava has an ERROR node whose first line is longer than the
// quote of a problem, and excerptBytes is the length of the quote.
const (
	longBrokenJava = "class A {\n  int a = 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20;\n}\n"
	excerptBytes   = 32
)

// problemsAllocs is a range over the problems of the broken source: the
// iterator and the loop body of the range, the source, the quote and the
// sentence of its ERROR node, and the sentence of its MISSING node.
const problemsAllocs = 6

// problem is one problem that [treesitter.Tree.Problems] yields.
type problem struct {
	at       position.Pos
	sentence string
}

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

	t.Run("Problems", func(t *testing.T) {
		t.Parallel()

		t.Run("yields the position and the sentence of each problem in document order", func(t *testing.T) {
			t.Parallel()

			tree := parse(t, brokenJava)
			errs := slices.Collect(tree.Errors())
			assert.Length(t, errs, 2, "the source has an ERROR and a MISSING node")
			var got []problem
			for at, sentence := range tree.Problems() {
				got = append(got, problem{at: at, sentence: sentence})
			}
			assert.Equal(t, got, []problem{
				{at: errorAt, sentence: strconv.Quote(errs[0].Text()) + " does not parse"},
				{at: missingAt, sentence: "; is missing here"},
			}, "the ERROR node quotes its source, and the MISSING node names its kind")
		})

		t.Run("cuts the quote of a long line at 32 bytes", func(t *testing.T) {
			t.Parallel()

			tree := parse(t, longBrokenJava)
			var long treesitter.Node
			for n := range tree.Errors() {
				if n.IsError() && len(n.Text()) > excerptBytes {
					long = n
					break
				}
			}
			assert.False(t, long.IsZero(), "the source has an ERROR node of more than 32 bytes")
			var got string
			for at, sentence := range tree.Problems() {
				if at == long.Pos() {
					got = sentence
				}
			}
			assert.Equal(t, got, strconv.Quote(long.Text()[:excerptBytes]+"...")+" does not parse",
				"the quote has the first 32 bytes and the mark of a cut")
		})

		t.Run("stops when the consumer stops", func(t *testing.T) {
			t.Parallel()

			seen := 0
			for range parse(t, brokenJava).Problems() {
				seen++
				break
			}
			assert.Equal(t, seen, 1, "the walk ends at the first break")
		})

		t.Run("returns nil for a closed tree", func(t *testing.T) {
			t.Parallel()

			tree := parse(t, brokenJava)
			tree.Close()
			assert.Nil(t, tree.Problems(), "a closed tree has no problems")
		})

		t.Run("returns nil for a clean tree", func(t *testing.T) {
			t.Parallel()

			assert.Nil(t, parse(t, cleanJava).Problems(), "the source fits the grammar")
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
// Each Close takes a tree parsed outside the count.
func TestTreeAllocs(t *testing.T) {
	checkAllocs(t, treeCalls(t))
	var last *treesitter.Tree
	assert.MaxAllocsWithSetup(t, func() *treesitter.Tree { return parse(t, cleanJava) }, func(tree *treesitter.Tree) {
		tree.Close()
		last = tree
	}, 0, "Close allocates nothing")
	assert.True(t, last.Root().IsZero(), "Close releases the tree")
}

// BenchmarkTree measures the root and error walk a frontend reads once
// per file, and the release of the file's tree.
func BenchmarkTree(b *testing.B) {
	benchCalls(b, treeCalls(b))

	b.Run("Close", func(b *testing.B) {
		ctx, src := b.Context(), []byte(cleanJava)
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

	tree, clean := parse(tb, brokenJava), parse(tb, cleanJava)
	var (
		root     treesitter.Node
		errs     int
		last     string
		problems iter.Seq2[position.Pos, string]
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
		{
			name: "Problems", caseName: "a tree with an ERROR and a MISSING node", allocs: problemsAllocs,
			call: func() {
				errs = 0
				for _, sentence := range tree.Problems() {
					errs++
					last = sentence
				}
			},
			check: func(tb assert.TB) {
				assert.Equal(tb, errs, 2, "Problems yields the ERROR and the MISSING node")
				assert.Equal(tb, last, "; is missing here", "Problems yields the sentence of the MISSING node last")
			},
		},
		{
			name: "Problems", caseName: "a clean tree",
			call:  func() { problems = clean.Problems() },
			check: func(tb assert.TB) { assert.Nil(tb, problems, "Problems returns nil for a clean tree") },
		},
	}
}
