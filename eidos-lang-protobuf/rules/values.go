// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"strconv"
	"strings"
	"unicode/utf8"

	protobuf "go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// deriveDepth bounds a derivation through a self-referential
// message, which proto admits: a Node with a repeated Node.
const deriveDepth = 8

// enumBits is the width of an enum number, which the wire encodes as
// an int32.
const enumBits = 32

// The fixed pairs the wire table derives, and the hint a string
// takes where the caller names nothing.
const (
	sampleInt       = "42"
	alternateInt    = "7"
	sampleFloat     = "1.5"
	alternateFloat  = "2.5"
	sampleTrue      = "true"
	sampleFalse     = "false"
	sampleZero      = "0"
	defaultHint     = "sample"
	sampleSuffix    = "-a"
	alternateSuffix = "-b"
)

// SamplesOf returns two values of a type that differ, for a check
// that must tell one from the other.
//
// The pair is derived per form: a scalar's from the wire table at
// the wire's width, a well-known wrapper's as its scalar's, a
// repeated field's as a one-element composite differing in the
// element, a map's as a one-entry composite differing in the key, a
// message's as a composite setting its first field that yields a
// pair, a oneof's through its first such variant, and an enum's as
// its first two declared values with distinct numbers, each the
// number converted to the enum type.
//
// The hint names the declaration that has the type and seeds a
// string's pair, so two string fields of one message do not derive
// alike. Every other form ignores it.
//
// Both halves refuse together. [rules.RefusedUnresolved] means a
// named type the view does not contain; [rules.RefusedNoLiteral]
// means the type admits no two distinguishable values, which a
// message with no usable field, an enum with one number, a stream
// and a well-known type without a scalar all do;
// [rules.RefusedDepth] means a self-referential type past the walk's
// budget.
func (r Rules) SamplesOf(ref *node.TypeRef, hint string, v rules.View) (rules.Sample, rules.Sample) {
	return r.derive(ref, hint, v, 0)
}

// derive is [Rules.SamplesOf] with the depth walked so far.
func (r Rules) derive(
	ref *node.TypeRef, hint string, v rules.View, depth int,
) (rules.Sample, rules.Sample) {
	if ref == nil {
		return refused(rules.RefusedNoLiteral)
	}
	if depth > deriveDepth {
		return refused(rules.RefusedDepth)
	}
	switch ref.Form {
	case symbol.FormOptional:
		return r.derive(child(ref, 0), hint, v, depth+1)
	case symbol.FormList:
		sample, alternate := r.derive(child(ref, 0), hint, v, depth+1)
		return element(ref, sample), element(ref, alternate)
	case symbol.FormMap:
		key, otherKey := r.derive(child(ref, 0), hint, v, depth+1)
		value, _ := r.derive(child(ref, 1), hint, v, depth+1)
		if !key.OK() || !otherKey.OK() || !value.OK() {
			return refused(firstRefusal(key, otherKey, value))
		}
		t := rules.EmitRef(ref)
		return rules.Of(emit.Composite(t, emit.KeyedEntry(key.Value, value.Value))),
			rules.Of(emit.Composite(t, emit.KeyedEntry(otherKey.Value, value.Value)))
	case symbol.FormStream:
		// A streaming side is many of its message, and one value of a
		// stream is not a value a check can write.
		return refused(rules.RefusedNoLiteral)
	case symbol.FormNamed:
		if ref.Target.IsZero() {
			return scalarPair(ref.Spelling, hint)
		}
		return r.declaredPair(ref, v, depth)
	default:
		return refused(rules.RefusedNoLiteral)
	}
}

// declaredPair derives the pair of a message, a oneof or an enum the
// view contains.
func (r Rules) declaredPair(
	ref *node.TypeRef, v rules.View, depth int,
) (rules.Sample, rules.Sample) {
	sym, held := v.Lookup(ref.Target)
	if !held {
		return refused(rules.RefusedUnresolved)
	}
	switch d := sym.(type) {
	case *node.Struct:
		for _, f := range d.Fields {
			if f == nil || f.Type == nil {
				continue
			}
			if sample, alternate := r.derive(f.Type, f.Name, v, depth+1); sample.OK() && alternate.OK() {
				t := rules.EmitRef(ref)
				return rules.Of(emit.Composite(t, emit.NamedField(f.Name, sample.Value))),
					rules.Of(emit.Composite(t, emit.NamedField(f.Name, alternate.Value)))
			}
		}
		// Every value of a message with nothing settable is one value,
		// and a check needs two.
		return refused(rules.RefusedNoLiteral)
	case *node.Enum:
		return enumPair(ref, d)
	case *node.Sum:
		// A oneof's value is one of its members, and the member's
		// field is what a check writes.
		for _, variant := range d.Variants {
			if variant == nil || len(variant.Fields) == 0 || variant.Fields[0].Type == nil {
				continue
			}
			f := variant.Fields[0]
			if sample, alternate := r.derive(f.Type, f.Name, v, depth+1); sample.OK() && alternate.OK() {
				t := rules.EmitRef(ref)
				return rules.Of(emit.Composite(t, emit.NamedField(f.Name, sample.Value))),
					rules.Of(emit.Composite(t, emit.NamedField(f.Name, alternate.Value)))
			}
		}
		return refused(rules.RefusedNoLiteral)
	default:
		return refused(rules.RefusedNoLiteral)
	}
}

// scalarPair derives the pair of a spelling the resolution step left
// without a target: a number's at the wire's width, a boolean's, a
// string's and a byte string's with the hint, and a well-known
// wrapper's as the scalar it wraps. Any other well-known type
// refuses with no literal, because the value a target writes for it
// is the target's own, and any other spelling refuses as unresolved:
// it names a message this workspace does not declare.
func scalarPair(spelling, hint string) (rules.Sample, rules.Sample) {
	if s, held := scalars[spelling]; held {
		if s.class == rules.ScalarFloat {
			return numbers(emit.LiteralFloat, sampleFloat, alternateFloat, s.bits)
		}
		return numbers(emit.LiteralInt, sampleInt, alternateInt, s.bits)
	}
	switch spelling {
	case spellBool:
		return pair(emit.LiteralBool, sampleTrue, sampleFalse)
	case spellString, spellBytes:
		if hint == "" {
			hint = defaultHint
		}
		return pair(emit.LiteralString, hint+sampleSuffix, hint+alternateSuffix)
	}
	if inner, wraps := wrapped(spelling); wraps {
		return scalarPair(inner, hint)
	}
	if _, known := protobuf.WellKnown(spelling); known {
		return refused(rules.RefusedNoLiteral)
	}
	return refused(rules.RefusedUnresolved)
}

// enumPair returns an enum's first two declared values with distinct
// numbers, each the number converted to the enum type. An alias
// shares its number with an earlier value, so it is no second value.
func enumPair(ref *node.TypeRef, e *node.Enum) (rules.Sample, rules.Sample) {
	t := rules.EmitRef(ref)
	var first int64
	found := false
	for _, variant := range e.Variants {
		n, numbered := enumNumber(variant)
		switch {
		case !numbered:
		case !found:
			first, found = n, true
		case n != first:
			return rules.Of(enumValue(t, first)), rules.Of(enumValue(t, n))
		}
	}
	return refused(rules.RefusedNoLiteral)
}

// enumNumber returns a variant's declared number, and false for a nil
// variant, one with no name, and one whose number is no int32. The
// number is read in any base protobuf writes, a minus sign apart
// from its digits included.
func enumNumber(variant *node.EnumVariant) (int64, bool) {
	if variant == nil || variant.Name == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(strings.Join(strings.Fields(variant.Value), ""), 0, enumBits)
	return n, err == nil
}

// enumValue returns one enum number as a value of the enum type: the
// number at the wire's thirty-two bits, converted.
func enumValue(t *emit.TypeRef, n int64) emit.Value {
	return emit.Conversion(t, emit.Number(emit.LiteralInt, strconv.FormatInt(n, 10), enumBits))
}

// ZeroValue returns the value a field of the type reads as when the
// schema sets nothing, and reports whether one exists.
//
// A scalar returns its zero at the wire's width, a repeated or map
// field the empty composite, and an enum its first declared value,
// which is protobuf's default for an enum field, converted to the
// enum type. An optional field, a well-known wrapper and a message
// field return nil: absence is the value there, which is what
// presence means.
//
// It reports false for a stream, for a named type the view does not
// contain, for an enum declaring no value, and for a spelling the
// wire table does not list.
func (Rules) ZeroValue(ref *node.TypeRef, v rules.View) (emit.Value, bool) {
	if ref == nil {
		return emit.Value{}, false
	}
	switch ref.Form {
	case symbol.FormList, symbol.FormMap:
		return emit.Composite(rules.EmitRef(ref)), true
	case symbol.FormOptional:
		return emit.Literal(emit.LiteralNil, ""), true
	case symbol.FormNamed:
	default:
		return emit.Value{}, false
	}
	if ref.Target.IsZero() {
		return scalarZero(ref.Spelling)
	}
	sym, held := v.Lookup(ref.Target)
	if !held {
		return emit.Value{}, false
	}
	switch d := sym.(type) {
	case *node.Struct, *node.Sum:
		// An absent message is absent: proto3 gives a message field no
		// zero of its own, and a reader asks whether it is set.
		return emit.Literal(emit.LiteralNil, ""), true
	case *node.Enum:
		for _, variant := range d.Variants {
			if n, numbered := enumNumber(variant); numbered {
				return enumValue(rules.EmitRef(ref), n), true
			}
		}
		return emit.Value{}, false
	default:
		return emit.Value{}, false
	}
}

// scalarZero returns the zero of a spelling the resolution step left
// without a target.
func scalarZero(spelling string) (emit.Value, bool) {
	if s, held := scalars[spelling]; held {
		if s.class == rules.ScalarFloat {
			return emit.Number(emit.LiteralFloat, sampleZero, s.bits), true
		}
		return emit.Number(emit.LiteralInt, sampleZero, s.bits), true
	}
	switch spelling {
	case spellBool:
		return emit.Literal(emit.LiteralBool, sampleFalse), true
	case spellString, spellBytes:
		return emit.Literal(emit.LiteralString, ""), true
	}
	if _, wraps := wrapped(spelling); wraps {
		return emit.Literal(emit.LiteralNil, ""), true
	}
	return emit.Value{}, false
}

// LiteralFor lifts a literal an author wrote in protobuf's grammar
// into the value tree, typed by the reference, and reports whether
// the text is a value of the type.
//
// protoc's grammar reads the text: true and false, an integer in
// decimal, octal or hexadecimal, a decimal float, a string in single
// or double quotes with protoc's escapes, adjacent strings
// concatenated, and an identifier, each number behind at most one
// minus sign. A number is written back as canonical decimal text.
//
// A number type takes an integer inside its width, and a float type
// any finite number inside its width, each stated at the wire's
// width. bool takes a truth value, string a string that is valid
// UTF-8, and bytes any string. An enum takes the name of one of its
// values, converted to the enum type. The optional form and a
// well-known wrapper take what their scalar takes, because a default
// is the value an absent field reads as. A message takes nothing.
// Where the reference names none of these, each literal lifts as its
// own kind, and an identifier, which only an enum can type, reports
// false. inf and nan are identifiers no literal spells, so they
// report false too. The file plays no part, because a proto literal
// reads the same in every scope.
func (r Rules) LiteralFor(
	_ *node.File, ref *node.TypeRef, text string, v rules.View,
) (emit.Value, bool) {
	lit, scanned := scanLiteral(strings.TrimSpace(text))
	if !scanned {
		return emit.Value{}, false
	}
	return lit.typed(r.literalType(ref, v, 0))
}

// literalType is what a literal is typed against: a shape, an enum
// with the reference its values convert to, or nothing at all for a
// type no literal is a value of. The zero literalType types
// nothing, so each literal lifts as its own kind.
type literalType struct {
	shape rules.TypeShape
	enum  *node.Enum
	ref   *node.TypeRef
	none  bool
}

// literalType returns what a literal of a reference's type is typed
// against.
func (r Rules) literalType(ref *node.TypeRef, v rules.View, depth int) literalType {
	switch {
	case ref == nil || depth > deriveDepth:
		return literalType{}
	case ref.Form == symbol.FormOptional:
		return r.literalType(child(ref, 0), v, depth+1)
	case ref.Form != symbol.FormNamed:
		return literalType{none: true}
	case ref.Target.IsZero():
		shape := r.Builtin(ref, v)
		if shape.Form == symbol.FormOptional && len(shape.Elems) == 1 {
			shape = shape.Elems[0]
		}
		return literalType{shape: shape}
	}
	sym, _ := v.Lookup(ref.Target)
	switch d := sym.(type) {
	case *node.Enum:
		return literalType{enum: d, ref: ref}
	case *node.Struct, *node.Sum:
		return literalType{none: true}
	default:
		return literalType{}
	}
}

// typed returns the literal as a value of a type.
func (lit literal) typed(t literalType) (emit.Value, bool) {
	switch {
	case t.none:
		return emit.Value{}, false
	case t.enum != nil:
		for _, variant := range t.enum.Variants {
			if lit.kind != literalIdent || variant == nil || variant.Name != lit.text {
				continue
			}
			if n, numbered := enumNumber(variant); numbered {
				return enumValue(rules.EmitRef(t.ref), n), true
			}
		}
		return emit.Value{}, false
	}
	switch t.shape.Form {
	case symbol.FormScalar:
		switch {
		case lit.kind != literalNumber:
			return emit.Value{}, false
		case t.shape.Class == rules.ScalarFloat:
			return floatValue(lit.number, t.shape.Bits)
		case !lit.integer:
			// An integer type takes an integer literal alone: protoc
			// refuses 2.0 where an int32 is declared.
			return emit.Value{}, false
		default:
			return intValue(lit.number, t.shape.Class, t.shape.Bits)
		}
	case symbol.FormBool:
		if lit.kind != literalBool {
			return emit.Value{}, false
		}
		return emit.Literal(emit.LiteralBool, lit.text), true
	case symbol.FormText:
		if lit.kind != literalString || !utf8.ValidString(lit.text) {
			return emit.Value{}, false
		}
		return emit.Literal(emit.LiteralString, lit.text), true
	case symbol.FormBytes:
		if lit.kind != literalString {
			return emit.Value{}, false
		}
		return emit.Literal(emit.LiteralString, lit.text), true
	}
	return lit.untyped()
}

// untyped returns the literal as its own kind: a truth value, a
// string, an integer's exact text, or a float's canonical text. An
// identifier has no kind of its own, so it reports false.
func (lit literal) untyped() (emit.Value, bool) {
	switch lit.kind {
	case literalBool:
		return emit.Literal(emit.LiteralBool, lit.text), true
	case literalString:
		return emit.Literal(emit.LiteralString, lit.text), true
	case literalNumber:
		if lit.integer {
			return emit.Literal(emit.LiteralInt, lit.number.ExactString()), true
		}
		return floatValue(lit.number, 0)
	default:
		return emit.Value{}, false
	}
}

// element places one derived value as a repeated field's single
// element, passing a refusal through.
func element(ref *node.TypeRef, s rules.Sample) rules.Sample {
	if !s.OK() {
		return s
	}
	return rules.Of(emit.Composite(rules.EmitRef(ref), emit.Element(s.Value)))
}

// pair returns two literal samples of one kind.
func pair(k emit.LiteralKind, sample, alternate string) (rules.Sample, rules.Sample) {
	return rules.Of(emit.Literal(k, sample)), rules.Of(emit.Literal(k, alternate))
}

// numbers returns two numeric samples of one kind at one width.
func numbers(k emit.LiteralKind, sample, alternate string, bits int) (rules.Sample, rules.Sample) {
	return rules.Of(emit.Number(k, sample, bits)), rules.Of(emit.Number(k, alternate, bits))
}

// refused returns one refusal twice.
func refused(why rules.Refusal) (rules.Sample, rules.Sample) {
	return rules.Refused(why), rules.Refused(why)
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
