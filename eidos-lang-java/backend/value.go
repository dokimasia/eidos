// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"fmt"
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
	// listClass and mapClass are the two collections Java states an
	// immutable factory for, and collectionsPackage is where they
	// live.
	listClass          = "List"
	mapClass           = "Map"
	collectionsPackage = "java/util"
	// factory is the static method both collections build through.
	factory = ".of("
	// memberSep joins a class and its static member.
	memberSep = "."
)

// target spells a value tree as Java, recording into the file's
// import set whatever its references and callees need.
type target struct{ set *render.ImportSet }

// Lang names the target for a refusal.
func (target) Lang() string { return string(java.Lang) }

// Literal spells one leaf. A number carries its own text; a string
// quotes in Java's own grammar; a boolean takes exactly the two
// spellings; the absent value is null. Raw text spells only where
// the author wrote it in Java.
func (t target) Literal(v emit.Value) (string, error) {
	switch v.Literal {
	case emit.LiteralInt, emit.LiteralFloat:
		if v.Text == "" {
			return "", render.RefuseValue(t.Lang(), "a %s literal carries no text", v.Literal)
		}
		return v.Text, nil
	case emit.LiteralString:
		return quoteString(v.Text), nil
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

// Callee spells a function as Owner.name, the static method of the
// class its identity's Owner names, and imports that class. A
// function whose identity names no Owner refuses, because Java has
// no free function to call.
func (t target) Callee(id symbol.Identity) (string, error) {
	if id.Name == "" {
		return "", render.RefuseValue(t.Lang(), "a call names a function that spells nothing")
	}
	if id.Owner == "" {
		return "", render.RefuseValue(t.Lang(),
			"%s is owned by no class, and Java calls no free function", id.Name)
	}
	t.use(id)
	return id.Owner + memberSep + id.Name, nil
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
		t.useCollection(listClass)
		return listClass + factory + strings.Join(parts, ", ") + ")", nil
	}
	if ref != nil && ref.Form == symbol.FormMap {
		for _, e := range entries {
			if e.Key == "" {
				return "", render.RefuseValue(t.Lang(),
					"a map value carries an entry with no key")
			}
			parts = append(parts, e.Key, e.Value)
		}
		t.useCollection(mapClass)
		return mapClass + factory + strings.Join(parts, ", ") + ")", nil
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

// useCollection records the import of the collection class a
// factory call names.
func (t target) useCollection(class string) {
	if t.set == nil {
		return
	}
	t.set.AddNamed(collectionsPackage, class)
}

// use records the import a reference or a callee in another
// package needs: the file-level class, which is the owner chain's
// first name where the declaration nests or belongs to a class.
func (t target) use(id symbol.Identity) {
	if id.Package == "" || id.Name == "" || t.set == nil {
		return
	}
	class := id.Name
	if id.Owner != "" {
		class, _, _ = strings.Cut(id.Owner, memberSep)
	}
	t.set.AddNamed(id.Package, class)
}

// quoteString spells a string literal in Java's grammar: the
// backslash, the double quote and the named control escapes \b \t
// \n \f \r, every other control character as an octal escape, and
// everything else as itself. No \u escape is written: Java decodes
// one before it reads the literal, and a decoded line break ends
// the string.
func quoteString(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\b':
			b.WriteString(`\b`)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		case '\f':
			b.WriteString(`\f`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if r < ' ' || r == 0x7f {
				fmt.Fprintf(&b, `\%03o`, r)
				continue
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
