// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package textfmt writes the comment forms and import statements the
// C-family targets share, and normalizes rendered source for the
// targets that run no formatter of their own.
//
// [LineDocs], [BlockDocs], [Inline], [Marked] and [MarkedInline] write
// documentation, trailing comments and structured markers. The comment
// writers split a text at its line breaks or fold it onto one line, and
// escape a block-comment delimiter inside a block comment, so no text
// ends a comment early. [ImportLines] writes each distinct import
// statement that a file's collected entries spell once.
//
// [Wrap] wraps a text into the lines of documentation at a width, such as
// [DocsWidth]. The [Markup] of a target names the words that its
// documentation syntax reads as the start of a block, and the wrap starts
// no line with such a word.
//
// [Normalize] is the finaliser a backend without a hermetic
// pretty-printer declares. It strips trailing whitespace, collapses
// runs of blank lines to one and ends the file with exactly one
// newline. It never reorders, rewraps or reindents: the output bytes
// are the template's, minus trailing whitespace and extra blank
// lines.
//
// # Allocation contract
//
// Every function sizes one buffer to its result before it writes, so a
// call allocates its result once. Each function's docblock names the
// inputs that allocate more. [Wrap] allocates its list of lines once, and
// a line only where it joins words that other spaces separate.
//
// # Dependency position
//
// lang/textfmt imports sdk/render, sdk/symbol and the Go stdlib.
package textfmt
