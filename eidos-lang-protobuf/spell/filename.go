// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"path"
	"strings"

	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Filename spells a unit's filename. protobuf's style guide names files in
// snake case, so the stem of the unit's [plugin.Unit.FileKey], the family
// word and the tag join with underscores and convert as one: a store unit
// of the schema family is written as store_schema.proto.
//
// The stem drops the key's own extension, whatever the source language
// spelled it as. A stem of lower-case letters, digits, underscores and
// hyphens keeps its spelling, as protoc writes a file name, so the sources
// test-extension.proto and test_extension.proto take two filenames. A
// per-package or per-plan unit has no file key, so its filename is the
// word and the tag alone.
//
// # Allocation contract
//
// Filename allocates the list of the filename's parts and the name with
// its extension. It allocates once more for the join of more than one
// part, and once more where the join is not snake case already: three
// allocations for a per-source unit of snake-case parts.
func Filename(u plugin.Unit) string {
	key := u.FileKey()
	base := path.Base(key)
	stem := strings.TrimSuffix(base, path.Ext(base))
	if key == "" || !strings.Contains(stem, "-") || !lowerStem(stem) {
		return naming.SnakeFilename(key, u.Word, u.Tag, protobuf.Extension)
	}
	rest := naming.SnakeFilename("", u.Word, u.Tag, "")
	if rest == "" {
		return stem + protobuf.Extension
	}
	return stem + "_" + rest + protobuf.Extension
}

// lowerStem reports whether a stem is made of lower-case letters, digits,
// underscores and hyphens alone.
func lowerStem(stem string) bool {
	for _, c := range stem {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' && c != '-' {
			return false
		}
	}
	return true
}
