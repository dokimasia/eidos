// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"errors"
	"io/fs"
	"path"
	"strings"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/frontend/library"
)

// openOp is the operation that the error of an open of the library store
// names.
const openOp = "open"

// Stores returns the stores that the load passes to the TypeScript frontend:
// the embedded library of [library.Release] under [typescript.LibraryStore].
// The store serves each file of the library of a project at <key>/<file>,
// as lib.es2024.d.ts/lib.es5.d.ts, so the files of two libraries are the
// members of two units. Stores reads no environment.
//
// Error modes: Stores returns no error.
func Stores(func(key string) string) (map[string]fs.FS, error) {
	return map[string]fs.FS{typescript.LibraryStore: libraryStore{}}, nil
}

// libraryStore is the tree of the library store: below each key, every file
// of the embedded library. Any number of goroutines may read it, and an open
// allocates what embed.FS allocates.
type libraryStore struct{}

// Open opens the file at <key>/<file> as the embedded file of its name, and
// the root and a key as the directory of the embedded files.
//
// Error modes: Open returns an [fs.PathError] with name as its path. It wraps
// fs.ErrInvalid for a path that fs.ValidPath refuses, and fs.ErrNotExist for
// a path below a file and for a file that the release does not have.
func (libraryStore) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: openOp, Path: name, Err: fs.ErrInvalid}
	}
	embedded := library.Dir
	if _, file, inside := strings.Cut(name, libraryStoreSep); inside {
		if strings.Contains(file, libraryStoreSep) {
			return nil, &fs.PathError{Op: openOp, Path: name, Err: fs.ErrNotExist}
		}
		embedded = path.Join(library.Dir, file)
	}
	f, err := library.Files.Open(embedded)
	if err != nil {
		if pe, is := errors.AsType[*fs.PathError](err); is {
			err = pe.Err
		}
		return nil, &fs.PathError{Op: openOp, Path: name, Err: err}
	}
	return f, nil
}
