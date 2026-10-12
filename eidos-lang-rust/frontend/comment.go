// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"strings"

	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
)

// lineEnd is the end of the source of a doc line comment: the newline that
// the grammar takes into the comment.
const lineEnd = "\r\n"

// unattached describes a comment that no declaration takes, in the refusal
// of a carrier in it.
const unattached = "a comment that no declaration takes"

// lastLine returns the line on which the source of a node ends. Where the
// node takes the newline of its last line, as a doc line comment does, the
// line is the line before the end of the node.
func lastLine(n treesitter.Node) int {
	end := n.End()
	if end.Col == 1 && end.Line > n.Pos().Line {
		return end.Line - 1
	}
	return end.Line
}

// commentWalk returns the comment walk over the kinds of v. A run above an
// item passes over the outer attributes between the item and its comments,
// and over the visibility of a positional field. An inner doc comment
// documents its module, so it ends a run. An outer doc comment documents
// the item after it, as rustc reads it, so a doc comment never trails. The
// text of a line comment in a run leaves out the newline that a doc line
// comment takes. The sweep passes over a function body, whose comments
// belong to statements that the model does not contain.
func commentWalk(v *vocabulary) treesitter.Comments {
	return treesitter.Comments{
		IsComment: func(n treesitter.Node) bool { return n.Kind() == v.lineComment || n.Kind() == v.blockComment },
		IsBlock:   func(n treesitter.Node) bool { return n.Kind() == v.blockComment },
		Passes: func(n treesitter.Node) bool {
			return n.Kind() == v.attributeItem || n.Kind() == v.visibilityModifier
		},
		Stops: func(n treesitter.Node) bool { return !n.Child(v.fieldInner).IsZero() },
		Trails: func(n treesitter.Node) bool {
			return n.Child(v.fieldOuter).IsZero() && n.Child(v.fieldInner).IsZero()
		},
		LastLine: lastLine,
		Text:     func(n treesitter.Node) string { return strings.TrimRight(n.Text(), lineEnd) },
		Opaque:   func(n treesitter.Node) bool { return n.Kind() == v.block },
	}
}

// declParts returns what the comments and the doc attributes of an item
// state: the documentation of its leading comments and then of its #[doc]
// attributes, and the carriers of its leading and its trailing comment.
// comment is the documentation of the trailing comment on one line, and
// empty where there is none.
func (l *lowering) declParts(n treesitter.Node, a attributes) (parts plugin.CommentParts, comment string) {
	parts = l.parts(l.comments.Leading(n))
	parts.Docs = append(parts.Docs, a.docs...)
	if tail := l.comments.Trailing(n); !tail.IsZero() {
		after := l.parts([]treesitter.Node{tail})
		parts.Carriers = append(parts.Carriers, after.Carriers...)
		comment = strings.Join(after.Docs, " ")
	}
	return parts, comment
}

// parts takes the texts that comments make apart through the pipeline of
// the unit, and the walk marks the comments taken. A run of line comments
// on consecutive lines is one text, so the continuation of a carrier folds
// across the lines as the author reads it. A block comment is a text of
// its own.
func (l *lowering) parts(comments []treesitter.Node) plugin.CommentParts {
	var parts plugin.CommentParts
	l.comments.Runs(comments, func(text string, at position.Pos) {
		one := l.w.u.Comment(text, at)
		parts.Docs = append(parts.Docs, one.Docs...)
		parts.Carriers = append(parts.Carriers, one.Carriers...)
	})
	return parts
}

// skip takes the leading and the trailing comments of an item without
// reading them, and passes the sweep over the item. The comments and the
// carriers of an item that the load leaves out leave with it.
func (l *lowering) skip(n treesitter.Node) {
	l.comments.Take(l.comments.Leading(n)...)
	l.comments.Take(l.comments.Trailing(n), n)
}

// refuse reports the carriers of the leading and the trailing comments of
// a node, and the markers among its attributes a, under
// [UnaddressedCarrier]. The node is a subject that the model cannot
// address, and what describes it.
func (l *lowering) refuse(n treesitter.Node, a attributes, what string) {
	parts, _ := l.declParts(n, attributes{})
	l.w.u.RefuseCarriers(UnaddressedCarrier, parts.Carriers, what)
	l.w.u.RefuseSugar(UnaddressedCarrier, a.sugars, pathSeparator, what)
}

// comment reports whether a node is a line or a block comment.
func (l *lowering) comment(n treesitter.Node) bool {
	k := n.Kind()
	return k == l.v.lineComment || k == l.v.blockComment
}
