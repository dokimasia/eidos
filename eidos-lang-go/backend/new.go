// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go/format"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/go/spell"
	"go.dokimi.dev/eidos/sdk/backend"
)

// New returns the Go rendering backend: the module's declared pieces
// composed through the kernel's kit into a [backend.Backend]. Its spoke
// is [spell.Type], which spells a type of another language in Go, and its
// binder of gRPC is [spell.Handler] with [spell.Home]. Its split is
// [spell.Split], which writes the declarations of an external family that
// test their package from inside into an internal test file. Rendered files
// finalise through go/format, so a file that does not parse is withheld
// and reported instead of written, and the bytes that remain are the
// bytes gofmt leaves.
//
// New allocates the kit's build, chiefly the parse of the file template
// and the seven kind templates: 1,613 allocations.
func New() backend.Backend {
	return backend.New(golang.Name, golang.Target, golang.Syntax()).
		Version(golang.Version).
		FileTemplate(FileTemplate).
		KindTemplates(KindTemplates()).
		Coverage(Coverage()).
		Funcs(Funcs).
		Naming(spell.Filename).
		Split(spell.Split).
		Packages(spell.Package).
		Respell(spell.Name).
		Policies(golang.Policies()...).
		Types(spell.Type).
		Binder(spell.Handler, spell.Home).
		Lower(Lower).
		Scaffold(Scaffold).
		Value(Value).
		Imports(Imports).
		TemplateImport(TemplateImport).
		Finalise(format.Source).
		Build()
}
