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
// unit of the stub family is written as store_stub.rs.
//
// The stem drops the key's own extension, whatever the source language
// spelled it as. A per-package or per-plan unit has no file key, so
// its filename is the word and the tag alone.
//
// # Allocation contract
//
// Filename allocates the list of the filename's parts and the name with
// its extension. It allocates once more for the join of more than one
// part, and once more where the join is not snake case already: three
// allocations for a per-source unit of snake-case parts.
func Filename(u plugin.Unit) string {
	return naming.SnakeFilename(u.FileKey(), u.Word, u.Tag, rust.Extension)
}
