// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package spell

import (
	"path"
	"strings"

	"go.dokimi.dev/eidos/core/plugin"
	java "go.dokimi.dev/eidos/lang-java"
	"go.dokimi.dev/eidos/lang/naming"
)

// Filename spells a unit's filename. Java names a file after the
// public type it holds, so the routing key's stem, the family
// word and the tag join and convert to Pascal case as one: a
// store unit of the stub family reaches disk as StoreStub.java.
//
// The stem drops the key's own extension, whatever the source
// language spelled it as. A plan unit carries no key, so its
// filename is the word and the tag alone.
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
	return naming.Pascal(strings.Join(parts, "_")) + java.Extension
}
