// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	golang "go.dokimi.dev/eidos/lang/go"
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
	return naming.SnakeFilename(u.Key, u.Word, u.Tag, golang.Extension)
}
