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
	// StructTemplate spells a class: fields, then methods with
	// their bodies, each member under its own doc block.
	StructTemplate = "{{docs .Doc}}public class {{.Name}} {\n" +
		"{{- range .Fields.Items}}\n{{docs .Doc \"    \"}}" +
		"    public {{spell .Type}} {{.Name}};\n" +
		"{{- end}}" +
		"{{- range .Methods.Items}}\n{{docs .Doc \"    \"}}" +
		"    public {{results .Returns}} {{.Name}}({{params .Params}}) {\n" +
		"{{body .}}    }\n" +
		"{{- end}}\n}\n"

	// InterfaceTemplate spells an interface: signatures alone,
	// implicitly public the way Java reads them.
	InterfaceTemplate = "{{docs .Doc}}public interface {{.Name}} {\n" +
		"{{- range .Methods.Items}}\n{{docs .Doc \"    \"}}" +
		"    {{results .Returns}} {{.Name}}({{params .Params}});\n" +
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
