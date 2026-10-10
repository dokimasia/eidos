// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"slices"
	"strings"

	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
)

// lineEnd is what a doc line comment's source ends with: the newline
// the grammar takes into the comment.
const lineEnd = "\r\n"

// lastLine returns the line a node's source ends on: the line before its
// end where the node takes the newline of its last line, as a doc line
// comment does.
func lastLine(n treesitter.Node) int {
	end := n.End()
	if end.Col == 1 && end.Line > n.Pos().Line {
		return end.Line - 1
	}
	return end.Line
}

// refuseCarriers reports every carrier on a subject the model cannot
// address, quoting it as the author wrote it, so an authored directive
// never vanishes into silence.
func refuseCarriers(u *plugin.SourceUnit, cs []plugin.Carrier, what string) {
	for _, c := range cs {
		u.Errorf(UnaddressedCarrier, c.Pos,
			"%q is on %s, which the model cannot address. Move it directly above a declaration",
			c.Mark+c.Payload, what)
	}
}

// refuseMarkers reports every marker of the brand on a subject the model
// cannot address under [UnaddressedCarrier], with its path as the author
// wrote it, so an authored directive never vanishes into silence.
func refuseMarkers(u *plugin.SourceUnit, sugars []plugin.Sugar, what string) {
	for _, s := range sugars {
		u.Errorf(UnaddressedCarrier, s.Pos,
			"the marker %s is on %s, which the model cannot address. Move it to a declaration",
			strings.Join(s.Path, pathSeparator), what)
	}
}

// leading returns the comments directly above an item, in source order:
// the run of comments that ends on the line above it, or on its own
// line, with no blank line inside the run. The outer attributes between
// an item and its comments are skipped, and so is a positional field's
// visibility. The run ends at an inner doc comment, which documents the
// module, and at a plain comment that shares a line with the item
// before it, which is that item's trailing comment. An outer doc comment
// on that line documents the item after it, as rustc reads it.
func (l *lowering) leading(n treesitter.Node) []treesitter.Node {
	var group []treesitter.Node
	next := n.Pos().Line
	for prev := n.PrevNamedSibling(); !prev.IsZero(); prev = prev.PrevNamedSibling() {
		if k := prev.Kind(); k == l.v.attributeItem || k == l.v.visibilityModifier {
			next = prev.Pos().Line
			continue
		}
		if !l.comment(prev) || l.innerDoc(prev) || lastLine(prev) < next-1 {
			break
		}
		if before := prev.PrevNamedSibling(); !l.doc(prev) && !before.IsZero() && !l.comment(before) &&
			before.Kind() != l.v.attributeItem && lastLine(before) == prev.Pos().Line {

			break
		}
		group = append(group, prev)
		next = prev.Pos().Line
	}
	slices.Reverse(group)
	return group
}

// trailing returns the plain comment that opens after a node on the line
// the node ends on, and the zero Node where there is none. A doc comment
// there documents the item after it, so it is never a trailing comment.
func (l *lowering) trailing(n treesitter.Node) treesitter.Node {
	next := n.NextNamedSibling()
	if l.comment(next) && !l.doc(next) && next.Pos().Line == lastLine(n) {
		return next
	}
	return treesitter.Node{}
}

// declParts returns what an item's comments and doc attributes state:
// the leading comments' documentation and then the #[doc] attributes',
// and the carriers of the leading and the trailing comment. comment is
// the trailing comment's documentation, one line, and empty where there
// is none.
func (l *lowering) declParts(n treesitter.Node, a attributes) (parts plugin.CommentParts, comment string) {
	parts = l.parts(l.leading(n))
	parts.Docs = append(parts.Docs, a.docs...)
	if tail := l.trailing(n); !tail.IsZero() {
		after := l.parts([]treesitter.Node{tail})
		parts.Carriers = append(parts.Carriers, after.Carriers...)
		comment = strings.Join(after.Docs, " ")
	}
	return parts, comment
}

// parts takes comments apart through the unit's pipeline and marks them
// taken. A run of line comments on consecutive lines feeds the pipeline
// as one text, each line without the newline a doc line comment takes,
// so a carrier's continuation folds across the lines the way the author
// reads it. A block comment feeds it alone.
func (l *lowering) parts(comments []treesitter.Node) plugin.CommentParts {
	var parts plugin.CommentParts
	take := func(one plugin.CommentParts) {
		parts.Docs = append(parts.Docs, one.Docs...)
		parts.Carriers = append(parts.Carriers, one.Carriers...)
	}
	var run []string
	var runAt position.Pos
	flush := func() {
		if len(run) > 0 {
			take(l.w.u.Comment(strings.Join(run, "\n"), runAt))
			run = run[:0]
		}
	}
	for _, c := range comments {
		l.taken[c.Pos()] = true
		if c.Kind() == l.v.blockComment {
			flush()
			take(l.w.u.Comment(c.Text(), c.Pos()))
			continue
		}
		if len(run) > 0 && c.Pos().Line != runAt.Line+len(run) {
			flush()
		}
		if len(run) == 0 {
			runAt = c.Pos()
		}
		run = append(run, strings.TrimRight(c.Text(), lineEnd))
	}
	flush()
	return parts
}

// skip takes an item's leading and trailing comments without reading
// them, and passes the sweep over the item: the comments and carriers
// of an item the load leaves out leave with it.
func (l *lowering) skip(n treesitter.Node) {
	for _, c := range l.leading(n) {
		l.taken[c.Pos()] = true
	}
	if tail := l.trailing(n); !tail.IsZero() {
		l.taken[tail.Pos()] = true
	}
	l.taken[n.Pos()] = true
}

// refuse reports the carriers of a node's leading and trailing comments,
// and the markers among its attributes a, under [UnaddressedCarrier]. what
// describes the node, a subject that the model cannot address.
func (l *lowering) refuse(n treesitter.Node, a attributes, what string) {
	parts, _ := l.declParts(n, attributes{})
	refuseCarriers(l.w.u, parts.Carriers, what)
	refuseMarkers(l.w.u, a.sugars, what)
}

// sweep refuses the carriers of every comment no declaration took,
// positioned, so an authored directive never vanishes into silence. It
// passes over an item the load left out, whose carriers leave with it,
// and over a function body, whose comments belong to statements the
// model does not contain.
func (l *lowering) sweep(n treesitter.Node) {
	for child := range n.NamedChildren() {
		switch {
		case l.taken[child.Pos()] || child.Kind() == l.v.block:
			continue
		case l.comment(child):
			refuseCarriers(l.w.u, l.w.u.Comment(child.Text(), child.Pos()).Carriers, "a comment no declaration takes")
		default:
			l.sweep(child)
		}
	}
}

// comment reports whether a node is a line or a block comment.
func (l *lowering) comment(n treesitter.Node) bool {
	k := n.Kind()
	return k == l.v.lineComment || k == l.v.blockComment
}

// doc reports whether a comment is a doc comment, outer or inner.
func (l *lowering) doc(c treesitter.Node) bool {
	return !c.Child(l.v.fieldOuter).IsZero() || l.innerDoc(c)
}

// innerDoc reports whether a comment is an inner doc comment, //! or
// /*! */, which documents the module it is in.
func (l *lowering) innerDoc(c treesitter.Node) bool {
	return !c.Child(l.v.fieldInner).IsZero()
}
