// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"strconv"
	"strings"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/scaffold"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// leaves is Go's spelling of the literal leaves. A number keeps the
// text the derivation wrote in Go's own spelling, a string quotes
// through the standard library, so every escape is Go's, and the
// absent value is nil, which every reference type takes.
var leaves = scaffold.Leaves{
	Lang:   golang.Lang,
	Absent: "nil",
	Quote:  strconv.Quote,
}

// target spells a value tree as Go, binding in the file's import set
// the packages its references and callees name. It is bound to one
// file, because the set is.
type target struct{ set *render.ImportSet }

// Lang names the target for a refusal.
func (target) Lang() string { return string(golang.Lang) }

// Literal spells one leaf through [leaves].
func (target) Literal(v emit.Value) (string, error) { return leaves.Literal(v) }

// Type spells a reference through the file's [Speller], so a value
// and a declaration bind one import under one name.
func (t target) Type(ref *emit.TypeRef) (string, error) {
	return NewSpeller(t.set).Spell(ref)
}

// Callee spells a function by its identity and binds its package's
// import. A call has no source spelling of its own, so the name is
// qualified by the name the package's import binds, which is the
// name its path assumes unless another import or a declaration of
// the file takes that name. A function in no package, and one in the
// file's own package, spells bare.
func (t target) Callee(id symbol.Identity) (string, error) {
	if id.Package == "" {
		return id.Name, nil
	}
	local := t.set.Bind(id.Package, golang.AssumedName(id.Package))
	if local == "" {
		return id.Name, nil
	}
	return local + qualifierSep + id.Name, nil
}

// Conversion spells Go's conversion, which every named type takes.
func (target) Conversion(_ *emit.TypeRef, typ, inner string) (string, error) {
	return typ + "(" + inner + ")", nil
}

// Composite spells a composite literal: named fields for a struct,
// keyed entries for a map, and bare elements for a slice or an
// array, which is one syntax over the three forms.
func (target) Composite(_ *emit.TypeRef, typ string, entries []scaffold.Entry) (string, error) {
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		switch {
		case e.Name != "":
			parts = append(parts, e.Name+": "+e.Value)
		case e.Key != "":
			parts = append(parts, e.Key+": "+e.Value)
		default:
			parts = append(parts, e.Value)
		}
	}
	return typ + "{" + strings.Join(parts, ", ") + "}", nil
}

// Address spells Go's address operator, which Go applies to a
// composite literal alone: the address of any other value, such as
// &1, does not compile, so it is refused.
func (t target) Address(inner emit.Value, spelled string) (string, error) {
	if inner.Kind != emit.ValueComposite {
		return "", render.RefuseValue(t.Lang(),
			"Go takes the address of a composite literal alone, and the value is a %s", inner.Kind)
	}
	return "&" + spelled, nil
}
