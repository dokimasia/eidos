// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package gosource loads Go packages from source, without a build
// cache.
//
// [Load] parses and type-checks one directory. [ParseDir] is the
// parse step alone, and [NewImporter] is the resolver the type
// checker uses. [ModuleRoot] and [ModulePath] report where the
// enclosing module starts and what it is called.
//
// Every loader here skips test files. [HandWritten] mode skips
// generated files too: a code generator whose input is closed over
// hand-written source cannot be fed its own output, and it runs in
// a tree holding no generated file yet. [Complete] mode reads them,
// because hand-written code in a dependency may refer to what that
// dependency's own generator produced.
//
// # Failure semantics
//
// Every function reports an error rather than panicking, and the
// message names the path it could not read. A directory holding no
// readable Go file is an error, not an empty package. An import
// cycle through the module's own packages is an error in either
// mode.
//
// # Dependency position
//
// core/internal/gosource imports only the Go stdlib.
package gosource
