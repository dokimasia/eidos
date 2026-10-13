// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript

import (
	"slices"
	"strings"
)

// The extensions TypeScript source is written under besides
// [Extension]: TSX, which parses with the TSX grammar, and the ES
// module and CommonJS forms.
const (
	ExtensionTSX = ".tsx"
	ExtensionMTS = ".mts"
	ExtensionCTS = ".cts"
)

// The extensions a module path leaves out. A declaration file's .d.ts,
// .d.mts and .d.cts count as one extension each, so a.d.ts and a.ts
// are both the module a. A specifier may name the file it imports by
// its emitted JavaScript extension, which names the source file, and
// so does a TypeScript extension.
var (
	declarationExtensions = []string{".d.ts", ".d.mts", ".d.cts"}
	moduleExtensions      = []string{
		Extension, ExtensionTSX, ExtensionMTS, ExtensionCTS, ".js", ".jsx", ".mjs", ".cjs",
	}
)

// ModulePath returns the module path a file or a specifier names: its
// path without its extension, a declaration file's whole one or the
// last of TypeScript's and JavaScript's. A path without either is its
// own module path. Every file is a module of its own, so the module
// path is the package the frontend loads a file's declarations into
// and the package the backend names a written file's package by. The
// module path is a part of p, so ModulePath allocates nothing.
func ModulePath(p string) string {
	for _, ext := range declarationExtensions {
		if trimmed, cut := strings.CutSuffix(p, ext); cut {
			return trimmed
		}
	}
	for _, ext := range moduleExtensions {
		if trimmed, cut := strings.CutSuffix(p, ext); cut {
			return trimmed
		}
	}
	return p
}

// DeclarationFile reports whether a file is a declaration file, every
// declaration of which is implemented elsewhere. It allocates nothing.
func DeclarationFile(file string) bool {
	return slices.ContainsFunc(declarationExtensions, func(ext string) bool {
		return strings.HasSuffix(file, ext)
	})
}

// The names of the module system of TypeScript that the frontend and the
// rules share.
const (
	// IndexModule is the module that resolution tries in a directory when
	// the path of a specifier has no module file.
	IndexModule = "index"
	// DefaultExport is the name of the default export of a module, and the
	// name that a default import binds.
	DefaultExport = "default"
)

// The openings of a relative module specifier, and the two relative
// specifiers that are a directory alone.
const (
	currentDir     = "./"
	parentDir      = "../"
	currentDirName = "."
	parentDirName  = ".."
)

// Relative reports whether a module specifier names a path relative to
// the directory of the importing file: it opens with ./ or ../, or it is
// . or .. alone. Any other specifier names a package or an ambient
// module. Relative allocates nothing.
func Relative(specifier string) bool {
	return strings.HasPrefix(specifier, currentDir) || strings.HasPrefix(specifier, parentDir) ||
		specifier == currentDirName || specifier == parentDirName
}

// packageSep separates the segments of a package path.
const packageSep = "/"

// JoinPackage appends a segment to a package path, such as the name of
// a namespace to the package of the module that declares it. Below the
// global package, which is the empty path, the result is the segment
// alone. For an empty segment it is the package alone. JoinPackage
// allocates the joined path, one allocation, and nothing when either part
// is empty.
func JoinPackage(pkg, segment string) string {
	switch {
	case pkg == "":
		return segment
	case segment == "":
		return pkg
	default:
		return pkg + packageSep + segment
	}
}
