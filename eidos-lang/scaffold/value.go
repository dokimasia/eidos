// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package scaffold

import (
	"strings"

	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// trueSpelling and falseSpelling are the two truth values. Every C-family
// target spells them alike, and a boolean literal contains one of them.
const (
	trueSpelling  = "true"
	falseSpelling = "false"
)

// callArgs is the number of spelled arguments of a call value that the
// walk keeps on the stack. A call with more arguments allocates its list.
const callArgs = 8

// Entry is one field of a composite in the form of its value. Name is set
// for a named field, Key for an entry of a map, and neither for an
// element of a list. Value is the spelling of the field.
type Entry struct {
	Name  string
	Key   string
	Value string
}

// Target is the spelling of value trees in one language.
//
// [Value] walks the tree, because the tree has the same shape in every
// language: a conversion wraps one value, a composite lists its fields in
// order, and a call and a construction list their arguments. Every
// C-family target spells a call as callee(args), so the walk writes the
// call. The target spells every other form, and its method refuses a form
// that the language has no syntax for.
//
// A method that spells a reference or a callee records the import that
// the spelling needs, so a target is bound to the import set of one file.
//
// # Concurrency
//
// [Value] calls the methods of a target one at a time from the calling
// goroutine. A target that is bound to the import set of one file is not
// safe for concurrent use.
//
// # Allocation contract
//
// The allocations of each method add to those of the walk, which [Value]
// lists.
type Target interface {
	// Lang returns the name of the language, which opens every refusal
	// of the target.
	Lang() string
	// Literal spells one leaf: a number, the content of a string, a
	// truth value, the absent value, or raw text in a source language.
	// A target refuses raw text in another language. [Leaves] is the
	// spelling that the C-family targets share.
	Literal(v emit.Value) (string, error)
	// Type spells the reference of a value, and records its import. The
	// walk passes a named reference with its spelling, and a structural
	// reference with or without one. A target restates a structural
	// reference without a spelling from its children, which the
	// substitution of a type parameter leaves, or refuses it.
	Type(t *emit.TypeRef) (string, error)
	// Callee spells the declaration that a call or a name value refers
	// to, and records its import. The walk passes only an identity with a
	// name.
	Callee(id symbol.Identity) (string, error)
	// Conversion spells a value converted to a type. The walk passes the
	// reference beside its spelling, because a language whose conversion
	// depends on the form reads the form from the reference.
	Conversion(ref *emit.TypeRef, typ, inner string) (string, error)
	// Composite spells a composite of a type from its entries. The walk
	// passes the reference beside its spelling, because a record, a map
	// and a list are one form in the tree and three syntaxes in most
	// targets.
	Composite(ref *emit.TypeRef, typ string, entries []Entry) (string, error)
	// Address spells the address of a value. The walk passes the value
	// beside its spelling, because a language that takes the address of
	// some values alone reads the kind of the value. Go takes the address
	// of a composite literal alone.
	Address(inner emit.Value, spelled string) (string, error)
	// Construction spells a construction of a type from the spellings of
	// its arguments, such as new Set([1]) in TypeScript, and records the
	// import of the type. The walk passes the reference unspelled, because
	// a construction needs the type at run time, and a language that
	// imports a declaration for its type checker alone imports the type of
	// a construction for its value. A language without a construction
	// form refuses it.
	Construction(ref *emit.TypeRef, args []string) (string, error)
}

// Leaves is the spelling of the literal leaves of one target, in which the
// C-family targets differ. [Leaves.Literal] is the [Target.Literal] of the
// target.
//
// # Concurrency
//
// [Leaves.Literal] reads a Leaves and never writes it. A Leaves is safe
// for concurrent use when its Quote and Number functions are.
//
// # Allocation contract
//
// [Leaves.Literal] allocates nothing itself. Quote and Number allocate
// their spellings.
type Leaves struct {
	// Lang is the language of the target. Its name opens every refusal,
	// and raw text in any other language is refused.
	Lang symbol.Lang
	// Absent is the spelling of the absent value, such as "nil".
	Absent string
	// Quote spells a string literal in the grammar of the target.
	Quote func(text string) string
	// Number spells a number for the target. A nil Number writes the
	// text of the derivation.
	Number func(v emit.Value) (string, error)
}

// Literal spells one leaf. A number spells through [Leaves.Number], a
// string through [Leaves.Quote], a boolean as one of its two spellings,
// and the absent value as [Leaves.Absent]. Raw text spells only in the
// language of the target, because no target translates the source of
// another language.
//
// Error modes, each a [render.ValueError] under the language of the
// target:
//   - A number without text.
//   - A boolean whose text is neither true nor false.
//   - Raw text in another language.
//   - A literal kind that the vocabulary does not declare.
//
// # Allocation contract
//
// Literal allocates nothing on success beyond the allocations of Quote
// and Number. A refusal allocates its error.
func (l Leaves) Literal(v emit.Value) (string, error) {
	lang := string(l.Lang)
	switch v.Literal {
	case emit.LiteralInt, emit.LiteralFloat:
		if v.Text == "" {
			return "", render.RefuseValue(lang, "a %s literal has no text", v.Literal)
		}
		if l.Number == nil {
			return v.Text, nil
		}
		return l.Number(v)
	case emit.LiteralString:
		return l.Quote(v.Text), nil
	case emit.LiteralBool:
		if v.Text != trueSpelling && v.Text != falseSpelling {
			return "", render.RefuseValue(lang,
				"a boolean literal spells %s or %s, not %q", trueSpelling, falseSpelling, v.Text)
		}
		return v.Text, nil
	case emit.LiteralNil:
		return l.Absent, nil
	case emit.LiteralRaw:
		if v.Lang != l.Lang {
			return "", render.RefuseValue(lang, "%q is written in %s, not %s", v.Text, v.Lang, lang)
		}
		return v.Text, nil
	default:
		return "", render.RefuseValue(lang, "no spelling for the %s literal", v.Literal)
	}
}

// Value spells one value tree through a target. It walks the tree and
// passes each part to the target, so a language states its syntax and
// not the recursion. A composite lists its entries in the order of the
// value. A call spells as the callee and its arguments, which commas join
// in parentheses. A name spells as the target spells a callee. A
// construction passes its type unspelled and the spellings of its
// arguments to the target.
//
// Error modes, each a [render.ValueError]:
//   - A value kind that the vocabulary does not declare.
//   - A conversion or an address without an inner value.
//   - A conversion, a composite or a construction without a type, or with
//     a named type that has no spelling.
//   - A call or a name without an identity, or with an identity that has
//     no name.
//
// Value returns an error of a method of the target unchanged.
//
// # Allocation contract
//
// The walk allocates the list of entries that a composite with fields
// passes to [Target.Composite], and the list of the spelled arguments
// that a construction with arguments passes to [Target.Construction], one
// allocation each, because the argument of an interface method escapes. A
// call allocates its spelling, one allocation, and the list of its spelled
// arguments where it has more than eight. Literals, names, conversions and
// addresses allocate nothing in the walk. The methods of the target add
// their own allocations.
func Value(t Target, v emit.Value) (string, error) {
	switch v.Kind {
	case emit.ValueLiteral:
		return t.Literal(v)
	case emit.ValueConversion:
		typ, err := namedType(t, v, "a conversion")
		if err != nil {
			return "", err
		}
		inner, err := wrapped(t, v, "a conversion")
		if err != nil {
			return "", err
		}
		return t.Conversion(v.Type, typ, inner)
	case emit.ValueComposite:
		return composite(t, v)
	case emit.ValueCall:
		return valueCall(t, v)
	case emit.ValueAddress:
		inner, err := wrapped(t, v, "an address")
		if err != nil {
			return "", err
		}
		return t.Address(*v.Inner, inner)
	case emit.ValueName:
		return callee(t, v, "a name value")
	case emit.ValueConstruction:
		return construction(t, v)
	default:
		return "", render.RefuseValue(t.Lang(), "no spelling for the %s value", v.Kind)
	}
}

// construction spells a construction from its type and the spellings of its
// arguments, in the order of the value.
func construction(t Target, v emit.Value) (string, error) {
	if err := typed(t, v, "a construction"); err != nil {
		return "", err
	}
	args := make([]string, 0, len(v.Args))
	for _, arg := range v.Args {
		spelled, err := Value(t, arg)
		if err != nil {
			return "", err
		}
		args = append(args, spelled)
	}
	return t.Construction(v.Type, args)
}

// composite spells a composite from its type and its fields, in the order
// of the value. Each field keeps its form: a name, a key, or neither.
func composite(t Target, v emit.Value) (string, error) {
	typ, err := namedType(t, v, "a composite")
	if err != nil {
		return "", err
	}
	entries := make([]Entry, 0, len(v.Fields))
	for _, f := range v.Fields {
		spelled, err := Value(t, f.Value)
		if err != nil {
			return "", err
		}
		entry := Entry{Name: f.Name, Value: spelled}
		if f.Key != nil {
			if entry.Key, err = Value(t, *f.Key); err != nil {
				return "", err
			}
		}
		entries = append(entries, entry)
	}
	return t.Composite(v.Type, typ, entries)
}

// valueCall spells a call as the callee, an open parenthesis, the
// arguments with commas between them, and a close parenthesis. It writes
// them into one buffer of the size of the spelling.
func valueCall(t Target, v emit.Value) (string, error) {
	fn, err := callee(t, v, "a call value")
	if err != nil {
		return "", err
	}
	var stack [callArgs]string
	args := stack[:0]
	n := len(fn) + len("()")
	for i, arg := range v.Args {
		spelled, err := Value(t, arg)
		if err != nil {
			return "", err
		}
		if i > 0 {
			n += len(nameSep)
		}
		n += len(spelled)
		args = append(args, spelled)
	}
	var b strings.Builder
	b.Grow(n)
	b.WriteString(fn)
	b.WriteByte('(')
	for i, arg := range args {
		if i > 0 {
			b.WriteString(nameSep)
		}
		b.WriteString(arg)
	}
	b.WriteByte(')')
	return b.String(), nil
}

// callee spells the declaration that a call or a name value refers to. A
// target cannot invent the declaration, so callee refuses a value without
// an identity and an identity without a name. what is the value in the
// refusal.
func callee(t Target, v emit.Value, what string) (string, error) {
	switch {
	case v.Callee.IsZero():
		return "", render.RefuseValue(t.Lang(), "%s has no identity", what)
	case v.Callee.Name == "":
		return "", render.RefuseValue(t.Lang(), "the identity of %s has no name", what)
	}
	return t.Callee(v.Callee)
}

// namedType spells the type of a conversion or a composite through the
// target, after [typed] admits it.
func namedType(t Target, v emit.Value, what string) (string, error) {
	if err := typed(t, v, what); err != nil {
		return "", err
	}
	return t.Type(v.Type)
}

// typed refuses the type of a conversion, a composite or a construction
// that the target cannot spell. A language cannot invent a named type, so
// typed refuses a value without a type and a named reference without a
// spelling. A structural reference without a spelling passes to the
// target, which restates it from its children or refuses it. what is the
// value in the refusal.
func typed(t Target, v emit.Value, what string) error {
	switch {
	case v.Type == nil:
		return render.RefuseValue(t.Lang(), "%s value has no type", what)
	case v.Type.Spelling == "" && v.Type.Form == symbol.FormNamed:
		return render.RefuseValue(t.Lang(), "the type of a value has no spelling")
	}
	return nil
}

// wrapped spells the inner value of a conversion or an address.
func wrapped(t Target, v emit.Value, what string) (string, error) {
	if v.Inner == nil {
		return "", render.RefuseValue(t.Lang(), "%s value has no inner value", what)
	}
	return Value(t, *v.Inner)
}
