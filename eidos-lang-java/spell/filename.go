// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"strings"

	"go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Filename spells a unit's filename. Java names a file after the
// public type it declares, so a unit with one type spells that
// type's own name. The backend's split gives every typed unit one
// type, and the generator's join puts the family word into the type
// name.
//
// A unit without a lone type falls back to the [naming.FilenameParts]
// of its [plugin.Unit.FileKey], word and tag, joined and converted to
// Pascal case as one. A per-package or per-plan unit has no file key,
// so its fallback is the word and the tag alone.
//
// # Allocation contract
//
// Filename allocates the name with its extension, and one more
// allocation where a lone type's name converts to Pascal case. The
// fallback allocates the list of parts and the name with its extension,
// and one more allocation each for the join of more than one part and
// for a Pascal-case conversion: four for a per-source unit.
func Filename(u plugin.Unit) string {
	if name, held := typeName(u); held {
		return naming.Pascal(name) + java.Extension
	}
	return naming.Pascal(strings.Join(naming.FilenameParts(u.FileKey(), u.Word, u.Tag), "_")) + java.Extension
}

// typeName returns the name of the one file-level type a unit
// declares, and false for a unit of any other declarations.
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
