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

// derived is the key of a declaration that a translated reference refers
// to. The key has the declaration's origin and its settled name.
type derived struct {
	origin symbol.Identity
	name   string
}

// qualify sets the package of every reference to a declaration of the
// plan that is written in a file of another package, so the target
// qualifies the reference and records its import. It qualifies two kinds
// of reference:
//
//   - A bare reference is a type reference without a target or a
//     package. The settle resolved it by the package of its unit and the
//     referent's settled name.
//   - A translated reference is a type reference whose target is a
//     declaration of another language than the plan's target. Its
//     referent is the declaration of the plan whose origin is the
//     reference's target and whose settled name is the reference's
//     spelling. The pass sets its package whatever its source recorded,
//     and clears the package where the referent is in the reference's
//     own package.
//
// The referent is in a file that the pass routes, or else in a kept file
// that the input's Others name. Where the referent's file derives no
// package, the referencing declaration is refused under
// [UnderivedPackage]. Where no file declares the referent of a translated
// reference, the referencing declaration is refused under
// [UntranslatedReference].
//
// A reference to a refused declaration is left as written, and so is a
// name in a body. A pass without Others, whose store the settle found no
// translated reference in, and where every file declares the package of
// its units, has no reference to qualify. It returns at once.
func (r *router) qualify() {
	if r.in.Others == nil && !r.relocated() && !r.in.Emit.Translates() {
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
	origins := map[derived]int{}
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
			origin, _ := emit.OriginOf(d)
			o := derived{origin: origin, name: name}
			if _, taken := origins[o]; !taken {
				origins[o] = fi
			}
		}
	}
	for i := range r.units {
		u := &r.units[i]
		for _, d := range u.Decls {
			if fi, routed := fileOf[d]; routed {
				r.qualifyDecl(d, u, fi, homes, origins)
			}
		}
	}
}

// errKeptUnderived is the reason a reference into a kept file without a
// package states: the kept name records the file's package and not the
// target's reason.
var errKeptUnderived = errors.New("layout: the file is kept from an earlier run without a package")

// qualifyDecl qualifies the bare and the translated references of one
// declaration written in the file at index fi. It refuses the
// declaration at its first translated reference whose referent no file
// declares, and at its first reference into a file whose package derives
// none.
func (r *router) qualifyDecl(
	d symbol.Symbol, u *plugin.Unit, fi int, homes map[declared]int, origins map[derived]int,
) {
	f := &r.files[fi]
	lang := symbol.Lang(r.in.Target)
	for s := range emit.All(d) {
		t, ref := s.(*emit.TypeRef)
		if !ref {
			continue
		}
		var (
			home  building
			found bool
		)
		switch {
		case lang != "" && !t.Target.IsZero() && t.Target.Lang != lang:
			home, found = r.referent(origins, t)
			if !found {
				origin, _ := emit.OriginOf(d)
				r.errorf(UntranslatedReference, r.positionOf(origin, u), nil,
					"%s references %s of %s, which the plan does not emit, and is refused",
					describe(d), t.Spelling, t.Target)
				r.refused[d] = struct{}{}
				return
			}
			t.Package = ""
		case t.Target.IsZero() && t.Package == "":
			home, found = r.home(homes, u.Pkg.Package, t.Spelling)
		}
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
// file of the pass, and else a kept file of the input's Others. It
// reports false where no file declares the name.
func (r *router) home(homes map[declared]int, pkg, name string) (building, bool) {
	if hi, generated := homes[declared{pkg: pkg, name: name}]; generated {
		return r.files[hi], true
	}
	return r.kept(pkg, name, symbol.Identity{})
}

// referent returns the file that declares the referent of a translated
// reference: a file of the pass, and else a kept file of the input's
// Others. A kept referent is a name of the target's package, or a name
// of a unit without a package, such as a per-plan unit. It reports false
// where no file declares the referent.
func (r *router) referent(origins map[derived]int, t *emit.TypeRef) (building, bool) {
	if hi, generated := origins[derived{origin: t.Target, name: t.Spelling}]; generated {
		return r.files[hi], true
	}
	if home, found := r.kept(t.Target.Package, t.Spelling, t.Target); found {
		return home, true
	}
	return r.kept("", t.Spelling, t.Target)
}

// kept returns the kept file of the input's Others that declares a
// settled name of a package, whose path and package the kept name
// records. A method is never the home of a type reference. A zero origin
// matches a name of any origin, and any other origin matches a name
// derived from it alone. It reports false for an input without Others,
// and where no kept file declares the name.
func (r *router) kept(pkg, name string, origin symbol.Identity) (building, bool) {
	if r.in.Others == nil {
		return building{}, false
	}
	kept := r.in.Others.InScope(pkg, "")
	at, _ := slices.BinarySearchFunc(kept, name, func(e plugin.NameEntry, settled string) int {
		return strings.Compare(e.Settled, settled)
	})
	for ; at < len(kept) && kept[at].Settled == name; at++ {
		e := &kept[at]
		if e.Kind == symbol.KindMethod || (!origin.IsZero() && e.Origin != origin) {
			continue
		}
		home := building{file: plugin.File{Path: e.File, Pkg: e.FilePkg}}
		if e.FilePkg.IsZero() {
			home.reason = errKeptUnderived
		}
		return home, true
	}
	return building{}, false
}

// relocated reports whether a file declares a package other than the
// package of a unit it contains. Where none does, every file of a
// unit's package declares that package, so no bare reference crosses
// into another package.
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
