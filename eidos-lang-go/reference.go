// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"fmt"
	"go/token"
	"strings"

	"golang.org/x/mod/module"
)

// The forms of the path element internal, which the go command reads in an
// import path: the element alone, at the start of a path, inside a path and
// at its end.
const (
	internalElem   = "internal"
	internalPrefix = internalElem + "/"
	internalSuffix = "/" + internalElem
	internalInfix  = internalSuffix + "/"
)

// CheckReference returns the reason that Go code of the package at the
// import path from cannot refer to the declaration name of the package at
// the import path pkg, and nil where it can. The rules of the go command and
// of the compiler decide:
//
//   - The code refers to a declaration of no package, such as a predeclared
//     type, and to every declaration of its own package.
//   - pkg is a valid import path, as [module.CheckImportPath] checks it.
//   - pkg is not an external test package. The go command compiles such a
//     package into the test binary of its directory, and no other package
//     imports it. An import path that ends in [TestSuffix] names one, as the
//     frontend loads it.
//   - Where pkg has the path element internal, from is the parent of the
//     last such element or a package below it. The suffix [TestSuffix] of
//     from belongs to the package that it tests. The parent of a path that
//     starts with internal is the standard library, which contains no
//     generated code.
//   - name is exported, because Go code refers to the exported names of
//     another package alone. An empty name checks the import of pkg alone.
//
// CheckReference allocates nothing for a declaration that the code can refer
// to. A refusal allocates its error.
func CheckReference(from, pkg, name string) error {
	if pkg == "" || pkg == from {
		return nil
	}
	if err := module.CheckImportPath(pkg); err != nil {
		return fmt.Errorf("%s: the go command refuses the import: %w", Lang, err)
	}
	if strings.HasSuffix(pkg, TestSuffix) {
		return fmt.Errorf("%s: %s is an external test package, and no other package imports it", Lang, pkg)
	}
	internal := true
	var parent string
	switch {
	case strings.HasSuffix(pkg, internalSuffix):
		parent = strings.TrimSuffix(pkg, internalSuffix)
	case strings.Contains(pkg, internalInfix):
		parent = pkg[:strings.LastIndex(pkg, internalInfix)]
	case pkg == internalElem || strings.HasPrefix(pkg, internalPrefix):
		return fmt.Errorf("%s: %s is internal to the standard library, and the package %s is outside it",
			Lang, pkg, from)
	default:
		internal = false
	}
	if internal {
		rest, under := strings.CutPrefix(strings.TrimSuffix(from, TestSuffix), parent)
		if !under || rest != "" && rest[0] != '/' {
			return fmt.Errorf("%s: %s is internal to %s, and the package %s is outside it", Lang, pkg, parent, from)
		}
	}
	if name != "" && !token.IsExported(name) {
		return fmt.Errorf("%s: %s is not exported from %s, and the code is in %s", Lang, name, pkg, from)
	}
	return nil
}
