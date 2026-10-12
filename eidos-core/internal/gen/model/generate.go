// Copyright Dokimasia B.V. 2026
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

// generatorName is the name of this generator in the generated-code
// marker of the preamble.
const generatorName = "model"

// output is one file that the generator writes. Its fields are the
// path, the template, the package, the model side and the tested
// package.
type output struct {
	// Path is the module-relative slash path of the file.
	Path string
	// Template is the name of its template in the embedded tree.
	Template string
	// Package is the Go package that the file declares.
	Package string
	// Side is the model of the file. It is empty for a file of neither
	// model.
	Side string
	// Model is the package that a black-box test file checks and
	// imports under the dot. The test then writes the declarations of
	// the package as the package writes them. It is empty for a file of
	// production code and for a test file that qualifies its imports.
	Model string
}

// outputs lists every file that the generator writes. [Generate] keys
// its result by path, so the order of the list does not change the
// output.
//
// The mirror guard reads the list too. A generated file under one of
// these packages that is not in the list is a stray, and the guard
// reports it.
var outputs = []output{
	{Path: "emit/symbols.gen.go", Template: "symbols.gen.go.tmpl", Package: EmitPackage, Side: EmitPackage},
	{
		Path: "emit/symbols.gen_test.go", Template: "symbols.gen_test.go.tmpl",
		Package: EmitTestPackage, Side: EmitPackage, Model: EmitPackage,
	},
	{Path: "emit/kinds.gen.go", Template: "kinds.gen.go.tmpl", Package: EmitPackage, Side: EmitPackage},
	{
		Path: "emit/kinds.gen_test.go", Template: "kinds.gen_test.go.tmpl",
		Package: EmitTestPackage, Side: EmitPackage, Model: EmitPackage,
	},
	{Path: "emit/slots.gen.go", Template: "slots.gen.go.tmpl", Package: EmitPackage, Side: EmitPackage},
	{
		Path: "emit/slots.gen_test.go", Template: "slots.gen_test.go.tmpl",
		Package: EmitTestPackage, Side: EmitPackage, Model: EmitPackage,
	},
	{Path: "emit/names.gen.go", Template: "names.gen.go.tmpl", Package: EmitPackage, Side: EmitPackage},
	{
		Path: "emit/names.gen_test.go", Template: "names.gen_test.go.tmpl",
		Package: EmitTestPackage, Side: EmitPackage, Model: EmitPackage,
	},
	{Path: "emit/walk.gen.go", Template: "walk.gen.go.tmpl", Package: EmitPackage, Side: EmitPackage},
	{
		Path: "emit/walk.gen_test.go", Template: "walk.gen_test.go.tmpl",
		Package: EmitTestPackage, Side: EmitPackage, Model: EmitPackage,
	},
	{Path: "match.gen.go", Template: "match.gen.go.tmpl", Package: RootPackage, Side: NodePackage},
	{Path: "match.gen_test.go", Template: "match.gen_test.go.tmpl", Package: RootTestPackage, Side: NodePackage},
	{Path: "node/symbols.gen.go", Template: "symbols.gen.go.tmpl", Package: NodePackage, Side: NodePackage},
	{
		Path: "node/symbols.gen_test.go", Template: "symbols.gen_test.go.tmpl",
		Package: NodeTestPackage, Side: NodePackage, Model: NodePackage,
	},
	{Path: "node/kinds.gen.go", Template: "kinds.gen.go.tmpl", Package: NodePackage, Side: NodePackage},
	{
		Path: "node/kinds.gen_test.go", Template: "kinds.gen_test.go.tmpl",
		Package: NodeTestPackage, Side: NodePackage, Model: NodePackage,
	},
	{Path: "node/walk.gen.go", Template: "walk.gen.go.tmpl", Package: NodePackage, Side: NodePackage},
	{
		Path: "node/walk.gen_test.go", Template: "walk.gen_test.go.tmpl",
		Package: NodeTestPackage, Side: NodePackage, Model: NodePackage,
	},
	{Path: "node/codec.gen.go", Template: "codec.gen.go.tmpl", Package: NodePackage, Side: NodePackage},
	{
		Path: "node/codec.gen_test.go", Template: "codec.gen_test.go.tmpl",
		Package: NodeTestPackage, Side: NodePackage, Model: NodePackage,
	},
	{
		Path: "node/fingerprint.gen.go", Template: "fingerprint.gen.go.tmpl",
		Package: NodePackage, Side: NodePackage,
	},
	{
		Path: "node/fingerprint.gen_test.go", Template: "fingerprint.gen_test.go.tmpl",
		Package: NodeTestPackage, Side: NodePackage, Model: NodePackage,
	},
	{Path: "symbol/kind.gen.go", Template: "kind.gen.go.tmpl", Package: SymbolPackage},
	{
		Path: "symbol/kind.gen_test.go", Template: "kind.gen_test.go.tmpl",
		Package: SymbolTestPackage, Model: SymbolPackage,
	},
	{Path: "symbol/fact.gen.go", Template: "fact.gen.go.tmpl", Package: SymbolPackage},
	{
		Path: "symbol/fact.gen_test.go", Template: "fact.gen_test.go.tmpl",
		Package: SymbolTestPackage, Model: SymbolPackage,
	},
	{Path: "emit/facts.gen.go", Template: "facts.gen.go.tmpl", Package: EmitPackage, Side: EmitPackage},
	{
		Path: "emit/facts.gen_test.go", Template: "facts.gen_test.go.tmpl",
		Package: EmitTestPackage, Side: EmitPackage, Model: EmitPackage,
	},
}

// OwnedDirs are the directories that the generator writes into. The
// module root is one of them, because the match file is there. The
// mirror guard scans each directory for strays, and in the module
// root it scans only the files directly under the root.
var OwnedDirs = []string{".", SymbolPackage, NodePackage, EmitPackage}

// data is the input of a template.
type data struct {
	// Header is the preamble of every generated file.
	Header string
	// Package is the Go package that the file declares.
	Package string
	// Model is the package that a black-box test file checks and
	// imports under the dot. It is empty for every other file.
	Model string
	// Kinds are the lowered kinds in schema order. They do not depend
	// on a model side.
	Kinds []KindSpec
	// Views are the kinds as the model side of this file declares
	// them.
	Views []view
	// NeedsSymbol reports whether the rendered file uses the symbol
	// package, so a template imports that package only when the
	// file needs it.
	NeedsSymbol bool
	// Identified reports whether every kind on this side has an
	// identity. The Declaration interface and its traversal require
	// one.
	Identified bool
	// Originated reports whether any kind on this side has origin
	// storage. A side with origin storage gets the OriginOf
	// function.
	Originated bool
	// Facts are the suffixes of the declared fact constants, in the
	// order of first use across the kinds in schema order.
	Facts []string
	// Fingerprint is the hash of the shape of the node model. Every
	// unit key folds it.
	Fingerprint string
}

// Generate renders every generated file from the schema under
// modRoot.
//
// It returns the whole output in memory and writes nothing. The keys
// are module-relative slash paths. A caller writes the output with
// [genfile.Write] or compares it with [genfile.Verify], so the mirror
// guard is a plain test.
//
// Generate returns nothing unless every file rendered and formatted,
// so a template fault cannot leave a half-generated tree.
func Generate(modRoot string) (genfile.Set, error) {
	schema, err := Lower(path.Join(modRoot, SchemaDir), modRoot)
	if err != nil {
		return nil, err
	}
	r, err := newRenderer(schema)
	if err != nil {
		return nil, err
	}
	return genfile.Render(outputs, func(out output) (string, []byte, error) {
		src, err := r.render(out)
		return out.Path, src, err
	})
}

// Regenerate renders the models from the schema of the module that
// contains dir, and writes them into that module.
//
// The go:generate wrapper calls Regenerate and reports only its exit
// status. Regenerate returns an error for a directory outside any
// module. It writes nothing unless every file rendered.
func Regenerate(dir string) error {
	root, err := gosource.ModuleRoot(dir)
	if err != nil {
		return err
	}
	return genfile.Regenerate(root, Generate)
}

// fingerprintOf hashes the shape of the node model. It hashes every
// kind and the name and type of every node-side field in schema order,
// then the facts in schema order, then every enum of the symbol package
// with the names and values of its constants.
//
// The hash covers the lowered schema only, so a documentation edit
// leaves it unchanged. A schema change that alters the graph of the
// same source changes the hash. A change of the values of an enum
// changes it too, because an encoded graph writes them as integers.
// The kinds and facts in schema order fix the values of the generated
// Kind and Fact constants. Every unit key folds the result, so a
// recorded graph is never served across either change.
func fingerprintOf(schema Schema) string {
	h := sha256.New()
	for _, k := range schema.Kinds {
		fmt.Fprintf(h, "kind %s\n", k.Name)
		for _, f := range k.Fields {
			if f.Side == SideEmit {
				continue
			}
			fmt.Fprintf(h, "field %s %s elem=%s slice=%t symbol=%t\n",
				f.Name, f.Type, f.Elem, f.Slice, f.IsSymbol)
		}
	}
	for _, fact := range factsOf(schema.Kinds) {
		fmt.Fprintf(h, "fact %s\n", fact)
	}
	for _, e := range schema.Enums {
		fmt.Fprintf(h, "enum %s\n", e.Type)
		for _, v := range e.Values {
			fmt.Fprintf(h, "value %s %s\n", v.Name, v.Value)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// slotsUseSymbol reports whether the values of any slot have a type of
// the symbol package. The slots file imports that package only then.
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

// viewsIdentified reports whether every kind has an identity: an ID
// field, or the zero identity of a kind without one.
//
// The predicate is "every". A traversal typed by the Declaration
// interface visits only the kinds that satisfy it, so a side
// declares the interface only when every kind does.
func viewsIdentified(views []view) bool {
	for _, v := range views {
		if v.IDStorage == "" && !v.ZeroIdentity {
			return false
		}
	}
	return len(views) > 0
}

// viewsOriginated reports whether any kind has origin storage.
//
// The predicate is "any", unlike the "every" of [viewsIdentified].
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
func newRenderer(schema Schema) (*renderer, error) {
	tmpl, err := template.New(templateDir).Funcs(template.FuncMap{
		// firstNamed returns the first kind that declares its own name.
		// The generated tests build their shared cases over it.
		"firstNamed": func(views []view) *view {
			for i := range views {
				if views[i].NameStorage != "" {
					return &views[i]
				}
			}
			return nil
		},
		// memberMethods returns the names of the methods of the
		// Membered interface. The generated tests group their member
		// cases by these names.
		"memberMethods": func() []string {
			names := make([]string, 0, len(memberMethods))
			for _, m := range memberMethods {
				names = append(names, m.Method)
			}
			return names
		},
	}).ParseFS(templates, path.Join(templateDir, templateGlob))
	if err != nil {
		return nil, fmt.Errorf("model: parse the templates: %w", err)
	}
	r := &renderer{
		templates:   tmpl,
		kinds:       schema.Kinds,
		facts:       factsOf(schema.Kinds),
		fingerprint: fingerprintOf(schema),
		views:       map[string][]view{},
	}
	for _, out := range outputs {
		if _, prepared := r.views[out.Side]; !prepared {
			r.views[out.Side] = viewsFor(schema.Kinds, out.Side)
		}
	}
	return r, nil
}

// render executes the template of one output.
func (r *renderer) render(out output) ([]byte, error) {
	views := r.views[out.Side]
	var buf bytes.Buffer
	err := r.templates.ExecuteTemplate(&buf, out.Template, data{
		Header:      genfile.Header(generatorName),
		Package:     out.Package,
		Model:       out.Model,
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
