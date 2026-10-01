// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"strings"

	"go.dokimi.dev/eidos/lang/naming"
	typescript "go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Filename spells a unit's filename. TypeScript names files in kebab
// case with dot-separated qualifiers, so each of the
// [naming.FilenameParts] of the unit's [plugin.Unit.FileKey], word and
// tag converts alone and the parts join with dots: a store unit of the
// stub family is written as store.stub.ts, and a family word spelled
// HTTPClient as http-client. A per-package or per-plan unit has no
// file key, so its filename is the word and the tag alone: a
// per-package suite family in the test companion is suite.test.ts.
func Filename(u plugin.Unit) string {
	parts := naming.FilenameParts(u.FileKey(), u.Word, u.Tag)
	for i, part := range parts {
		parts[i] = naming.Kebab(part)
	}
	return strings.Join(parts, ".") + typescript.Extension
}
