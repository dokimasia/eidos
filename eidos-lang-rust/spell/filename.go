// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"go.dokimi.dev/eidos/lang/naming"
	rust "go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Filename spells a unit's filename. Rust names modules in snake
// case, so the stem of the unit's [plugin.Unit.FileKey], the family
// word and the tag join with underscores and convert as one: a store
// unit of the stub family reaches disk as store_stub.rs.
//
// The stem drops the key's own extension, whatever the source language
// spelled it as. A per-package or per-plan unit has no file key, so
// its filename is the word and the tag alone.
func Filename(u plugin.Unit) string {
	return naming.SnakeFilename(u.FileKey(), u.Word, u.Tag, rust.Extension)
}
