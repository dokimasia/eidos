// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend

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
	// StructTemplate spells a struct and its fields, its type
	// parameters behind the name, each field under its own
	// docblock, carrying its tag in backquotes and its trailing
	// comment where the declaration states them.
	StructTemplate = "{{docs .Doc}}type {{.Name}}{{typeparams .TypeParams}} struct {\n" +
		"{{- range .Fields.Items}}\n{{docs .Doc \"\\t\"}}\t{{.Name}} {{spell .Type}}" +
		"{{with .Tag}} `{{.}}`{{end}}{{with .Comment}} // {{.}}{{end}}\n" +
		"{{- end}}\n}\n"

	// InterfaceTemplate spells an interface and the methods it
	// requires, its type parameters behind the name, each method
	// under its own docblock. An interface method states no body,
	// no receiver and no parameter list of its own, which Go
	// refuses on interface methods, so it is a signature alone.
	InterfaceTemplate = "{{docs .Doc}}type {{.Name}}{{typeparams .TypeParams}} interface {\n" +
		"{{- range .Methods.Items}}\n{{docs .Doc \"\\t\"}}" +
		"\t{{.Name}}({{params .Params}}){{results .Returns}}\n" +
		"{{- end}}\n}\n"

	// FunctionTemplate spells a function, its type parameters
	// behind the name, and places its body.
	FunctionTemplate = "{{docs .Doc}}func {{.Name}}{{typeparams .TypeParams}}" +
		"({{params .Params}}){{results .Returns}} {\n{{body .}}}\n"

	// MethodTemplate spells a method: Go states one at the package
	// level with a receiver, never inside the type it attaches to.
	// The method's own type parameters spell behind its name, the
	// form Go accepts since 1.27; the receiver's spell inside the
	// receiver's reference.
	MethodTemplate = "{{docs .Doc}}func ({{receiver .}}) " +
		"{{.Name}}{{typeparams .TypeParams}}({{params .Params}})" +
		"{{results .Returns}} {\n{{body .}}}\n"

	// AliasTemplate spells a type alias, its type parameters
	// behind the name.
	AliasTemplate = "{{docs .Doc}}type {{.Name}}{{typeparams .TypeParams}} = {{spell .Target}}\n"

	// ConstantTemplate spells a constant, typed where the
	// declaration states a type, its trailing comment beside the
	// value where one is stated.
	ConstantTemplate = "{{docs .Doc}}const {{.Name}}" +
		"{{with .Type}} {{spell .}}{{end}} = {{.Value}}" +
		"{{with .Comment}} // {{.}}{{end}}\n"

	// VariableTemplate spells a variable, its trailing comment
	// beside the type where one is stated.
	VariableTemplate = "{{docs .Doc}}var {{.Name}} {{spell .Type}}" +
		"{{with .Comment}} // {{.}}{{end}}\n"
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
