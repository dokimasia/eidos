// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package scaffold

import (
	"fmt"

	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Entry is one settled field of a composite, in the form the
// value stated it: a named field carries Name, a map's entry
// carries Key, and a list's element carries neither.
type Entry struct {
	Name  string
	Key   string
	Value string
}

// Target is what one language states about spelling a value tree.
//
// The walk is shared, because the tree's shape is the same
// wherever it is written: a conversion holds one value, a
// composite holds its fields in order, a call holds its arguments.
// Every spelling below the walk is the target's own, and a form
// the target has no syntax for refuses through its own method
// rather than being guessed at.
//
// A method that spells a reference or a callee records whatever
// import the spelling needs, which is why the target is bound to
// one file's import set rather than shared across a render.
type Target interface {
	// Lang names the target, so a refusal reads the way every
	// other refusal in that language does.
	Lang() string
	// Literal spells one leaf: a number, a string's content, a
	// truth value, the absent value, or raw text in a source
	// language, which a target refuses unless it is its own. The
	// whole leaf arrives rather than its parts, because raw text
	// is spelled by the language that wrote it and nothing else.
	Literal(v emit.Value) (string, error)
	// Type spells a reference a value names, and records its
	// import.
	Type(t *emit.TypeRef) (string, error)
	// Callee spells a function a value calls, and records its
	// import.
	Callee(id symbol.Identity) (string, error)
	// Conversion spells a value converted to a type: the reference
	// beside its spelling, because a language whose conversion
	// depends on the form reads the form off the reference rather
	// than off the text.
	Conversion(ref *emit.TypeRef, typ, inner string) (string, error)
	// Composite spells a composite of a type from its entries, the
	// reference beside its spelling for the same reason: a record,
	// a map and a list are one form here and three syntaxes in
	// most targets.
	Composite(ref *emit.TypeRef, typ string, entries []Entry) (string, error)
	// Call spells an application of a callee to its arguments.
	Call(callee string, args []string) (string, error)
	// Address spells the address of a value.
	Address(inner string) (string, error)
}

// Value spells one value tree through a target.
//
// It walks the tree and hands each spelling to the target, so a
// language states its syntax once and never its recursion. A value
// the vocabulary does not declare, a conversion or an address
// carrying nothing, and a composite or a conversion naming no
// type are each refused naming what is missing, because a value
// written half-formed compiles to something nobody derived.
func Value(t Target, v emit.Value) (string, error) {
	switch v.Kind {
	case emit.ValueLiteral:
		return t.Literal(v)
	case emit.ValueConversion:
		typ, err := namedType(t, v, "a conversion")
		if err != nil {
			return "", err
		}
		wrapped, err := wrapped(t, v, "a conversion")
		if err != nil {
			return "", err
		}
		return t.Conversion(v.Type, typ, wrapped)
	case emit.ValueComposite:
		return composite(t, v)
	case emit.ValueCall:
		return valueCall(t, v)
	case emit.ValueAddress:
		spelled, err := wrapped(t, v, "an address")
		if err != nil {
			return "", err
		}
		return t.Address(spelled)
	default:
		return "", fmt.Errorf("%s: no spelling for the %s value", t.Lang(), v.Kind)
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

// valueCall spells an application of a callee to its arguments.
func valueCall(t Target, v emit.Value) (string, error) {
	if v.Callee.IsZero() {
		return "", fmt.Errorf("%s: a call value names no callee", t.Lang())
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
	return t.Call(callee, args)
}

// namedType spells the type a value names, refusing one that names
// none: a conversion and a composite both need it, and a language
// cannot invent it.
func namedType(t Target, v emit.Value, what string) (string, error) {
	if v.Type == nil {
		return "", fmt.Errorf("%s: %s value names no type", t.Lang(), what)
	}
	return t.Type(v.Type)
}

// wrapped spells the one value a conversion or an address holds.
func wrapped(t Target, v emit.Value, what string) (string, error) {
	if v.Inner == nil {
		return "", fmt.Errorf("%s: %s value wraps nothing", t.Lang(), what)
	}
	return Value(t, *v.Inner)
}
