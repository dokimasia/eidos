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

// leading returns the comments directly above a declaration, in source
// order: the run of comments that ends on the line above it, or on its
// own line, with no blank line inside the run, and then the comments
// among its modifiers, as between an annotation and its keyword. A
// comment that shares a line with the declaration before it is that
// declaration's trailing comment and ends the run.
func (l *lowering) leading(n treesitter.Node) []treesitter.Node {
	var group []treesitter.Node
	next := n.Pos().Line
	for prev := n.PrevNamedSibling(); !prev.IsZero(); prev = prev.PrevNamedSibling() {
		if !l.comment(prev) || prev.End().Line < next-1 {
			break
		}
		if before := prev.PrevNamedSibling(); !before.IsZero() && !l.comment(before) &&
			before.End().Line == prev.Pos().Line {

			break
		}
		group = append(group, prev)
		next = prev.Pos().Line
	}
	slices.Reverse(group)
	for child := range l.firstOf(n, l.v.modifiers).NamedChildren() {
		if l.comment(child) {
			group = append(group, child)
		}
	}
	return group
}

// trailing returns the comment that opens after a declaration on the
// line the declaration ends on, and the zero Node where there is none.
func (l *lowering) trailing(n treesitter.Node) treesitter.Node {
	next := n.NextNamedSibling()
	if l.comment(next) && next.Pos().Line == n.End().Line {
		return next
	}
	return treesitter.Node{}
}

// declParts returns what a declaration's comments state: the leading
// comments' documentation, and the carriers of the leading and the
// trailing comment. comment is the trailing comment's documentation, one
// line, and empty where there is none.
func (l *lowering) declParts(n treesitter.Node) (parts plugin.CommentParts, comment string) {
	parts = l.parts(l.leading(n))
	if tail := l.trailing(n); !tail.IsZero() {
		after := l.parts([]treesitter.Node{tail})
		parts.Carriers = append(parts.Carriers, after.Carriers...)
		comment = strings.Join(after.Docs, " ")
	}
	return parts, comment
}

// parts takes comments apart through the unit's pipeline and marks them
// taken. A run of line comments on consecutive lines feeds the pipeline
// as one text, so a carrier's continuation folds across the lines the
// way the author reads it. A block comment, a Javadoc comment among
// them, feeds it alone.
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
			take(l.u.Comment(strings.Join(run, "\n"), runAt))
			run = run[:0]
		}
	}
	for _, c := range comments {
		l.taken[c.Pos()] = true
		if c.Kind() == l.v.blockComment {
			flush()
			take(l.u.Comment(c.Text(), c.Pos()))
			continue
		}
		if len(run) > 0 && c.Pos().Line != runAt.Line+len(run) {
			flush()
		}
		if len(run) == 0 {
			runAt = c.Pos()
		}
		run = append(run, c.Text())
	}
	flush()
	return parts
}

// skip takes a declaration's leading and trailing comments without
// reading them, and passes the sweep over the declaration: the comments
// and carriers of a declaration the load leaves out leave with it.
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
// and the markers of its modifiers, under [UnaddressedCarrier]. what
// describes the node, a subject that the model cannot address.
func (l *lowering) refuse(n treesitter.Node, what string) {
	parts, _ := l.declParts(n)
	refuseCarriers(l.u, parts.Carriers, what)
	_, sugars := l.annotationsIn(l.firstOf(n, l.v.modifiers))
	for _, s := range sugars {
		l.u.Errorf(UnaddressedCarrier, s.Pos,
			"the marker %s is on %s, which the model cannot address. Move it to a declaration",
			strings.Join(s.Path, nameSeparator), what)
	}
}

// sweep refuses the carriers of every comment no declaration took,
// positioned, so an authored directive never vanishes into silence. It
// passes over a declaration the load left out, whose carriers leave with
// it, and over a method's and a constructor's body, whose comments belong
// to statements the model does not contain.
func (l *lowering) sweep(n treesitter.Node) {
	for child := range n.NamedChildren() {
		switch k := child.Kind(); {
		case l.taken[child.Pos()] || k == l.v.block || k == l.v.constructorBody:
			continue
		case l.comment(child):
			refuseCarriers(l.u, l.u.Comment(child.Text(), child.Pos()).Carriers, "a comment no declaration takes")
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
