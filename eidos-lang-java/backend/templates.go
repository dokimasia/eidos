// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend

import (
	"go.dokimi.dev/eidos/core/render"
	"go.dokimi.dev/eidos/core/symbol"
)

// FileTemplate is the Java file skeleton: the package clause, the
// import block the file collected, then its declarations. The
// generated-code marker is not here: the output contract writes
// it after the formatter ran.
const FileTemplate = "{{" + FuncPackage + " .Pkg}}{{" + render.BuiltinImports + "}}" +
	"{{" + render.BuiltinDecls + "}}"

// The kind templates. The inventory is two entries by design:
// Java states everything inside a type, so a class and an
// interface are the only file-level declarations, and every
// other kind is reported as one the target cannot spell. Members
// render as public, because a generated API exists to be called.
const (
	// StructTemplate spells a class: annotation lines above the
	// declaration, its keywords and type parameters behind the
	// name, then fields with initializers and methods with their
	// bodies, each member under its own doc block and annotations,
	// its keywords in Java's stated order. An overriding method
	// carries the Override annotation, and an abstract method is a
	// signature alone. A generic method's own parameter list
	// spells before its return type, which is where Java states
	// it.
	StructTemplate = "{{docs .Doc}}{{annotate .Annotations}}" +
		"{{typemods .}}class {{.Name}}{{typeparams .TypeParams}}{{heritage .}} {\n" +
		"{{- range .Fields.Items}}\n{{docs .Doc \"    \"}}{{annotate .Annotations \"    \"}}" +
		"    {{fieldmods .}}{{spell .Type}} {{.Name}}{{with .Value}} = {{.}}{{end}};\n" +
		"{{- end}}" +
		"{{- range .Methods.Items}}\n{{docs .Doc \"    \"}}{{annotate .Annotations \"    \"}}" +
		"{{if .Override}}    @Override\n{{end}}" +
		"    {{methodmods .}}{{with typeparams .TypeParams}}{{.}} {{end}}" +
		"{{results .Returns}} {{.Name}}({{params .Params}}){{throws .Throws}}" +
		"{{if .Abstract}};{{else}} {\n{{body .}}    }{{end}}\n" +
		"{{- end}}\n}\n"

	// InterfaceTemplate spells an interface: annotation lines
	// above the declaration, its keywords and type parameters
	// behind the name, then signatures, implicitly public the way
	// Java reads them, a generic method's own parameter list
	// before its return type. A method carrying a body spells
	// default at instance level or static at type level, and
	// places the body.
	InterfaceTemplate = "{{docs .Doc}}{{annotate .Annotations}}" +
		"{{typemods .}}interface {{.Name}}{{typeparams .TypeParams}}{{heritage .}} {\n" +
		"{{- range .Methods.Items}}\n{{docs .Doc \"    \"}}{{annotate .Annotations \"    \"}}" +
		"    {{sigmods .}}{{with typeparams .TypeParams}}{{.}} {{end}}" +
		"{{results .Returns}} {{.Name}}({{params .Params}}){{throws .Throws}}" +
		"{{if .HasDefault}} {\n{{body .}}    }{{else}};{{end}}\n" +
		"{{- end}}\n}\n"
)

// KindTemplates maps each emit kind to the template that spells
// it. A kind absent from the map is one the Java backend cannot
// spell at file level: the render reports it and skips that
// declaration, which is what the feature matrix records.
func KindTemplates() map[symbol.Kind]string {
	return map[symbol.Kind]string{
		symbol.KindStruct:    StructTemplate,
		symbol.KindInterface: InterfaceTemplate,
	}
}
