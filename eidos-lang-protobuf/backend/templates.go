// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// FileTemplate is the skeleton of a protobuf file. It writes the
// documentation of the header and the version statement, the package
// statement of the clause of the file, the import block, the file options
// and the declarations, with a blank line between two parts. A file without
// a clause has no package statement. The skeleton records the imports of the
// header before it writes the import block. The output contract writes the
// marker of generated code after the formatter runs, so the template does
// not contain it.
const FileTemplate = "{{$h := " + render.BuiltinHeader + "}}{{" + FuncUseImports + " $h}}" +
	"{{" + FuncDocs + " $h.Doc}}{{" + FuncSyntax + " $h}}\n" +
	"{{with .Pkg.Name}}\npackage {{.}};\n{{end}}" +
	"{{with " + FuncImportBlock + " $h}}\n{{.}}{{end}}" +
	"{{with " + FuncFileOptions + " $h}}\n{{.}}{{end}}" +
	"{{with " + render.BuiltinDecls + "}}\n{{.}}{{end}}"

// blockItems writes the members of a block: the text of each member, and
// each nested declaration through its kind template behind its indentation.
const blockItems = "{{range $b.Items}}{{if .Nested}}{{" + render.BuiltinNested + " .Indent .Nested}}\n" +
	"{{else}}{{.Text}}{{end}}{{end}}"

// BlockTemplate renders a message, an enum, a oneof and a service through
// [Vocabulary.Block]: the documentation, the head, the opening brace with the
// trailing comment, the members and the closing brace. A nested declaration
// renders through the same template, and its block reads the scope that the
// block of its message recorded.
const BlockTemplate = "{{$b := " + FuncBlock + " " + render.BuiltinHeader + " .}}{{" + FuncDocs + " $b.Doc}}" +
	"{{$b.Head}} {{$b.Open}}" + blockItems + "}\n"

// KindTemplates returns a map from each emit kind that protobuf declares to
// the template that renders it. A message, an enum, a oneof and a service
// render through [BlockTemplate]. The cluster renders the extensions at file
// level through the extend group, and [RefusedKinds] refuses every other
// kind that the canonical fixture emits. KindTemplates builds a new map on
// every call for the caller to keep.
func KindTemplates() map[symbol.Kind]string {
	return map[symbol.Kind]string{
		symbol.KindStruct:    BlockTemplate,
		symbol.KindEnum:      BlockTemplate,
		symbol.KindSum:       BlockTemplate,
		symbol.KindInterface: BlockTemplate,
	}
}

// RefusedKinds returns a map from each emit kind that protobuf does not
// declare to the reason. The render reports the reason with every
// declaration of the kind that it skips. RefusedKinds builds a new map on
// every call for the caller to keep.
func RefusedKinds() map[symbol.Kind]string {
	return map[symbol.Kind]string{
		symbol.KindFunction: "a protobuf schema declares no function",
		symbol.KindMethod:   "a protobuf schema declares an rpc in its service alone",
		symbol.KindAlias:    "a protobuf schema declares no alias of a type",
		symbol.KindConstant: "a protobuf schema declares no value",
		symbol.KindVariable: "a protobuf schema declares no value",
	}
}
