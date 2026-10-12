// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package toolchain

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"testing/fstest"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// loadBrand is the brand under which [Surface] and [AssertDocumented]
// load generated output. A file in the frame of this brand is the output
// of the load itself, so the load leaves it out, and a fixture does not
// use this brand.
const loadBrand output.Brand = "toolchain"

// These constants are the contracts that [AssertSurface] and
// [AssertDocumented] share. A contract that starts with a colon follows
// the language of the frontend.
const (
	frontendContract = "toolchain: a frontend loads the generated output"
	loadsContract    = ": the generated output loads through its frontend"
	publicContract   = ": the generated output declares something public"
)

// declared is one public declaration at file level of a generated file,
// with the path of its file, the path of its package, and its name. The
// name is the name of the identity of the declaration, after the owner
// and a dot where the identity has an owner, such as the host of a
// method.
type declared struct {
	decl node.Declaration
	file string
	pkg  string
	name string
}

// loadOutput loads the project of g through the frontend f under ctx,
// and returns the public declarations at file level of the generated
// files of g, in the order of the paths of their packages, then of their
// files, then of the declarations in a file. The project has the files of
// the sources of g and the generated files, as [Prepare] lays them out.
// The load runs under [loadBrand] and reads no store, so a dependent
// frontend reports each import that it cannot place as a Warning.
//
// A method at file level is public where its own visibility is public
// and the type of its receiver is public too. A method whose package
// declares no type of the name of its receiver is public by its own
// visibility.
//
// Error modes: loadOutput returns the error of a source that does not
// read, the error of the load, and an error that joins one error for each
// finding of Error severity of the load, with its code, its position and
// its message.
func loadOutput(ctx context.Context, f plugin.Frontend, g Generated) ([]declared, error) {
	files, err := g.laidOut()
	if err != nil {
		return nil, err
	}
	tree := make(fstest.MapFS, len(files))
	for path, b := range files {
		tree[path] = &fstest.MapFile{Data: b}
	}
	sink := diag.NewSink()
	cfg := load.Config{FS: tree, Frontends: []plugin.Frontend{f}, Sink: sink, Brand: loadBrand}
	graph, _, err := load.Load(ctx, cfg)
	if err != nil {
		return nil, err
	}
	var errs []error
	for d := range sink.All() {
		if d.Severity == diag.SeverityError {
			errs = append(errs, fmt.Errorf("toolchain: the load reports %s at %s: %s", d.Code, d.Pos, d.Msg))
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	var out []declared
	for pkg := range graph.Packages() {
		hosts := map[string]symbol.Visibility{}
		for _, file := range pkg.Files {
			for _, s := range file.Decls {
				if d, isDecl := s.(node.Declaration); isDecl && d.Identity().Owner == "" {
					hosts[d.Identity().Name] = visibility(d)
				}
			}
		}
		for _, file := range pkg.Files {
			if _, generated := g.Files[file.Path]; !generated {
				continue
			}
			for _, s := range file.Decls {
				d, isDecl := s.(node.Declaration)
				if !isDecl || !public(d, hosts) {
					continue
				}
				id := d.Identity()
				name := id.Name
				if id.Owner != "" {
					name = id.Owner + "." + id.Name
				}
				out = append(out, declared{decl: d, file: file.Path, pkg: pkg.ID.Package, name: name})
			}
		}
	}
	slices.SortStableFunc(out, func(a, b declared) int {
		return cmp.Or(cmp.Compare(a.pkg, b.pkg), cmp.Compare(a.file, b.file))
	})
	return out, nil
}

// public reports whether a declaration at file level is public. A method
// is public where its own visibility is public and the visibility of its
// type in hosts, where hosts has the type, is public too. hosts has the
// visibility of each declaration at file level of the package, by name.
// The load gives a method at file level its file as its host and the name
// of its receiver as the owner of its identity, so the owner is the name
// of its type.
func public(d node.Declaration, hosts map[string]symbol.Visibility) bool {
	if visibility(d) != symbol.VisibilityPublic {
		return false
	}
	m, isMethod := d.(*node.Method)
	if !isMethod {
		return true
	}
	host, held := hosts[m.ID.Owner]
	return !held || host == symbol.VisibilityPublic
}

// visibility returns the visibility of a declaration of a kind that has
// one, and [symbol.VisibilityUnknown] for any other symbol.
func visibility(s symbol.Symbol) symbol.Visibility {
	switch d := s.(type) {
	case *node.Struct:
		return d.Visibility
	case *node.Interface:
		return d.Visibility
	case *node.Enum:
		return d.Visibility
	case *node.Sum:
		return d.Visibility
	case *node.Alias:
		return d.Visibility
	case *node.Function:
		return d.Visibility
	case *node.Method:
		return d.Visibility
	case *node.Field:
		return d.Visibility
	case *node.Constant:
		return d.Visibility
	case *node.Variable:
		return d.Visibility
	default:
		return symbol.VisibilityUnknown
	}
}

// spelling returns the spelling of a type reference, and the empty string
// for a reference that the frontend did not record.
func spelling(t *node.TypeRef) string {
	if t == nil {
		return ""
	}
	return t.Spelling
}
