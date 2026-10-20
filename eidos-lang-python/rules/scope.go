// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"strings"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/sdk/node"
)

// bindingOf returns what the imports of the file f bind the local name to:
// the package path of a module, and the name of a declaration of the module
// where the import names one. import a.b binds a to the module a, import
// a.b as c binds c to the module a/b, and from a import b as c binds c to
// the declaration b of the module a. A later import binds a name over an
// earlier one, as it does when the module runs. bindingOf reports false for
// a name that no import binds, and for a nil file. It allocates nothing.
func bindingOf(f *node.File, local string) (module, name string, ok bool) {
	if f == nil {
		return "", "", false
	}
	for _, imp := range f.Imports {
		if imp == nil {
			continue
		}
		if imp.Alias == local {
			module, name, ok = imp.Path, "", true
			continue
		}
		if head, _, _ := strings.Cut(imp.Path, python.PackageSep); imp.Alias == "" && len(imp.Names) == 0 &&
			!imp.Wildcard && head == local {

			module, name, ok = head, "", true
			continue
		}
		for _, b := range imp.Names {
			if b != nil && (b.Alias == local || b.Alias == "" && b.Name == local) {
				module, name, ok = imp.Path, b.Name, true
			}
		}
	}
	return module, name, ok
}

// canonical returns the module and the name that a dotted name means in the
// file f of the package pkg. The rules compare the result with a name of
// enum, dataclasses or typing for a callee, a decorator and a base.
//
//   - A name that pkg declares is in pkg itself.
//   - Through the import that binds its first name, the module is the
//     module of the import, and the name is the rest of the dotted name in
//     it. dataclasses.dataclass after import dataclasses is dataclass of
//     dataclasses, and make after from enum import auto as make is auto of
//     enum.
//   - A name without a dot that nothing binds is in builtins, and a dotted
//     name whose first name nothing binds has the empty module.
//
// canonical allocates the name of a dotted name through a name that an
// import binds, and nothing otherwise.
func canonical(pkg *node.Package, f *node.File, dotted string) (module, name string) {
	head, rest, nested := strings.Cut(dotted, python.ModuleSep)
	boundModule, boundName, imported := bindingOf(f, head)
	switch {
	case declares(pkg, head):
		return pkg.ID.Package, dotted
	case !imported && !nested:
		return builtinsModule, dotted
	case !imported:
		return "", dotted
	case boundName != "" && !nested:
		return boundModule, boundName
	case boundName != "":
		return boundModule, boundName + python.ModuleSep + rest
	default:
		return boundModule, rest
	}
}

// declares reports whether a file of the package pkg declares the name at
// its top level, where the declarations of the file are. A nil package
// declares nothing. It allocates nothing.
func declares(pkg *node.Package, name string) bool {
	if pkg == nil {
		return false
	}
	for _, f := range pkg.Files {
		for _, sym := range f.Decls {
			if d, named := sym.(node.Declaration); named && d.Identity().Name == name {
				return true
			}
		}
	}
	return false
}
