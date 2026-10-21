// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"bytes"
	"errors"
	"io/fs"
	"path"
	"strings"
	"time"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/frontend/library"
)

// openOp is the operation that the error of an open of the library store
// names.
const openOp = "open"

// fragmentMode is the mode of a fragment of the library store: a regular
// file that every user can read, as an embedded file is.
const fragmentMode fs.FileMode = 0o444

// libraryStore is the tree of the library store: below each key, every file
// of the embedded library and the fragments of its global names. Any
// number of goroutines may read it. An open of a file allocates what
// embed.FS allocates, and an open of a fragment allocates its statements.
type libraryStore struct{}

// Open opens the file at <key>/<file> as the embedded file of its name, the
// fragment at <key>/<file>#<name> as the statements of the name in that
// file, and the root and a key as the directory of the embedded files. A
// fragment has the statements of [library.Index] in the order of the file,
// each followed by a line break.
//
// Error modes: Open returns an [fs.PathError] with name as its path. It wraps
// fs.ErrInvalid for a path that fs.ValidPath refuses, and fs.ErrNotExist for
// a path below a file, for a file that the release does not have and for a
// fragment of a name that the file does not declare. It wraps the cause of
// the embed for a fragment of a path that is no file of the embed, such as
// the directory of the library.
func (libraryStore) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: openOp, Path: name, Err: fs.ErrInvalid}
	}
	embedded := library.Dir
	if _, file, inside := strings.Cut(name, libraryStoreSep); inside {
		if strings.Contains(file, libraryStoreSep) {
			return nil, &fs.PathError{Op: openOp, Path: name, Err: fs.ErrNotExist}
		}
		if base, decl, isFragment := strings.Cut(file, fragmentSep); isFragment {
			return openFragment(name, base, decl)
		}
		embedded = path.Join(library.Dir, file)
	}
	f, err := library.Files.Open(embedded)
	if err != nil {
		return nil, openError(name, err)
	}
	return f, nil
}

// fragment is the open file of the statements of one global name of a file
// of the library. It is its own [fs.FileInfo]: a regular file with the size
// of the statements, the mode of an embedded file and no modification time,
// as an embedded file has. Read, Seek and ReadAt read the statements.
type fragment struct {
	*bytes.Reader
	name string
}

var (
	_ fs.File     = (*fragment)(nil)
	_ fs.FileInfo = (*fragment)(nil)
)

// Stat returns the fragment as its FileInfo.
func (f *fragment) Stat() (fs.FileInfo, error) { return f, nil }

// Close closes nothing, because the statements are in memory.
func (*fragment) Close() error { return nil }

// Name returns the base name of the path of the fragment, as
// lib.es5.d.ts#Error.
func (f *fragment) Name() string { return f.name }

// Mode returns the mode of a regular file that every user can read.
func (*fragment) Mode() fs.FileMode { return fragmentMode }

// ModTime returns the zero time, as the stat of an embedded file does.
func (*fragment) ModTime() time.Time { return time.Time{} }

// IsDir reports false, because a fragment is a file.
func (*fragment) IsDir() bool { return false }

// Sys returns nil, because no file system backs a fragment.
func (*fragment) Sys() any { return nil }

// Stores returns the stores that the load passes to the TypeScript frontend:
// the embedded library of [library.Release] under [typescript.LibraryStore].
// The store serves each file of the library of a project at <key>/<file>,
// as lib.es2024.d.ts/lib.es5.d.ts, and the statements of one global name of
// such a file at <key>/<file>#<name>, as lib.es2024.d.ts/lib.es5.d.ts#Error.
// Each library has a key of its own, so the units of two libraries have
// different members. Stores does not read the environment.
//
// Error modes: Stores returns no error.
func Stores(func(key string) string) (map[string]fs.FS, error) {
	return map[string]fs.FS{typescript.LibraryStore: libraryStore{}}, nil
}

// openFragment opens the fragment at name: the statements of the global
// name decl in the embedded file of the library, in the order of the file,
// each followed by a line break.
//
// Error modes: openFragment returns the error of [openError] for a file
// that does not read from the embed, and an [fs.PathError] with name as its
// path that wraps fs.ErrNotExist for a name that the file does not declare.
func openFragment(name, file, decl string) (fs.File, error) {
	src, err := sourceOf(file)
	if err != nil {
		return nil, openError(name, err)
	}
	statements := declarations()[decl][file]
	if len(statements) == 0 {
		return nil, &fs.PathError{Op: openOp, Path: name, Err: fs.ErrNotExist}
	}
	size := len(statements)
	for _, s := range statements {
		//dokimi:mutate-skip aor: a larger size still sizes one buffer, so the open allocates as often
		size += s.End - s.Start
	}
	b := make([]byte, 0, size)
	for _, s := range statements {
		b = append(append(b, src[s.Start:s.End]...), '\n')
	}
	return &fragment{Reader: bytes.NewReader(b), name: path.Base(name)}, nil
}

// openError returns the error of an open of the library store at name for
// the error err of the embed: an [fs.PathError] with name as its path, which
// wraps the cause of err.
func openError(name string, err error) error {
	if pe, is := errors.AsType[*fs.PathError](err); is {
		err = pe.Err
	}
	return &fs.PathError{Op: openOp, Path: name, Err: err}
}
