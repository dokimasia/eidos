// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package naming

import (
	"path"
	"strings"
)

// FilenameParts returns the parts a unit's filename is built from, in
// order: the stem of the unit's file key without its extension,
// whatever the source language spelled it as, then the family word and
// the tag. An empty part is left out, so a per-package or per-plan
// unit, whose file key is empty, yields the word and the tag alone.
// Each target joins and cases the parts in its own filename
// convention. It allocates the list, one allocation.
func FilenameParts(key, word, tag string) []string {
	parts := make([]string, 0, 3)
	if stem := path.Base(key); key != "" && stem != "." {
		parts = append(parts, strings.TrimSuffix(stem, path.Ext(stem)))
	}
	if word != "" {
		parts = append(parts, word)
	}
	if tag != "" {
		parts = append(parts, tag)
	}
	return parts
}

// SnakeFilename joins the [FilenameParts] of a routing key, a family
// word and a tag with underscores, converts the join to snake case,
// and appends the extension, which is how the snake-cased languages
// name files. It allocates the parts, their join and the name, three
// allocations, and a fourth where the join is not snake case already.
func SnakeFilename(key, word, tag, ext string) string {
	return Snake(strings.Join(FilenameParts(key, word, tag), "_")) + ext
}
