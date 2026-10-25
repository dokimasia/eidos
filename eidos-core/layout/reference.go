// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"errors"
	"fmt"
	"path"
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

// derived is the key by which a translated or an emitted reference finds
// its referent. The key has the origin of the declaration and its settled
// name.
type derived struct {
	origin symbol.Identity
	name   string
}

// routedAt is where the pass routed a declaration: the index of its file,
// and whether its unit is a record of a family that claims its files.
type routedAt struct {
	file   int
	record bool
}

// qualify sets the package of every reference to a declaration of the
// plan that is written in a file of another package, so the target
// qualifies the reference and records its import. The file of the
// reference lists the package of the file of the referent among its
// [plugin.File.Referents]. It qualifies three kinds of reference:
//
//   - A bare reference is a type reference without a target or a
//     package. The settle resolved it by the package of its unit and the
//     referent's settled name. The pass sets Emitted on a bare reference
//     that it qualifies into another package.
//   - A translated reference is a type reference whose target is a
//     declaration of another language than the plan's target. Its
//     referent is the declaration of the plan whose origin is the
//     reference's target and whose settled name is the reference's
//     spelling. The pass sets its package whatever its source recorded,
//     and clears the package where the referent is in the reference's
//     own package.
//   - An emitted reference is a type reference with a target and with
//     Emitted set. Its target can be in any language. The pass finds its
//     referent and sets its package as it does for a translated
//     reference.
//
// The referent is in a file that the pass routes, or else in a kept file
// that the input's Others name. The referent of a translated or an
// emitted reference can be a record of a family that claims its files,
// and the referent of a bare reference cannot. Where the referent's file
// derives no package, the referencing declaration is refused under
// [UnderivedPackage]. Where no file declares the referent of a translated
// reference, the referencing declaration is refused under
// [UntranslatedReference]. Where no file declares the referent of an
// emitted reference, it is refused under [UnemittedReference].
//
// A reference to a refused declaration is left as written, and so is a
// name in a body. The pass returns at once where the input has no Others,
// the settle did not find a translated or an emitted reference in the
// store, and every file declares the package of its units. Such a pass
// has no reference to qualify.
func (r *router) qualify() {
	if r.in.Others == nil && !r.relocated() && !r.in.Emit.Translates() {
		return
	}
	fileOf := map[symbol.Symbol]routedAt{}
	for fi := range r.files {
		for _, u := range r.files[fi].file.Units {
			for _, d := range u.Decls {
				if !r.isRefused(d) {
					fileOf[d] = routedAt{file: fi, record: u.Template != ""}
				}
			}
		}
	}
	homes := map[declared]int{}
	origins := map[derived]int{}
	for i := range r.units {
		u := &r.units[i]
		for _, d := range u.Decls {
			at, routed := fileOf[d]
			name := emit.DeclaredName(d)
			if !routed || name == "" {
				continue
			}
			k := declared{pkg: u.Pkg.Package, name: name}
			if _, taken := homes[k]; !taken && !at.record {
				homes[k] = at.file
			}
			origin, _ := emit.OriginOf(d)
			o := derived{origin: origin, name: name}
			if _, taken := origins[o]; !taken {
				origins[o] = at.file
			}
		}
	}
	for i := range r.units {
		u := &r.units[i]
		for _, d := range u.Decls {
			if at, routed := fileOf[d]; routed {
				r.qualifyDecl(d, u, at.file, homes, origins)
			}
		}
	}
}

// errKeptUnderived is the reason a reference into a kept file without a
// package states: the kept name records the file's package and not the
// target's reason.
var errKeptUnderived = errors.New("layout: the file is kept from an earlier run without a package")

// errNoBinder is the reason of the refusal of a bound reference in a plan
// whose backend has no binder.
var errNoBinder = errors.New("layout: the backend of the plan has no binder")

// qualifyDecl qualifies the bare, the translated and the emitted
// references of one declaration written in the file at index fi. It
// refuses the declaration at its first translated or emitted reference
// whose referent no file declares. It also refuses the declaration at its
// first reference into a file whose package derives none.
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
		case t.Bound && lang != "" && !t.Target.IsZero() && t.Target.Lang != lang:
			if !r.qualifyBound(d, u, f, t) {
				return
			}
			continue
		case t.Emitted && !t.Target.IsZero():
			home, found = r.referent(origins, t)
			if !found {
				origin, _ := emit.OriginOf(d)
				r.errorf(UnemittedReference, r.positionOf(origin, u), nil,
					"%s is refused because the plan does not emit %s of %s", Describe(d), t.Spelling, t.Target)
				r.refused[d] = struct{}{}
				return
			}
			t.Package = ""
		case lang != "" && !t.Target.IsZero() && t.Target.Lang != lang:
			home, found = r.referent(origins, t)
			if !found {
				origin, _ := emit.OriginOf(d)
				r.errorf(UntranslatedReference, r.positionOf(origin, u), nil,
					"%s references %s of %s, which the plan does not emit, and is refused",
					Describe(d), t.Spelling, t.Target)
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
				Describe(d), t.Spelling, home.file.Path, home.reason)
			r.refused[d] = struct{}{}
			return
		case home.file.Pkg.Package != f.file.Pkg.Package:
			t.Package = home.file.Pkg.Package
			if t.Target.IsZero() {
				t.Emitted = true
			}
			f.refer(home.file.Pkg)
		}
	}
}

// qualifyBound qualifies the bound reference t of the declaration d, which
// the file f writes. It sets the package of t to the package that
// [router.boundHome] returns where that package is not the package of f,
// records the package among the referents of f, and reports true. It
// refuses d and reports false where the plan has no binder, under
// [UntranslatedReference], and where the binder returns no package, under
// [UnderivedPackage].
func (r *router) qualifyBound(d symbol.Symbol, u *plugin.Unit, f *building, t *emit.TypeRef) bool {
	pkg, err := r.boundHome(t.Target)
	if err != nil {
		code := UnderivedPackage
		if r.in.Binder == nil {
			code = UntranslatedReference
		}
		origin, _ := emit.OriginOf(d)
		r.errorf(code, r.positionOf(origin, u), nil,
			"%s references %s of %s, the code of a binding, and is refused: %v", Describe(d), t.Spelling, t.Target, err)
		r.refused[d] = struct{}{}
		return false
	}
	t.Package = ""
	if pkg.Package != f.file.Pkg.Package {
		t.Package = pkg.Package
		f.refer(pkg)
	}
	return true
}

// boundHome returns the package of the code that the code generator of a
// binding writes for target. It places the source file of target where the
// default placement of the plan writes the code of the file: beside it, or
// below the output directory of the plan where the plan centralises its
// files. The placement has the residents and the modules of the directory
// and the import base of the plan, as the placement of a routed file has.
// It returns the package that the binder of the input returns for the
// placement.
//
// Error modes: an error for an input without a binder, an error for a
// target without a workspace source, and the error of the binder.
func (r *router) boundHome(target symbol.Identity) (symbol.Identity, error) {
	if r.in.Binder == nil {
		return symbol.Identity{}, errNoBinder
	}
	file, found := r.sourceFile(target)
	if !found {
		return symbol.Identity{}, fmt.Errorf("layout: %s has no workspace source beside which a binding writes", target)
	}
	dir := path.Dir(file)
	if r.in.Config.Policy == PolicyCentralised {
		dir = path.Join(r.in.Config.Dir, dir)
	}
	at := plugin.Placement{
		Path:    path.Join(dir, path.Base(file)),
		Origin:  target.PackageIdentity(),
		Modules: r.in.Modules,
	}
	if r.in.Residents != nil {
		at.Residents = r.in.Residents(dir)
	}
	if base := r.in.Config.ImportBase; base != "" {
		at.ImportBase, at.BaseDir = base, r.in.Config.Dir
	}
	return r.in.Binder.Home(target, at)
}

// refer records the package of a file that a reference of b is qualified
// into among the referents of b, once and in the order of the package
// paths. It allocates the list of referents as it grows.
func (b *building) refer(pkg symbol.Identity) {
	at, held := slices.BinarySearchFunc(b.file.Referents, pkg.Package, func(id symbol.Identity, p string) int {
		return strings.Compare(id.Package, p)
	})
	if !held {
		b.file.Referents = slices.Insert(b.file.Referents, at, pkg)
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

// referent returns the file that declares the referent of a translated or
// an emitted reference: a file of the pass, and else a kept file of the
// input's Others. A kept referent is a name of the target's package, or a name
// of a unit without a package, such as a per-plan unit. A referent can be a
// record. It reports false where no file declares the referent.
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
// matches a name of any origin other than a record, because a bare
// reference does not resolve to a record. Any other origin matches a name
// derived from it alone, a record included. It reports false for an input
// without Others, and where no kept file declares the name.
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
		switch {
		case e.Kind == symbol.KindMethod:
		case origin.IsZero() && e.Record:
		case !origin.IsZero() && e.Origin != origin:
		default:
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

// importable refuses each routed declaration with a reference that the file
// of the declaration cannot refer to, as the import rule of the input
// decides. It runs after [router.qualify], so the rule reads the package that
// a reference has in its file. It checks no declaration that a finding
// refused, and no file whose package derives none. An input without an
// import rule checks nothing. importable allocates nothing beyond what the
// rule allocates and what a refusal reports.
func (r *router) importable() {
	if r.in.Imports == nil {
		return
	}
	for fi := range r.files {
		f := &r.files[fi]
		if f.reason != nil {
			continue
		}
		for ui := range f.file.Units {
			u := &f.file.Units[ui]
			for _, d := range u.Decls {
				if !r.isRefused(d) {
					r.importableDecl(d, u, f)
				}
			}
		}
	}
}

// importableDecl refuses the declaration d of the unit u under
// [UnimportableReference] at the first of its references that the file f
// cannot refer to, and reports the reason of the import rule. It reports
// once for a declaration.
func (r *router) importableDecl(d symbol.Symbol, u *plugin.Unit, f *building) {
	for s := range emit.All(d) {
		t, ref := s.(*emit.TypeRef)
		if !ref {
			continue
		}
		if err := r.in.Imports.Importable(f.file.Pkg, t); err != nil {
			origin, _ := emit.OriginOf(d)
			r.errorf(UnimportableReference, r.positionOf(origin, u), nil,
				"%s references %s, which the file %s of the package %s cannot import, and is refused: %v",
				Describe(d), t.Spelling, f.file.Path, f.file.Pkg.Package, err)
			r.refused[d] = struct{}{}
			return
		}
	}
}
