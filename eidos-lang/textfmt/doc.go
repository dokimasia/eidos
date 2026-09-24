// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package textfmt writes the comment forms and import statements the
// C-family targets share, and normalizes rendered source for the
// targets that run no formatter of their own.
//
// [LineDocs], [BlockDocs], [Inline] and [Marked] write documentation,
// trailing comments and structured markers. The comment writers split
// a text at its line breaks or fold it onto one line, and escape a
// block-comment delimiter inside a block comment, so no text ends a
// comment early. [ImportLines] writes one import statement per
// collected entry.
//
// [Normalize] is the finaliser a backend without a hermetic
// pretty-printer declares. It strips trailing whitespace, collapses
// runs of blank lines to one and ends the file with exactly one
// newline. It never reorders, rewraps or reindents: the output bytes
// are the template's, minus trailing whitespace and extra blank
// lines.
//
// # Dependency position
//
// lang/textfmt imports sdk/render, sdk/symbol and the Go stdlib.
package textfmt
