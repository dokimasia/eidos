// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"slices"
	"strings"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// qualifierMark separates the qualifier of a qualified name from the name.
const qualifierMark = "."

// fileUses is a parsed file of a package, with what the uses of its
// directives read once every file of the package has parsed: the bindings
// of the imports of the file, the directives that the lowering of the file
// attached, and the packages that the file uses. used is nil at full depth,
// where no import is pruned.
type fileUses struct {
	file     *node.File
	bindings golang.Bindings
	attached []plugin.Attachment
	used     map[string]bool
}

// recordUses records a use of the unit for each declaration outside the
// package of a lowered file that a reference of the file can resolve to.
// A reference is a named type of a declaration of the file, or a scalar or
// an element of a list that an argument of a directive on the file states,
// such as the type that a witness names. The uses of a reference are its
// candidates in the bindings of the file outside the package of the file:
// the declaration that its qualifier binds, the name in every unaliased
// import for a qualifier that no import binds, and the name in every dot
// import for an exported bare name. attached contains the directives that
// the lowering of the file attached. recordUses adds the package of each
// use to used where used is not nil.
//
// A function body and a value are no reference, so a package that only a
// body or a value names is no use. The dependency round places a package
// for each use, so such a package does not load, and the rules give a
// reference into it a stand-in.
//
// # Allocation contract
//
// recordUses allocates the uses that [plugin.SourceUnit.Use] records and
// the entries of used. It allocates no list of candidates for a reference
// whose qualifier an import binds, and none for a bare name of a file
// without a dot import, whose one candidate is in the package of the file.
func recordUses(
	u *plugin.SourceUnit, l *lowered, pkgPath string, file *node.File, attached []plugin.Attachment,
	used map[string]bool,
) {
	use := func(id symbol.Identity) {
		if id.Package == pkgPath {
			return
		}
		u.Use(id)
		if used != nil {
			used[id.Package] = true
		}
	}
	candidates := func(spelling string) {
		for _, id := range l.bindings.Candidates(pkgPath, spelling) {
			use(id)
		}
	}
	dotted := slices.ContainsFunc(file.Imports, func(imp *node.Import) bool {
		return imp.Wildcard || imp.Alias == golang.DotAlias
	})
	for _, decl := range file.Decls {
		node.Walk(decl, func(s symbol.Symbol) bool {
			ref, is := s.(*node.TypeRef)
			switch {
			case !is || ref.Form != symbol.FormNamed:
			case ref.Package != "":
				use(symbol.Identity{Lang: Lang, Package: ref.Package, Name: golang.Unqualified(ref.Spelling)})
			case dotted || strings.Contains(ref.Spelling, qualifierMark):
				candidates(ref.Spelling)
			}
			return true
		})
	}
	for _, a := range attached {
		for _, arg := range a.Raw.Args {
			candidates(arg.Value.Text)
			for _, elem := range arg.Value.List {
				candidates(elem.Text)
			}
		}
	}
}

// recordPackageUses records a use of the unit for each declaration outside
// the package that an argument of a directive of a pending file can
// resolve to through the imports of files, the files of the package. The
// candidates of an argument are those of [golang.PackageBindings.Candidates],
// so an argument whose qualifier the imports of its own file bind has none.
// The package of each use joins the used packages of each pending file
// that prunes, so a signature-only file keeps the import through which the
// use resolves.
//
// # Allocation contract
//
// recordPackageUses allocates the list of the candidates of an argument
// whose qualifier the imports of its own file do not bind and the imports
// of another file bind, the uses that [plugin.SourceUnit.Use] records, and
// the entries of the used packages.
func recordPackageUses(u *plugin.SourceUnit, pkgPath string, files []*node.File, pending []fileUses) {
	shared := golang.PackageBindings(files)
	resolve := func(p fileUses, spelling string) {
		candidates, _ := shared.Candidates(p.bindings, spelling)
		for _, id := range candidates {
			if id.Package == pkgPath {
				continue
			}
			u.Use(id)
			for _, q := range pending {
				if q.used != nil {
					q.used[id.Package] = true
				}
			}
		}
	}
	for _, p := range pending {
		for _, a := range p.attached {
			for _, arg := range a.Raw.Args {
				resolve(p, arg.Value.Text)
				for _, elem := range arg.Value.List {
					resolve(p, elem.Text)
				}
			}
		}
	}
}
