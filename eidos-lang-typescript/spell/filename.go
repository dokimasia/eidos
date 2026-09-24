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
// case with dot-separated qualifiers, so each of the unit's
// [naming.FilenameParts] converts alone and the parts join with
// dots: a store unit of the stub family is written as store.stub.ts,
// and a family word spelled HTTPClient as http-client.
func Filename(u plugin.Unit) string {
	parts := naming.FilenameParts(u.Key, u.Word, u.Tag)
	for i, part := range parts {
		parts[i] = naming.Kebab(part)
	}
	return strings.Join(parts, ".") + typescript.Extension
}
