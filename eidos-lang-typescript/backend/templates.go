// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// FileTemplate is the skeleton of a TypeScript file. It renders the
// import block of the file and then its declarations. It has no package
// clause, because a TypeScript file is a module. The output contract
// writes the marker of generated code after the formatter runs, so the
// template does not contain it.
const FileTemplate = "{{" + render.BuiltinImports + "}}{{" + render.BuiltinDecls + "}}"

// Each kind template renders one emit kind that TypeScript declares at
// module level. No template renders a method. TypeScript declares a
// member inside its type, so a method renders through the template of
// its host, and the backend refuses a standalone method.
const (
	// StructTemplate renders a class. The decorator lines precede the
	// declaration, and the keywords and the type parameters follow the
	// name. The class then contains its fields with their initializers
	// and its methods with their bodies. Each member has its own doc
	// block and decorators, and its keywords in the order of the
	// TypeScript grammar. An optional member has a ? after its key. An
	// abstract method is a signature alone. An index signature takes
	// static and readonly alone. A constructing method renders in the
	// constructor form with its accessibility alone, without a name and
	// without results, because a TypeScript constructor has neither.
	StructTemplate = "{{docs .Doc}}{{decorators .Annotations}}" +
		"{{mods .}}class {{.Name}}{{typeparams .TypeParams}}{{heritage .}} {\n" +
		"{{- range .Fields.Items}}\n{{memberdocs .Doc}}{{memberdecorators .Annotations}}" +
		"  {{membermods .}}{{hard .}}{{propkey .}}{{if .Optional}}?{{end}}: {{spell .Type}}{{with .Value}} = {{.}}{{end}};" +
		"{{with .Comment}} // {{.}}{{end}}\n" +
		"{{- end}}" +
		"{{- range .Methods.Items}}\n{{memberdocs .Doc}}{{memberdecorators .Annotations}}" +
		"{{if .Indexer}}  {{indexmods .}}{{indexsig .}}{{else if .Constructs}}" +
		"  {{ctormods .}}constructor({{params .Params}}) {\n{{memberbody .}}  }{{else}}" +
		"  {{membermods .}}{{accessor .}}{{hard .}}{{methodkey .}}{{if .Optional}}?{{end}}" +
		"{{typeparams .TypeParams}}({{params .Params}}){{returns .}}" +
		"{{if .Abstract}};{{else}} {\n{{memberbody .}}  }{{end}}{{end}}{{with .Comment}} // {{.}}{{end}}\n" +
		"{{- end}}\n}{{with .Comment}} // {{.}}{{end}}\n"

	// InterfaceTemplate renders an interface with its type parameters
	// after the name. Its members are properties and method, index and
	// construct signatures with their own parameter lists. A property is
	// readonly where the model marks it immutable, and so is an index
	// signature. An optional member has a ? after its key. A signature has
	// no body, and its only keywords are the modifiers of a signature. An
	// async method signature returns a promise of its result.
	InterfaceTemplate = "{{docs .Doc}}{{mods .}}interface {{.Name}}" +
		"{{typeparams .TypeParams}}{{heritage .}} {\n" +
		"{{- range .Fields.Items}}\n{{memberdocs .Doc}}  {{propmods .}}{{propkey .}}{{if .Optional}}?{{end}}: {{spell .Type}};" +
		"{{with .Comment}} // {{.}}{{end}}\n" +
		"{{- end}}" +
		"{{- range .Methods.Items}}\n{{memberdocs .Doc}}" +
		"{{if .Indexer}}  {{sigmods .}}{{indexsig .}}" +
		"{{else if .Constructs}}  {{sigmods .}}new {{typeparams .TypeParams}}({{params .Params}}){{results .Returns}};" +
		"{{else}}  {{sigmods .}}{{methodkey .}}{{if .Optional}}?{{end}}{{typeparams .TypeParams}}" +
		"({{params .Params}}){{returns .}};{{end}}{{with .Comment}} // {{.}}{{end}}\n" +
		"{{- end}}\n}{{with .Comment}} // {{.}}{{end}}\n"

	// FunctionTemplate renders a module-level function with its body.
	// The function is async where the model marks it, and its type
	// parameters follow the name.
	FunctionTemplate = "{{docs .Doc}}{{mods .}}function {{.Name}}{{typeparams .TypeParams}}" +
		"({{params .Params}}){{returns .}} {\n{{body .}}}{{with .Comment}} // {{.}}{{end}}\n"

	// AliasTemplate renders a type alias with its type parameters after
	// the name.
	AliasTemplate = "{{docs .Doc}}{{mods .}}type {{.Name}}{{typeparams .TypeParams}}" +
		" = {{spell .Target}};{{with .Comment}} // {{.}}{{end}}\n"

	// EnumTemplate renders an enum with one member per variant. Each
	// member has its own doc block. Its key is quoted where the name is
	// not an identifier, and its value follows an equals sign.
	EnumTemplate = "{{docs .Doc}}{{mods .}}{{if .Const}}const {{end}}enum {{.Name}} {\n" +
		"{{- range .Variants.Items}}\n{{memberdocs .Doc}}" +
		"  {{enumkey .}}{{with .Value}} = {{.}}{{end}},{{with .Comment}} // {{.}}{{end}}\n" +
		"{{- end}}\n}{{with .Comment}} // {{.}}{{end}}\n"

	// ConstantTemplate renders a constant. It writes the type where the
	// declaration has one, and the trailing comment after the semicolon.
	ConstantTemplate = "{{docs .Doc}}{{mods .}}const {{.Name}}" +
		"{{with .Type}}: {{spell .}}{{end}} = {{.Value}};" +
		"{{with .Comment}} // {{.}}{{end}}\n"

	// VariableTemplate renders a module-level binding. The keyword is
	// const for an immutable declaration and let otherwise. An
	// initializer follows an equals sign, and the trailing comment
	// follows the semicolon. The keyword helper refuses an immutable
	// binding without an initializer, because TypeScript requires the
	// initializer of a const in its declaration.
	VariableTemplate = "{{docs .Doc}}{{mods .}}{{binding .}} {{.Name}}" +
		"{{with .Type}}: {{spell .}}{{end}}{{with .Value}} = {{.}}{{end}};" +
		"{{with .Comment}} // {{.}}{{end}}\n"
)

// KindTemplates returns a map from each emit kind to the template that
// renders it. The lowering reshapes a file-level kind that the map does
// not contain, as it reshapes a sum, or [RefusedKinds] refuses it.
// KindTemplates builds a new map on every call for the caller to keep.
// The map and its one group allocate twice.
func KindTemplates() map[symbol.Kind]string {
	return map[symbol.Kind]string{
		symbol.KindStruct:    StructTemplate,
		symbol.KindInterface: InterfaceTemplate,
		symbol.KindFunction:  FunctionTemplate,
		symbol.KindEnum:      EnumTemplate,
		symbol.KindAlias:     AliasTemplate,
		symbol.KindConstant:  ConstantTemplate,
		symbol.KindVariable:  VariableTemplate,
	}
}

// RefusedKinds returns a map from each emit kind that TypeScript cannot
// declare at module level to the reason. The render reports the reason
// with every declaration of the kind that it skips. RefusedKinds builds
// a new map on every call for the caller to keep. The map and its one
// group allocate twice.
func RefusedKinds() map[symbol.Kind]string {
	return map[symbol.Kind]string{
		symbol.KindMethod: "TypeScript declares a method inside the class or interface it belongs to",
	}
}
