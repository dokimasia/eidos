// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package treesitter

import (
	"iter"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/sdk/position"
)

// The parts of the sentences of [Tree.Problems]: what follows the kind of
// a MISSING node and the excerpt of an ERROR node, the number of bytes of
// an excerpt, and the mark of an excerpt that is cut.
const (
	missingHere  = " is missing here"
	doesNotParse = " does not parse"
	excerptBytes = 32
	cutMark      = "..."
)

// Tree is one parsed file and its source. The zero Tree is a closed tree.
//
// # Concurrency
//
// One goroutine uses a Tree and its nodes at a time.
//
// # Allocation contract
//
// [Tree.Root], [Tree.Errors] and [Tree.Close] allocate nothing on the
// Go heap.
type Tree struct {
	grammar *Grammar
	path    string
	src     []byte
	// tree is the tree of the runtime, and nil once the tree is closed.
	tree rawTree
	// root is the root node of the tree, which the parse reads once.
	root rawNode
}

// Root returns the root node, and the zero Node for a closed tree.
func (t *Tree) Root() Node {
	if t.tree == nil {
		return Node{}
	}
	return Node{tree: t, node: t.root}
}

// Errors yields the ERROR and MISSING nodes of the tree in document order.
// It does not descend into an ERROR node, because the nodes inside one are
// the recovery of the parser from the same error. It skips every subtree
// without an error, so a clean tree costs one check. A closed tree yields
// nothing.
func (t *Tree) Errors() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		root := t.Root()
		if root.IsZero() || !hasError(root.node) {
			return
		}
		c := newCursor(root.node)
		defer c.close()
		for {
			n := c.node()
			if isError(n) || isMissing(n) {
				if !yield(Node{tree: t, node: n}) {
					return
				}
			} else if hasError(n) && c.firstChild() {
				continue
			}
			for !c.nextSibling() {
				if !c.parent() {
					return
				}
			}
		}
	}
}

// Problems yields the position and the sentence of each ERROR and MISSING
// node of the tree, in the order of [Tree.Errors]. The sentence of a
// MISSING node is "X is missing here", where X is the name of its kind.
// The sentence of an ERROR node is the first line of its source in quotes,
// cut at 32 bytes, followed by "does not parse". A frontend reports
// the problems through the syntax report of its unit. Problems returns nil
// for a clean tree and for a closed one, so a frontend skips the report of
// a clean file.
//
// # Allocation contract
//
// Problems allocates nothing for a clean tree, and the iterator for a tree
// with errors. The walk allocates the sentence of a MISSING node, and the
// source, the quote and the sentence of an ERROR node.
func (t *Tree) Problems() iter.Seq2[position.Pos, string] {
	if root := t.Root(); root.IsZero() || !hasError(root.node) {
		return nil
	}
	return func(yield func(position.Pos, string) bool) {
		for n := range t.Errors() {
			var sentence string
			if n.IsMissing() {
				sentence = t.grammar.KindName(n.Kind()) + missingHere
			} else {
				line, _, _ := strings.Cut(n.Text(), "\n")
				if len(line) > excerptBytes {
					line = line[:excerptBytes] + cutMark
				}
				sentence = strconv.Quote(line) + doesNotParse
			}
			if !yield(n.Pos(), sentence) {
				return
			}
		}
	}
}

// Close releases the C memory of the tree, and every node of the tree
// reports [Node.IsZero] afterwards. A second call releases nothing.
func (t *Tree) Close() {
	if t.tree == nil {
		return
	}
	deleteTree(t.tree)
	t.tree = nil
}
