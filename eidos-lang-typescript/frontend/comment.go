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

// lineMarker opens a line comment, and a run of line comments on
// consecutive lines reads as one text.
const lineMarker = "//"

// leading returns the comments directly above a node, in source order:
// the run of comments that ends on the line above it, or on its own
// line, with no blank line inside the run. The decorators between a
// class member and its comments are skipped. A comment that shares a
// line with the item before it is that item's trailing comment and
// ends the run.
func (l *lowering) leading(n treesitter.Node) []treesitter.Node {
	var group []treesitter.Node
	next := n.Pos().Line
	for prev := n.PrevNamedSibling(); !prev.IsZero(); prev = prev.PrevNamedSibling() {
		if prev.Kind() == l.v.decorator {
			next = prev.Pos().Line
			continue
		}
		if prev.Kind() != l.v.comment || prev.End().Line < next-1 {
			break
		}
		if before := prev.PrevNamedSibling(); !before.IsZero() && before.Kind() != l.v.comment &&
			before.End().Line == prev.Pos().Line {

			break
		}
		group = append(group, prev)
		next = prev.Pos().Line
	}
	slices.Reverse(group)
	return group
}

// trailing returns the comment that opens on the line a node's content
// ends on, after it, and the zero Node where there is none. The
// grammar folds such a comment into the node itself where the node
// ends without a semicolon, as the last child of its innermost tail,
// and otherwise it is the node's next sibling.
func (l *lowering) trailing(n treesitter.Node) treesitter.Node {
	for inner := n; ; {
		last := l.lastNamed(inner)
		if last.IsZero() || last.End() != inner.End() {
			break
		}
		if last.Kind() == l.v.comment && last.PrevNamedSibling().End().Line == last.Pos().Line {
			return last
		}
		inner = last
	}
	next := n.NextNamedSibling()
	if next.Kind() == l.v.comment && next.Pos().Line == n.End().Line {
		return next
	}
	return treesitter.Node{}
}

// lastNamed returns a node's last named child, and the zero Node for a
// node without one.
func (*lowering) lastNamed(n treesitter.Node) treesitter.Node {
	var last treesitter.Node
	for child := range n.NamedChildren() {
		last = child
	}
	return last
}

// declParts returns what a declaration's comments state: the leading
// comments' documentation, and the carriers of the leading and the
// trailing comment. comment is the trailing comment's documentation,
// one line, and empty where there is none.
func (l *lowering) declParts(outermost treesitter.Node) (parts plugin.CommentParts, comment string) {
	parts = l.parts(l.leading(outermost))
	if tail := l.trailing(outermost); !tail.IsZero() {
		after := l.parts([]treesitter.Node{tail})
		parts.Carriers = append(parts.Carriers, after.Carriers...)
		comment = strings.Join(after.Docs, " ")
	}
	return parts, comment
}

// parts takes comments apart through the unit's pipeline and marks
// them consumed. The comments are a run [lowering.leading] returns, or
// one comment, so a run of line comments in them is on consecutive
// lines and feeds the pipeline as one text: a carrier's continuation
// folds across the lines the way the author reads it. A block comment
// feeds it alone.
func (l *lowering) parts(comments []treesitter.Node) plugin.CommentParts {
	var parts plugin.CommentParts
	take := func(one plugin.CommentParts) {
		parts.Docs = append(parts.Docs, one.Docs...)
		parts.Carriers = append(parts.Carriers, one.Carriers...)
		parts.Annotations = append(parts.Annotations, one.Annotations...)
	}
	var run []string
	var runAt position.Pos
	flush := func() {
		if len(run) > 0 {
			take(l.u.Comment(strings.Join(run, "\n"), runAt))
			run = run[:0]
		}
	}
	for _, c := range comments {
		l.consumed[c.Pos()] = true
		text := c.Text()
		if !strings.HasPrefix(text, lineMarker) {
			flush()
			take(l.u.Comment(text, c.Pos()))
			continue
		}
		if len(run) == 0 {
			runAt = c.Pos()
		}
		run = append(run, text)
	}
	flush()
	return parts
}

// skip consumes a node's leading and trailing comments without reading
// them: the comments of a declaration the load leaves out, whose
// carriers belong to that declaration and leave with it.
func (l *lowering) skip(outermost treesitter.Node) {
	for _, c := range l.leading(outermost) {
		l.consumed[c.Pos()] = true
	}
	if tail := l.trailing(outermost); !tail.IsZero() {
		l.consumed[tail.Pos()] = true
	}
}

// sweep refuses the carriers of every comment no declaration took,
// positioned, so an authored directive never vanishes into silence. A
// comment inside a function's body belongs to the body's statements,
// which the model does not contain, so the sweep passes over a body.
func (l *lowering) sweep(n treesitter.Node) {
	for child := range n.NamedChildren() {
		switch {
		case child.Kind() == l.v.comment:
			if !l.consumed[child.Pos()] {
				refuseCarriers(l.u, l.u.Comment(child.Text(), child.Pos()).Carriers, "a comment no declaration takes")
			}
		case child.Kind() == l.v.statementBlock && !l.declares(n):
			continue
		default:
			l.sweep(child)
		}
	}
}

// declares reports whether a statement block's parent is a container
// of declarations: a namespace, an ambient module or a declare global
// block, whose block contains declarations and not statements.
func (l *lowering) declares(parent treesitter.Node) bool {
	k := parent.Kind()
	return k == l.v.internalModule || k == l.v.module || k == l.v.ambientDeclaration
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
