// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package treesitter

import (
	"iter"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.dokimi.dev/eidos/sdk/position"
)

// compactTokens is the number of tokens for which [Node.Compact] allocates
// no spans. A type of a signature has fewer tokens.
const compactTokens = 32

// Node is a value handle on one syntax node of an open [Tree]. The zero
// Node is the absent node, and so is every node of a closed tree. An absent
// node reports [Node.IsZero], and every other accessor on it returns a zero
// value.
type Node struct {
	tree *Tree
	node rawNode
}

// IsZero reports whether the node is absent, as the zero Node and every
// node of a closed tree are.
func (n Node) IsZero() bool { return n.tree == nil || n.tree.tree == nil }

// Kind returns the kind of the node, and the zero Kind for an absent node.
func (n Node) Kind() Kind {
	if n.IsZero() {
		return 0
	}
	return symbolOf(n.node)
}

// Named reports whether the node is of a named kind, and not a keyword
// or punctuation of the grammar.
func (n Node) Named() bool { return !n.IsZero() && isNamed(n.node) }

// IsError reports whether the node is an ERROR node, which is source that
// the parser could not fit into the grammar.
func (n Node) IsError() bool { return !n.IsZero() && isError(n.node) }

// IsMissing reports whether the node is a MISSING node, which the
// parser inserted to recover from an error and which spans no source.
func (n Node) IsMissing() bool { return !n.IsZero() && isMissing(n.node) }

// IsExtra reports whether the node is an extra of the grammar, such as
// a comment, which the grammar admits between any two tokens.
func (n Node) IsExtra() bool { return !n.IsZero() && isExtra(n.node) }

// Text returns the source bytes of the node as a string.
func (n Node) Text() string {
	if n.IsZero() {
		return ""
	}
	return string(n.tree.src[startByte(n.node):endByte(n.node)])
}

// Bytes returns the source of the node without a copy, and nil for an
// absent node. The tree shares the bytes with the source that the parse
// was given, so a caller does not change them. A check of the source,
// such as a prefix, allocates nothing through Bytes.
func (n Node) Bytes() []byte {
	if n.IsZero() {
		return nil
	}
	return n.tree.src[startByte(n.node):endByte(n.node):endByte(n.node)]
}

// TextThrough returns the source from the first byte of the node through
// the last byte of another node of the same tree, as written. It is the
// text of a run of sibling tokens that no single node spans, such as one
// argument of a token tree. It returns the empty string for an absent node,
// for nodes of two trees, and for a last node that ends before the node
// starts.
func (n Node) TextThrough(last Node) string {
	if n.IsZero() || last.IsZero() || n.tree != last.tree {
		return ""
	}
	from, to := startByte(n.node), endByte(last.node)
	if to < from {
		return ""
	}
	return string(n.tree.src[from:to])
}

// Compact returns the tokens of the node in source order, without the
// whitespace between them and without its extras. Two separated tokens keep
// one space between them where the character before the gap and the
// character after it can both be part of an identifier, which is a letter,
// a digit, an underscore or a dollar sign. So "Map< String,  List<T> >"
// compacts to "Map<String,List<T>>" and "keyof  T" to "keyof T", and a
// reformatted node returns the same text. The whitespace inside a token,
// such as a string literal, is part of the token and remains.
//
// The runtime walks the subtree in one call. A node of at most 32 tokens
// allocates the string alone, and a larger node also allocates its spans.
func (n Node) Compact() string {
	if n.IsZero() {
		return ""
	}
	var small [compactTokens]span
	spans := small[:]
	count := tokens(n.node, spans)
	if count > len(spans) {
		spans = make([]span, count)
		tokens(n.node, spans)
	}
	spans = spans[:count]
	size := 0
	for _, s := range spans {
		size += int(s.to-s.from) + 1
	}
	src := n.tree.src
	var b strings.Builder
	b.Grow(size)
	end := -1 // the end byte of the last token written
	for _, s := range spans {
		from, to := int(s.from), int(s.to)
		if end >= 0 && from > end && identifierByte(src[:end], true) && identifierByte(src[from:], false) {
			b.WriteByte(' ')
		}
		b.Write(src[from:to])
		end = to
	}
	return b.String()
}

// identifierByte reports whether the rune at one end of src can be part of
// an identifier in the languages of the grammars, which is a letter, a
// digit, an underscore or a dollar sign. It reads the final rune where last
// is set, and the first rune otherwise.
func identifierByte(src []byte, last bool) bool {
	var r rune
	if last {
		r, _ = utf8.DecodeLastRune(src)
	} else {
		r, _ = utf8.DecodeRune(src)
	}
	return r == '_' || r == '$' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// Pos returns the position at which the node starts, with the path of the
// tree, the 1-based line and the 1-based byte column.
func (n Node) Pos() position.Pos {
	if n.IsZero() {
		return position.Pos{}
	}
	return n.at(startPoint(n.node))
}

// End returns the position of the byte after the last byte of the node,
// counted as [Node.Pos] counts.
func (n Node) End() position.Pos {
	if n.IsZero() {
		return position.Pos{}
	}
	return n.at(endPoint(n.node))
}

// Child returns the first child under a field, and the zero Node when
// the node has none.
func (n Node) Child(f Field) Node {
	if n.IsZero() {
		return Node{}
	}
	return n.wrap(childByField(n.node, f))
}

// Children yields every child under a field in source order. The runtime
// moves past the children of other fields, with one call for each child
// that the walk yields.
func (n Node) Children(f Field) iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if n.IsZero() || f == 0 {
			return
		}
		c := newCursor(n.node)
		defer c.close()
		for child := c.fieldStep(true, f); present(child); child = c.fieldStep(false, f) {
			if !yield(n.wrap(child)) {
				return
			}
		}
	}
}

// AllChildren yields every child in source order. The children are the
// named ones, the keywords and the punctuation of the grammar, and the
// extras, such as comments.
func (n Node) AllChildren() iter.Seq[Node] {
	return n.children(false)
}

// NamedChildren yields every child of a named kind in source order. The
// runtime moves past the unnamed children, with one call for each child
// that the walk yields.
func (n Node) NamedChildren() iter.Seq[Node] {
	return n.children(true)
}

// Content yields the named children of the node that are not extras, in
// source order, such as the members of a body without its comments.
func (n Node) Content() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		for child := range n.NamedChildren() {
			if !child.IsExtra() && !yield(child) {
				return
			}
		}
	}
}

// FirstOf returns the first named child of the node whose kind is one of
// kinds, and the zero Node where the node has none.
func (n Node) FirstOf(kinds ...Kind) Node {
	for child := range n.NamedChildren() {
		if slices.Contains(kinds, child.Kind()) {
			return child
		}
	}
	return Node{}
}

// LastNamed returns the last named child of the node, and the zero Node
// for a node without one.
func (n Node) LastNamed() Node {
	var last Node
	for child := range n.NamedChildren() {
		last = child
	}
	return last
}

// HasToken reports whether the node has an unnamed child spelled text,
// such as a keyword or a mark of the grammar. It compares the source of
// each child in place and allocates nothing.
func (n Node) HasToken(text string) bool {
	for child := range n.AllChildren() {
		if !isNamed(child.node) && string(n.tree.src[startByte(child.node):endByte(child.node)]) == text {
			return true
		}
	}
	return false
}

// Parent returns the parent of the node, and the zero Node for the root.
func (n Node) Parent() Node {
	if n.IsZero() {
		return Node{}
	}
	return n.wrap(parentOf(n.node))
}

// PrevNamedSibling returns the named sibling before the node, and the
// zero Node when there is none.
func (n Node) PrevNamedSibling() Node {
	if n.IsZero() {
		return Node{}
	}
	return n.wrap(prevNamed(n.node))
}

// NextNamedSibling returns the named sibling after the node, and the
// zero Node when there is none.
func (n Node) NextNamedSibling() Node {
	if n.IsZero() {
		return Node{}
	}
	return n.wrap(nextNamed(n.node))
}

// children yields the children of the node in source order, and only the
// named children where named is set.
func (n Node) children(named bool) iter.Seq[Node] {
	return func(yield func(Node) bool) {
		if n.IsZero() {
			return
		}
		c := newCursor(n.node)
		defer c.close()
		for child := c.step(true, named); present(child); child = c.step(false, named) {
			if !yield(n.wrap(child)) {
				return
			}
		}
	}
}

// wrap returns a node of the same tree, and the zero Node for the null
// node of the runtime.
func (n Node) wrap(node rawNode) Node {
	if !present(node) {
		return Node{}
	}
	return Node{tree: n.tree, node: node}
}

// at returns a position of the tree from the zero-based row and byte
// column of the runtime.
func (n Node) at(p rawPoint) position.Pos {
	return position.Pos{File: n.tree.path, Line: int(p.row) + 1, Col: int(p.column) + 1}
}
