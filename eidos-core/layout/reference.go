// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"errors"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// declared keys a declaration the way the settle resolves a bare
// reference to it: the package path of the unit it was emitted in, and
// its settled name.
type declared struct {
	pkg  string
	name string
}

// qualify sets the package of every bare reference to a declaration of
// the plan that is written in a file of another package, so the target
// qualifies the reference and records its import. A bare reference is
// a type reference with no target and no package, and the settle
// resolved it by the package of its unit and the referent's settled
// name. The referent is in a file that the pass routes, or else in a
// kept file that the input's Others name. Where the referent's file
// derives no package, the referencing declaration is refused under
// [UnderivedPackage].
//
// A reference to a refused declaration is left as written, and so is a
// name in a body. Without Others, a pass where every file declares the
// package of its units has no reference to qualify, and returns at once.
func (r *router) qualify() {
	if r.in.Others == nil && !r.relocated() {
		return
	}
	fileOf := map[symbol.Symbol]int{}
	for fi := range r.files {
		for _, u := range r.files[fi].file.Units {
			for _, d := range u.Decls {
				if !r.isRefused(d) {
					fileOf[d] = fi
				}
			}
		}
	}
	homes := map[declared]int{}
	for i := range r.units {
		u := &r.units[i]
		for _, d := range u.Decls {
			fi, routed := fileOf[d]
			name := emit.DeclaredName(d)
			if !routed || name == "" {
				continue
			}
			k := declared{pkg: u.Pkg.Package, name: name}
			if _, taken := homes[k]; !taken {
				homes[k] = fi
			}
		}
	}
	for i := range r.units {
		u := &r.units[i]
		for _, d := range u.Decls {
			if fi, routed := fileOf[d]; routed {
				r.qualifyDecl(d, u, fi, homes)
			}
		}
	}
}

// errKeptUnderived is the reason a reference into a kept file without a
// package states: the kept name records the file's package and not the
// target's reason.
var errKeptUnderived = errors.New("layout: the file is kept from an earlier run without a package")

// qualifyDecl qualifies the bare references of one declaration written
// in the file at index fi, or refuses the declaration at its first
// reference into a file whose package derives none.
func (r *router) qualifyDecl(d symbol.Symbol, u *plugin.Unit, fi int, homes map[declared]int) {
	f := &r.files[fi]
	for s := range emit.All(d) {
		t, ref := s.(*emit.TypeRef)
		if !ref || !t.Target.IsZero() || t.Package != "" {
			continue
		}
		home, found := r.home(homes, u.Pkg.Package, t.Spelling)
		if !found || home.file.Path == f.file.Path {
			continue
		}
		switch {
		case home.file.Pkg.IsZero():
			origin, _ := emit.OriginOf(d)
			r.errorf(UnderivedPackage, r.positionOf(origin, u), nil,
				"%s references %s in %s, a file the target derives no package for, and is refused: %v",
				describe(d), t.Spelling, home.file.Path, home.reason)
			r.refused[d] = struct{}{}
			return
		case home.file.Pkg.Package != f.file.Pkg.Package:
			t.Package = home.file.Pkg.Package
		}
	}
}

// home returns the file that declares a settled name of a package: a
// file of the pass, and else a kept file of the input's Others, whose
// path and package the kept name records. A method is never the home of
// a bare reference. It reports false where no file declares the name.
func (r *router) home(homes map[declared]int, pkg, name string) (building, bool) {
	if hi, generated := homes[declared{pkg: pkg, name: name}]; generated {
		return r.files[hi], true
	}
	if r.in.Others == nil {
		return building{}, false
	}
	kept := r.in.Others.InScope(pkg, "")
	at, _ := slices.BinarySearchFunc(kept, name, func(e plugin.NameEntry, settled string) int {
		return strings.Compare(e.Settled, settled)
	})
	for ; at < len(kept) && kept[at].Settled == name; at++ {
		if e := &kept[at]; e.Kind != symbol.KindMethod {
			home := building{file: plugin.File{Path: e.File, Pkg: e.FilePkg}}
			if e.FilePkg.IsZero() {
				home.reason = errKeptUnderived
			}
			return home, true
		}
	}
	return building{}, false
}

// relocated reports whether a file declares a package other than the
// package of a unit it contains. Where none does, every file of a
// unit's package declares that package, so no reference crosses into
// another package.
func (r *router) relocated() bool {
	for fi := range r.files {
		f := &r.files[fi]
		for _, u := range f.file.Units {
			if u.Pkg.Package != f.file.Pkg.Package {
				return true
			}
		}
	}
	return false
}
