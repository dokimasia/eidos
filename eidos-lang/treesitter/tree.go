// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package treesitter

import (
	"iter"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// Tree is one parsed file and the source it parsed. One goroutine uses
// a Tree and its nodes at a time. The zero Tree is a closed one.
type Tree struct {
	grammar *Grammar
	path    string
	src     []byte
	// tree is the runtime's tree, and nil once the tree is closed.
	tree *ts.Tree
	// root is the tree's root node, which the parse reads once.
	root rawNode
}

// Root returns the root node, and the zero Node for a closed tree.
func (t *Tree) Root() Node {
	if t.tree == nil {
		return Node{}
	}
	return Node{tree: t, node: t.root}
}

// Errors yields the tree's ERROR and MISSING nodes in document order.
// It does not descend into an ERROR node, because the nodes inside
// one are the parser's recovery from the same error. It skips every
// subtree without an error, so a clean tree costs one check.
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

// Close releases the tree's C memory, and every node of the tree
// reports [Node.IsZero] afterwards. It is safe to call twice, because
// the runtime's Close ignores the nil tree the first call leaves.
func (t *Tree) Close() {
	t.tree.Close()
	t.tree = nil
}
