// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"strings"

	java "go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Filename spells a unit's filename. Java names a file after the
// public type it declares, so a unit with one type spells that
// type's own name. The backend's split hands over every typed unit
// in that shape, and the generator's join puts the family word into
// the type name.
//
// A unit without a lone type falls back to its
// [naming.FilenameParts], joined and converted to Pascal case as
// one.
func Filename(u plugin.Unit) string {
	if name, held := typeName(u); held {
		return naming.Pascal(name) + java.Extension
	}
	return naming.Pascal(strings.Join(naming.FilenameParts(u.Key, u.Word, u.Tag), "_")) + java.Extension
}

// typeName returns the name of the one file-level type a unit
// declares, and false for every other shape.
func typeName(u plugin.Unit) (string, bool) {
	if len(u.Decls) != 1 {
		return "", false
	}
	switch d := u.Decls[0].(type) {
	case *emit.Struct:
		return d.Name, d.Name != ""
	case *emit.Interface:
		return d.Name, d.Name != ""
	case *emit.Enum:
		return d.Name, d.Name != ""
	default:
		return "", false
	}
}
