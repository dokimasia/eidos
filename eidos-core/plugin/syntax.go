// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

// CommentSyntax is a language's comment forms, declared once per
// satellite and shared by the kits: the frontend strips comments
// with it, and the output contract writes the generated-file
// header through it. A data family states the comment forms of the
// format of its file in [Output.Comment].
type CommentSyntax struct {
	// Line holds the line-comment openers, first one canonical.
	Line []string
	// Blocks holds the block forms.
	Blocks []CommentBlock
	// Directives declares that the language's comments carry tool
	// directives — the go:build kin, an open tool:name shape the
	// host toolchain itself defines. The comment split lowers a
	// directive line as an annotation only under this declaration;
	// a language without the convention keeps every such line as
	// documentation, because prose spelling word:word is prose.
	Directives bool
}

// IsZero reports whether s is the zero syntax: Line and Blocks are empty,
// and Directives is false. A data family without a comment syntax has
// the zero syntax. It allocates nothing.
func (s CommentSyntax) IsZero() bool {
	return len(s.Line) == 0 && len(s.Blocks) == 0 && !s.Directives
}

// CommentBlock is one block-comment form, gutter included, so a
// doc block's continuation lines strip and render the same way.
type CommentBlock struct {
	Open   string
	Close  string
	Gutter string
}
