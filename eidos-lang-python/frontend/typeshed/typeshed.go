// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typeshed

import "embed"

// Commit is the commit of typeshed whose stdlib directory [Files] embeds.
const Commit = "e75a39fec6f375f5002d54c5607b44bedc025a4b"

// The paths of the tree of [Files]: the directory of the stubs of the
// standard library, and the table of versions in it.
const (
	StdlibDir    = "stdlib"
	VersionsFile = StdlibDir + "/VERSIONS"
)

// Files is the embedded tree of typeshed in the layout of its repository:
// below [StdlibDir], the stub of each module at its slash path, as
// stdlib/collections/abc.pyi, the stub of each package directory at its
// __init__.pyi, and [VersionsFile]. The stubs are at most four directories
// below StdlibDir. Any number of goroutines may read the tree, and a read
// allocates what embed.FS allocates.
//
//go:embed stdlib/VERSIONS stdlib/*.pyi stdlib/*/*.pyi stdlib/*/*/*.pyi stdlib/*/*/*/*.pyi
var Files embed.FS

// VersionsText is the text of [VersionsFile], for a reader that needs the
// table of versions without the tree, such as the backend, which tells the
// modules of the standard library apart in the import block. [ParseVersions]
// reads it.
//
//go:embed stdlib/VERSIONS
var VersionsText []byte
