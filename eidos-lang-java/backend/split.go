// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Split reshapes one unit into one unit per file-level type,
// because Java names a file after the public type it declares: the
// filename spelling reads the lone type's name off each split unit,
// and the routing key is untouched, so the file's source derivation
// survives the reshaping. Declarations of any other kind remain
// together under the original key, where the render reports each
// under the kind the backend refuses. Each split unit's provenance
// narrows to its own type's origin.
//
// # Allocation contract
//
// Split allocates the list of units, each type's list of its origin,
// and the list of the declarations of other kinds. Each split unit's
// declaration list is a one-element window on the original unit's.
func Split(u plugin.Unit) []plugin.Unit {
	out := make([]plugin.Unit, 0, len(u.Decls))
	var rest []symbol.Symbol
	for i, d := range u.Decls {
		switch d.(type) {
		case *emit.Struct, *emit.Interface, *emit.Enum:
		default:
			rest = append(rest, d)
			continue
		}
		su := u
		su.Decls = u.Decls[i : i+1 : i+1]
		su.Origins = originsOf(d)
		out = append(out, su)
	}
	if len(rest) > 0 {
		ru := u
		ru.Decls = rest
		out = append(out, ru)
	}
	return out
}

// originsOf returns a declaration's origin as a list of one, and
// nothing for a declaration without an origin.
func originsOf(d symbol.Symbol) []symbol.Identity {
	if id, held := emit.OriginOf(d); held && !id.IsZero() {
		return []symbol.Identity{id}
	}
	return nil
}
