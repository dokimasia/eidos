// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"path"
	"strings"
	"text/template"

	"go.dokimi.dev/eidos/core/internal/genfile"
	"go.dokimi.dev/eidos/core/internal/gosource"
)

//go:embed templates/*.tmpl
var templates embed.FS

// templateDir is the directory of the embedded templates.
// templateGlob matches every template in it.
const (
	templateDir  = "templates"
	templateGlob = "*.tmpl"
)

// generatorName is the name the preamble's generated-code marker
// gives this generator.
const generatorName = "model"

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

// outputs lists every file the generator writes. [Generate] keys
// its result by path, so the order of the list changes no output.
//
// The mirror guard reads the list too. A generated file under one
// of these packages that the list does not name is a stray, and the
// guard reports it.
var outputs = []output{
	{Path: "emit/symbols.gen.go", Template: "symbols.gen.go.tmpl", Package: EmitPackage, Side: EmitPackage},
	{Path: "emit/symbols.gen_test.go", Template: "symbols.gen_test.go.tmpl", Package: EmitPackage, Side: EmitPackage},
	{Path: "emit/kinds.gen.go", Template: "kinds.gen.go.tmpl", Package: EmitPackage, Side: EmitPackage},
	{Path: "emit/kinds.gen_test.go", Template: "kinds.gen_test.go.tmpl", Package: EmitPackage, Side: EmitPackage},
	{Path: "emit/slots.gen.go", Template: "slots.gen.go.tmpl", Package: EmitPackage, Side: EmitPackage},
	{
		Path: "emit/slots.gen_test.go", Template: "slots.gen_test.go.tmpl",
		Package: EmitPackage, Side: EmitPackage,
	},
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
	{
		Path: "node/fingerprint.gen.go", Template: "fingerprint.gen.go.tmpl",
		Package: NodePackage, Side: NodePackage,
	},
	{
		Path: "node/fingerprint.gen_test.go", Template: "fingerprint.gen_test.go.tmpl",
		Package: NodePackage, Side: NodePackage,
	},
	{Path: "symbol/kind.gen.go", Template: "kind.gen.go.tmpl", Package: SymbolPackage},
	{Path: "symbol/kind.gen_test.go", Template: "kind.gen_test.go.tmpl", Package: SymbolPackage},
	{Path: "symbol/fact.gen.go", Template: "fact.gen.go.tmpl", Package: SymbolPackage},
	{Path: "symbol/fact.gen_test.go", Template: "fact.gen_test.go.tmpl", Package: SymbolPackage},
	{Path: "emit/facts.gen.go", Template: "facts.gen.go.tmpl", Package: EmitPackage, Side: EmitPackage},
	{Path: "emit/facts.gen_test.go", Template: "facts.gen_test.go.tmpl", Package: EmitPackage, Side: EmitPackage},
}

// OwnedDirs are the directories the generator writes into. The
// module root is one of them, because the match file is there. The
// mirror guard scans each directory for strays, and in the module
// root it scans only the files directly under the root.
var OwnedDirs = []string{".", SymbolPackage, NodePackage, EmitPackage}

// data is what a template renders against.
type data struct {
	// Header is the preamble every generated file opens with.
	Header string
	// Package is the Go package the file declares.
	Package string
	// Kinds are the lowered kinds in schema order, independent of
	// any model side.
	Kinds []KindSpec
	// Views are the kinds prepared for this file's model side.
	Views []view
	// NeedsSymbol reports whether the rendered file uses the symbol
	// package, so a template imports that package only when the
	// file needs it.
	NeedsSymbol bool
	// Identified reports whether every kind on this side has an
	// identity, which the Declaration interface and the traversal
	// typed by it require.
	Identified bool
	// Originated reports whether any kind on this side has origin
	// storage. A side with origin storage gets the OriginOf
	// function.
	Originated bool
	// Facts are the declared fact constant suffixes, first
	// encounter across kinds in schema order.
	Facts []string
	// Fingerprint is the node model's shape hash, which every unit
	// key folds.
	Fingerprint string
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
	r, err := newRenderer(kinds)
	if err != nil {
		return nil, err
	}
	return genfile.Render(outputs, func(out output) (string, []byte, error) {
		src, err := r.render(out)
		return out.Path, src, err
	})
}

// Regenerate renders the models from the schema of the module
// enclosing dir and writes them into it.
//
// The go:generate wrapper calls Regenerate and reports only its
// exit status. A directory outside any module is refused, and
// nothing is written unless every file rendered.
func Regenerate(dir string) error {
	root, err := gosource.ModuleRoot(dir)
	if err != nil {
		return err
	}
	return genfile.Regenerate(root, Generate)
}

// fingerprintOf hashes the node model's shape: every kind and every
// node-side field, name and type spelling, in schema order.
//
// The hash covers the lowered schema only, so a documentation edit
// leaves it unchanged. A schema change that alters the graph the
// same source produces changes the hash. Every unit key folds the
// result, so a recorded graph is never served across a schema
// change.
func fingerprintOf(kinds []KindSpec) string {
	h := sha256.New()
	for _, k := range kinds {
		fmt.Fprintf(h, "kind %s\n", k.Name)
		for _, f := range k.Fields {
			if f.Side == SideEmit {
				continue
			}
			fmt.Fprintf(h, "field %s %s elem=%s slice=%t symbol=%t\n",
				f.Name, f.Type, f.Elem, f.Slice, f.IsSymbol)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// slotsUseSymbol reports whether any slot's values are typed by the
// symbol package, which decides whether the slots file imports it.
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

// viewsIdentified reports whether every kind has an identity.
//
// The predicate is "every". A traversal typed by the Declaration
// interface visits only the kinds that satisfy it, so a side
// declares the interface only when every kind does.
func viewsIdentified(views []view) bool {
	for _, v := range views {
		if v.IDStorage == "" {
			return false
		}
	}
	return len(views) > 0
}

// viewsOriginated reports whether any kind has origin storage.
//
// The predicate is "any", the opposite of [viewsIdentified].
// OriginOf returns false for a kind without origin storage, so one
// kind with it is enough to declare the function.
func viewsOriginated(views []view) bool {
	for _, v := range views {
		if v.OriginStorage != "" {
			return true
		}
	}
	return false
}

// renderer renders the outputs of one [Generate] run from templates
// parsed once. It computes the facts and the fingerprint once per
// run and the views once per model side.
type renderer struct {
	templates   *template.Template
	kinds       []KindSpec
	facts       []string
	fingerprint string
	views       map[string][]view
}

// newRenderer parses every template and prepares the shared data.
func newRenderer(kinds []KindSpec) (*renderer, error) {
	tmpl, err := template.New(templateDir).Funcs(template.FuncMap{
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
	}).ParseFS(templates, path.Join(templateDir, templateGlob))
	if err != nil {
		return nil, fmt.Errorf("model: parse the templates: %w", err)
	}
	r := &renderer{
		templates:   tmpl,
		kinds:       kinds,
		facts:       factsOf(kinds),
		fingerprint: fingerprintOf(kinds),
		views:       map[string][]view{},
	}
	for _, out := range outputs {
		if _, prepared := r.views[out.Side]; !prepared {
			r.views[out.Side] = viewsFor(kinds, out.Side)
		}
	}
	return r, nil
}

// render executes one output's template.
func (r *renderer) render(out output) ([]byte, error) {
	views := r.views[out.Side]
	var buf bytes.Buffer
	err := r.templates.ExecuteTemplate(&buf, out.Template, data{
		Header:      genfile.Header(generatorName),
		Package:     out.Package,
		Kinds:       r.kinds,
		Views:       views,
		NeedsSymbol: slotsUseSymbol(views),
		Identified:  viewsIdentified(views),
		Originated:  viewsOriginated(views),
		Facts:       r.facts,
		Fingerprint: r.fingerprint,
	})
	if err != nil {
		return nil, fmt.Errorf("model: render %s: %w", out.Path, err)
	}
	return buf.Bytes(), nil
}
