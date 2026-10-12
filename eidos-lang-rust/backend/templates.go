// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// FileTemplate is the skeleton of a Rust file. It renders the use block
// of the file and then its declarations. It has no package clause,
// because a Rust file is a module. The output contract writes the marker
// of generated code after the formatter runs, so the template does not
// contain it.
const FileTemplate = "{{" + render.BuiltinImports + "}}{{" + render.BuiltinDecls + "}}"

// Each kind template renders one emit kind that Rust declares at module
// level. No kind template renders a standalone method. Rust groups
// methods in one impl block per receiver, and a template that renders
// one declaration at a time cannot write that block, so the cluster
// writes it through the impl group. No template renders a variable, and
// the backend refuses it. Rust declares a module-level binding as a
// static with a constant initializer, and the initializer of a variable
// runs at run time.
//
// The type parameter list of a type definition takes defaults. The list
// of an impl block restates the parameters of the definition without
// defaults. The lists of a function, a method and an associated type
// refuse a default.
const (
	// StructTemplate renders a struct and its fields. The attribute
	// lines precede the declaration, the visibility precedes the name,
	// and the type parameters follow it. Each field has its own doc
	// lines, attributes and visibility. The member methods follow in one
	// impl block, which restates the type parameters of the struct on
	// the impl and on its target, because Rust declares methods outside
	// their type.
	StructTemplate = "{{docs .Doc}}{{attrs .Annotations}}" +
		"{{structmods .}}struct {{.Name}}{{typeparams .TypeParams}} {\n" +
		"{{- range .Fields.Items}}\n{{memberdocs .Doc}}{{memberattrs .Annotations}}" +
		"    {{fieldmods .}}{{.Name}}: {{spell .Type}},{{with .Comment}} // {{.}}{{end}}\n" +
		"{{- end}}\n}{{with .Comment}} // {{.}}{{end}}\n" +
		"{{if .Methods.Len}}\nimpl{{implparams .TypeParams}} {{.Name}}{{typenames .TypeParams}} {\n" +
		"{{- range .Methods.Items}}\n{{memberdocs .Doc}}{{memberattrs .Annotations}}" +
		"    {{implfn .}}fn {{.Name}}{{fnparams .TypeParams}}({{selfparams .}})" +
		"{{results .Returns}} {\n{{memberbody .}}    }{{with .Comment}} // {{.}}{{end}}\n" +
		"{{- end}}\n}\n{{end}}"

	// InterfaceTemplate renders a trait, with its visibility before the
	// name and its type parameters after it. The associated types come
	// first, each with its own doc lines and attributes. The method
	// signatures follow, each with its own parameter list. A method at
	// instance level takes the receiver by reference, a method at type
	// level takes no receiver, and an async method has the async
	// keyword. A method with a default renders its body, and every other
	// method ends as a signature.
	InterfaceTemplate = "{{docs .Doc}}{{attrs .Annotations}}" +
		"{{vis .Visibility .Name}}trait {{.Name}}{{typeparams .TypeParams}}{{supertraits .}} {\n" +
		"{{- range .Types.Items}}\n{{memberdocs .Doc}}{{memberattrs .Annotations}}" +
		"    {{assoctype .}}{{with .Comment}} // {{.}}{{end}}\n" +
		"{{- end}}" +
		"{{- range .Methods.Items}}\n{{memberdocs .Doc}}{{memberattrs .Annotations}}" +
		"    {{traitfn .}}fn {{.Name}}{{fnparams .TypeParams}}({{selfparams .}})" +
		"{{results .Returns}}{{if .HasDefault}} {\n{{memberbody .}}    }{{else}};{{end}}{{with .Comment}} // {{.}}{{end}}\n" +
		"{{- end}}\n}{{with .Comment}} // {{.}}{{end}}\n"

	// FunctionTemplate renders a free function with its body. The
	// attribute lines precede the declaration, the visibility and async
	// precede fn, and the type parameters follow the name.
	FunctionTemplate = "{{docs .Doc}}{{attrs .Annotations}}{{fnmods .}}fn " +
		"{{.Name}}{{fnparams .TypeParams}}" +
		"({{params .Params}}){{results .Returns}} {\n{{body .}}}{{with .Comment}} // {{.}}{{end}}\n"

	// EnumTemplate renders an enum without payloads, with one variant per
	// line. The repr attribute of an underlying type follows the other
	// attributes. Each variant has its own doc lines and attributes, and
	// its value as the discriminant.
	EnumTemplate = "{{docs .Doc}}{{attrs .Annotations}}{{repr .}}" +
		"{{enummods .}}enum {{.Name}} {\n" +
		"{{- range .Variants.Items}}\n{{memberdocs .Doc}}{{memberattrs .Annotations}}" +
		"    {{.Name}}{{with .Value}} = {{.}}{{end}},{{with .Comment}} // {{.}}{{end}}\n" +
		"{{- end}}\n}{{with .Comment}} // {{.}}{{end}}\n"

	// SumTemplate renders a data enum with one variant per line. The
	// attribute lines precede the declaration, the visibility precedes
	// the name, and the type parameters follow it. Each variant has its
	// own doc lines and attributes, and its payload inline as named
	// fields in braces or as bare types in parentheses.
	SumTemplate = "{{docs .Doc}}{{attrs .Annotations}}" +
		"{{summods .}}enum {{.Name}}{{typeparams .TypeParams}} {\n" +
		"{{- range .Variants.Items}}\n{{memberdocs .Doc}}{{memberattrs .Annotations}}" +
		"    {{.Name}}{{sumpayload .}},{{with .Comment}} // {{.}}{{end}}\n" +
		"{{- end}}\n}{{with .Comment}} // {{.}}{{end}}\n"

	// AliasTemplate renders a type alias, with its visibility before the
	// name and its type parameters after it. The keywords helper refuses
	// a defined type, because a Rust alias is transparent.
	AliasTemplate = "{{docs .Doc}}{{attrs .Annotations}}" +
		"{{aliasmods .}}type {{.Name}}{{typeparams .TypeParams}}" +
		" = {{spell .Target}};{{with .Comment}} // {{.}}{{end}}\n"

	// ConstantTemplate renders a constant with its trailing comment after
	// the semicolon. A helper writes the type and refuses a constant
	// without a type or a value, because rustc requires both.
	ConstantTemplate = "{{docs .Doc}}{{attrs .Annotations}}" +
		"{{vis .Visibility .Name}}const {{.Name}}: {{consttype .}} = {{.Value}};" +
		"{{with .Comment}} // {{.}}{{end}}\n"
)

// KindTemplates returns a map from each emit kind to the template that
// renders it. The cluster renders a file-level kind that the map does
// not contain through a group template, as it renders a method, or
// [RefusedKinds] refuses it. KindTemplates builds a new map on every
// call for the caller to keep. The map and its one group allocate twice.
func KindTemplates() map[symbol.Kind]string {
	return map[symbol.Kind]string{
		symbol.KindStruct:    StructTemplate,
		symbol.KindInterface: InterfaceTemplate,
		symbol.KindFunction:  FunctionTemplate,
		symbol.KindEnum:      EnumTemplate,
		symbol.KindSum:       SumTemplate,
		symbol.KindAlias:     AliasTemplate,
		symbol.KindConstant:  ConstantTemplate,
	}
}

// RefusedKinds returns a map from each emit kind that Rust cannot
// declare at module level to the reason. The render reports the reason
// with every declaration of the kind that it skips. RefusedKinds builds
// a new map on every call for the caller to keep. The map and its one
// group allocate twice.
func RefusedKinds() map[symbol.Kind]string {
	return map[symbol.Kind]string{
		symbol.KindVariable: "Rust declares a module-level binding as a static, whose initializer is constant",
	}
}
