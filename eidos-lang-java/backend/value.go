// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"strconv"
	"strings"

	java "go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/lang/scaffold"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The spellings Java's value forms are written with.
const (
	// nullSpelling is Java's absent value, which every reference
	// type takes.
	nullSpelling = "null"
	// trueSpelling and falseSpelling are the two truth values.
	trueSpelling  = "true"
	falseSpelling = "false"
	// listFactory and mapFactory build the two immutable
	// collections Java states a factory for.
	listFactory = "List.of("
	mapFactory  = "Map.of("
	// collectionsPackage is where those factories live.
	collectionsPackage = "java/util"
)

// target spells a value tree as Java, recording into the file's
// import set whatever its references and callees need.
type target struct{ set *render.ImportSet }

// Lang names the target for a refusal.
func (target) Lang() string { return string(java.Lang) }

// Literal spells one leaf. A number carries its own text; a string
// quotes through the standard library, whose escapes Java reads
// the same way; a boolean takes exactly the two spellings; the
// absent value is null. Raw text spells only where the author
// wrote it in Java.
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
		return nullSpelling, nil
	case emit.LiteralRaw:
		if v.Lang != java.Lang {
			return "", render.RefuseValue(t.Lang(),
				"%q is written in %s, which Java cannot spell", v.Text, v.Lang)
		}
		return v.Text, nil
	default:
		return "", render.RefuseValue(t.Lang(), "no spelling for the %s literal", v.Literal)
	}
}

// Type spells a reference and records the import its package
// needs.
func (t target) Type(ref *emit.TypeRef) (string, error) {
	if ref == nil || ref.Spelling == "" {
		return "", render.RefuseValue(t.Lang(), "a value names a type that spells nothing")
	}
	t.use(ref.Target)
	return Spell(ref), nil
}

// Callee spells a function by name and records its import: an
// imported class resolves bare, so nothing qualifies at the call.
func (t target) Callee(id symbol.Identity) (string, error) {
	if id.Name == "" {
		return "", render.RefuseValue(t.Lang(), "a call names a function that spells nothing")
	}
	t.use(id)
	return id.Name, nil
}

// Conversion spells a cast, which is Java's one conversion form.
func (target) Conversion(_ *emit.TypeRef, typ, inner string) (string, error) {
	return "(" + typ + ") " + inner, nil
}

// Composite spells the two collection factories for a list and a
// map, and a constructor call for a record. A record whose value
// names its fields refuses: a Java constructor takes every
// component positionally and in order, so a value setting some of
// them cannot be spelled without inventing the rest.
func (t target) Composite(
	ref *emit.TypeRef, typ string, entries []scaffold.Entry,
) (string, error) {
	parts := make([]string, 0, len(entries)*2)
	if ref != nil && (ref.Form == symbol.FormList || ref.Form == symbol.FormArray) {
		for _, e := range entries {
			parts = append(parts, e.Value)
		}
		t.useCollections()
		return listFactory + strings.Join(parts, ", ") + ")", nil
	}
	if ref != nil && ref.Form == symbol.FormMap {
		for _, e := range entries {
			if e.Key == "" {
				return "", render.RefuseValue(t.Lang(),
					"a map value carries an entry with no key")
			}
			parts = append(parts, e.Key, e.Value)
		}
		t.useCollections()
		return mapFactory + strings.Join(parts, ", ") + ")", nil
	}
	for _, e := range entries {
		if e.Name != "" {
			return "", render.RefuseValue(t.Lang(),
				"a %s value names the field %s, and a Java constructor takes every "+
					"component positionally", typ, e.Name)
		}
		parts = append(parts, e.Value)
	}
	return "new " + typ + "(" + strings.Join(parts, ", ") + ")", nil
}

// Call spells an application.
func (target) Call(callee string, args []string) (string, error) {
	return callee + "(" + strings.Join(args, ", ") + ")", nil
}

// Address refuses: Java has no address operator, and a reference
// to a value is the value.
func (t target) Address(string) (string, error) {
	return "", render.RefuseValue(t.Lang(), "Java spells no address of a value")
}

// useCollections records the import the two collection factories
// need.
func (t target) useCollections() {
	if t.set == nil {
		return
	}
	t.set.Add(collectionsPackage)
}

// use records the import a reference or a callee in another
// package needs.
func (t target) use(id symbol.Identity) {
	if id.Package == "" || id.Name == "" || t.set == nil {
		return
	}
	t.set.AddNamed(id.Package, id.Name)
}
