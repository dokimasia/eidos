// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package spell

import (
	"path"
	"strings"

	golang "go.dokimi.dev/eidos/lang-go"
	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Filename spells a unit's filename. Go names files in snake case,
// so the routing key's stem, the family word and the tag join
// with underscores and convert as one: a family word spelled
// HTTPClient reaches the filename as http_client.
//
// A plan unit carries no key, so its filename is the word and the
// tag alone.
func Filename(u plugin.Unit) string {
	parts := make([]string, 0, 3)
	if stem := path.Base(u.Key); u.Key != "" && stem != "." {
		parts = append(parts, strings.TrimSuffix(stem, path.Ext(stem)))
	}
	if u.Word != "" {
		parts = append(parts, u.Word)
	}
	if u.Tag != "" {
		parts = append(parts, u.Tag)
	}
	return naming.Snake(strings.Join(parts, "_")) + golang.Extension
}
