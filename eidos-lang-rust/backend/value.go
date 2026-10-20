// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/lang/scaffold"
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
	// emptyBraces closes a struct literal that names no field, which
	// Rust accepts for every struct that has no field.
	emptyBraces = " {}"
	// mapFrom is the function of a map type of the standard library that
	// builds a map from an array of pairs.
	mapFrom = "from"
	// closureOpen and closureClose enclose the parameters of a closure,
	// and closureParam is a parameter that the body of the closure does
	// not read.
	closureOpen  = "|"
	closureClose = "| "
	closureParam = "_"
	// emptyBlock is the body of a closure without results, and listSep
	// separates the parameters of a closure and the results of a closure
	// that returns a tuple.
	emptyBlock = "{}"
	listSep    = ", "
	// unicodeOpen opens the braced Unicode escape of a control
	// character, and unicodeDigits is the most hex digits a Unicode
	// scalar value takes.
	unicodeOpen   = `\u{`
	unicodeDigits = 6
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

// target spells a value tree as Rust, using through the file's
// import set what its references and callees name.
type target struct{ set *render.ImportSet }

// Lang names the target for a refusal.
func (target) Lang() string { return string(rust.Lang) }

// Literal spells one leaf through [leaves].
func (target) Literal(v emit.Value) (string, error) { return leaves.Literal(v) }

// Type spells a reference in an expression's path through
// [Speller.SpellValue] of the file, so a value and a declaration use one
// item under one name, and a struct literal of Pair<Vec<i32>> spells
// Pair::<Vec<i32>> { .. }. A reference that states no type, itself or in
// an argument list, refuses. A structural reference without a spelling,
// which the substitution of a type parameter leaves, states no type
// either, because the spelling of a Rust composite does not follow from
// its structure.
func (t target) Type(ref *emit.TypeRef) (string, error) {
	if !complete(ref) {
		return "", render.RefuseValue(t.Lang(), unstatedType)
	}
	return NewSpeller(t.set).SpellValue(ref)
}

// Callee spells the function of a call or the item of a name value under
// the name that its use binds. The use binds the name, so nothing
// qualifies it in the value. A member of a type, such as a variant of an
// enum, spells its path through its owner, as Color::Red, and an associated
// item whose name holds its path spells it, as Duration::from_secs. The use
// binds the first segment of the path, the type. A declaration in no
// module, and an identity without a name, spell their own path.
func (t target) Callee(id symbol.Identity) (string, error) {
	path := id.Name
	if id.Owner != "" {
		path = strings.ReplaceAll(id.Owner, symbol.OwnerSep, pathSep) + pathSep + id.Name
	}
	if id.Package == "" || id.Name == "" {
		return path, nil
	}
	head, rest, nested := strings.Cut(path, pathSep)
	bound := t.set.BindItem(id.Package, head, false)
	if !nested {
		return bound, nil
	}
	return bound + pathSep + rest, nil
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

// Composite spells the literal of a composite by its entries:
//
//   - keyed entries are a map, which Rust builds from an array of pairs
//     through the function from of the map type, as
//     HashMap::<String, i32>::from([("a".to_string(), 1)]);
//   - the elements of a list are a vector literal, and the elements of an
//     array an array expression;
//   - the elements of a tuple are a tuple expression, with the comma that a
//     tuple of one member needs;
//   - the elements of any other type are the literal of a tuple struct, as
//     Point(1, 2);
//   - named fields are a struct literal. A struct literal that names no
//     field closes on empty braces, which Rust accepts for every struct
//     without fields, a unit struct included.
//
// A composite that mixes keyed entries, elements and named fields is
// refused, because no literal of Rust has two of them.
func (t target) Composite(
	ref *emit.TypeRef, typ string, entries []scaffold.Entry,
) (string, error) {
	keys, names := 0, 0
	for _, e := range entries {
		switch {
		case e.Key != "":
			keys++
		case e.Name != "":
			names++
		}
	}
	parts := make([]string, 0, len(entries))
	switch {
	case keys > 0 && keys < len(entries), names > 0 && names < len(entries):
		return "", render.RefuseValue(t.Lang(),
			"a %s value mixes the forms of its entries, and a literal of Rust has one form", typ)
	case keys > 0:
		for _, e := range entries {
			parts = append(parts, "("+e.Key+", "+e.Value+")")
		}
		return typ + pathSep + mapFrom + "([" + strings.Join(parts, ", ") + "])", nil
	case names > 0:
		for _, e := range entries {
			parts = append(parts, e.Name+": "+e.Value)
		}
		return typ + " { " + strings.Join(parts, ", ") + " }", nil
	}
	for _, e := range entries {
		parts = append(parts, e.Value)
	}
	elements := strings.Join(parts, ", ")
	switch {
	case ref != nil && ref.Form == symbol.FormArray:
		return "[" + elements + "]", nil
	case ref != nil && ref.Form == symbol.FormList:
		return vecMacro + elements + "]", nil
	case ref != nil && ref.Form == symbol.FormTuple && len(parts) == 1:
		return "(" + elements + ",)", nil
	case ref != nil && ref.Form == symbol.FormTuple:
		return "(" + elements + ")", nil
	case len(parts) == 0:
		return typ + emptyBraces, nil
	default:
		return typ + "(" + elements + ")", nil
	}
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

// Construction refuses: Rust builds a value through a struct literal or a
// call of an associated function, such as HashSet::from, and has no
// expression that constructs a type from arguments.
func (t target) Construction(ref *emit.TypeRef, _ []string) (string, error) {
	return "", render.RefuseValue(t.Lang(),
		"Rust builds a value through a struct literal or a call, and has no construction of %s", ref.Spelling)
}

// Func spells a closure of a function type, as |_| 0. It writes a
// wildcard pattern for each parameter of the type. The body of the
// closure is the result, and a function without results has an empty
// block, as |_, _| {}. A function of two or more results returns them as
// a tuple, because its function type returns a tuple. A closure without captures coerces to a function pointer. Func
// does not write a type, so the file does not use anything for the
// closure. A reference that is not a function type refuses, such as the
// Box of a trait object, whose value is a call that boxes a closure.
func (t target) Func(ref *emit.TypeRef, results []string) (string, error) {
	if ref.Form != symbol.FormFunc {
		return "", render.RefuseValue(t.Lang(),
			"Rust writes a closure of a function type, and %s is not one", ref.Spelling)
	}
	params := max(ref.Split, 0)
	n := len(closureOpen) + params*(len(closureParam)+len(listSep)) + len(closureClose) + len(emptyBlock) + len("()")
	for _, r := range results {
		n += len(r) + len(listSep)
	}
	var b strings.Builder
	b.Grow(n)
	b.WriteString(closureOpen)
	for i := range params {
		if i > 0 {
			b.WriteString(listSep)
		}
		b.WriteString(closureParam)
	}
	b.WriteString(closureClose)
	switch len(results) {
	case 0:
		b.WriteString(emptyBlock)
	case 1:
		b.WriteString(results[0])
	default:
		b.WriteByte('(')
		for i, r := range results {
			if i > 0 {
				b.WriteString(listSep)
			}
			b.WriteString(r)
		}
		b.WriteByte(')')
	}
	return b.String(), nil
}

// Make refuses: Rust has no make.
func (t target) Make(_ *emit.TypeRef, typ string) (string, error) {
	return "", render.RefuseValue(t.Lang(), "Rust has no make of %s", typ)
}

// Value spells one value of the value tree as Rust, through a target that
// uses in set the items that the references and the callees of the value
// name. The value builtin calls it, so a template writes a value as a
// body of the same file writes it. A value that Rust has no form for
// returns a [render.ValueError].
//
// Value allocates what [scaffold.Value] allocates: the text of each part
// of the value that it joins. A literal does not allocate.
func Value(v emit.Value, set *render.ImportSet) (string, error) {
	return scaffold.Value(target{set: set}, v)
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
// everything else as itself. It sizes its buffer for the string and
// its quotes, one allocation, and an escape grows it once more.
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
				var scratch [unicodeDigits]byte
				b.WriteString(unicodeOpen)
				b.Write(strconv.AppendUint(scratch[:0], uint64(r), 16))
				b.WriteByte('}')
				continue
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
