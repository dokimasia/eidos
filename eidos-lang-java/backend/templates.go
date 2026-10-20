// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// FileTemplate is the skeleton of a Java file. It renders the package
// clause, the import block of the file and then its declarations. The
// output contract writes the marker of generated code after the
// formatter runs, so the template does not contain it.
const FileTemplate = "{{" + FuncPackage + " .Pkg}}{{" + render.BuiltinImports + "}}" +
	"{{" + render.BuiltinDecls + "}}"

// Java declares everything inside a type, so a class, an interface and
// an enum are the only file-level declarations, and the backend refuses
// every other file-level kind. A member renders as public, because other
// code calls a generated API. Each template renders a file-level type
// and a member type alike. The keywords that it writes are therefore
// those of a member type, and the lowering refuses a file-level type
// with a keyword that only a member type can have.
const (
	// StructTemplate renders a class. The annotation lines precede the
	// declaration, and the keywords and the type parameters follow the
	// name. The class contains its fields with their initializers, its
	// methods with their bodies, and then its member types at member
	// depth through their own kind templates. Each member has its own doc
	// block, annotations, and keywords in the order of the Java grammar.
	// An overriding method gets the Override annotation, and an abstract
	// method is a signature alone. The type parameter list of a generic
	// method precedes its return type, where Java declares it.
	StructTemplate = "{{docs .Doc}}{{annotate .Annotations}}" +
		"{{typemods .}}class {{.Name}}{{typeparams .TypeParams}}{{heritage .}} {\n" +
		"{{- range .Fields.Items}}\n{{memberdocs .Doc}}{{memberannotate .Annotations}}" +
		"    {{fieldmods .}}{{spell .Type}} {{.Name}}{{with .Value}} = {{.}}{{end}};" +
		"{{with .Comment}} // {{.}}{{end}}\n" +
		"{{- end}}" +
		"{{- range .Methods.Items}}\n{{memberdocs .Doc}}{{memberannotate .Annotations}}" +
		"{{if .Override}}    @Override\n{{end}}" +
		"    {{methodmods .}}{{with typeparams .TypeParams}}{{.}} {{end}}" +
		"{{results .Returns}} {{.Name}}({{params .Params}}){{throws .Throws}}" +
		"{{if .Abstract}};{{else}} {\n{{memberbody .}}    }{{end}}{{with .Comment}} // {{.}}{{end}}\n" +
		"{{- end}}" +
		"{{- range .Types.Items}}\n{{nested \"    \" .}}\n{{- end}}\n}{{with .Comment}} // {{.}}{{end}}\n"

	// InterfaceTemplate renders an interface. The annotation lines
	// precede the declaration, and the keywords and the type parameters
	// follow the name. The interface contains its properties as
	// constants, its signatures, and then its member types at member
	// depth through their own kind templates. A signature is public by
	// default in Java, and the type parameter list of a generic method
	// precedes its return type. A member type refuses a visibility
	// narrower than public. A method with a body renders as default at
	// instance level or as static at type level, with its body.
	InterfaceTemplate = "{{docs .Doc}}{{annotate .Annotations}}" +
		"{{typemods .}}interface {{.Name}}{{typeparams .TypeParams}}{{heritage .}} {\n" +
		"{{- range .Fields.Items}}\n{{memberdocs .Doc}}{{memberannotate .Annotations}}" +
		"    {{constantmods .}}{{spell .Type}} {{.Name}} = {{.Value}};" +
		"{{with .Comment}} // {{.}}{{end}}\n" +
		"{{- end}}" +
		"{{- range .Methods.Items}}\n{{memberdocs .Doc}}{{memberannotate .Annotations}}" +
		"    {{sigmods .}}{{with typeparams .TypeParams}}{{.}} {{end}}" +
		"{{results .Returns}} {{.Name}}({{params .Params}}){{throws .Throws}}" +
		"{{if .HasDefault}} {\n{{memberbody .}}    }{{else}};{{end}}{{with .Comment}} // {{.}}{{end}}\n" +
		"{{- end}}" +
		"{{- range .Types.Items}}\n{{membertype .}}{{nested \"    \" .}}\n{{- end}}\n}" +
		"{{with .Comment}} // {{.}}{{end}}\n"
)

// EnumTemplate renders an enum class. The annotation lines precede the
// declaration, and the contracts of the enum follow its name. The enum
// contains one constant per variant, each with its own doc block and
// annotations. The instance fields and methods that a Java enum can
// declare follow the closing semicolon, and the template renders them as
// the class template renders its members.
const EnumTemplate = "{{docs .Doc}}{{annotate .Annotations}}" +
	"{{typemods .}}enum {{.Name}}{{heritage .}} {\n" +
	"{{- range .Variants.Items}}\n{{memberdocs .Doc}}{{memberannotate .Annotations}}" +
	"    {{enumvariant .}},{{with .Comment}} // {{.}}{{end}}\n" +
	"{{- end}}" +
	"{{- if or .Fields.Len .Methods.Len}}\n    ;{{end}}" +
	"{{- range .Fields.Items}}\n{{memberdocs .Doc}}{{memberannotate .Annotations}}" +
	"    {{fieldmods .}}{{spell .Type}} {{.Name}}{{with .Value}} = {{.}}{{end}};" +
	"{{with .Comment}} // {{.}}{{end}}\n" +
	"{{- end}}" +
	"{{- range .Methods.Items}}\n{{memberdocs .Doc}}{{memberannotate .Annotations}}" +
	"{{if .Override}}    @Override\n{{end}}" +
	"    {{methodmods .}}{{with typeparams .TypeParams}}{{.}} {{end}}" +
	"{{results .Returns}} {{.Name}}({{params .Params}}){{throws .Throws}}" +
	"{{if .Abstract}};{{else}} {\n{{memberbody .}}    }{{end}}{{with .Comment}} // {{.}}{{end}}\n" +
	"{{- end}}\n}{{with .Comment}} // {{.}}{{end}}\n"

// KindTemplates returns a map from each emit kind to the template that
// renders it. The lowering reshapes a file-level kind that the map does
// not contain, as it reshapes a sum, or [RefusedKinds] refuses it.
// KindTemplates builds a new map on every call for the caller to keep.
// The map and its one group allocate twice.
func KindTemplates() map[symbol.Kind]string {
	return map[symbol.Kind]string{
		symbol.KindStruct:    StructTemplate,
		symbol.KindInterface: InterfaceTemplate,
		symbol.KindEnum:      EnumTemplate,
	}
}

// RefusedKinds returns a map from each emit kind that Java cannot
// declare at file level to the reason. The render reports the reason
// with every declaration of the kind that it skips. Each such kind is a
// member of a type in Java, so a generator declares it inside a class.
// RefusedKinds builds a new map on every call for the caller to keep.
// The map and its one group allocate twice.
func RefusedKinds() map[symbol.Kind]string {
	return map[symbol.Kind]string{
		symbol.KindFunction: "Java declares every function as a method inside a class",
		symbol.KindMethod:   "Java declares a method inside the type it belongs to",
		symbol.KindAlias:    "Java has no type alias",
		symbol.KindConstant: "Java declares a constant as a static final field inside a class",
		symbol.KindVariable: "Java declares a variable as a field inside a class",
	}
}
