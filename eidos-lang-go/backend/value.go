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

// The spellings Go's value forms are written with.
const (
	// nilSpelling is Go's absent value, which every reference type
	// takes.
	nilSpelling = "nil"
	// trueSpelling and falseSpelling are Go's two truth values,
	// the only texts a boolean literal may carry.
	trueSpelling  = "true"
	falseSpelling = "false"
	// qualifierSep separates a package qualifier from the name it
	// qualifies.
	qualifierSep = "."
)

// target spells a value tree as Go, recording into the file's
// import set whatever its references and callees need. It is bound
// to one file, because the set is.
type target struct{ set *render.ImportSet }

// Lang names the target for a refusal.
func (target) Lang() string { return string(golang.Lang) }

// Literal spells one leaf. A number carries its own text, which
// the derivation wrote in Go's own spelling; a string quotes
// through the standard library, so an escape is Go's; a boolean
// takes exactly the two spellings; the absent value is nil. Raw
// text spells only where the author wrote it in Go, because
// nothing translates another language's source.
func (t target) Literal(v emit.Value) (string, error) {
	switch v.Literal {
	case emit.LiteralInt, emit.LiteralFloat:
		if v.Text == "" {
			return "", render.RefuseValue(t.Lang(), "a %s literal carries no text", v.Literal)
		}
		return v.Text, nil
	case emit.LiteralString:
		return strconv.Quote(v.Text), nil
	case emit.LiteralBool:
		if v.Text != trueSpelling && v.Text != falseSpelling {
			return "", render.RefuseValue(t.Lang(),
				"a boolean literal spells %s or %s, not %q", trueSpelling, falseSpelling, v.Text)
		}
		return v.Text, nil
	case emit.LiteralNil:
		return nilSpelling, nil
	case emit.LiteralRaw:
		if v.Lang != golang.Lang {
			return "", render.RefuseValue(t.Lang(),
				"%q is written in %s, which Go cannot spell", v.Text, v.Lang)
		}
		return v.Text, nil
	default:
		return "", render.RefuseValue(t.Lang(), "no spelling for the %s literal", v.Literal)
	}
}

// Type spells a reference and records the import its package
// needs. The spelling is the reference's own, which is what a Go
// graph rendering back to Go states.
func (t target) Type(ref *emit.TypeRef) (string, error) {
	if ref == nil || ref.Spelling == "" {
		return "", render.RefuseValue(t.Lang(), "a value names a type that spells nothing")
	}
	t.use(ref.Target)
	return Spell(ref), nil
}

// Callee spells a function by its identity and records its import:
// a call carries no source spelling of its own, so the name is
// qualified by the last segment of its package where it has one.
// A function in the file's own package spells bare, because a
// package never qualifies its own names.
func (t target) Callee(id symbol.Identity) (string, error) {
	if id.Name == "" {
		return "", render.RefuseValue(t.Lang(), "a call names a function that spells nothing")
	}
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

// Call spells an application.
func (target) Call(callee string, args []string) (string, error) {
	return callee + "(" + strings.Join(args, ", ") + ")", nil
}

// Address spells Go's address operator, which a composite takes
// directly.
func (target) Address(inner string) (string, error) { return "&" + inner, nil }

// use records the import a reference or a callee in another
// package needs; one in no package, a builtin, records nothing,
// and the set drops one in the file's own package.
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
