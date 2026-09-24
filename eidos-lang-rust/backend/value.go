// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"fmt"
	"strings"

	rust "go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/lang/scaffold"
	"go.dokimi.dev/eidos/lang/spellref"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The spellings Rust's value forms are written with.
const (
	// vecMacro opens a vector literal, which is how Rust spells a
	// list of values.
	vecMacro = "vec!["
	// floatPoint completes a float literal written without a
	// fraction.
	floatPoint = ".0"
	// castOp converts a number to another numeric type.
	castOp = " as "
	// turbofishOpener opens an argument list in an expression's
	// path.
	turbofishOpener = "::<"
	// emptyBraces closes a struct literal that names no field, which
	// Rust accepts for every struct that has no field.
	emptyBraces = " {}"
)

// leaves is Rust's spelling of the literal leaves: a number spells
// through [number], a string quotes in Rust's own grammar, and the
// absent value is None, which an Option takes.
var leaves = scaffold.Leaves{
	Lang:   rust.Lang,
	Absent: "None",
	Quote:  quoteString,
	Number: number,
}

// target spells a value tree as Rust, recording into the file's
// import set the paths its references and callees need.
type target struct{ set *render.ImportSet }

// Lang names the target for a refusal.
func (target) Lang() string { return string(rust.Lang) }

// Literal spells one leaf through [leaves].
func (target) Literal(v emit.Value) (string, error) { return leaves.Literal(v) }

// Type spells a reference in an expression's path and records the
// use its module needs. Rust opens the argument list there with the
// turbofish, so a struct literal of Pair<Vec<i32>> spells
// Pair::<Vec<i32>> { .. }, and the nested arguments, which are in
// type position, keep their angle brackets. A reference that states
// no type, itself or in an argument list, refuses.
func (t target) Type(ref *emit.TypeRef) (string, error) {
	if !complete(ref) {
		return "", render.RefuseValue(t.Lang(), unstatedType)
	}
	t.use(ref.Target)
	if len(ref.Args) == 0 {
		return ref.Spelling, nil
	}
	args := make([]string, 0, len(ref.Args))
	for _, a := range ref.Args {
		args = append(args, spellref.Spell(a, genericsOpener, genericsCloser, unitSpelling))
	}
	return ref.Spelling + turbofishOpener + strings.Join(args, ", ") + genericsCloser, nil
}

// Callee spells a function by name and records its use: a used
// path binds the name, so nothing qualifies at the call.
func (t target) Callee(id symbol.Identity) (string, error) {
	t.use(id)
	return id.Name, nil
}

// Conversion spells a value converted to a type: a cast to a numeric
// primitive, which is how Rust converts a number to another numeric
// type, and a tuple-struct construction for a declared type, which
// is what a defined type over one value is in Rust. A conversion to
// bool, char, str or String refuses, because Rust converts to those
// through a trait or a method, which the vocabulary does not express.
func (t target) Conversion(ref *emit.TypeRef, typ, inner string) (string, error) {
	if numeric, builtin := builtinTypes[bareName(ref)]; builtin {
		if !numeric {
			return "", render.RefuseValue(t.Lang(),
				"Rust converts to %s through a trait or a method, and a value converts to it", typ)
		}
		return inner + castOp + typ, nil
	}
	return typ + "(" + inner + ")", nil
}

// Composite spells a struct literal for a record and a vector
// literal for a list. A struct literal that names no field closes on
// empty braces, which Rust accepts for every struct without fields,
// a unit struct included. A map is refused: Rust states no map
// literal, and a collection built from an array of pairs is a call
// the vocabulary does not express.
func (t target) Composite(
	ref *emit.TypeRef, typ string, entries []scaffold.Entry,
) (string, error) {
	for _, e := range entries {
		if e.Key != "" {
			return "", render.RefuseValue(t.Lang(),
				"Rust spells no map literal, and a %s value has a keyed entry", typ)
		}
	}
	if ref != nil && (ref.Form == symbol.FormList || ref.Form == symbol.FormArray) {
		parts := make([]string, 0, len(entries))
		for _, e := range entries {
			parts = append(parts, e.Value)
		}
		if ref.Form == symbol.FormArray {
			return "[" + strings.Join(parts, ", ") + "]", nil
		}
		return vecMacro + strings.Join(parts, ", ") + "]", nil
	}
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Name == "" {
			return "", render.RefuseValue(t.Lang(),
				"a %s value has a positional element, and a struct literal names every field", typ)
		}
		parts = append(parts, e.Name+": "+e.Value)
	}
	if len(parts) == 0 {
		return typ + emptyBraces, nil
	}
	return typ + " { " + strings.Join(parts, ", ") + " }", nil
}

// Address spells a shared borrow, which is Rust's address of a
// value of any kind. A cast borrows in parentheses, because & binds
// tighter than as.
func (target) Address(inner emit.Value, spelled string) (string, error) {
	if inner.Kind == emit.ValueConversion && builtinTypes[bareName(inner.Type)] {
		return "&(" + spelled + ")", nil
	}
	return "&" + spelled, nil
}

// use records the use a reference or a callee in another module
// needs.
func (t target) use(id symbol.Identity) {
	if id.Package == "" || id.Name == "" || t.set == nil {
		return
	}
	t.set.AddNamed(id.Package, id.Name)
}

// number spells a number for Rust. A float written without a
// fraction or an exponent takes ".0", because Rust reads "0" as an
// integer literal and refuses it where a float is expected. Every
// other number keeps the text the derivation wrote.
func number(v emit.Value) (string, error) {
	if v.Literal == emit.LiteralFloat && !strings.ContainsAny(v.Text, ".eE") {
		return v.Text + floatPoint, nil
	}
	return v.Text, nil
}

// quoteString spells a string literal in Rust's grammar: the
// backslash, the double quote and the named escapes \n \r \t \0,
// every other control character as a braced Unicode escape, and
// everything else as itself.
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
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case 0:
			b.WriteString(`\0`)
		default:
			if r < ' ' || r == 0x7f {
				fmt.Fprintf(&b, `\u{%x}`, r)
				continue
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
