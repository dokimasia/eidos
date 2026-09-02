// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package naming

import (
	"path"
	"strings"
)

// SnakeFilename joins a routing key's stem, a family word and a
// tag with underscores, converts the join to snake case, and
// appends the extension: the filename shape the snake-cased
// languages share. The stem drops the key's own extension,
// whatever the source language spelled it as, and an empty part is
// skipped, so a plan unit's filename is the word and the tag
// alone.
func SnakeFilename(key, word, tag, ext string) string {
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
	return Snake(strings.Join(parts, "_")) + ext
}
