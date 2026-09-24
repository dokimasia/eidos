// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package scaffold

import (
	"strings"

	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// trueSpelling and falseSpelling are the two truth values every
// C-family target spells alike, the only texts a boolean literal
// may contain.
const (
	trueSpelling  = "true"
	falseSpelling = "false"
)

// Entry is one settled field of a composite, in the form the
// value stated it: a named field sets Name, a map's entry sets Key,
// and a list's element sets neither.
type Entry struct {
	Name  string
	Key   string
	Value string
}

// Target is what one language states about spelling a value tree.
//
// The walk is shared, because the tree's shape is the same wherever
// it is written: a conversion wraps one value, a composite lists its
// fields in order, and a call lists its arguments. A call spells as
// callee(args) in every C-family target, so the walk writes it. Every
// other spelling is the target's own, and a form the target has no
// syntax for is refused by the target's method.
//
// A method that spells a reference or a callee records the import the
// spelling needs, so a target is bound to one file's import set.
type Target interface {
	// Lang names the target, so a refusal reads the way every
	// other refusal in that language does.
	Lang() string
	// Literal spells one leaf: a number, a string's content, a truth
	// value, the absent value, or raw text in a source language,
	// which a target refuses unless the language is its own. [Leaves]
	// is the spelling the C-family targets share.
	Literal(v emit.Value) (string, error)
	// Type spells a reference a value names, and records its import.
	// The walk passes only a reference that spells.
	Type(t *emit.TypeRef) (string, error)
	// Callee spells a function a value calls, and records its import.
	// The walk passes only an identity with a name.
	Callee(id symbol.Identity) (string, error)
	// Conversion spells a value converted to a type. The reference is
	// passed beside its spelling, because a language whose conversion
	// depends on the form reads the form from the reference.
	Conversion(ref *emit.TypeRef, typ, inner string) (string, error)
	// Composite spells a composite of a type from its entries, the
	// reference beside its spelling for the same reason: a record, a
	// map and a list are one form in the tree and three syntaxes in
	// most targets.
	Composite(ref *emit.TypeRef, typ string, entries []Entry) (string, error)
	// Address spells the address of a value from the value and its
	// spelling. The value is passed beside the spelling, because a
	// language that takes the address of some values alone, such as
	// Go's composite literals, reads the value's kind.
	Address(inner emit.Value, spelled string) (string, error)
}

// Leaves is one target's spelling of the literal leaves: the parts in
// which the C-family targets differ. Its [Leaves.Literal] method is
// the target's [Target.Literal].
type Leaves struct {
	// Lang is the target's language. Its name opens every refusal,
	// and raw text written in any other language is refused.
	Lang symbol.Lang
	// Absent spells the absent value, such as "nil".
	Absent string
	// Quote spells a string literal in the target's grammar.
	Quote func(text string) string
	// Number spells a number for the target. Nil writes the text the
	// derivation wrote.
	Number func(v emit.Value) (string, error)
}

// Literal spells one leaf. A number spells through [Leaves.Number],
// a string quotes through [Leaves.Quote], a boolean takes exactly the
// two spellings, and the absent value is [Leaves.Absent]. Raw text
// spells only where the author wrote it in the target's language,
// because nothing translates another language's source. Every
// refusal is a [render.ValueError].
func (l Leaves) Literal(v emit.Value) (string, error) {
	lang := string(l.Lang)
	switch v.Literal {
	case emit.LiteralInt, emit.LiteralFloat:
		if v.Text == "" {
			return "", render.RefuseValue(lang, "a %s literal carries no text", v.Literal)
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

// Value spells one value tree through a target: it walks the tree
// and hands each spelling to the target, so a language states its
// syntax once and never its recursion.
//
// The walk
// refuses a value the vocabulary does not declare, a conversion or an
// address wrapping nothing, a composite or a conversion naming no
// type or a type that spells nothing, and a call naming no callee or
// a callee without a name. Each refusal names what is missing and is
// a [render.ValueError], so the render reports it under its value
// code.
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
	default:
		return "", render.RefuseValue(t.Lang(), "no spelling for the %s value", v.Kind)
	}
}

// composite spells a composite from its type and its fields, in
// the order the value states them, each in the form it states: a
// name, a key, or neither.
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

// valueCall spells an application of a callee to its arguments:
// the callee, an open parenthesis, the arguments joined with commas,
// and a close.
func valueCall(t Target, v emit.Value) (string, error) {
	switch {
	case v.Callee.IsZero():
		return "", render.RefuseValue(t.Lang(), "a call value names no callee")
	case v.Callee.Name == "":
		return "", render.RefuseValue(t.Lang(), "a call names a function that spells nothing")
	}
	callee, err := t.Callee(v.Callee)
	if err != nil {
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
	return callee + "(" + strings.Join(args, nameSep) + ")", nil
}

// namedType spells the type a value names. A conversion and a
// composite both need one, and a language cannot invent it, so a
// value naming no type, or a type that spells nothing, is refused.
func namedType(t Target, v emit.Value, what string) (string, error) {
	switch {
	case v.Type == nil:
		return "", render.RefuseValue(t.Lang(), "%s value names no type", what)
	case v.Type.Spelling == "":
		return "", render.RefuseValue(t.Lang(), "a value names a type that spells nothing")
	}
	return t.Type(v.Type)
}

// wrapped spells the one value a conversion or an address wraps.
func wrapped(t Target, v emit.Value, what string) (string, error) {
	if v.Inner == nil {
		return "", render.RefuseValue(t.Lang(), "%s value wraps nothing", what)
	}
	return Value(t, *v.Inner)
}
