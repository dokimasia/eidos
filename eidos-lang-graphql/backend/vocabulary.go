// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"bytes"
	"text/template"

	"go.dokimi.dev/eidos/lang/textfmt"
	"go.dokimi.dev/eidos/sdk/render"
)

// The names of the vocabulary are constants, so a template and the map of
// its helpers use the same name. The names of the documentation helpers
// are the names that every backend declares.
const (
	// FuncDocs is the template name of [Vocabulary.Docs].
	FuncDocs = render.HelperDocs
	// FuncWrapDocs is the template name of [WrapDocs].
	FuncWrapDocs = render.HelperWrapDocs
	// FuncStruct is the template name of [Vocabulary.Struct].
	FuncStruct = "structdecl"
	// FuncInterface is the template name of [Vocabulary.Interface].
	FuncInterface = "interfacedecl"
	// FuncEnum is the template name of [Vocabulary.Enum].
	FuncEnum = "enumdecl"
	// FuncAlias is the template name of [Vocabulary.Alias].
	FuncAlias = "aliasdecl"
	// FuncFunction is the template name of [Vocabulary.Function].
	FuncFunction = "functiondecl"
	// FuncSchema is the template name of [Vocabulary.Schema].
	FuncSchema = "schema"
	// FuncExtension is the template name of [Vocabulary.Extension].
	FuncExtension = "extension"
)

// Vocabulary is the template vocabulary of one render worker. Its writers
// write the declarations of the kind templates, the schema statement of the
// file skeleton and the blocks of the extend group, and [Vocabulary.Docs]
// writes the descriptions of the templates of a generator. Each writer
// writes its text into one buffer that the vocabulary reuses, and returns a
// copy of the text. The zero value is ready for use.
//
// # Concurrency
//
// A Vocabulary is not safe for concurrent use, because its writers share
// the buffer. The pass binds one vocabulary to each of its workers.
//
// # Allocation contract
//
// A writer allocates the text that it returns, once, and the quoted text of
// each description of one line. The buffer grows until the longest text of
// the vocabulary fits. A writer that writes nothing allocates nothing.
type Vocabulary struct {
	b bytes.Buffer
}

// Funcs returns the template vocabulary that the file skeleton, the kind
// templates and the extend template call, bound to a new [Vocabulary]. A
// GraphQL file has no imports, so no helper records into the import set.
// Every helper takes a fixed list of arguments, because text/template
// allocates the list of a variadic call.
func Funcs(*render.ImportSet) template.FuncMap {
	v := &Vocabulary{}
	return template.FuncMap{
		FuncDocs:      v.Docs,
		FuncWrapDocs:  WrapDocs,
		FuncStruct:    v.Struct,
		FuncInterface: v.Interface,
		FuncEnum:      v.Enum,
		FuncAlias:     v.Alias,
		FuncFunction:  v.Function,
		FuncSchema:    v.Schema,
		FuncExtension: v.Extension,
	}
}

// Docs writes the documentation lines of a declaration at file level as its
// description: one line as a string, and two or more lines as a block
// string. No lines write nothing. The writers of the declarations write the
// description themselves, so the kind templates do not call Docs.
func (v *Vocabulary) Docs(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	v.b.Reset()
	writeDescription(&v.b, lines, "")
	return v.b.String()
}

// WrapDocs wraps text at [textfmt.DocsWidth] into the lines that
// [Vocabulary.Docs] writes. A GraphQL description is CommonMark, so the
// wrap starts no line with a block opener. It allocates what
// [textfmt.Wrap] allocates.
func WrapDocs(text string) []string {
	return textfmt.Wrap(text, textfmt.DocsWidth, textfmt.MarkupMarkdown)
}
