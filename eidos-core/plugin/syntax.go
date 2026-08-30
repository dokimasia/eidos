// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package plugin

// CommentSyntax is a language's comment forms, declared once per
// satellite and shared by the kits: the frontend strips comments
// with it, and the output contract writes the generated-file
// header through it.
type CommentSyntax struct {
	// Line holds the line-comment openers, first one canonical.
	Line []string
	// Blocks holds the block forms.
	Blocks []CommentBlock
}

// CommentBlock is one block-comment form, gutter included, so a
// doc block's continuation lines strip and render the same way.
type CommentBlock struct {
	Open   string
	Close  string
	Gutter string
}
