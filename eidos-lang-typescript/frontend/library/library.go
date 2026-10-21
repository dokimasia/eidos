// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package library

import "embed"

// Release is the release of TypeScript whose library [Files] embeds.
const Release = "7.0.2"

// Dir is the directory of [Files] that contains the declaration files.
const Dir = "libs"

// Files is the library of TypeScript at [Release]: below [Dir], each
// declaration file of the release, as libs/lib.es2024.d.ts. Any number of
// goroutines may read the tree, and a read allocates what embed.FS
// allocates.
//
//go:embed libs/*.d.ts
var Files embed.FS

// Statement is one top-level statement of a file of [Files] that declares
// a name of the global scope. It has the name and the byte range of the
// statement in the file. The range of a statement of a script starts where
// the range of the statement before it ends, so it contains the comments
// above the statement. It ends at the line break that ends the last line of
// the statement. A statement of a module file covers the whole file.
type Statement struct {
	Name  string
	Start int
	End   int
}

// Entry is what one file of [Files] declares in the global scope. Module
// reports a file with a top-level import or export, whose declare global
// blocks declare its global names. Statements are in the order of the
// file: one for each top-level declaration of a script, and one for each
// name that the global blocks of a module declare.
type Entry struct {
	Module     bool
	Statements []Statement
}
