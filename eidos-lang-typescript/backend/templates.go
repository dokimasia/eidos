// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend

import (
	"go.dokimi.dev/eidos/core/render"
	"go.dokimi.dev/eidos/core/symbol"
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
	// StructTemplate spells a class: fields, then methods with
	// their bodies, each member under its own doc block.
	StructTemplate = "{{docs .Doc}}export class {{.Name}} {\n" +
		"{{- range .Fields.Items}}\n{{docs .Doc \"  \"}}  {{.Name}}: {{spell .Type}};\n" +
		"{{- end}}" +
		"{{- range .Methods.Items}}\n{{docs .Doc \"  \"}}" +
		"  {{.Name}}({{params .Params}}){{results .Returns}} {\n{{body .}}  }\n" +
		"{{- end}}\n}\n"

	// InterfaceTemplate spells an interface: properties and method
	// signatures, no bodies.
	InterfaceTemplate = "{{docs .Doc}}export interface {{.Name}} {\n" +
		"{{- range .Fields.Items}}\n{{docs .Doc \"  \"}}  {{.Name}}: {{spell .Type}};\n" +
		"{{- end}}" +
		"{{- range .Methods.Items}}\n{{docs .Doc \"  \"}}" +
		"  {{.Name}}({{params .Params}}){{results .Returns}};\n" +
		"{{- end}}\n}\n"

	// FunctionTemplate spells a module-level function and places
	// its body.
	FunctionTemplate = "{{docs .Doc}}export function {{.Name}}({{params .Params}})" +
		"{{results .Returns}} {\n{{body .}}}\n"

	// AliasTemplate spells a type alias.
	AliasTemplate = "{{docs .Doc}}export type {{.Name}} = {{spell .Target}};\n"

	// ConstantTemplate spells a constant, typed where the
	// declaration states a type.
	ConstantTemplate = "{{docs .Doc}}export const {{.Name}}" +
		"{{with .Type}}: {{spell .}}{{end}} = {{.Value}};\n"

	// VariableTemplate spells a module-level binding.
	VariableTemplate = "{{docs .Doc}}export let {{.Name}}" +
		"{{with .Type}}: {{spell .}}{{end}};\n"
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
		symbol.KindAlias:     AliasTemplate,
		symbol.KindConstant:  ConstantTemplate,
		symbol.KindVariable:  VariableTemplate,
	}
}
