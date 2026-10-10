// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"strings"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Filename spells a unit's filename in Go's snake case. The stem of the
// unit's [plugin.Unit.FileKey], the family word and the tag join with
// underscores and convert as one, so a family word spelled HTTPClient
// becomes http_client in the filename.
//
// A per-package or per-plan unit has no file key, and its filename is
// the word and the tag alone. The suite family's per-package unit in
// the test companion is suite_test.go, which compiles into the test
// binary.
//
// A unit whose package path ends in [golang.TestSuffix] belongs to an
// external test package. Go declares such a package in test files
// alone, so the filename ends in _test.go: a per-package stub of
// store_test is stub_test.go, and a stub of the source store_test.go is
// store_test_stub_test.go.
//
// # Allocation contract
//
// Filename allocates the list of the filename's parts and the name with
// its extension. It allocates once more for each step that applies: the
// join of more than one part, a snake-case conversion, and the test
// suffix. A per-source unit whose parts are snake case already costs
// three allocations.
func Filename(u plugin.Unit) string {
	name := naming.SnakeFilename(u.FileKey(), u.Word, u.Tag, "")
	if strings.HasSuffix(u.Pkg.Package, golang.TestSuffix) && !strings.HasSuffix(name, golang.TestSuffix) {
		name += golang.TestSuffix
	}
	return name + golang.Extension
}
