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
	"go.dokimi.dev/eidos/sdk/plugin"
)

// dependencies is the dependency round of the TypeScript frontend. The first
// round places one unit for each library: each key among the shared inputs
// of the round, which the partition declares for the files of a project, and
// each key among the needs of any round, which the directives of a source
// file extend. A unit is every file of its library, as [libraryUnit] lists
// them. A need that is a module specifier gets no unit and no report: a
// relative or mapped specifier refers to a file of the workspace, and a
// package specifier to a package of node_modules, which the load does not
// read. A load without the library store places no library.
//
// Error modes: dependencies returns the error of a file of the library that
// does not read for another cause than an absent store.
func dependencies(
	_ context.Context, round *plugin.DependencyRound, r plugin.StoreReader,
) ([][]plugin.SourceRef, error) {
	var keys []string
	if round.Number == 1 {
		for _, s := range round.Shared {
			if key, _, isLibrary := libraryPath(s); isLibrary {
				keys = append(keys, key)
			}
		}
	}
	for _, n := range round.Needs {
		if key, file, isLibrary := libraryPath(n.Path); isLibrary && file == "" {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	keys = slices.Compact(keys)
	out := make([][]plugin.SourceRef, 0, len(keys))
	for _, key := range keys {
		members, err := libraryUnit(r, key)
		switch {
		case errors.Is(err, plugin.ErrStoreAbsent):
		case err != nil:
			return nil, err
		default:
			out = append(out, members)
		}
	}
	return out, nil
}

// libraryUnit returns the members of the unit of the library of a key: the
// store path of every file of the library, in the order of [sortLibrary].
// The files of a library are its root files, which the key lists, and each
// file that a lib directive at the head of one of its files refers to,
// followed to the end. It reads every file through r, so each read folds
// into the key of the unit. A directive of a library file with a name that
// TypeScript 7.0.2 does not know adds nothing.
//
// Error modes: libraryUnit returns an error that wraps
// [plugin.ErrStoreAbsent] for a load without the store, and the error of a
// file that does not read, naming the key.
func libraryUnit(r plugin.StoreReader, key string) ([]plugin.SourceRef, error) {
	files := strings.Split(key, typescript.LibraryKeySep)
	seen := make(map[string]bool, len(files))
	for _, file := range files {
		seen[file] = true
	}
	for i := 0; i < len(files); i++ {
		member := plugin.StorePath(typescript.LibraryStore, key+libraryStoreSep+files[i])
		src, err := r.Read(member)
		if err != nil {
			return nil, fmt.Errorf("frontend: library %s: %w", key, err)
		}
		for _, ref := range libReferences(member, src) {
			if file, known := fileOfLibrary(ref.name); known && !seen[file] {
				seen[file] = true
				files = append(files, file)
			}
		}
	}
	sortLibrary(files)
	out := make([]plugin.SourceRef, 0, len(files))
	for _, file := range files {
		out = append(out, plugin.SourceRef{Path: plugin.StorePath(typescript.LibraryStore, key+libraryStoreSep+file)})
	}
	return out, nil
}
