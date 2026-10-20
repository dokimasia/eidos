// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package typeshed embeds the stubs of the standard library of Python from
// typeshed at one commit, and reads the table of the versions that each
// module exists in.
//
// [Files] is the tree of typeshed at [Commit] in the layout of its
// repository: below [StdlibDir], a stub for each module, as
// stdlib/collections/abc.pyi, the stub of each package directory, as
// stdlib/collections/__init__.pyi, and [VersionsFile]. The frontend of the
// satellite opens the tree as its typeshed store, so every machine loads
// one standard library. [ParseVersions] reads VERSIONS into [Versions],
// whose [Versions.Includes] reports whether a module exists in a version
// of Python.
//
// # Licence
//
// typeshed is licensed under the Apache License 2.0, and parts of it under
// the MIT licence. LICENSE beside this file reproduces both. A refresh of
// the tree copies the stubs and VERSIONS of a newer commit and changes
// [Commit].
//
// # Dependency position
//
// frontend/typeshed imports the Go stdlib.
package typeshed
