// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backend

import (
	"go.dokimi.dev/eidos/core/render"
	"go.dokimi.dev/eidos/core/symbol"
)

// FileTemplate is the Rust file skeleton: the use block the file
// collected, then its declarations. There is no package clause;
// the file is the module. The generated-code marker is not here:
// the output contract writes it after the formatter ran.
const FileTemplate = "{{" + render.BuiltinImports + "}}{{" + render.BuiltinDecls + "}}"

// The kind templates, one per emit kind Rust declares at module
// level. A standalone method is absent by design: Rust groups
// methods under an impl block per receiver, which a template
// rendering one declaration at a time cannot write, so the kind
// is reported rather than guessed at. A variable is absent too:
// the model carries no initialiser, and a Rust static requires
// one.
const (
	// StructTemplate spells a struct and its fields, public the
	// way a generated API is consumed, each field under its own
	// doc lines.
	StructTemplate = "{{docs .Doc}}pub struct {{.Name}} {\n" +
		"{{- range .Fields.Items}}\n{{docs .Doc \"    \"}}    pub {{.Name}}: {{spell .Type}},\n" +
		"{{- end}}\n}\n"

	// InterfaceTemplate spells a trait: method signatures taking
	// the receiver by reference, no bodies.
	InterfaceTemplate = "{{docs .Doc}}pub trait {{.Name}} {\n" +
		"{{- range .Methods.Items}}\n{{docs .Doc \"    \"}}" +
		"    fn {{.Name}}(&self{{with params .Params}}, {{.}}{{end}})" +
		"{{results .Returns}};\n" +
		"{{- end}}\n}\n"

	// FunctionTemplate spells a free function and places its body.
	FunctionTemplate = "{{docs .Doc}}pub fn {{.Name}}({{params .Params}})" +
		"{{results .Returns}} {\n{{body .}}}\n"

	// AliasTemplate spells a type alias.
	AliasTemplate = "{{docs .Doc}}pub type {{.Name}} = {{spell .Target}};\n"

	// ConstantTemplate spells a constant. Rust states a constant's
	// type always, so a declaration stating none reaches the unit
	// type and the compiler's refusal names the file.
	ConstantTemplate = "{{docs .Doc}}pub const {{.Name}}: {{spell .Type}} = {{.Value}};\n"
)

// KindTemplates maps each emit kind to the template that spells
// it. A kind absent from the map is one the Rust backend cannot
// spell at module level: the render reports it and skips that
// declaration, which is what the feature matrix records.
func KindTemplates() map[symbol.Kind]string {
	return map[symbol.Kind]string{
		symbol.KindStruct:    StructTemplate,
		symbol.KindInterface: InterfaceTemplate,
		symbol.KindFunction:  FunctionTemplate,
		symbol.KindAlias:     AliasTemplate,
		symbol.KindConstant:  ConstantTemplate,
	}
}
