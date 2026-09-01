// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"bytes"
	"embed"
	"fmt"
	"path"
	"strings"
	"text/template"

	"go.dokimi.dev/eidos/core/internal/genfile"
)

//go:embed templates/*.tmpl
var templates embed.FS

// templateDir holds the embedded templates.
const templateDir = "templates"

// headerTemplate is the shared preamble every output file opens
// with. It is defined rather than rendered on its own.
const headerTemplate = "header.tmpl"

// output is one file the generator writes: which template renders
// it, into which package, and where it arrives.
type output struct {
	// Path is the file's module-relative slash path.
	Path string
	// Template names its template within the embedded tree.
	Template string
	// Package is the Go package the file declares.
	Package string
	// Side is the model the file belongs to, empty for a file that
	// belongs to neither.
	Side string
}

// outputs are every file the generator owns, in path order.
//
// The list is the mirror guard's subject too: a generated file
// under one of these packages that this list does not name is a
// stray, and the guard says so.
var outputs = []output{
	{Path: "emit/symbols.gen.go", Template: "symbols.gen.go.tmpl", Package: EmitPackage, Side: EmitPackage},
	{Path: "emit/symbols.gen_test.go", Template: "symbols.gen_test.go.tmpl", Package: EmitPackage, Side: EmitPackage},
	{Path: "emit/kinds.gen.go", Template: "kinds.gen.go.tmpl", Package: EmitPackage, Side: EmitPackage},
	{Path: "emit/kinds.gen_test.go", Template: "kinds.gen_test.go.tmpl", Package: EmitPackage, Side: EmitPackage},
	{Path: "emit/slots.gen.go", Template: "slots.gen.go.tmpl", Package: EmitPackage, Side: EmitPackage},
	{Path: "emit/names.gen.go", Template: "names.gen.go.tmpl", Package: EmitPackage, Side: EmitPackage},
	{Path: "emit/names.gen_test.go", Template: "names.gen_test.go.tmpl", Package: EmitPackage, Side: EmitPackage},
	{Path: "emit/walk.gen.go", Template: "walk.gen.go.tmpl", Package: EmitPackage, Side: EmitPackage},
	{Path: "emit/walk.gen_test.go", Template: "walk.gen_test.go.tmpl", Package: EmitPackage, Side: EmitPackage},
	{Path: "match.gen.go", Template: "match.gen.go.tmpl", Package: RootPackage, Side: NodePackage},
	{Path: "match.gen_test.go", Template: "match.gen_test.go.tmpl", Package: RootTestPackage, Side: NodePackage},
	{Path: "node/symbols.gen.go", Template: "symbols.gen.go.tmpl", Package: NodePackage, Side: NodePackage},
	{Path: "node/symbols.gen_test.go", Template: "symbols.gen_test.go.tmpl", Package: NodePackage, Side: NodePackage},
	{Path: "node/kinds.gen.go", Template: "kinds.gen.go.tmpl", Package: NodePackage, Side: NodePackage},
	{Path: "node/kinds.gen_test.go", Template: "kinds.gen_test.go.tmpl", Package: NodePackage, Side: NodePackage},
	{Path: "node/walk.gen.go", Template: "walk.gen.go.tmpl", Package: NodePackage, Side: NodePackage},
	{Path: "node/walk.gen_test.go", Template: "walk.gen_test.go.tmpl", Package: NodePackage, Side: NodePackage},
	{Path: "symbol/kind.gen.go", Template: "kind.gen.go.tmpl", Package: SymbolPackage},
	{Path: "symbol/kind.gen_test.go", Template: "kind.gen_test.go.tmpl", Package: SymbolPackage},
	{Path: "symbol/fact.gen.go", Template: "fact.gen.go.tmpl", Package: SymbolPackage},
	{Path: "symbol/fact.gen_test.go", Template: "fact.gen_test.go.tmpl", Package: SymbolPackage},
	{Path: "emit/facts.gen.go", Template: "facts.gen.go.tmpl", Package: EmitPackage, Side: EmitPackage},
	{Path: "emit/facts.gen_test.go", Template: "facts.gen_test.go.tmpl", Package: EmitPackage, Side: EmitPackage},
}

// OwnedDirs are the directories the generator writes into. The
// mirror guard scans them for strays.
var OwnedDirs = []string{SymbolPackage, NodePackage, EmitPackage}

// data is what a template renders against.
type data struct {
	// Package is the Go package the file declares.
	Package string
	// Kinds are the lowered kinds, for a file that needs the schema
	// order rather than one model side.
	Kinds []KindSpec
	// Views are the kinds prepared for this file's model side.
	Views []view
	// NeedsSymbol says whether the rendered file reaches the shared
	// vocabulary, so a template imports it only when it uses it.
	NeedsSymbol bool
	// Identified says whether every kind on this side names itself,
	// which is what lets the side declare the Declaration interface
	// and the traversal typed by it.
	Identified bool
	// Originated says whether any kind on this side carries origin
	// storage, which is why the side gets the OriginOf function.
	Originated bool
	// Facts are the declared fact constant suffixes, first
	// encounter across kinds in schema order.
	Facts []string
}

// Generate renders every generated file from the schema under
// modRoot.
//
// It returns the whole output in memory, keyed by module-relative
// slash path, and writes nothing. A caller puts it on disk with
// [genfile.Write] or compares it with [genfile.Verify], which is
// what makes the mirror guard a plain test.
//
// Nothing is returned unless every file rendered and formatted, so
// a template fault cannot leave a half-generated tree.
func Generate(modRoot string) (genfile.Set, error) {
	kinds, err := Lower(path.Join(modRoot, SchemaDir), modRoot)
	if err != nil {
		return nil, err
	}

	set := make(genfile.Set, len(outputs))
	for _, out := range outputs {
		rendered, err := render(out, kinds)
		if err != nil {
			return nil, err
		}
		formatted, err := genfile.Format(out.Path, rendered)
		if err != nil {
			return nil, err
		}
		set[out.Path] = formatted
	}
	return set, nil
}

// slotsUseSymbol reports whether any slot holds values typed by the
// shared vocabulary, which is what decides the slots file's import.
func slotsUseSymbol(views []view) bool {
	for _, v := range views {
		for _, slot := range v.Slots {
			if strings.Contains(slot.Decl, symbolQualifier) {
				return true
			}
		}
	}
	return false
}

// viewsIdentified reports whether every kind carries an identity.
//
// The predicate is "every" rather than "any": the Declaration
// interface is only worth declaring on a side where no kind fails to
// satisfy it, because a traversal typed by it would otherwise drop
// whichever kinds did.
func viewsIdentified(views []view) bool {
	for _, v := range views {
		if v.IDStorage == "" {
			return false
		}
	}
	return len(views) > 0
}

// viewsOriginated reports whether any kind carries origin storage.
//
// The predicate is "any" rather than "every", the opposite of
// [viewsIdentified]: OriginOf is a function returning false for the
// kinds without the role, so one carrying kind already justifies it.
func viewsOriginated(views []view) bool {
	for _, v := range views {
		if v.OriginStorage != "" {
			return true
		}
	}
	return false
}

// render executes one output's template.
func render(out output, kinds []KindSpec) ([]byte, error) {
	tmpl, err := template.New(out.Template).Funcs(template.FuncMap{
		// firstNamed returns the first kind declaring its own name,
		// which the generated tests build their shared cases over.
		"firstNamed": func(views []view) *view {
			for i := range views {
				if views[i].NameStorage != "" {
					return &views[i]
				}
			}
			return nil
		},
	}).ParseFS(templates,
		path.Join(templateDir, headerTemplate),
		path.Join(templateDir, out.Template))
	if err != nil {
		return nil, fmt.Errorf("model: parse template %s: %w", out.Template, err)
	}

	views := viewsFor(kinds, out.Side)

	var buf bytes.Buffer
	err = tmpl.ExecuteTemplate(&buf, out.Template, data{
		Package:     out.Package,
		Kinds:       kinds,
		Views:       views,
		NeedsSymbol: slotsUseSymbol(views),
		Identified:  viewsIdentified(views),
		Originated:  viewsOriginated(views),
		Facts:       factsOf(kinds),
	})
	if err != nil {
		return nil, fmt.Errorf("model: render %s: %w", out.Path, err)
	}
	return buf.Bytes(), nil
}
