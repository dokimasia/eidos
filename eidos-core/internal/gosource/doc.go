// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package gosource loads Go packages from source, without a build
// cache.
//
// [Load] parses and type-checks one directory. [ParseDir] is the
// parse step alone, and [NewImporter] is the resolver the type
// checker uses. [ModuleRoot] and [ModulePath] answer where the
// enclosing module starts and what it is called.
//
// Every loader here reads hand-written sources only: generated and
// test files are skipped. A code generator whose input is closed
// over hand-written source cannot be fed its own output, and it
// runs in a tree holding no generated file yet.
//
// # Failure semantics
//
// Every function reports an error rather than panicking, and the
// message names the path it could not read. A directory holding no
// hand-written Go file is an error, not an empty package.
//
// # Dependency position
//
// core/internal/gosource imports only the Go stdlib.
package gosource
