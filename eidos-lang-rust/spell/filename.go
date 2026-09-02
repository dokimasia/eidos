// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"go.dokimi.dev/eidos/lang/naming"
	rust "go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Filename spells a unit's filename. Rust names modules in snake
// case, so the routing key's stem, the family word and the tag
// join with underscores and convert as one: a store unit of the
// stub family reaches disk as store_stub.rs.
//
// The stem drops the key's own extension, whatever the source
// language spelled it as. A plan unit carries no key, so its
// filename is the word and the tag alone.
func Filename(u plugin.Unit) string {
	return naming.SnakeFilename(u.Key, u.Word, u.Tag, rust.Extension)
}
