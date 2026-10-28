// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"strings"
	"text/template"

	"go.dokimi.dev/eidos/lang/textfmt"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The names of the vocabulary are constants, so a template and the map of
// its helpers use the same name. The names of the documentation helpers
// are the names that every backend declares.
const (
	// FuncDocs is the template name of [Docs].
	FuncDocs = render.HelperDocs
	// FuncWrapDocs is the template name of [WrapDocs].
	FuncWrapDocs = render.HelperWrapDocs
	// FuncBlock is the template name of [Vocabulary.Block]. The word block
	// is a keyword of text/template.
	FuncBlock = "blockof"
	// FuncExtension is the template name of [Vocabulary.Extension].
	FuncExtension = "extension"
	// FuncSyntax is the template name of [Syntax].
	FuncSyntax = "syntax"
	// FuncUseImports is the template name of [Vocabulary.UseImports].
	FuncUseImports = "useimports"
	// FuncFileOptions is the template name of [FileOptions].
	FuncFileOptions = "fileoptions"
	// FuncImportBlock is the template name of [Vocabulary.ImportBlock].
	FuncImportBlock = "importblock"
)

// The layout of a block. A member is two spaces deeper than its block, as
// buf format writes it, and a nested block is two spaces deeper again.
const (
	memberIndent = "  "
	nestedIndent = memberIndent + memberIndent
)

// The syntax of a comment: the mark of a line comment, the gap between the
// mark and its text, and the opening brace of a block, alone and at the end
// of its line.
const (
	commentMark   = "//"
	commentGap    = " "
	blockOpen     = "{"
	blockOpenLine = blockOpen + "\n"
)

// space separates two words of a statement.
const space = " "

// lineFold writes each line break of a trailing comment as a space, because
// a line comment ends at the end of its line.
var lineFold = strings.NewReplacer("\r\n", commentGap, "\n", commentGap)

// Vocabulary is the template vocabulary of one file: the blocks of its
// declarations, its version statement, its imports and its options. The
// pass binds one vocabulary to each of its workers, with the import set of
// the worker, so the writer of a vocabulary records the imports of the file
// under render.
//
// A vocabulary passes the scope of a block to the blocks of its nested
// declarations. The block of a message records the scope of its members
// for each nested declaration that it lists, and the template renders each
// of them through its kind template, whose block reads the record. A block
// of a declaration without a record is at file level, and it starts the
// records of its declaration anew.
//
// # Concurrency
//
// A Vocabulary is not safe for concurrent use, because its records and its
// import set are not.
//
// # Allocation contract
//
// [NewVocabulary] allocates the vocabulary, its two record maps and its
// writer. A block allocates its body, its items and the text of each
// member, and the first record of a nested declaration grows a map. A
// refusal allocates its error.
type Vocabulary struct {
	writer Writer
	set    *render.ImportSet
	// parents records the context of each nested declaration that a block
	// listed, which the block of the nested declaration reads.
	parents map[symbol.Symbol]parent
	// groups records the message of each field that proto2 writes as a
	// group, which the block of a oneof reads for its members.
	groups map[*emit.Field]*emit.Struct
}

// parent is the context of a nested declaration: the scope of the block
// that lists it, the message that declares it, and the field of a group.
type parent struct {
	scope Scope
	// host is the message that declares the declaration, and nil at file
	// level and in an extend block at file level.
	host *emit.Struct
	// group is the field of a message that proto2 writes as a group, and
	// nil for every other declaration.
	group *emit.Field
	// member marks the group of a member of a oneof, which takes no label.
	member bool
}

// NewVocabulary returns the vocabulary of the file whose import set is set.
// set must not be nil.
func NewVocabulary(set *render.ImportSet) *Vocabulary {
	return &Vocabulary{
		writer:  NewWriter(set),
		set:     set,
		parents: map[symbol.Symbol]parent{},
		groups:  map[*emit.Field]*emit.Struct{},
	}
}

// Funcs returns the template vocabulary that the file skeleton, the kind
// templates and the extend template call. The helpers record each import
// that a reference or the header needs into set, the import set of one
// file. Every helper takes a fixed list of arguments, because text/template
// allocates the list of a variadic call.
func Funcs(set *render.ImportSet) template.FuncMap {
	v := NewVocabulary(set)
	return template.FuncMap{
		FuncDocs:        Docs,
		FuncWrapDocs:    WrapDocs,
		FuncBlock:       v.Block,
		FuncExtension:   v.Extension,
		FuncSyntax:      Syntax,
		FuncUseImports:  v.UseImports,
		FuncImportBlock: v.ImportBlock,
		FuncFileOptions: FileOptions,
	}
}

// Docs writes the documentation lines of a declaration as line comments,
// one comment for each line and for each line break inside a line. An empty
// line writes the mark alone, so no comment ends in a space. Docs allocates
// the comments once, and nothing for no lines.
func Docs(lines []string) string {
	return comments(lines, "")
}

// WrapDocs wraps text at [textfmt.DocsWidth] into the lines that [Docs]
// writes. A protobuf comment is text, so no word of the text is markup. It
// allocates what [textfmt.Wrap] allocates.
func WrapDocs(text string) []string {
	return textfmt.Wrap(text, textfmt.DocsWidth, textfmt.MarkupNone)
}

// comments writes the documentation lines as line comments behind indent,
// by the rules of [Docs]. It sizes its buffer once, for the mark, the gap and
// the line break of each part of each line.
func comments(lines []string, indent string) string {
	if len(lines) == 0 {
		return ""
	}
	size := 0
	for _, text := range lines {
		parts := strings.Count(text, "\n") + 1
		size += len(text) + parts*(len(indent)+len(commentMark)+len(commentGap)+1)
	}
	var b strings.Builder
	b.Grow(size)
	for _, text := range lines {
		for line := range strings.SplitSeq(text, "\n") {
			b.WriteString(indent)
			b.WriteString(commentMark)
			if line = strings.TrimSuffix(line, "\r"); line != "" {
				b.WriteString(commentGap)
				b.WriteString(line)
			}
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// trailing writes the trailing comment of a member after a space, with each
// line break as a space, and nothing for an empty comment.
func trailing(comment string) string {
	if comment == "" {
		return ""
	}
	return commentGap + commentMark + commentGap + lineFold.Replace(comment)
}
