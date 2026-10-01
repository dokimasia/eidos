// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"context"
	"io/fs"
)

// Need is one import path that no loaded package of the frontend's
// language declares, and the paths of the files that import it,
// sorted.
type Need struct {
	Path string
	From []string
}

// DependencyRound is what one dependency round hands a frontend.
type DependencyRound struct {
	// Number counts the rounds from one.
	Number int

	// Needs are the import paths no loaded package declares and no
	// earlier round passed, sorted by path. The first round runs even
	// when it has none.
	Needs []Need

	// Shared lists the shared inputs the frontend's partition declared
	// on the workspace's units, sorted and without repeats: the build
	// files a language reads its dependency set from, such as the
	// nearest go.mod above each workspace file.
	Shared []string
}

// StoreReader is a dependency round's recorded door: reads and
// directory listings over the workspace tree and the stores. Every
// read and every listing folds into the key of every unit the round
// returns. A qualified path naming a store the load does not provide
// returns an error wrapping [ErrStoreAbsent].
type StoreReader interface {
	FileReader

	// ReadDir returns one directory's entries sorted by name, for a
	// workspace path or a qualified one.
	ReadDir(path string) ([]fs.DirEntry, error)
}

// Dependent is the optional role of a frontend whose sources name
// declarations outside the workspace tree. After the workspace's units
// parse, the load calls Dependencies once per round, parses every
// returned unit at [DepthSignatures], and calls it again with the
// imports the returned units name. A round after the first runs only
// when it has a need no earlier round passed, and the rounds end at
// the first round that returns no unit the load has not loaded.
type Dependent interface {
	// Dependencies returns the units that declare the round's needs,
	// grouped as Partition groups, and every unit the language's build
	// declares whatever the needs are, such as a Java classpath. A
	// member is a qualified store path, or a workspace path the
	// selection does not claim, such as a Go vendor tree. A need the
	// language cannot place yields no unit, and its references keep
	// their spellings. A returned error is fatal to the load.
	Dependencies(ctx context.Context, round DependencyRound, r StoreReader) ([][]SourceRef, error)
}
