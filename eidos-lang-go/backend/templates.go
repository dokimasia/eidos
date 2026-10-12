// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// FileTemplate is the skeleton of a Go file. It renders the package
// clause, the import block of the file and then its declarations. gofmt
// decides the blank lines, so the skeleton renders structure and not
// layout. The output contract writes the marker of generated code after
// the formatter runs, so the template does not contain it.
const FileTemplate = "package {{" + FuncPackage + " .Pkg}}\n" +
	"\n{{" + render.BuiltinImports + "}}\n{{" + render.BuiltinDecls + "}}"

// Each kind template renders one emit kind that Go declares at the top
// level. The template of a host renders a kind that Go declares inside
// the host, such as a field or a parameter, through the vocabulary. Such
// a kind has no template of its own.
const (
	// StructTemplate renders a struct and its fields, with its type
	// parameters after the name. The embedded types come first, as Go
	// promotes their members. Each embed has its own docblock and
	// directives, and its tag and trailing comment as a field has them.
	// The embeds precede the nominal parents, because Go expresses both
	// by embedding, and the promotion gives the members without the
	// subtyping. Implements renders nothing, because Go satisfies an
	// interface structurally and the methods make the claim. The fields
	// follow, each with its own docblock, its tag in backquotes and its
	// trailing comment. The trailing comment of the type ends the line of
	// the closing brace. The member methods follow the type as
	// package-level declarations with the receiver that the lowering
	// set, because Go declares a method outside its type. The guard
	// refuses every fact that Go cannot express before the template
	// renders a byte.
	StructTemplate = "{{docs .Doc}}{{with .Annotations}}{{directives .}}{{end}}{{guard .}}type {{.Name}}{{typeparams .TypeParams}} struct {\n" +
		"{{- range embeds .Embeds}}\n{{memberdocs .Doc}}{{with .Annotations}}{{memberdirectives .}}{{end}}\t{{spell .Ref}}" +
		"{{with .Tag}} `{{.}}`{{end}}{{with .Comment}} // {{.}}{{end}}\n{{- end}}" +
		"{{- range .Extends}}\n\t{{spell .}}\n{{- end}}" +
		"{{- range .Fields.Items}}\n{{memberdocs .Doc}}{{with .Annotations}}{{memberdirectives .}}{{end}}{{guard .}}\t{{.Name}} {{spell .Type}}" +
		"{{tag .}}{{with .Comment}} // {{.}}{{end}}\n" +
		"{{- end}}\n}{{with .Comment}} // {{.}}{{end}}\n" +
		"{{range .Methods.Items}}\n{{nested \"\" .}}\n{{end}}"

	// InterfaceTemplate renders an interface and its methods, with its
	// type parameters after the name. The embedded interfaces come
	// first, the embeds and then the extended interfaces, because Go
	// expresses the supertypes of an interface by embedding. Each embed
	// has its own docblock, directives and trailing comment, as the embed
	// of a struct has them. Its guard refuses a tag, because only a struct
	// field has a tag in Go. Each method follows with its own docblock. A
	// Go interface method has no body, no receiver and no type
	// parameters, so the template renders a signature alone, after its
	// own guard.
	InterfaceTemplate = "{{docs .Doc}}{{with .Annotations}}{{directives .}}{{end}}{{guard .}}type {{.Name}}{{typeparams .TypeParams}} interface {\n" +
		"{{- range embeds .Embeds}}\n{{memberdocs .Doc}}{{with .Annotations}}{{memberdirectives .}}{{end}}{{embedguard .}}" +
		"\t{{spell .Ref}}{{with .Comment}} // {{.}}{{end}}\n{{- end}}" +
		"{{- range .Extends}}\n\t{{spell .}}\n{{- end}}" +
		"{{- range .Methods.Items}}\n{{memberdocs .Doc}}{{sigguard .}}" +
		"\t{{.Name}}({{params .Params}}){{results .Returns}}{{with .Comment}} // {{.}}{{end}}\n" +
		"{{- end}}\n}{{with .Comment}} // {{.}}{{end}}\n"

	// FunctionTemplate renders a function with its body, with its type
	// parameters after the name, after the guard. An empty body is a pair
	// of braces on the line of the signature.
	FunctionTemplate = "{{docs .Doc}}{{with .Annotations}}{{directives .}}{{end}}{{guard .}}func {{.Name}}{{typeparams .TypeParams}}" +
		"({{params .Params}}){{results .Returns}} {{with body .}}{\n{{.}}}{{else}}{}{{end}}" +
		"{{with .Comment}} // {{.}}{{end}}\n"

	// MethodTemplate renders a method at the package level with its
	// receiver, because Go declares a method outside its type. The type
	// parameters of the method follow its name, a form that Go accepts
	// since Go 1.27. The type parameters of the receiver are in the
	// reference of the receiver. An empty body is a pair of braces on the
	// line of the signature, as in the marker method of a sum. The guard
	// refuses every fact that Go cannot express.
	MethodTemplate = "{{docs .Doc}}{{with .Annotations}}{{directives .}}{{end}}{{guard .}}func ({{receiver .}}) " +
		"{{.Name}}{{typeparams .TypeParams}}({{params .Params}})" +
		"{{results .Returns}} {{with body .}}{\n{{.}}}{{else}}{}{{end}}{{with .Comment}} // {{.}}{{end}}\n"

	// AliasTemplate renders a type alias or a defined type, with its type
	// parameters after the name, after the guard. A transparent alias has
	// the equals sign. A defined type has none, so its constants and
	// methods belong to the defined type.
	AliasTemplate = "{{docs .Doc}}{{with .Annotations}}{{directives .}}{{end}}{{guard .}}type {{.Name}}{{typeparams .TypeParams}} " +
		"{{if not .Defined}}= {{end}}{{spell .Target}}{{with .Comment}} // {{.}}{{end}}\n"

	// ConstantTemplate renders a constant after the guard. It writes the
	// type where the declaration has one, and the trailing comment after
	// the value.
	ConstantTemplate = "{{docs .Doc}}{{with .Annotations}}{{directives .}}{{end}}{{guard .}}const {{.Name}}" +
		"{{with .Type}} {{spell .}}{{end}} = {{.Value}}" +
		"{{with .Comment}} // {{.}}{{end}}\n"

	// VariableTemplate renders a variable after the guard. The
	// initializer follows an equals sign, and the trailing comment
	// follows the type or the initializer.
	VariableTemplate = "{{docs .Doc}}{{with .Annotations}}{{directives .}}{{end}}{{guard .}}var {{.Name}}{{vartype .}}" +
		"{{with .Value}} = {{.}}{{end}}{{with .Comment}} // {{.}}{{end}}\n"
)

// KindTemplates returns a map from each emit kind to the template that
// renders it. The lowering reshapes a file-level kind that the map does
// not contain, as it reshapes an enum and a sum. KindTemplates builds a
// new map on every call for the caller to keep. The map and its one
// group allocate twice.
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
