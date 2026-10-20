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
