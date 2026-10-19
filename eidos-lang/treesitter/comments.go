// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package treesitter

import (
	"slices"
	"strings"

	"go.dokimi.dev/eidos/sdk/position"
)

// runSep joins the line comments of one run into the text that a frontend
// takes apart.
const runSep = "\n"

// Comments attributes the comments of one tree to the declarations that a
// frontend lowers. It finds the run above a declaration and the comment
// after it on its last line, and it sweeps every comment that no
// declaration took. The algorithm is the same for every grammar. The
// parameters are the facts of a language, such as the siblings that a run
// passes over.
//
// A frontend fills IsComment and IsBlock. Every other parameter is
// optional, and its zero value is stated beside it.
//
// # Concurrency
//
// A Comments serves one tree on one goroutine. A frontend copies a
// Comments with its parameters for each tree that it lowers.
//
// # Allocation contract
//
// [Comments.Leading] allocates the list of the comments that it returns.
// [Comments.Runs] allocates the text of each run once where Text is nil,
// and the text of each line and their join where Text is set, beside the
// growth of its buffers and of the set of taken nodes. [Comments.Take]
// allocates the growth of that set. [Comments.Trailing] and
// [Comments.Sweep] allocate what the parameters allocate.
type Comments struct {
	// IsComment reports whether a node is a comment.
	IsComment func(n Node) bool
	// IsBlock reports whether a comment is a block comment, which is a
	// text of its own and never part of a run.
	IsBlock func(n Node) bool
	// Passes reports whether a sibling between a declaration and its
	// comments is passed over, such as a decorator. Nil passes nothing.
	Passes func(n Node) bool
	// Stops reports whether a comment ends the run above a declaration,
	// such as a Rust inner doc comment. Nil stops at no comment.
	Stops func(n Node) bool
	// Trails reports whether a comment can be a trailing comment. A Rust
	// doc comment documents the item after it, so it never trails. Nil
	// admits every comment.
	Trails func(n Node) bool
	// LastLine returns the line on which the source of a node ends. Nil
	// returns the line of [Node.End].
	LastLine func(n Node) int
	// Text returns the text of a line comment in a run, such as the text
	// without the newline that a Rust doc comment takes. Nil returns
	// [Node.Text].
	Text func(n Node) string
	// Inner returns the child of a declaration whose comments join the
	// run above the declaration, such as the modifiers of a Java
	// declaration. Nil joins none.
	Inner func(n Node) Node
	// Folded reports that the grammar folds a trailing comment into the
	// last child of a node that ends without a terminator, as the grammar
	// of TypeScript does.
	Folded bool
	// Opaque reports whether the sweep passes over a node, such as a
	// function body. Nil passes over nothing.
	Opaque func(n Node) bool

	// taken is the start byte of every node that a run or [Comments.Take]
	// took.
	taken map[uint32]struct{}
	// run is the buffer of the line comments of the run that
	// [Comments.Runs] joins, and lines is the buffer of their texts where
	// Text is set.
	run   []Node
	lines []string
}

// Leading returns the comments directly above n in source order. A run ends
// on the line above n, or on its line, and has no blank line inside it. It
// passes over the siblings that Passes reports, and ends at a comment that
// Stops reports. It ends at a comment that a run or [Comments.Take] took,
// because that comment belongs to another subject. It also ends at a
// comment that shares a line with the sibling before it, where Trails
// admits the comment. Such a comment is the trailing comment of that
// sibling. The comments of the child that Inner returns follow the run.
func (c *Comments) Leading(n Node) []Node {
	var group []Node
	next := n.Pos().Line
	for prev := n.PrevNamedSibling(); !prev.IsZero(); prev = prev.PrevNamedSibling() {
		if c.Passes != nil && c.Passes(prev) {
			next = prev.Pos().Line
			continue
		}
		if !c.IsComment(prev) || c.Stops != nil && c.Stops(prev) || c.lastLine(prev) < next-1 || c.isTaken(prev) {
			break
		}
		before := prev.PrevNamedSibling()
		if (c.Trails == nil || c.Trails(prev)) && !before.IsZero() && !c.IsComment(before) &&
			(c.Passes == nil || !c.Passes(before)) && c.lastLine(before) == prev.Pos().Line {

			break
		}
		group = append(group, prev)
		next = prev.Pos().Line
	}
	slices.Reverse(group)
	if c.Inner != nil {
		for child := range c.Inner(n).NamedChildren() {
			if c.IsComment(child) {
				group = append(group, child)
			}
		}
	}
	return group
}

// Trailing returns the comment after n on its last line, and the zero Node
// where there is none. Where the grammar folds the comment, it is the last
// child of the innermost tail of n, after a sibling on its line. Otherwise
// it is the next sibling of n, where Trails admits it.
func (c *Comments) Trailing(n Node) Node {
	if c.Folded {
		for inner := n; ; {
			last := inner.LastNamed()
			if last.IsZero() || last.End() != inner.End() {
				break
			}
			if c.IsComment(last) && last.PrevNamedSibling().End().Line == last.Pos().Line {
				return last
			}
			inner = last
		}
	}
	next := n.NextNamedSibling()
	if c.IsComment(next) && (c.Trails == nil || c.Trails(next)) && next.Pos().Line == c.lastLine(n) {
		return next
	}
	return Node{}
}

// Runs calls yield once for each text of comments, in order. A text is a
// run of line comments on consecutive lines, joined by newlines, or one
// block comment. at is the position of the first comment of the text. Runs
// marks every comment taken.
func (c *Comments) Runs(comments []Node, yield func(text string, at position.Pos)) {
	c.run = c.run[:0]
	var at position.Pos
	for _, n := range comments {
		c.Take(n)
		if c.IsBlock(n) {
			c.flush(at, yield)
			yield(n.Text(), n.Pos())
			continue
		}
		if len(c.run) > 0 && n.Pos().Line != at.Line+len(c.run) {
			c.flush(at, yield)
		}
		if len(c.run) == 0 {
			at = n.Pos()
		}
		c.run = append(c.run, n)
	}
	c.flush(at, yield)
}

// Take marks nodes taken without reading them, such as the comments of a
// declaration that the load leaves out. The sweep passes over a taken node,
// so a frontend can take a whole declaration that the load leaves out.
func (c *Comments) Take(nodes ...Node) {
	if c.taken == nil {
		c.taken = map[uint32]struct{}{}
	}
	for _, n := range nodes {
		if !n.IsZero() {
			c.taken[startByte(n.node)] = struct{}{}
		}
	}
}

// Sweep calls untaken for every comment under root that no run took and
// that [Comments.Take] did not mark, in source order. It passes over a
// taken node and over a node that Opaque reports.
func (c *Comments) Sweep(root Node, untaken func(n Node)) {
	for child := range root.NamedChildren() {
		switch {
		case c.isTaken(child):
			continue
		case c.IsComment(child):
			untaken(child)
		case c.Opaque != nil && c.Opaque(child):
			continue
		default:
			c.Sweep(child, untaken)
		}
	}
}

// flush yields the lines of the buffered run as one text at its position,
// and empties the buffer. It yields nothing for an empty buffer. Where
// Text is nil, it writes the text of a run of more than one line from the
// source in one allocation.
func (c *Comments) flush(at position.Pos, yield func(text string, at position.Pos)) {
	var text string
	switch {
	case len(c.run) == 0:
		return
	case c.Text != nil:
		c.lines = c.lines[:0]
		for _, n := range c.run {
			c.lines = append(c.lines, c.Text(n))
		}
		text = strings.Join(c.lines, runSep)
	case len(c.run) == 1:
		text = c.run[0].Text()
	default:
		size := len(runSep) * (len(c.run) - 1)
		for _, n := range c.run {
			size += int(endByte(n.node) - startByte(n.node))
		}
		var b strings.Builder
		b.Grow(size)
		for i, n := range c.run {
			if i > 0 {
				b.WriteString(runSep)
			}
			b.Write(n.tree.src[startByte(n.node):endByte(n.node)])
		}
		text = b.String()
	}
	c.run = c.run[:0]
	yield(text, at)
}

// isTaken reports whether a run or [Comments.Take] took a node.
func (c *Comments) isTaken(n Node) bool {
	_, taken := c.taken[startByte(n.node)]
	return taken
}

// lastLine returns the line on which the source of a node ends, through
// LastLine where it is set.
func (c *Comments) lastLine(n Node) int {
	if c.LastLine != nil {
		return c.LastLine(n)
	}
	return n.End().Line
}
