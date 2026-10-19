// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// FileTemplate is the skeleton of a GraphQL file: the schema statement of
// the header and then the declarations. A file has no imports, because a
// GraphQL schema has one namespace. The output contract writes the marker of
// generated code after the formatter runs, so the template does not contain
// it.
const FileTemplate = "{{" + FuncSchema + " " + render.BuiltinHeader + "}}{{" + render.BuiltinDecls + "}}"

// The kind templates write a declaration through the writer of its kind,
// which writes the description of the declaration before it. One call of a
// helper renders one declaration, because text/template allocates the
// results of every call.
const (
	structTemplate    = "{{" + FuncStruct + " .}}"
	interfaceTemplate = "{{" + FuncInterface + " .}}"
	enumTemplate      = "{{" + FuncEnum + " .}}"
	aliasTemplate     = "{{" + FuncAlias + " .}}"
	functionTemplate  = "{{" + FuncFunction + " .}}"
)

// KindTemplates returns a map from each emit kind that GraphQL declares at
// file level to the template that renders it: a struct, an interface, an
// enum, an alias and a function. The lowering writes a sum as an alias and
// structs, and the cluster renders the fields and the methods at file level
// through the extend group. KindTemplates builds a new map on every call for
// the caller to keep.
func KindTemplates() map[symbol.Kind]string {
	return map[symbol.Kind]string{
		symbol.KindStruct:    structTemplate,
		symbol.KindInterface: interfaceTemplate,
		symbol.KindEnum:      enumTemplate,
		symbol.KindAlias:     aliasTemplate,
		symbol.KindFunction:  functionTemplate,
	}
}

// RefusedKinds returns a map from each emit kind that GraphQL does not
// declare to the reason. The render reports the reason with every
// declaration of the kind that it skips. RefusedKinds builds a new map on
// every call for the caller to keep.
func RefusedKinds() map[symbol.Kind]string {
	return map[symbol.Kind]string{
		symbol.KindConstant: "a GraphQL schema declares no constant",
		symbol.KindVariable: "a GraphQL schema declares no variable",
	}
}
