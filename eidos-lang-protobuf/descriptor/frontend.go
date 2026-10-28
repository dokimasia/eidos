// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package descriptor

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/lang/protobuf/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	sdkfrontend "go.dokimi.dev/eidos/sdk/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Store is the name of the store that contains the files of a request that
// the request does not generate, at their proto paths. The dependency round
// of the frontend loads every file of the store.
const Store = "protoc"

// Registry is the workspace path of the FileDescriptorSet of every file of
// a request. The frontend reads the custom options of a declaration through
// it, because an extension of an options message is declared in another
// file. Every unit declares it as a shared input.
const Registry = ".protoc/descriptors.binpb"

// MissingSourceInfo reports a file to generate whose descriptor has no
// source code info, at the start of the file.
//
// Severity is Error. Without source code info the file has no comments, so
// its directives would not load and no finding would report their loss. buf
// includes source code info unless --exclude-source-info is set. The file
// still loads every declaration.
var MissingSourceInfo = diag.MustRegister(protobuf.CodePrefix, diag.CodeSpec{
	Number:  7,
	Meaning: "a file to generate has no source code info",
})

// New builds the descriptor frontend through the kit. The frontend claims
// every file whose name ends in [protobuf.Extension]. Each such file
// contains one serialized FileDescriptorProto. It has the name, the
// language, the syntax, the keys, the resolution and the importer role of
// the source frontend of the satellite, so a composition registers one of
// the two. Its dependency round loads every file of [Store].
//
// # Allocation contract
//
// New allocates what the kit allocates for the frontend.
func New() plugin.Frontend {
	return sdkfrontend.New(protobuf.Name, frontend.Lang, protobuf.Syntax()).
		Version(protobuf.Version).
		Match("**/*" + protobuf.Extension).
		Units(partition).
		Parse(parse).
		Resolve(frontend.Resolve).
		Imports(frontend.ImportPath).
		Dependencies(dependencies).
		Keys(protobuf.Keys).
		Build()
}

// partition returns one unit per file in path order, each with [Registry]
// as its shared input. It returns no error.
func partition(_ context.Context, files []plugin.SourceRef, _ plugin.FileReader) ([][]plugin.SourceRef, error) {
	ordered := make([]plugin.SourceRef, len(files))
	for i, f := range files {
		ordered[i] = plugin.SourceRef{Path: f.Path, Shared: []string{Registry}}
	}
	slices.SortFunc(ordered, func(a, b plugin.SourceRef) int { return strings.Compare(a.Path, b.Path) })
	units := make([][]plugin.SourceRef, len(ordered))
	for i := range ordered {
		units[i] = ordered[i : i+1 : i+1]
	}
	return units, nil
}

// dependencies returns one unit for each file of [Store] in the first
// round, in path order, and nothing in a later round. A load without the
// store has no dependency.
//
// Error modes: the error of a listing of the store for a cause other than
// an absent store.
func dependencies(
	_ context.Context,
	round *plugin.DependencyRound,
	r plugin.StoreReader,
) ([][]plugin.SourceRef, error) {
	if round.Number > 1 {
		return nil, nil
	}
	var units [][]plugin.SourceRef
	err := walk(r, "", func(file string) {
		units = append(units, []plugin.SourceRef{{Path: plugin.StorePath(Store, file), Shared: []string{Registry}}})
	})
	if errors.Is(err, plugin.ErrStoreAbsent) {
		return nil, nil
	}
	return units, err
}

// walk calls visit with the path of each proto file under dir in the
// store, in path order.
func walk(r plugin.StoreReader, dir string, visit func(file string)) error {
	entries, err := r.ReadDir(plugin.StorePath(Store, dir))
	if err != nil {
		return err
	}
	for _, e := range entries {
		at := path.Join(dir, e.Name())
		switch {
		case e.IsDir():
			if err := walk(r, at, visit); err != nil {
				return err
			}
		case e.Type().IsRegular() && strings.HasSuffix(at, protobuf.Extension):
			visit(at)
		default:
			// Another entry of the store is no proto file.
		}
	}
	return nil
}

// registryFiles reads the descriptors of [Registry] through the unit, and
// returns no file for a load whose tree does not contain it.
//
// Error modes: the error of a read for a cause other than an absent file.
func registryFiles(u *plugin.SourceUnit) ([]byte, error) {
	b, err := u.Read(Registry)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return b, err
}
