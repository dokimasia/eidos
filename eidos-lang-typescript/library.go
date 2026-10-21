// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript

import (
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// LibraryStore is the name of the store of the declaration files of the
// TypeScript library. A path inside the store is <key>/<file>, a file of the
// library of a project, as lib.es2024.d.ts/lib.es5.d.ts. The path
// <key>/<file>#<name> is the fragment of the statements of one global name
// of the file, as lib.es2024.d.ts/lib.es5.d.ts#Error. The key of a library
// is its root files, sorted and joined with [LibraryKeySep], and the
// package of its declarations is the qualified path of the key.
const LibraryStore = "typescript-lib"

// LibraryKeySep joins the root files of a library in its key, as
// lib.es2024.d.ts+lib.esnext.disposable.d.ts.
const LibraryKeySep = "+"

// Library reports whether pkg is the package of a library of TypeScript, or
// a namespace inside one: a qualified path inside [LibraryStore]. It
// allocates nothing.
func Library(pkg string) bool {
	store, _, qualified := plugin.CutStorePath(pkg)
	return qualified && store == LibraryStore
}

// Global reports whether a reference refers to a declaration of the global
// scope of TypeScript, where target is the target of the reference and pkg
// is its package. That is a reference whose target and package are empty,
// or a reference to a declaration of a library. A declaration in the global
// package of the workspace has a target, and Global reports false for a
// reference to it. It allocates nothing.
func Global(target symbol.Identity, pkg string) bool {
	if target.IsZero() {
		return pkg == ""
	}
	return Library(target.Package)
}
