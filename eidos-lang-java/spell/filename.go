// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package spell

import (
	"path"
	"strings"

	java "go.dokimi.dev/eidos/lang-java"
	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Filename spells a unit's filename. Java names a file after the
// public type it holds, so a unit holding one type spells that
// type's own name: the backend's split hands every typed unit
// over in that shape, and the family word reaches the type name
// through the generator's join rather than the filename.
//
// A unit holding no lone type falls back to the routing key's
// stem, the family word and the tag, joined and converted to
// Pascal case as one; the stem drops the key's own extension,
// whatever the source language spelled it as. A plan unit
// carries no key, so its fallback is the word and the tag alone.
func Filename(u plugin.Unit) string {
	if name, held := typeName(u); held {
		return naming.Pascal(name) + java.Extension
	}
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

// typeName returns the name of the one file-level type a unit
// holds, and false for every other shape.
func typeName(u plugin.Unit) (string, bool) {
	if len(u.Decls) != 1 {
		return "", false
	}
	switch d := u.Decls[0].(type) {
	case *emit.Struct:
		return d.Name, d.Name != ""
	case *emit.Interface:
		return d.Name, d.Name != ""
	default:
		return "", false
	}
}
