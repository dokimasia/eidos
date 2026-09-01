// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
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
	// parameters behind the name: embedded types first, the way Go
	// promotes members, the embeds then the nominal parents,
	// because embedding is Go's one idiom for both and the
	// promotion carries the members without the subtyping. A
	// stated Implements spells nothing: satisfaction is
	// structural, and the methods themselves carry the claim.
	// Fields follow, each under its own docblock, carrying its tag
	// in backquotes and its trailing comment where the declaration
	// states them. Member methods follow the type as package-level
	// declarations, the receiver the lowering filled, because Go
	// states a method outside the type it attaches to. The guard
	// refuses what Go states nowhere before a byte renders.
	StructTemplate = "{{docs .Doc}}{{with .Annotations}}{{directives .}}{{end}}{{guard .}}type {{.Name}}{{typeparams .TypeParams}} struct {\n" +
		"{{- range .Embeds}}\n\t{{spell .Ref}}\n{{- end}}" +
		"{{- range .Extends}}\n\t{{spell .}}\n{{- end}}" +
		"{{- range .Fields.Items}}\n{{docs .Doc \"\\t\"}}{{guard .}}\t{{.Name}} {{spell .Type}}" +
		"{{with .Tag}} `{{.}}`{{end}}{{with .Comment}} // {{.}}{{end}}\n" +
		"{{- end}}\n}\n" +
		"{{range .Methods.Items}}\n{{nested \"\" .}}\n{{end}}"

	// InterfaceTemplate spells an interface and the methods it
	// requires, its type parameters behind the name: embedded
	// interfaces first, both the embeds and the nominal widening,
	// because Go spells an interface's supertypes as embedding,
	// then each method under its own docblock. An interface method
	// states no body, no receiver and no parameter list of its
	// own, which Go refuses on interface methods, so it is a
	// signature alone under its own guard.
	InterfaceTemplate = "{{docs .Doc}}{{with .Annotations}}{{directives .}}{{end}}{{guard .}}type {{.Name}}{{typeparams .TypeParams}} interface {\n" +
		"{{- range .Embeds}}\n\t{{spell .Ref}}\n{{- end}}" +
		"{{- range .Extends}}\n\t{{spell .}}\n{{- end}}" +
		"{{- range .Methods.Items}}\n{{docs .Doc \"\\t\"}}{{sigguard .}}" +
		"\t{{.Name}}({{params .Params}}){{results .Returns}}\n" +
		"{{- end}}\n}\n"

	// FunctionTemplate spells a function, its type parameters
	// behind the name, and places its body, under the guard.
	FunctionTemplate = "{{docs .Doc}}{{with .Annotations}}{{directives .}}{{end}}{{guard .}}func {{.Name}}{{typeparams .TypeParams}}" +
		"({{params .Params}}){{results .Returns}} {\n{{body .}}}\n"

	// MethodTemplate spells a method: Go states one at the package
	// level with a receiver, never inside the type it attaches to.
	// The method's own type parameters spell behind its name, the
	// form Go accepts since 1.27; the receiver's spell inside the
	// receiver's reference. The guard refuses what Go states
	// nowhere.
	MethodTemplate = "{{docs .Doc}}{{with .Annotations}}{{directives .}}{{end}}{{guard .}}func ({{receiver .}}) " +
		"{{.Name}}{{typeparams .TypeParams}}({{params .Params}})" +
		"{{results .Returns}} {\n{{body .}}}\n"

	// AliasTemplate spells a type alias or a defined type, its
	// type parameters behind the name, under the guard: the equals
	// sign is the transparent alias's, and a defined type drops it,
	// which is what makes its constants and methods its own.
	AliasTemplate = "{{docs .Doc}}{{with .Annotations}}{{directives .}}{{end}}{{guard .}}type {{.Name}}{{typeparams .TypeParams}} " +
		"{{if not .Defined}}= {{end}}{{spell .Target}}\n"

	// ConstantTemplate spells a constant, typed where the
	// declaration states a type, its trailing comment beside the
	// value where one is stated, under the guard.
	ConstantTemplate = "{{docs .Doc}}{{with .Annotations}}{{directives .}}{{end}}{{guard .}}const {{.Name}}" +
		"{{with .Type}} {{spell .}}{{end}} = {{.Value}}" +
		"{{with .Comment}} // {{.}}{{end}}\n"

	// VariableTemplate spells a variable, its initializer behind
	// an equals sign and its trailing comment beside the type
	// where the declaration states them, under the guard.
	VariableTemplate = "{{docs .Doc}}{{with .Annotations}}{{directives .}}{{end}}{{guard .}}var {{.Name}}{{vartype .}}" +
		"{{with .Value}} = {{.}}{{end}}{{with .Comment}} // {{.}}{{end}}\n"
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
