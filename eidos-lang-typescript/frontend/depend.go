// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/frontend/library"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// dependencies is the dependency round of the TypeScript frontend. It places
// one unit for each use of a global name of a library, with the members that
// [unitOf] lists, and it places the unit of two names of one group once. A
// use of a package that is not a library places nothing, and neither does a
// use of a namespace inside a library or of a name that the library does not
// declare. A need places nothing. A relative or a mapped specifier refers to
// a file of the workspace, and a package specifier refers to a package below
// node_modules. The load does not read node_modules. A load without the
// library store places nothing.
//
// Error modes: dependencies returns the error of [libraryOf] for a library
// that does not read.
func dependencies(
	_ context.Context, round *plugin.DependencyRound, r plugin.StoreReader,
) ([][]plugin.SourceRef, error) {
	var out [][]plugin.SourceRef
	libraries := map[string][]string{}
	placed := map[string]bool{}
	for _, use := range round.Uses {
		key, inner, isLibrary := libraryPath(use.Package)
		if !isLibrary || inner != "" {
			continue
		}
		files, listed := libraries[key]
		if !listed {
			var err error
			files, err = libraryOf(r, key)
			switch {
			case errors.Is(err, plugin.ErrStoreAbsent):
			case err != nil:
				return nil, err
			}
			libraries[key] = files
		}
		unit := unitOf(key, files, use.Name)
		if len(unit) == 0 || placed[unit[0].Path] {
			continue
		}
		placed[unit[0].Path] = true
		out = append(out, unit)
	}
	return out, nil
}

// libraryOf returns the files of the library of a key, as [keyFiles] lists
// them. It reads the first root of the key through r. The read folds into
// the record of the round, and a load without the store has no library.
//
// Error modes: libraryOf returns the error of the read of the root, which
// wraps [plugin.ErrStoreAbsent] for a load without the store, and the error
// of [keyFiles]. The text of each error contains the key.
func libraryOf(r plugin.StoreReader, key string) ([]string, error) {
	root, _, _ := strings.Cut(key, typescript.LibraryKeySep)
	if _, err := r.Read(plugin.StorePath(typescript.LibraryStore, key+libraryStoreSep+root)); err != nil {
		return nil, fmt.Errorf("frontend: library %s: %w", key, err)
	}
	files, err := keyFiles(key)
	if err != nil {
		return nil, fmt.Errorf("frontend: library %s: %w", key, err)
	}
	return files, nil
}

// unitOf returns the members of the unit of a global name in the library of
// a key, in the order of files, the files of the library. A script file has
// a fragment for each name of the group of the name that it declares, in the
// order of the names. A module file that declares a name of the group is a
// member whole. The group of a name is the name and each global name of
// every module file of the library that declares a name of the group. The
// global declarations of a module file refer to the declarations of its
// module scope, so the global names of the file share one unit. unitOf
// returns no member where the library does not declare the name.
func unitOf(key string, files []string, name string) []plugin.SourceRef {
	decls := declarations()
	group := []string{name}
	for i := 0; i < len(group); i++ {
		for file := range decls[group[i]] {
			if !library.Index[file].Module || !slices.Contains(files, file) {
				continue
			}
			for _, s := range library.Index[file].Statements {
				if !slices.Contains(group, s.Name) {
					group = append(group, s.Name)
				}
			}
		}
	}
	slices.Sort(group)
	var out []plugin.SourceRef
	for _, file := range files {
		if library.Index[file].Module {
			if slices.ContainsFunc(group, func(n string) bool { return len(decls[n][file]) > 0 }) {
				out = append(out, plugin.SourceRef{
					Path: plugin.StorePath(typescript.LibraryStore, key+libraryStoreSep+file),
				})
			}
			continue
		}
		for _, n := range group {
			if len(decls[n][file]) > 0 {
				out = append(out, plugin.SourceRef{
					Path: plugin.StorePath(typescript.LibraryStore, key+libraryStoreSep+file+fragmentSep+n),
				})
			}
		}
	}
	return out
}
