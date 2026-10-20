// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"strings"

	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
)

// commentMark opens a comment of Python.
const commentMark = "#"

// typeCommentMark opens the text of a type comment after its #, as in
// # type: int, and ignoreDirective is the text of the type comment that
// silences a type checker, as in # type: ignore[attr-defined].
const (
	typeCommentMark = "type:"
	ignoreDirective = "ignore"
)

// blanks are the characters that separate the parts of a comment.
const blanks = " \t"

// unattached describes a comment that no declaration takes, in the refusal
// of a carrier in it.
const unattached = "a comment that no declaration takes"

// commentWalk returns the comment walk over the kinds of v. Python has line
// comments alone, so no comment is a block. The sweep passes over the body
// of a callable, whose comments belong to statements that the model does
// not contain, and reads the body of a class and the blocks of the
// statements of a module, which contain declarations.
func commentWalk(v *vocabulary) treesitter.Comments {
	return treesitter.Comments{
		IsComment: func(n treesitter.Node) bool { return n.Kind() == v.comment },
		IsBlock:   func(treesitter.Node) bool { return false },
		Opaque: func(n treesitter.Node) bool {
			return n.Kind() == v.block && n.Parent().Kind() == v.functionDefinition
		},
	}
}

// declParts returns the carriers of the comments of a declaration: the run
// of comments directly above outermost, which is the declaration or its
// decorated definition, and the trailing comment of the declaration. The
// documentation of Python is its docstrings, so the text of a comment that
// is no carrier is no documentation. comment is the text of the trailing
// comment without its #, and empty where there is none. The trailing
// comment of a class or a callable is the comment on the line of its name,
// and the trailing comment of any other statement is the comment after it
// on its last line.
func (l *lowering) declParts(outermost, decl treesitter.Node) (carriers []plugin.Carrier, comment string) {
	carriers = l.parts(l.comments.Leading(outermost)).Carriers
	tail := l.comments.Trailing(outermost)
	if k := decl.Kind(); k == l.v.classDefinition || k == l.v.functionDefinition {
		tail = l.headComment(decl)
	}
	if !tail.IsZero() {
		after := l.parts([]treesitter.Node{tail})
		carriers = append(carriers, after.Carriers...)
		comment = strings.Join(after.Docs, " ")
	}
	return carriers, comment
}

// headComment returns the comment on the line of the name of a class or a
// callable, and the zero Node where there is none. The grammar puts the
// comment of a definition with a body on its own lines before the body, and
// the comment of a definition whose body is on the line of its name at the
// end of the body. A type comment is the signature of a callable and no
// comment of it.
func (l *lowering) headComment(decl treesitter.Node) treesitter.Node {
	line := decl.Child(l.v.fieldName).Pos().Line
	for _, parent := range [...]treesitter.Node{decl, decl.Child(l.v.fieldBody)} {
		for child := range parent.NamedChildren() {
			if child.Kind() != l.v.comment || child.Pos().Line != line {
				continue
			}
			text := strings.TrimLeft(strings.TrimPrefix(child.Text(), commentMark), blanks)
			if !strings.HasPrefix(text, typeCommentMark) {
				return child
			}
		}
	}
	return treesitter.Node{}
}

// typeComment returns the text of the type comment of a node after its
// mark, as int for # type: int, and the position of the text. The type
// comment of an assignment is the comment after it on its last line, and
// the type comment of a callable is a comment before its body. A comment of
// another text is no type comment. The comment # type: ignore is a
// directive of a type checker and no type. typeComment takes the comment
// that it returns, and returns the empty text where the node has none.
func (l *lowering) typeComment(n treesitter.Node) (string, position.Pos) {
	var candidates []treesitter.Node
	if n.Kind() == l.v.functionDefinition {
		for child := range n.NamedChildren() {
			if child.Kind() == l.v.comment {
				candidates = append(candidates, child)
			}
		}
	} else if tail := l.comments.Trailing(n); !tail.IsZero() {
		candidates = append(candidates, tail)
	}
	for _, c := range candidates {
		raw := c.Text()
		lead := strings.TrimLeft(strings.TrimPrefix(raw, commentMark), blanks)
		body, typed := strings.CutPrefix(lead, typeCommentMark)
		value := strings.TrimLeft(body, blanks)
		text := strings.TrimRight(value, blanks)
		if !typed || text == "" || strings.HasPrefix(text, ignoreDirective) {
			continue
		}
		l.comments.Take(c)
		at := c.Pos()
		at.Col += len(raw) - len(value)
		return text, at
	}
	return "", position.Pos{}
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

// refuse reports the carriers of the comments of a statement under
// [UnaddressedCarrier]. The statement is a subject that the model cannot
// address, and what describes it.
func (l *lowering) refuse(outermost treesitter.Node, what string) {
	carriers, _ := l.declParts(outermost, outermost)
	l.u.RefuseCarriers(UnaddressedCarrier, carriers, what)
}
