// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

// openOp is the operation of the *fs.PathError that [rooted.Open] returns.
const openOp = "open"

// rooted is the file system of a plugin directory below a slash-separated
// prefix, such as the template tree templates/golang. Each Open opens the
// file through [os.OpenInRoot] with the plugin directory as the root, so a
// symbolic link that leaves the directory fails the open, and so does an
// absolute symbolic link. No handle on the directory remains open between
// two opens, so a workspace that keeps the plugin does not keep the
// directory open. A rooted has no state, and is safe for concurrent use.
// Open allocates the joined path and the file, and an error allocates its
// *fs.PathError.
type rooted struct {
	// dir is the plugin directory.
	dir string
	// prefix is the slash-separated path of a directory below dir, and the
	// empty string for dir itself.
	prefix string
}

var _ fs.FS = rooted{}

// Open opens the file name below the prefix.
//
// Error modes: Open returns a *fs.PathError with the operation open and the
// path name. It wraps [fs.ErrInvalid] for a name that [fs.ValidPath]
// refuses, and the cause of the failure for a file that does not open, so
// a missing file matches [fs.ErrNotExist].
func (r rooted) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: openOp, Path: name, Err: fs.ErrInvalid}
	}
	f, err := os.OpenInRoot(r.dir, filepath.FromSlash(path.Join(r.prefix, name)))
	if err != nil {
		if pe, is := errors.AsType[*fs.PathError](err); is {
			err = pe.Err
		}
		return nil, &fs.PathError{Op: openOp, Path: name, Err: err}
	}
	return f, nil
}
