// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package parse

import (
	"github.com/bufbuild/protocompile/experimental/ast"
	"github.com/bufbuild/protocompile/experimental/parser"
	"github.com/bufbuild/protocompile/experimental/report"
	"github.com/bufbuild/protocompile/experimental/source"
	"github.com/bufbuild/protocompile/experimental/token/keyword"

	"go.dokimi.dev/eidos/lang/protobuf"
)

// File parses the proto source text of the file at path with the
// experimental parser of protocompile. The report suppresses warnings,
// because each warning of the parser is about the style of a schema.
//
// The parser knows the editions up to 2024, and edition 2026 has the
// grammar of edition 2024. A file whose edition statement states 2026 is
// therefore parsed from the copy that [Text] returns, and File reports that
// it parsed such a copy. Both values have four bytes, so every position in
// the tree of the copy is the position in the file. A syntax statement that
// states 2026 parses from the text of the file, and the caller reports the
// version.
//
// # Allocation contract
//
// File allocates what the parser allocates for one parse. A file of
// edition 2026 allocates the copy of its text and a second parse.
func File(path, text string) (tree *ast.File, r *report.Report, patched bool) {
	tree, r = parsed(path, text)
	copied, patched := copyOf(tree, text)
	if !patched {
		return tree, r, false
	}
	tree, r = parsed(path, copied)
	return tree, r, true
}

// Text returns the text that the parser and the compiler of protocompile
// read for the file at path: text itself, and for a file whose edition
// statement states 2026 a copy that states 2024, with true. Both know the
// editions up to 2024, so the copy checks the grammar, the references and
// the options of a file of edition 2026, and not the defaults that 2026
// changes. A text whose edition statement does not parse returns as it is.
//
// # Allocation contract
//
// Text allocates what the parser allocates for one parse, and the copy of a
// file of edition 2026.
func Text(path, text string) (string, bool) {
	tree, _ := parsed(path, text)
	return copyOf(tree, text)
}

// parsed parses text as the file at path into a tree and a report that
// suppresses warnings. The report contains every error of the parse, so the
// result of the parser, which reports whether the parse found none, adds
// nothing to it.
func parsed(path, text string) (*ast.File, *report.Report) {
	r := &report.Report{SuppressWarnings: true}
	tree, _ := parser.Parse(path, source.NewFile(path, text), r)
	return tree, r
}

// copyOf returns the copy of text whose edition statement states 2024 and
// true, where the tree of text has an edition statement that states 2026,
// and text and false otherwise.
func copyOf(tree *ast.File, text string) (string, bool) {
	statement := tree.Syntax()
	content := statement.Value().AsLiteral().AsString().RawContent()
	if statement.Keyword() != keyword.Edition || content.Text() != protobuf.Edition2026 {
		return text, false
	}
	return text[:content.Start] + protobuf.Edition2024 + text[content.End:], true
}
