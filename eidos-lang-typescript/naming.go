// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package typescript

import (
	"path"
	"strings"

	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/lang/naming"
)

// Naming spells a unit's filename. TypeScript names files in
// kebab case with dot-separated qualifiers, so the routing key's
// stem, the family word and the tag each convert alone and join
// with dots: a store unit of the stub family reaches disk as
// store.stub.ts, and a family word spelled HTTPClient reaches it
// as http-client.
//
// The stem drops the key's own extension, whatever the source
// language spelled it as. A plan unit carries no key, so its
// filename is the word and the tag alone.
func Naming(u plugin.Unit) string {
	parts := make([]string, 0, 3)
	if stem := path.Base(u.Key); u.Key != "" && stem != "." {
		parts = append(parts, naming.Kebab(strings.TrimSuffix(stem, path.Ext(stem))))
	}
	if u.Word != "" {
		parts = append(parts, naming.Kebab(u.Word))
	}
	if u.Tag != "" {
		parts = append(parts, naming.Kebab(u.Tag))
	}
	return strings.Join(parts, ".") + Extension
}
