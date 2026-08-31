// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package golang

import (
	"go.dokimi.dev/eidos/core/render"
	"go.dokimi.dev/eidos/core/symbol"
)

// FileTemplate is the Go file skeleton: the package clause, the
// import block the file collected, then its declarations. gofmt
// settles the blank lines, so the skeleton spells structure and
// not layout. The generated-code marker is not here: the output
// contract writes it after the formatter ran.
const FileTemplate = "package {{" + FuncPackage + " .Pkg}}\n" +
	"\n{{" + render.BuiltinImports + "}}\n{{" + render.BuiltinDecls + "}}"

// The kind templates, one per emit kind Go declares standalone.
// A kind Go states inside its host, such as a field or a
// parameter, is written by the host's template through the
// vocabulary rather than by a template of its own.
const (
	// StructTemplate spells a struct and its fields, each field
	// under its own docblock. The model carries no trailing
	// comment and no tag, so neither renders yet.
	StructTemplate = "{{docs .Doc}}type {{.Name}} struct {\n" +
		"{{- range .Fields.Items}}\n{{docs .Doc \"\\t\"}}\t{{.Name}} {{spell .Type}}\n" +
		"{{- end}}\n}\n"

	// InterfaceTemplate spells an interface and the methods it
	// requires, each under its own docblock. An interface method
	// states no body and no receiver, so it is a signature alone.
	InterfaceTemplate = "{{docs .Doc}}type {{.Name}} interface {\n" +
		"{{- range .Methods.Items}}\n{{docs .Doc \"\\t\"}}" +
		"\t{{.Name}}({{params .Params}}){{results .Returns}}\n" +
		"{{- end}}\n}\n"

	// FunctionTemplate spells a function and places its body.
	FunctionTemplate = "{{docs .Doc}}func {{.Name}}({{params .Params}})" +
		"{{results .Returns}} {\n{{body .}}}\n"

	// MethodTemplate spells a method: Go states one at the package
	// level with a receiver, never inside the type it attaches to.
	MethodTemplate = "{{docs .Doc}}func ({{receiver .}}) {{.Name}}({{params .Params}})" +
		"{{results .Returns}} {\n{{body .}}}\n"

	// AliasTemplate spells a type alias.
	AliasTemplate = "{{docs .Doc}}type {{.Name}} = {{spell .Target}}\n"

	// ConstantTemplate spells a constant, typed where the
	// declaration states a type.
	ConstantTemplate = "{{docs .Doc}}const {{.Name}}" +
		"{{with .Type}} {{spell .}}{{end}} = {{.Value}}\n"

	// VariableTemplate spells a variable.
	VariableTemplate = "{{docs .Doc}}var {{.Name}} {{spell .Type}}\n"
)

// KindTemplates maps each emit kind to the template that spells
// it. A kind absent from the map is one Go's backend cannot
// spell: the render reports it and skips that declaration, which
// is what the feature matrix records.
func KindTemplates() map[symbol.Kind]string {
	return map[symbol.Kind]string{
		symbol.KindStruct:    StructTemplate,
		symbol.KindInterface: InterfaceTemplate,
		symbol.KindFunction:  FunctionTemplate,
		symbol.KindMethod:    MethodTemplate,
		symbol.KindAlias:     AliasTemplate,
		symbol.KindConstant:  ConstantTemplate,
		symbol.KindVariable:  VariableTemplate,
	}
}
