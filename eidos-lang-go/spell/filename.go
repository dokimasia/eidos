// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"strings"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Filename spells a unit's filename. Go names files in snake case,
// so the stem of the unit's [plugin.Unit.FileKey], the family word and
// the tag join with underscores and convert as one: a family word
// spelled HTTPClient becomes http_client in the filename.
//
// A per-package or per-plan unit has no file key, so its filename is
// the word and the tag alone: a per-package suite family in the test
// companion is suite_test.go, which compiles into the test binary.
//
// A unit whose package path ends in [golang.TestSuffix] belongs to an
// external test package, and only a test file declares one, so its
// filename ends in _test.go: a per-package stub of store_test is
// stub_test.go, and a stub of the source store_test.go is
// store_test_stub_test.go.
func Filename(u plugin.Unit) string {
	name := naming.SnakeFilename(u.FileKey(), u.Word, u.Tag, "")
	if strings.HasSuffix(u.Pkg.Package, golang.TestSuffix) && !strings.HasSuffix(name, golang.TestSuffix) {
		name += golang.TestSuffix
	}
	return name + golang.Extension
}
