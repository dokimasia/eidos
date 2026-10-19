// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Filename spells the filename of a unit. The guide to naming of
// graphql.org names files in snake case, so the stem of the
// [plugin.Unit.FileKey] of the unit, the family word and the tag join with
// underscores and convert as one. A catalog unit of the family schema is
// written as catalog_schema.graphql.
//
// The stem drops the extension of the key, whatever the source language
// spelled it as. A per-package or per-plan unit has no file key, so its
// filename is the word and the tag alone.
//
// # Allocation contract
//
// Filename allocates the list of the parts of the filename and the name
// with its extension. It allocates once more for the join of more than one
// part, and once more where the join is not snake case already: three
// allocations for a per-source unit of snake-case parts.
func Filename(u plugin.Unit) string {
	return naming.SnakeFilename(u.FileKey(), u.Word, u.Tag, graphql.Extension)
}
