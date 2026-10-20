// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package library embeds the declaration files of the library of TypeScript
// at one release.
//
// [Files] is the library of TypeScript at [Release]: one declaration file for
// each library below [Dir], as libs/lib.es2024.d.ts. The files are those of
// internal/bundled/libs of typescript-go at the tag of the release, which the
// npm package typescript of the release installs. The frontend of the
// satellite serves them as its library store, so every machine loads one
// library.
//
// # Licence
//
// Microsoft licenses the files under the Apache License 2.0. LICENSE beside
// this file reproduces the licence. NOTICE.txt reproduces the notices of the
// release, which cover the W3C and WHATWG texts of the DOM library. A refresh
// of the library copies the files of a newer release and changes [Release].
//
// # Dependency position
//
// frontend/library imports the Go stdlib.
package library
