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
	// StructTemplate spells a struct and its fields: attribute
	// lines above the declaration, its visibility and type
	// parameters behind the name, each field under its own doc
	// lines and attributes with its own visibility.
	StructTemplate = "{{docs .Doc}}{{attrs .Annotations}}" +
		"{{structmods .}}struct {{.Name}}{{typeparams .TypeParams}} {\n" +
		"{{- range .Fields.Items}}\n{{docs .Doc \"    \"}}{{attrs .Annotations \"    \"}}" +
		"    {{fieldmods .}}{{.Name}}: {{spell .Type}},\n" +
		"{{- end}}\n}\n"

	// InterfaceTemplate spells a trait, its visibility and type
	// parameters behind the name: method signatures taking the
	// receiver by reference at instance level and standing alone
	// at type level, async where stated, each with its own
	// parameter list. A method carrying a default body places it;
	// the rest close as signatures.
	InterfaceTemplate = "{{docs .Doc}}{{attrs .Annotations}}" +
		"{{vis .Visibility .Name}}trait {{.Name}}{{typeparams .TypeParams}}{{supertraits .}} {\n" +
		"{{- range .Methods.Items}}\n{{docs .Doc \"    \"}}{{attrs .Annotations \"    \"}}" +
		"    {{traitfn .}}fn {{.Name}}{{typeparams .TypeParams}}({{selfparams .}})" +
		"{{results .Returns}}{{if .HasDefault}} {\n{{body .}}    }{{else}};{{end}}\n" +
		"{{- end}}\n}\n"

	// FunctionTemplate spells a free function: attribute lines
	// above the declaration, its visibility and asynchrony before
	// fn, its type parameters behind the name, and places its
	// body.
	FunctionTemplate = "{{docs .Doc}}{{attrs .Annotations}}{{fnmods .}}fn " +
		"{{.Name}}{{typeparams .TypeParams}}" +
		"({{params .Params}}){{results .Returns}} {\n{{body .}}}\n"

	// EnumTemplate spells a payloadless enum: one variant per
	// line under its own doc lines and attributes, a stated value
	// as its discriminant.
	EnumTemplate = "{{docs .Doc}}{{attrs .Annotations}}" +
		"{{enummods .}}enum {{.Name}} {\n" +
		"{{- range .Variants.Items}}\n{{docs .Doc \"    \"}}{{attrs .Annotations \"    \"}}" +
		"    {{.Name}}{{with .Value}} = {{.}}{{end}},\n" +
		"{{- end}}\n}\n"

	// SumTemplate spells a data enum: attribute lines above the
	// declaration, its visibility and type parameters behind the
	// name, one variant per line under its own doc lines and
	// attributes, each payload inline as named fields in braces or
	// bare types in parentheses.
	SumTemplate = "{{docs .Doc}}{{attrs .Annotations}}" +
		"{{summods .}}enum {{.Name}}{{typeparams .TypeParams}} {\n" +
		"{{- range .Variants.Items}}\n{{docs .Doc \"    \"}}{{attrs .Annotations \"    \"}}" +
		"    {{.Name}}{{sumpayload .}},\n" +
		"{{- end}}\n}\n"

	// AliasTemplate spells a type alias, its visibility and type
	// parameters behind the name; a defined type refuses through
	// the keywords helper, because a Rust alias is transparent.
	AliasTemplate = "{{docs .Doc}}{{attrs .Annotations}}" +
		"{{aliasmods .}}type {{.Name}}{{typeparams .TypeParams}}" +
		" = {{spell .Target}};\n"

	// ConstantTemplate spells a constant. Rust states a constant's
	// type always, so a declaration stating none reaches the unit
	// type and the compiler's refusal names the file.
	ConstantTemplate = "{{docs .Doc}}{{attrs .Annotations}}" +
		"{{vis .Visibility .Name}}const {{.Name}}: {{spell .Type}} = {{.Value}};\n"
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
		symbol.KindEnum:      EnumTemplate,
		symbol.KindSum:       SumTemplate,
		symbol.KindAlias:     AliasTemplate,
		symbol.KindConstant:  ConstantTemplate,
	}
}
