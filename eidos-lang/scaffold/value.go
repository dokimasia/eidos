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

// callArgs is how many spelled arguments of a call value the walk
// keeps on the stack. A call with more arguments allocates its list.
const callArgs = 8

// Entry is one settled field of a composite, in the form the
// value stated it: a named field sets Name, a map's entry sets Key,
// and a list's element sets neither. Value is the field's spelling.
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
//
// # Concurrency
//
// [Value] calls a target's methods one at a time from the calling
// goroutine. A target bound to one file's import set is not safe for
// concurrent use.
//
// # Allocation contract
//
// Each method's allocations add to the walk's own, which [Value]
// lists.
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
//
// # Concurrency
//
// [Leaves.Literal] reads a Leaves and never writes it. A Leaves is
// safe for concurrent use when its Quote and Number functions are.
//
// # Allocation contract
//
// [Leaves.Literal] allocates nothing itself. Quote and Number allocate
// what their spellings need.
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
// because nothing translates another language's source.
//
// Error modes, each a [render.ValueError] that names the target's
// language:
//   - A number without text.
//   - A boolean whose text is neither true nor false.
//   - Raw text written in another language.
//   - A literal kind the vocabulary does not declare.
//
// # Allocation contract
//
// Literal allocates nothing on success beyond what Quote and Number
// allocate. A refusal allocates its error.
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

// Value spells one value tree through a target: it walks the tree
// and hands each spelling to the target, so a language states its
// syntax once and never its recursion. A composite lists its entries
// in the order the value states them, and a call spells as the callee,
// its arguments comma-joined in parentheses.
//
// Error modes, each a [render.ValueError] that names what is missing:
//   - A value kind the vocabulary does not declare.
//   - A conversion or an address that wraps nothing.
//   - A conversion or a composite that names no type, or a type that
//     spells nothing.
//   - A call that names no callee, or a callee without a name.
//
// An error a target method returns passes through unchanged.
//
// # Allocation contract
//
// The walk allocates the list of entries a composite with fields
// passes to [Target.Composite], one allocation, because an interface
// method's argument escapes. A call allocates its spelling, one
// allocation, and the list of its spelled arguments where it has more
// than eight. Literals, conversions and addresses allocate nothing in
// the walk. The target's methods add their own allocations.
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
// and a close, written into one buffer sized to the spelling.
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
	var stack [callArgs]string
	args := stack[:0]
	n := len(callee) + len("()")
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
	b.WriteString(callee)
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
