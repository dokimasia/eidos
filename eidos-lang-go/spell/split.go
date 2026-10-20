// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell

import (
	"slices"

	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Split splits a unit with origins in [plugin.Unit.Inside] into the part
// that tests its package from outside and the part that tests it from
// inside, because Go compiles an external test package apart from the
// package under test, with access to its exported identifiers alone. The
// inside part keeps the origins of Inside, and [Filename] names its test
// file apart, so the layout writes it into the package under test. Each
// part keeps its declarations in the order of the unit, and its
// provenance narrows to the origins of its declarations. A unit without
// an inside origin is the one part that Split returns.
//
// # Allocation contract
//
// Split allocates the list of parts, and nothing more for a unit without
// an inside origin. A unit with one also allocates the declarations and
// the origins of each part.
func Split(u plugin.Unit) []plugin.Unit {
	if len(u.Inside) == 0 {
		return []plugin.Unit{u}
	}
	tests := func(d symbol.Symbol) bool {
		origin, _ := emit.OriginOf(d)
		_, held := slices.BinarySearchFunc(u.Inside, origin, symbol.Identity.Compare)
		return held
	}
	n := 0
	for _, d := range u.Decls {
		if tests(d) {
			n++
		}
	}
	outside, inside := make([]symbol.Symbol, 0, len(u.Decls)-n), make([]symbol.Symbol, 0, n)
	for _, d := range u.Decls {
		if tests(d) {
			inside = append(inside, d)
			continue
		}
		outside = append(outside, d)
	}
	parts := make([]plugin.Unit, 0, 2)
	if len(outside) > 0 {
		part := u
		part.Decls, part.Origins, part.Inside = outside, originsOf(outside), nil
		parts = append(parts, part)
	}
	if len(inside) > 0 {
		part := u
		part.Decls, part.Origins = inside, originsOf(inside)
		parts = append(parts, part)
	}
	return parts
}

// originsOf returns the distinct origins of decls that are not zero,
// sorted by identity, and nil where none has an origin. It allocates the
// list at the length of decls.
func originsOf(decls []symbol.Symbol) []symbol.Identity {
	out := make([]symbol.Identity, 0, len(decls))
	for _, d := range decls {
		if origin, _ := emit.OriginOf(d); !origin.IsZero() {
			out = append(out, origin)
		}
	}
	if len(out) == 0 {
		return nil
	}
	slices.SortFunc(out, symbol.Identity.Compare)
	return slices.Compact(out)
}
