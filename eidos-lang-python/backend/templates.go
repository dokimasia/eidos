// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// FileTemplate is the skeleton of a Python file. The helper module writes
// the start of the module from the header of the file, and then the
// declarations, which it reads to leave out the imports that the file does
// not use. [Layout.Format] lays out the blank lines between the parts and
// the declarations as ruff's formatter does. The output contract writes the
// marker of generated code after the formatter runs, so the template does
// not contain it.
const FileTemplate = "{{" + FuncModule + " " + render.BuiltinHeader + " " + render.BuiltinDecls + "}}"

// The parts of the template of a class after its head, inside the action
// of the class writer: each nested type one level inside the class, the
// lines of the fields, and each method, with a blank line before each part.
// A method places its content through the memberbody builtin. A part that
// ranges over a list opens with the list, so a class without the part runs
// no range.
const (
	classTypes   = "{{with .Types}}{{range .}}\n{{" + render.BuiltinNested + ` "    " .}}` + "\n{{end}}{{end}}"
	classFields  = "{{with .Fields}}\n{{.}}{{end}}"
	classMethods = "{{with .Methods}}{{range .}}\n{{" + FuncMethod + " $ . (" + render.BuiltinMemberBody +
		" .)}}{{end}}{{end}}"
)

// Each kind template renders one emit kind that Python declares at the
// level of a module, and calls the writer of its kind first with the header
// of the file. The writer starts the declaration before it writes, and a
// template without members renders through the one call, because
// text/template allocates the results of every call. No template renders a
// method. Python declares a method inside its class, so a method renders
// through the template of its host, and the backend refuses a standalone
// method.
const (
	// StructTemplate renders a class through [Writer.Struct]: its head, with
	// its decorators, its bases and its docstring, and then its nested
	// types, its fields and its methods.
	StructTemplate = "{{with " + FuncStruct + " " + render.BuiltinHeader + " .}}{{.Head}}" +
		classTypes + classFields + classMethods + "{{end}}"

	// InterfaceTemplate renders a Protocol or a TypedDict through
	// [Writer.Interface] as [StructTemplate] renders a class.
	InterfaceTemplate = "{{with " + FuncInterface + " " + render.BuiltinHeader + " .}}{{.Head}}" +
		classTypes + classFields + classMethods + "{{end}}"

	// EnumTemplate renders an enum through [Writer.Enum]: its head, its
	// members and its methods.
	EnumTemplate = "{{with " + FuncEnum + " " + render.BuiltinHeader + " .}}{{.Head}}" +
		classFields + classMethods + "{{end}}"

	// FunctionTemplate renders a function with its decorators, its
	// signature, its docstring and the content that the body builtin
	// places.
	FunctionTemplate = "{{" + FuncFunction + " " + render.BuiltinHeader + " . (" + render.BuiltinBody + " .)}}"

	// AliasTemplate renders an alias: a type statement, an assignment
	// annotated with TypeAlias, or a NewType.
	AliasTemplate = "{{" + FuncAlias + " " + render.BuiltinHeader + " .}}"

	// ConstantTemplate renders a constant as an assignment annotated with
	// Final.
	ConstantTemplate = "{{" + FuncConstant + " " + render.BuiltinHeader + " .}}"

	// VariableTemplate renders a variable of a module.
	VariableTemplate = "{{" + FuncVariable + " " + render.BuiltinHeader + " .}}"
)

// KindTemplates returns a map from each emit kind to the template that
// renders it. The lowering reshapes a sum into classes and an alias before
// any template runs, so the map has no template for a sum, and
// [RefusedKinds] refuses the standalone method. KindTemplates builds a new
// map on every call for the caller to keep. The map and its one group
// allocate twice.
func KindTemplates() map[symbol.Kind]string {
	return map[symbol.Kind]string{
		symbol.KindStruct:    StructTemplate,
		symbol.KindInterface: InterfaceTemplate,
		symbol.KindEnum:      EnumTemplate,
		symbol.KindFunction:  FunctionTemplate,
		symbol.KindAlias:     AliasTemplate,
		symbol.KindConstant:  ConstantTemplate,
		symbol.KindVariable:  VariableTemplate,
	}
}

// RefusedKinds returns a map from each emit kind that Python cannot declare
// at the level of a module to the reason. The render reports the reason
// with every declaration of the kind that it skips. RefusedKinds builds a
// new map on every call for the caller to keep. The map and its one group
// allocate twice.
func RefusedKinds() map[symbol.Kind]string {
	return map[symbol.Kind]string{
		symbol.KindMethod: "Python declares a method inside the class it belongs to",
	}
}
