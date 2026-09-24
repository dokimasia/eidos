// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// FileTemplate is the Java file skeleton: the package clause, the
// import block the file collected, then its declarations. The
// generated-code marker is not here: the output contract writes
// it after the formatter ran.
const FileTemplate = "{{" + FuncPackage + " .Pkg}}{{" + render.BuiltinImports + "}}" +
	"{{" + render.BuiltinDecls + "}}"

// The kind templates. Java states everything inside a type, so a
// class, an interface and an enum are the only file-level
// declarations, and every other kind is reported as one the target
// cannot spell. Members render as public, because a generated API
// exists to be called. Each template renders a file-level type and a
// member type alike, so the keywords it writes are a member type's,
// and the lowering refuses a file-level type stating what only a
// member type can spell.
const (
	// StructTemplate spells a class: annotation lines above the
	// declaration, its keywords and type parameters behind the
	// name, then fields with initializers and methods with their
	// bodies, each member under its own doc block and annotations,
	// its keywords in Java's stated order, then the member types at
	// member depth through their own kind templates. An overriding
	// method takes the Override annotation, and an abstract method is
	// a signature alone. A generic method's own parameter list spells
	// before its return type, which is where Java states it.
	StructTemplate = "{{docs .Doc}}{{annotate .Annotations}}" +
		"{{typemods .}}class {{.Name}}{{typeparams .TypeParams}}{{heritage .}} {\n" +
		"{{- range .Fields.Items}}\n{{docs .Doc \"    \"}}{{annotate .Annotations \"    \"}}" +
		"    {{fieldmods .}}{{spell .Type}} {{.Name}}{{with .Value}} = {{.}}{{end}};" +
		"{{with .Comment}} // {{.}}{{end}}\n" +
		"{{- end}}" +
		"{{- range .Methods.Items}}\n{{docs .Doc \"    \"}}{{annotate .Annotations \"    \"}}" +
		"{{if .Override}}    @Override\n{{end}}" +
		"    {{methodmods .}}{{with typeparams .TypeParams}}{{.}} {{end}}" +
		"{{results .Returns}} {{.Name}}({{params .Params}}){{throws .Throws}}" +
		"{{if .Abstract}};{{else}} {\n{{body .}}    }{{end}}{{with .Comment}} // {{.}}{{end}}\n" +
		"{{- end}}" +
		"{{- range .Types.Items}}\n{{nested \"    \" .}}\n{{- end}}\n}{{with .Comment}} // {{.}}{{end}}\n"

	// InterfaceTemplate spells an interface: annotation lines
	// above the declaration, its keywords and type parameters
	// behind the name, then its properties as constants, then
	// signatures, implicitly public the way Java reads them, a
	// generic method's own parameter list before its return type,
	// then the member types at member depth through their own kind
	// templates, each refusing a scope narrower than public. A
	// method with a body spells default at instance level or static
	// at type level, and places the body.
	InterfaceTemplate = "{{docs .Doc}}{{annotate .Annotations}}" +
		"{{typemods .}}interface {{.Name}}{{typeparams .TypeParams}}{{heritage .}} {\n" +
		"{{- range .Fields.Items}}\n{{docs .Doc \"    \"}}{{annotate .Annotations \"    \"}}" +
		"    {{constantmods .}}{{spell .Type}} {{.Name}} = {{.Value}};" +
		"{{with .Comment}} // {{.}}{{end}}\n" +
		"{{- end}}" +
		"{{- range .Methods.Items}}\n{{docs .Doc \"    \"}}{{annotate .Annotations \"    \"}}" +
		"    {{sigmods .}}{{with typeparams .TypeParams}}{{.}} {{end}}" +
		"{{results .Returns}} {{.Name}}({{params .Params}}){{throws .Throws}}" +
		"{{if .HasDefault}} {\n{{body .}}    }{{else}};{{end}}{{with .Comment}} // {{.}}{{end}}\n" +
		"{{- end}}" +
		"{{- range .Types.Items}}\n{{membertype .}}{{nested \"    \" .}}\n{{- end}}\n}" +
		"{{with .Comment}} // {{.}}{{end}}\n"
)

// EnumTemplate spells an enum class: annotation lines above the
// declaration, one constant per variant under its own doc block
// and annotations, then the instance fields and methods a Java
// enum may declare behind the closing semicolon, each spelled the
// way the class template spells its members.
const EnumTemplate = "{{docs .Doc}}{{annotate .Annotations}}" +
	"{{typemods .}}enum {{.Name}} {\n" +
	"{{- range .Variants.Items}}\n{{docs .Doc \"    \"}}{{annotate .Annotations \"    \"}}" +
	"    {{enumvariant .}},{{with .Comment}} // {{.}}{{end}}\n" +
	"{{- end}}" +
	"{{- if or .Fields.Len .Methods.Len}}\n    ;{{end}}" +
	"{{- range .Fields.Items}}\n{{docs .Doc \"    \"}}{{annotate .Annotations \"    \"}}" +
	"    {{fieldmods .}}{{spell .Type}} {{.Name}}{{with .Value}} = {{.}}{{end}};" +
	"{{with .Comment}} // {{.}}{{end}}\n" +
	"{{- end}}" +
	"{{- range .Methods.Items}}\n{{docs .Doc \"    \"}}{{annotate .Annotations \"    \"}}" +
	"{{if .Override}}    @Override\n{{end}}" +
	"    {{methodmods .}}{{with typeparams .TypeParams}}{{.}} {{end}}" +
	"{{results .Returns}} {{.Name}}({{params .Params}}){{throws .Throws}}" +
	"{{if .Abstract}};{{else}} {\n{{body .}}    }{{end}}{{with .Comment}} // {{.}}{{end}}\n" +
	"{{- end}}\n}{{with .Comment}} // {{.}}{{end}}\n"

// KindTemplates maps each emit kind to the template that spells
// it. A kind absent from the map is one the Java backend cannot
// spell at file level: the render reports it and skips that
// declaration, which is what the feature matrix records.
func KindTemplates() map[symbol.Kind]string {
	return map[symbol.Kind]string{
		symbol.KindStruct:    StructTemplate,
		symbol.KindInterface: InterfaceTemplate,
		symbol.KindEnum:      EnumTemplate,
	}
}
