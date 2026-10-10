// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"go.dokimi.dev/eidos/core/internal/pathset"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
)

// collide refuses what one tree cannot contain: a file whose units
// derive from two packages, and two files whose paths clash under the
// rule every output sink stages by. Each collision is one
// [PathCollision] at the second file's first declaration, with the
// first file's as related, and the declarations of both files are
// refused. The files are sorted by path, so the first is the earlier
// path.
func (r *router) collide() {
	var tree pathset.Set
	for fi := range r.files {
		f := &r.files[fi]
		r.mixed(f)
		clash, other := tree.Clash(f.file.Path)
		if clash == pathset.ClashNone {
			tree.Add(f.file.Path)
			continue
		}
		first := &r.files[r.byPath[other]]
		related := []position.Pos{r.fileAt(first)}
		switch clash {
		case pathset.ClashCase:
			r.errorf(PathCollision, r.fileAt(f), related,
				"%s and %s differ only in case, and a case-insensitive filesystem stores them as one file",
				first.file.Path, f.file.Path)
		case pathset.ClashDirectory:
			r.errorf(PathCollision, r.fileAt(f), related,
				"%s is routed as a file, and %s needs it as a directory", f.file.Path, first.file.Path)
		default:
			r.errorf(PathCollision, r.fileAt(f), related,
				"%s needs %s as a directory, and %s is routed as a file",
				f.file.Path, first.file.Path, first.file.Path)
		}
		r.refuse(first)
		r.refuse(f)
	}
}

// mixed refuses a file whose units derive from two packages, because a
// file declares one package. A unit without a package, such as a plan
// unit, derives from none and mixes with any.
func (r *router) mixed(f *building) {
	var first *plugin.Unit
	for i := range f.file.Units {
		u := &f.file.Units[i]
		switch {
		case u.Pkg.IsZero():
		case first == nil:
			first = u
		case u.Pkg != first.Pkg:
			r.errorf(PathCollision, r.firstAt(u), []position.Pos{r.firstAt(first)},
				"declarations of %s and of %s route to %s, and a file declares one package",
				first.Pkg, u.Pkg, f.file.Path)
			r.refuse(f)
			return
		}
	}
}

// fileAt positions a finding about a file at its first declaration's
// origin.
func (r *router) fileAt(f *building) position.Pos {
	return r.firstAt(&f.file.Units[0])
}
