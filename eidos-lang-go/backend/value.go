// Copyright Dokimasia B.V. 2026
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

// Callee spells the declaration of a call or a name value by its
// identity, and binds the import of its package through [bind], as a type
// reference binds one. The value has no source spelling of its own, so
// the name of the import qualifies the name of the declaration. Where
// another import or a declaration of the file takes that name, the import
// binds a numbered name. A declaration in no package, and one in the
// package of the file, spells bare. Callee returns the error of bind for a
// package that the go command refuses to import.
func (t target) Callee(id symbol.Identity) (string, error) {
	if id.Package == "" {
		return id.Name, nil
	}
	local, err := bind(t.set, id.Package)
	if err != nil {
		return "", err
	}
	if local == "" {
		return id.Name, nil
	}
	return local + qualifierSep + id.Name, nil
}

// Conversion spells Go's conversion, which every named type takes. A type
// that starts with the operator * or <-, or with the keyword func, is in
// parentheses, because Go reads *T(x) as the dereference of T(x).
func (target) Conversion(_ *emit.TypeRef, typ, inner string) (string, error) {
	if strings.HasPrefix(typ, pointerMark) || strings.HasPrefix(typ, receiveMark) || strings.HasPrefix(typ, funcOpen) {
		typ = "(" + typ + ")"
	}
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

// Construction refuses: Go builds a value through a composite literal or
// the call of a function, and has no expression that constructs a type
// from arguments.
func (t target) Construction(ref *emit.TypeRef, _ []string) (string, error) {
	return "", render.RefuseValue(t.Lang(),
		"Go builds a value through a composite literal or a call, and has no construction of %s", ref.Spelling)
}

// Func spells a function literal of a function type, as
// func(context.Context) error { return nil }. It writes the signature
// through the file's [Speller], so the file imports each type of the
// signature. The body returns the results, and a function without results
// has an empty body, as func(string) {}. The literal has the parameter
// names of the type. A type that is not a function type refuses, because
// Go writes a literal of a defined function type through a conversion of
// the literal of its signature.
func (t target) Func(ref *emit.TypeRef, results []string) (string, error) {
	if ref.Form != symbol.FormFunc {
		return "", render.RefuseValue(t.Lang(),
			"Go writes a function literal of a function type, and %s is not one", ref.Spelling)
	}
	sig, err := NewSpeller(t.set).Spell(ref)
	if err != nil {
		return "", err
	}
	if len(results) == 0 {
		return sig + " {}", nil
	}
	return sig + " { return " + strings.Join(results, ", ") + " }", nil
}

// Make spells make of a type, which Go writes for a channel of every
// direction, a map and a slice: make(chan string). The type is in the
// spelling of the file, which records its imports.
func (target) Make(_ *emit.TypeRef, typ string) (string, error) {
	return "make(" + typ + ")", nil
}

// Value spells one value of the value tree as Go, through a target that
// binds in set the packages that the references and the callees of the
// value name. The value builtin calls it, so a template writes a value as
// a body of the same file writes it. A value that Go has no form for
// returns a [render.ValueError].
//
// Value allocates what [scaffold.Value] allocates: the text of each part
// of the value that it joins, and the qualified name of a callee of
// another package. A literal does not allocate.
func Value(v emit.Value, set *render.ImportSet) (string, error) {
	return scaffold.Value(target{set: set}, v)
}
