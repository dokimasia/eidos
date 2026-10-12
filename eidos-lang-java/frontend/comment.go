// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"strings"

	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
)

// unattached describes a comment that no declaration takes, in the refusal
// of a carrier in it.
const unattached = "a comment that no declaration takes"

// commentWalk returns the comment walk over the kinds of v. A comment is a
// line or a block comment, and a Javadoc comment is a block comment. The
// comments among the modifiers of a declaration, as between an annotation
// and its keyword, follow the run above it. The sweep passes over the body
// of a method and of a constructor, whose comments belong to statements
// that the model does not contain.
func commentWalk(v *vocabulary) treesitter.Comments {
	return treesitter.Comments{
		IsComment: func(n treesitter.Node) bool { return n.Kind() == v.lineComment || n.Kind() == v.blockComment },
		IsBlock:   func(n treesitter.Node) bool { return n.Kind() == v.blockComment },
		Inner:     func(n treesitter.Node) treesitter.Node { return n.FirstOf(v.modifiers) },
		Opaque:    func(n treesitter.Node) bool { return n.Kind() == v.block || n.Kind() == v.constructorBody },
	}
}

// declParts returns what the comments of a declaration state: the
// documentation of its leading comments, and the carriers of its leading
// and its trailing comment. comment is the documentation of the trailing
// comment on one line, and empty where there is none.
func (l *lowering) declParts(n treesitter.Node) (parts plugin.CommentParts, comment string) {
	parts = l.parts(l.comments.Leading(n))
	if tail := l.comments.Trailing(n); !tail.IsZero() {
		after := l.parts([]treesitter.Node{tail})
		parts.Carriers = append(parts.Carriers, after.Carriers...)
		comment = strings.Join(after.Docs, " ")
	}
	return parts, comment
}

// parts takes the texts that comments make apart through the pipeline of
// the unit, and the walk marks the comments taken.
func (l *lowering) parts(comments []treesitter.Node) plugin.CommentParts {
	var parts plugin.CommentParts
	l.comments.Runs(comments, func(text string, at position.Pos) {
		one := l.u.Comment(text, at)
		parts.Docs = append(parts.Docs, one.Docs...)
		parts.Carriers = append(parts.Carriers, one.Carriers...)
	})
	return parts
}

// skip takes the leading and the trailing comments of a declaration
// without reading them, and passes the sweep over the declaration. The
// comments and the carriers of a declaration that the load leaves out
// leave with it.
func (l *lowering) skip(n treesitter.Node) {
	l.comments.Take(l.comments.Leading(n)...)
	l.comments.Take(l.comments.Trailing(n), n)
}

// refuse reports the carriers of the leading and the trailing comments of
// a node, and the markers of its modifiers, under [UnaddressedCarrier].
// The node is a subject that the model cannot address, and what describes
// it.
func (l *lowering) refuse(n treesitter.Node, what string) {
	parts, _ := l.declParts(n)
	l.u.RefuseCarriers(UnaddressedCarrier, parts.Carriers, what)
	_, sugars := l.annotationsIn(n.FirstOf(l.v.modifiers))
	l.u.RefuseSugar(UnaddressedCarrier, sugars, nameSeparator, what)
}

// comment reports whether a node is a line or a block comment.
func (l *lowering) comment(n treesitter.Node) bool {
	k := n.Kind()
	return k == l.v.lineComment || k == l.v.blockComment
}
