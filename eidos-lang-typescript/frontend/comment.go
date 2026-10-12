// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"bytes"
	"strings"

	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
)

// lineMarker opens a line comment, and a run of line comments on
// consecutive lines reads as one text.
const lineMarker = "//"

// unattached describes a comment that no declaration takes, in the refusal
// of a carrier in it.
const unattached = "a comment that no declaration takes"

// commentWalk returns the comment walk over the kinds of v. A comment is a
// block comment where its text does not open with //. A run passes over
// the decorators between a class member and its comments, and the grammar
// folds a trailing comment into a declaration that ends without a
// semicolon. The sweep passes over a statement block, whose comments
// belong to statements that the model does not contain. It reads the block
// of a namespace, an ambient module and a declare global block, which
// contains declarations.
func commentWalk(v *vocabulary) treesitter.Comments {
	return treesitter.Comments{
		IsComment: func(n treesitter.Node) bool { return n.Kind() == v.comment },
		IsBlock:   func(n treesitter.Node) bool { return !bytes.HasPrefix(n.Bytes(), []byte(lineMarker)) },
		Passes:    func(n treesitter.Node) bool { return n.Kind() == v.decorator },
		Folded:    true,
		Opaque: func(n treesitter.Node) bool {
			if n.Kind() != v.statementBlock {
				return false
			}
			k := n.Parent().Kind()
			return k != v.internalModule && k != v.module && k != v.ambientDeclaration
		},
	}
}

// declParts returns what the comments of a declaration state: the
// documentation of its leading comments, and the carriers of its leading
// and its trailing comment. comment is the documentation of the trailing
// comment on one line, and empty where there is none.
func (l *lowering) declParts(outermost treesitter.Node) (parts plugin.CommentParts, comment string) {
	parts = l.parts(l.comments.Leading(outermost))
	if tail := l.comments.Trailing(outermost); !tail.IsZero() {
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
		parts.Annotations = append(parts.Annotations, one.Annotations...)
	})
	return parts
}

// skip takes the leading and the trailing comments of a node without
// reading them. They are the comments of a declaration that the load
// leaves out, and their carriers leave with the declaration.
func (l *lowering) skip(outermost treesitter.Node) {
	l.comments.Take(l.comments.Leading(outermost)...)
	l.comments.Take(l.comments.Trailing(outermost))
}
