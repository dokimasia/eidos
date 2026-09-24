// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// FileTemplate is the TypeScript file skeleton: the import block
// the file collected, then its declarations. There is no package
// clause; the file is the module. The generated-code marker is
// not here: the output contract writes it after the formatter
// ran.
const FileTemplate = "{{" + render.BuiltinImports + "}}{{" + render.BuiltinDecls + "}}"

// The kind templates, one per emit kind TypeScript declares at
// module level. A method is absent by design: TypeScript states
// members inside their type, so methods render through their
// host's template, and a standalone method is reported as a kind
// the target cannot spell.
const (
	// StructTemplate spells a class: decorator lines above the
	// declaration, its keywords and type parameters behind the
	// name, then fields with initializers and methods with their
	// bodies, each member under its own doc block and decorators,
	// its keywords in TypeScript's stated order. An abstract
	// method is a signature alone; a constructing method spells
	// the constructor form, its own name and results dropped
	// because the language grants a constructor neither.
	StructTemplate = "{{docs .Doc}}{{decorators .Annotations}}" +
		"{{mods .}}class {{.Name}}{{typeparams .TypeParams}}{{heritage .}} {\n" +
		"{{- range .Fields.Items}}\n{{docs .Doc \"  \"}}{{decorators .Annotations \"  \"}}" +
		"  {{membermods .}}{{hard .}}{{propkey .}}{{if .Optional}}?{{end}}: {{spell .Type}}{{with .Value}} = {{.}}{{end}};" +
		"{{with .Comment}} // {{.}}{{end}}\n" +
		"{{- end}}" +
		"{{- range .Methods.Items}}\n{{docs .Doc \"  \"}}{{decorators .Annotations \"  \"}}" +
		"{{if .Indexer}}  {{indexsig .}}{{else if .Constructs}}" +
		"  {{membermods .}}constructor({{params .Params}}) {\n{{body .}}  }{{else}}" +
		"  {{membermods .}}{{accessor .}}{{hard .}}{{methodkey .}}{{typeparams .TypeParams}}({{params .Params}}){{returns .}}" +
		"{{if .Abstract}};{{else}} {\n{{body .}}  }{{end}}{{end}}{{with .Comment}} // {{.}}{{end}}\n" +
		"{{- end}}\n}{{with .Comment}} // {{.}}{{end}}\n"

	// InterfaceTemplate spells an interface, its type parameters
	// behind the name: properties, readonly where stated, and
	// method signatures with their own parameter lists, no bodies
	// and no other keywords.
	InterfaceTemplate = "{{docs .Doc}}{{mods .}}interface {{.Name}}" +
		"{{typeparams .TypeParams}}{{heritage .}} {\n" +
		"{{- range .Fields.Items}}\n{{docs .Doc \"  \"}}  {{propmods .}}{{propkey .}}{{if .Optional}}?{{end}}: {{spell .Type}};" +
		"{{with .Comment}} // {{.}}{{end}}\n" +
		"{{- end}}" +
		"{{- range .Methods.Items}}\n{{docs .Doc \"  \"}}" +
		"{{if .Indexer}}  {{indexsig .}}" +
		"{{else if .Constructs}}  new {{typeparams .TypeParams}}({{params .Params}}){{results .Returns}};" +
		"{{else}}  {{sigmods .}}{{methodkey .}}{{typeparams .TypeParams}}({{params .Params}}){{results .Returns}};{{end}}{{with .Comment}} // {{.}}{{end}}\n" +
		"{{- end}}\n}{{with .Comment}} // {{.}}{{end}}\n"

	// FunctionTemplate spells a module-level function, async
	// where stated, its type parameters behind the name, and
	// places its body.
	FunctionTemplate = "{{docs .Doc}}{{mods .}}function {{.Name}}{{typeparams .TypeParams}}" +
		"({{params .Params}}){{returns .}} {\n{{body .}}}{{with .Comment}} // {{.}}{{end}}\n"

	// AliasTemplate spells a type alias, its type parameters
	// behind the name.
	AliasTemplate = "{{docs .Doc}}{{mods .}}type {{.Name}}{{typeparams .TypeParams}}" +
		" = {{spell .Target}};{{with .Comment}} // {{.}}{{end}}\n"

	// EnumTemplate spells an enum: one member per variant, its
	// stated value behind an equals sign, each under its own doc
	// block.
	EnumTemplate = "{{docs .Doc}}{{mods .}}{{if .Const}}const {{end}}enum {{.Name}} {\n" +
		"{{- range .Variants.Items}}\n{{docs .Doc \"  \"}}" +
		"  {{.Name}}{{with .Value}} = {{.}}{{end}},{{with .Comment}} // {{.}}{{end}}\n" +
		"{{- end}}\n}{{with .Comment}} // {{.}}{{end}}\n"

	// ConstantTemplate spells a constant, typed where the
	// declaration states a type, its trailing comment behind the
	// semicolon.
	ConstantTemplate = "{{docs .Doc}}{{mods .}}const {{.Name}}" +
		"{{with .Type}}: {{spell .}}{{end}} = {{.Value}};" +
		"{{with .Comment}} // {{.}}{{end}}\n"

	// VariableTemplate spells a module-level binding: const where
	// the declaration is immutable, let otherwise, its
	// initializer behind an equals sign where one is stated, its
	// trailing comment behind the semicolon.
	VariableTemplate = "{{docs .Doc}}{{mods .}}{{binding .}} {{.Name}}" +
		"{{with .Type}}: {{spell .}}{{end}}{{with .Value}} = {{.}}{{end}};" +
		"{{with .Comment}} // {{.}}{{end}}\n"
)

// KindTemplates maps each emit kind to the template that spells
// it. A kind absent from the map is one the TypeScript backend
// cannot spell at module level: the render reports it and skips
// that declaration, which is what the feature matrix records.
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
