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

// qualifierSep separates a package qualifier from the name it
// qualifies.
const qualifierSep = "."

// leaves is Go's spelling of the literal leaves. A number keeps the
// text the derivation wrote in Go's own spelling, a string quotes
// through the standard library, so every escape is Go's, and the
// absent value is nil, which every reference type takes.
var leaves = scaffold.Leaves{
	Lang:   golang.Lang,
	Absent: "nil",
	Quote:  strconv.Quote,
}

// target spells a value tree as Go, recording into the file's
// import set the packages its references and callees need. It is
// bound to one file, because the set is.
type target struct{ set *render.ImportSet }

// Lang names the target for a refusal.
func (target) Lang() string { return string(golang.Lang) }

// Literal spells one leaf through [leaves].
func (target) Literal(v emit.Value) (string, error) { return leaves.Literal(v) }

// Type spells a reference and records the import its package
// needs. The spelling is the reference's own, which is what a Go
// graph rendering back to Go states.
func (t target) Type(ref *emit.TypeRef) (string, error) {
	t.use(ref.Target)
	return Spell(ref), nil
}

// Callee spells a function by its identity and records its import.
// A call has no source spelling of its own, so the name is qualified
// by the last segment of its package where it has one. A function in
// the file's own package spells bare, because a package never
// qualifies its own names.
func (t target) Callee(id symbol.Identity) (string, error) {
	t.use(id)
	if id.Package == "" || t.set != nil && id.Package == t.set.Home() {
		return id.Name, nil
	}
	return qualifier(id.Package) + qualifierSep + id.Name, nil
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

// use records the import a reference or a callee in another
// package needs. One in no package, a builtin, records nothing, and
// the set drops one in the file's own package.
func (t target) use(id symbol.Identity) {
	if id.Package == "" || t.set == nil {
		return
	}
	t.set.Add(id.Package)
}

// qualifier returns the name an import path binds by default: its
// last segment.
func qualifier(path string) string {
	return path[strings.LastIndex(path, "/")+1:]
}
