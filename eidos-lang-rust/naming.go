// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package rust

import (
	"path"
	"strings"

	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/lang/naming"
)

// Naming spells a unit's filename. Rust names modules in snake
// case, so the routing key's stem, the family word and the tag
// join with underscores and convert as one: a store unit of the
// stub family reaches disk as store_stub.rs.
//
// The stem drops the key's own extension, whatever the source
// language spelled it as. A plan unit carries no key, so its
// filename is the word and the tag alone.
func Naming(u plugin.Unit) string {
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
	return naming.Snake(strings.Join(parts, "_")) + Extension
}
