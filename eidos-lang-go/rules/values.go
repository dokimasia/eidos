// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"slices"
	"strconv"
	"strings"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// deriveDepth bounds a derivation through self-referential types:
// a struct holding a pointer to itself derives its field's value
// this many levels down and refuses past it.
const deriveDepth = 8

// The fixed pairs the builtin table derives, and the hint a string
// takes where the caller names nothing.
const (
	sampleInt        = "42"
	alternateInt     = "7"
	sampleFloat      = "1.5"
	alternateFloat   = "2.5"
	sampleSmall      = "1"
	alternateSmall   = "2"
	sampleTrue       = "true"
	sampleFalse      = "false"
	sampleZero       = "0"
	sampleSeconds    = "1700000000"
	alternateSeconds = "1700000001"
	defaultHint      = "sample"
	sampleSuffix     = "-a"
	alternateSuffix  = "-b"
	timePackage      = "time"
	timeUnix         = "Unix"
)

// SamplesOf derives a type's two distinguishable values: a builtin's
// pair from the fixed table, a defined type's as a conversion of
// its underlying type's, a struct's as a composite setting its
// first settable exported field, a slice's and an array's as a
// one-element composite differing in the element, a map's as a
// one-entry composite differing in the key, a pointer's as the
// address of the inner value, time.Time as a call and
// time.Duration as a conversion. An interface, a function type, a
// channel and an inline body refuse with no literal; a named type
// the view does not hold refuses as unresolved.
func (r Rules) SamplesOf(
	ref *node.TypeRef,
	hint string,
	v rules.View,
) (rules.Sample, rules.Sample) {
	return r.derive(ref, hint, v, 0)
}

// derive is [Rules.SamplesOf] with the depth walked so far.
func (r Rules) derive(
	ref *node.TypeRef,
	hint string,
	v rules.View,
	depth int,
) (rules.Sample, rules.Sample) {
	if ref == nil {
		return refused(rules.RefusedNoLiteral)
	}
	if depth > deriveDepth {
		return refused(rules.RefusedDepth)
	}
	switch ref.Form {
	case symbol.FormOptional:
		sample, alternate := r.derive(child(ref, 0), hint, v, depth+1)
		return lift(sample, emit.Address), lift(alternate, emit.Address)
	case symbol.FormList, symbol.FormArray:
		sample, alternate := r.derive(child(ref, 0), hint, v, depth+1)
		return lift(sample, elements(ref)), lift(alternate, elements(ref))
	case symbol.FormMap:
		key, otherKey := r.derive(child(ref, 0), hint, v, depth+1)
		value, _ := r.derive(child(ref, 1), hint, v, depth+1)
		if !key.OK() || !otherKey.OK() || !value.OK() {
			return refused(firstRefusal(key, otherKey, value))
		}
		return rules.Of(
				entries(ref, key.Value, value.Value),
			), rules.Of(
				entries(ref, otherKey.Value, value.Value),
			)
	case symbol.FormNamed:
		if ref.Target.IsZero() {
			return builtinPair(ref, hint)
		}
		return r.declaredPair(ref, hint, v, depth)
	default:
		return refused(rules.RefusedNoLiteral)
	}
}

// declaredPair derives the pair of a named type the view holds.
func (r Rules) declaredPair(
	ref *node.TypeRef,
	hint string,
	v rules.View,
	depth int,
) (rules.Sample, rules.Sample) {
	sym, held := v.Lookup(ref.Target)
	if !held {
		return refused(rules.RefusedUnresolved)
	}
	switch d := sym.(type) {
	case *node.Struct:
		for _, f := range d.Fields {
			if f == nil || f.Type == nil || !exported(f.Name) {
				continue
			}
			sample, alternate := r.derive(f.Type, f.Name, v, depth+1)
			if !sample.OK() || !alternate.OK() {
				return refused(firstRefusal(sample, alternate))
			}
			t := rules.EmitRef(ref)
			return rules.Of(emit.Composite(t, emit.ValueField{Name: f.Name, Value: sample.Value})),
				rules.Of(emit.Composite(t, emit.ValueField{Name: f.Name, Value: alternate.Value}))
		}
		// No settable exported field: every value of the type is
		// one value, and a check needs two.
		return refused(rules.RefusedNoLiteral)
	case *node.Alias:
		if d.Target == nil {
			return refused(rules.RefusedNoLiteral)
		}
		sample, alternate := r.derive(d.Target, hint, v, depth+1)
		if !d.Defined {
			return sample, alternate
		}
		t := rules.EmitRef(ref)
		return lift(sample, converted(t)), lift(alternate, converted(t))
	case *node.Enum:
		t := rules.EmitRef(ref)
		if first, second, exact := r.twoVariantValues(d, v); exact {
			return rules.Of(emit.Conversion(t, first)), rules.Of(emit.Conversion(t, second))
		}
		return rules.Of(emit.Conversion(t, emit.Literal(emit.LiteralInt, sampleSmall))),
			rules.Of(emit.Conversion(t, emit.Literal(emit.LiteralInt, alternateSmall)))
	default:
		return refused(rules.RefusedNoLiteral)
	}
}

// builtinPair derives the pair of a builtin or standard spelling.
func builtinPair(ref *node.TypeRef, hint string) (rules.Sample, rules.Sample) {
	spelling := named(ref)
	switch {
	case spelling == spellByte || spelling == spellUint8 || spelling == spellRune:
		return pair(emit.LiteralInt, sampleSmall, alternateSmall)
	case floating(spelling):
		return pair(emit.LiteralFloat, sampleFloat, alternateFloat)
	case numeric(spelling):
		return pair(emit.LiteralInt, sampleInt, alternateInt)
	case spelling == boolSpelling:
		return pair(emit.LiteralBool, sampleTrue, sampleFalse)
	case spelling == spellString:
		if hint == "" {
			hint = defaultHint
		}
		return pair(emit.LiteralString, hint+sampleSuffix, hint+alternateSuffix)
	case spelling == spellTime:
		unix := symbol.Identity{
			Lang:    golang.Lang,
			Package: timePackage,
			Name:    timeUnix,
			Kind:    symbol.KindFunction,
		}
		zero := emit.Literal(emit.LiteralInt, sampleZero)
		return rules.Of(emit.Call(unix, emit.Literal(emit.LiteralInt, sampleSeconds), zero)),
			rules.Of(emit.Call(unix, emit.Literal(emit.LiteralInt, alternateSeconds), zero))
	case spelling == spellDuration:
		t := rules.EmitRef(ref)
		return rules.Of(emit.Conversion(t, emit.Literal(emit.LiteralInt, sampleSmall))),
			rules.Of(emit.Conversion(t, emit.Literal(emit.LiteralInt, alternateSmall)))
	default:
		return refused(rules.RefusedNoLiteral)
	}
}

// twoVariantValues returns the exact values of an enumeration's
// first two variants, where the frontend stamped both and they
// differ.
func (r Rules) twoVariantValues(e *node.Enum, v rules.View) (emit.Value, emit.Value, bool) {
	key, held := r.constKey(v)
	if !held {
		return emit.Value{}, emit.Value{}, false
	}
	var texts []string
	for _, variant := range e.Variants {
		if variant == nil {
			continue
		}
		if text, stamped := rules.Fact(v, variant.ID, key); stamped &&
			!slices.Contains(texts, text) {
			texts = append(texts, text)
		}
		if len(texts) == 2 {
			return emit.Literal(
					emit.LiteralRaw,
					texts[0],
				), emit.Literal(
					emit.LiteralRaw,
					texts[1],
				), true
		}
	}
	return emit.Value{}, emit.Value{}, false
}

// ZeroValue returns the zero value of a type as Go spells it: nil
// for a pointer, a slice, a map, a function, a channel and an
// interface, an empty composite for a struct and an array, the
// builtin zeros, a conversion of the underlying zero for a defined
// type, and false for a spelling the rules cannot place.
func (r Rules) ZeroValue(ref *node.TypeRef, v rules.View) (emit.Value, bool) {
	if ref == nil {
		return emit.Value{}, false
	}
	switch ref.Form {
	case symbol.FormOptional, symbol.FormList, symbol.FormMap, symbol.FormFunc, symbol.FormStream:
		return emit.Literal(emit.LiteralNil, ""), true
	case symbol.FormArray:
		return emit.Composite(rules.EmitRef(ref)), true
	case symbol.FormNamed:
	default:
		return emit.Value{}, false
	}
	if ref.Target.IsZero() {
		return builtinZero(ref)
	}
	sym, held := v.Lookup(ref.Target)
	if !held {
		return emit.Value{}, false
	}
	switch d := sym.(type) {
	case *node.Struct:
		return emit.Composite(rules.EmitRef(ref)), true
	case *node.Interface, *node.Sum:
		return emit.Literal(emit.LiteralNil, ""), true
	case *node.Enum:
		return emit.Conversion(rules.EmitRef(ref), emit.Literal(emit.LiteralInt, sampleZero)), true
	case *node.Alias:
		if d.Target == nil {
			return emit.Value{}, false
		}
		inner, ok := r.ZeroValue(d.Target, v)
		if !ok || !d.Defined {
			return inner, ok
		}
		return emit.Conversion(rules.EmitRef(ref), inner), true
	default:
		return emit.Value{}, false
	}
}

// builtinZero returns a builtin's zero value.
func builtinZero(ref *node.TypeRef) (emit.Value, bool) {
	spelling := named(ref)
	switch {
	case floating(spelling):
		return emit.Literal(emit.LiteralFloat, sampleZero), true
	case numeric(spelling):
		return emit.Literal(emit.LiteralInt, sampleZero), true
	case spelling == boolSpelling:
		return emit.Literal(emit.LiteralBool, sampleFalse), true
	case spelling == spellString:
		return emit.Literal(emit.LiteralString, ""), true
	case spelling == spellTime:
		return emit.Composite(rules.EmitRef(ref)), true
	case spelling == spellDuration:
		return emit.Conversion(rules.EmitRef(ref), emit.Literal(emit.LiteralInt, sampleZero)), true
	case spelling == spellAny || spelling == errorSpelling:
		return emit.Literal(emit.LiteralNil, ""), true
	default:
		return emit.Value{}, false
	}
}

// LiteralFor lifts a Go literal into the value tree: nil, the two
// booleans, an integer, a float, a quoted or raw string, and a rune
// literal as its integer value. Any other text is no literal, and
// the file plays no part, because a Go literal reads the same in
// every scope.
func (Rules) LiteralFor(
	_ *node.File,
	_ *node.TypeRef,
	text string,
	_ rules.View,
) (emit.Value, bool) {
	text = strings.TrimSpace(text)
	switch text {
	case "":
		return emit.Value{}, false
	case "nil":
		return emit.Literal(emit.LiteralNil, ""), true
	case sampleTrue, sampleFalse:
		return emit.Literal(emit.LiteralBool, text), true
	}
	if _, err := strconv.ParseInt(text, 0, 64); err == nil {
		return emit.Literal(emit.LiteralInt, text), true
	}
	if _, err := strconv.ParseFloat(text, 64); err == nil {
		return emit.Literal(emit.LiteralFloat, text), true
	}
	if unquoted, err := strconv.Unquote(text); err == nil {
		if strings.HasPrefix(text, "'") {
			r, _, _, err := strconv.UnquoteChar(text[1:len(text)-1], '\'')
			if err != nil {
				return emit.Value{}, false
			}
			return emit.Literal(emit.LiteralInt, strconv.Itoa(int(r))), true
		}
		return emit.Literal(emit.LiteralString, unquoted), true
	}
	return emit.Value{}, false
}

// pair returns two literal samples of one kind.
func pair(k emit.LiteralKind, sample, alternate string) (rules.Sample, rules.Sample) {
	return rules.Of(emit.Literal(k, sample)), rules.Of(emit.Literal(k, alternate))
}

// refused returns one refusal twice.
func refused(why rules.Refusal) (rules.Sample, rules.Sample) {
	return rules.Refused(why), rules.Refused(why)
}

// lift wraps a derived sample, and passes a refusal through.
func lift(s rules.Sample, wrap func(emit.Value) emit.Value) rules.Sample {
	if !s.OK() {
		return s
	}
	return rules.Of(wrap(s.Value))
}

// converted returns the wrap converting a value to a type.
func converted(t *emit.TypeRef) func(emit.Value) emit.Value {
	return func(inner emit.Value) emit.Value { return emit.Conversion(t, inner) }
}

// elements returns the wrap placing one value as a slice's or an
// array's one element.
func elements(ref *node.TypeRef) func(emit.Value) emit.Value {
	t := rules.EmitRef(ref)
	return func(inner emit.Value) emit.Value {
		return emit.Value{Kind: emit.ValueComposite, Type: t, Args: []emit.Value{inner}}
	}
}

// entries returns a map composite holding one entry, the key then
// the value in its positional arguments.
func entries(ref *node.TypeRef, key, value emit.Value) emit.Value {
	return emit.Value{
		Kind: emit.ValueComposite,
		Type: rules.EmitRef(ref),
		Args: []emit.Value{key, value},
	}
}

// child returns a structural reference's child, or nil.
func child(ref *node.TypeRef, i int) *node.TypeRef {
	if i < len(ref.Elems) {
		return ref.Elems[i]
	}
	return nil
}

// firstRefusal returns the first refusal among samples, and no
// literal where none refused.
func firstRefusal(samples ...rules.Sample) rules.Refusal {
	for _, s := range samples {
		if s.Refusal != rules.RefusedNone {
			return s.Refusal
		}
	}
	return rules.RefusedNoLiteral
}
