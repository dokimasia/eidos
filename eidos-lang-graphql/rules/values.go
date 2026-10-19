// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"go/constant"
	"go/token"
	"slices"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/lang/graphql/internal/value"
	"go.dokimi.dev/eidos/lang/numeric"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// deriveDepth bounds a walk through a type that refers to itself, such as an
// object type with a field of its own type.
const deriveDepth = 8

// The pairs that a sample derives, the zero of a number, the hint of a
// string where the caller names none, and the sign of a negative number.
const (
	sampleInt       = "42"
	alternateInt    = "7"
	sampleFloat     = "1.5"
	alternateFloat  = "2.5"
	sampleTrue      = "true"
	sampleFalse     = "false"
	zeroNumber      = "0"
	defaultHint     = "sample"
	sampleSuffix    = "-a"
	alternateSuffix = "-b"
	minusSign       = "-"
)

// DeriveSamples returns two values of a type that differ, for a check that
// must tell one from the other. Each form derives its pair in its own way:
//
//   - Int takes 42 and 7, Float 1.5 and 2.5, and Boolean true and false. A
//     String and an ID take the hint with the suffixes -a and -b, so two
//     fields of one type do not derive alike.
//   - A custom scalar of the table takes the pair of its shape: an integer
//     at the width of its shape, and the hinted strings for bytes. Every
//     integer scalar of the table is an integer.
//   - A nullable type takes the pair of its type, and a list takes a list
//     of one element that differs in the element.
//   - An object type and an input object take a composite that sets their
//     first field with a pair. A sum of a selection takes the composite of
//     its first variant with a pair, and a union takes the pair of its first
//     member with a pair.
//   - An enum takes its first two values, each its name converted to the
//     enum type.
//
// The element of a list and a field of a composite take the values that an
// author stated for them first, read through [rules.View.Authored]. The
// derivation completes each half that no author stated.
//
// Both halves refuse together, with one of these reasons:
//
//   - [rules.RefusedUnresolved]: a named type that the view does not
//     contain, and a spelling without a target that names no built-in
//     scalar.
//   - [rules.RefusedNoLiteral]: a type without two values that a check can
//     write. An interface, a stream, an enum of one value, a custom scalar
//     outside the table or of a well-known shape, and a composite without a
//     field with a pair are such types.
//   - [rules.RefusedDepth]: a type that refers to itself past the budget of
//     the walk.
//
// # Allocation contract
//
// A pair of a scalar allocates nothing beyond the two hinted texts of a
// string. A pair of a composite and of a list allocates the reference to its
// type, which restates every child of the reference, and the field list of
// each composite. A pair of an enum allocates the reference and the
// conversion of each value.
func (r Rules) DeriveSamples(ref *node.TypeRef, hint string, v rules.View) (rules.Sample, rules.Sample) {
	return r.derive(ref, hint, v, 0)
}

// derive is [Rules.DeriveSamples] with the depth of the walk so far.
func (r Rules) derive(ref *node.TypeRef, hint string, v rules.View, depth int) (rules.Sample, rules.Sample) {
	switch {
	case ref == nil:
		return rules.RefusedPair(rules.RefusedNoLiteral)
	case depth > deriveDepth:
		return rules.RefusedPair(rules.RefusedDepth)
	case ref.Form == symbol.FormOptional && len(ref.Elems) == 1:
		return r.derive(ref.Elems[0], hint, v, depth+1)
	case ref.Form == symbol.FormList && len(ref.Elems) == 1:
		sample, alternate := r.partPair(symbol.Identity{}, ref.Elems[0], hint, v, depth+1)
		if !sample.OK() || !alternate.OK() {
			return rules.RefusedPair(rules.FirstRefusal(sample, alternate))
		}
		t := rules.EmitRef(ref)
		return rules.Of(emit.Composite(t, emit.Element(sample.Value))),
			rules.Of(emit.Composite(t, emit.Element(alternate.Value)))
	case ref.Form != symbol.FormNamed:
		return rules.RefusedPair(rules.RefusedNoLiteral)
	case ref.Target.IsZero():
		return builtinPair(ref.Spelling, hint)
	default:
		return r.declaredPair(ref, hint, v, depth)
	}
}

// partPair returns the pair of one part of a composite: the values that an
// author stated on the declaration of the part and on its type, and the
// derivation for each half that no author stated. The element of a list has
// no declaration of its own, so subject is zero for it.
func (r Rules) partPair(
	subject symbol.Identity, ref *node.TypeRef, hint string, v rules.View, depth int,
) (rules.Sample, rules.Sample) {
	sample, alternate := v.Authored(r, subject, ref)
	if sample.OK() && alternate.OK() {
		return sample, alternate
	}
	derived, derivedAlternate := r.derive(ref, hint, v, depth)
	return rules.Complete(sample, alternate, derived, derivedAlternate)
}

// declaredPair derives the pair of a named type that the view contains.
func (r Rules) declaredPair(ref *node.TypeRef, hint string, v rules.View, depth int) (rules.Sample, rules.Sample) {
	sym, held := v.Lookup(ref.Target)
	if !held {
		return rules.RefusedPair(rules.RefusedUnresolved)
	}
	switch d := sym.(type) {
	case *node.Struct:
		return r.fieldsPair(ref, d.Fields, v, depth)
	case *node.Sum:
		for _, variant := range d.Variants {
			if sample, alternate := r.fieldsPair(ref, variant.Fields, v, depth); sample.OK() {
				return sample, alternate
			}
		}
		return rules.RefusedPair(rules.RefusedNoLiteral)
	case *node.Enum:
		var first string
		for _, variant := range d.Variants {
			switch {
			case first == "":
				first = variant.Name
			case variant.Name != first:
				t := rules.EmitRef(ref)
				return rules.Of(emit.Conversion(t, emit.Literal(emit.LiteralString, first))),
					rules.Of(emit.Conversion(t, emit.Literal(emit.LiteralString, variant.Name)))
			}
		}
		return rules.RefusedPair(rules.RefusedNoLiteral)
	case *node.Alias:
		if d.Target != nil {
			for _, member := range d.Target.Elems {
				if sample, alternate := r.derive(member, hint, v, depth+1); sample.OK() {
					return sample, alternate
				}
			}
			return rules.RefusedPair(rules.RefusedNoLiteral)
		}
		shape, tabled := graphql.ScalarShape(d.Annotations)
		switch {
		case tabled && shape.Form == symbol.FormScalar:
			return rules.NumberPair(emit.LiteralInt, sampleInt, alternateInt, shape.Bits)
		case tabled && shape.Form == symbol.FormBytes:
			return stringPair(hint)
		default:
			return rules.RefusedPair(rules.RefusedNoLiteral)
		}
	default:
		return rules.RefusedPair(rules.RefusedNoLiteral)
	}
}

// fieldsPair returns the composites of the type ref that set the first of
// fields with a pair, and refuses with no literal where no field has one.
func (r Rules) fieldsPair(
	ref *node.TypeRef, fields []*node.Field, v rules.View, depth int,
) (rules.Sample, rules.Sample) {
	for _, f := range fields {
		if sample, alternate := r.partPair(f.ID, f.Type, f.Name, v, depth+1); sample.OK() && alternate.OK() {
			t := rules.EmitRef(ref)
			return rules.Of(emit.Composite(t, emit.NamedField(f.Name, sample.Value))),
				rules.Of(emit.Composite(t, emit.NamedField(f.Name, alternate.Value)))
		}
	}
	return rules.RefusedPair(rules.RefusedNoLiteral)
}

// builtinPair derives the pair of a spelling that the resolution step left
// without a target: the pair of a built-in scalar, and a refusal as
// unresolved for every other spelling, which names a type that the view
// does not declare.
func builtinPair(spelling, hint string) (rules.Sample, rules.Sample) {
	switch spelling {
	case graphql.IntScalar:
		return rules.NumberPair(emit.LiteralInt, sampleInt, alternateInt, intBits)
	case graphql.FloatScalar:
		return rules.NumberPair(emit.LiteralFloat, sampleFloat, alternateFloat, floatBits)
	case graphql.BooleanScalar:
		return rules.Pair(emit.LiteralBool, sampleTrue, sampleFalse)
	case graphql.StringScalar, graphql.IDScalar:
		return stringPair(hint)
	default:
		return rules.RefusedPair(rules.RefusedUnresolved)
	}
}

// stringPair returns the pair of a string: the hint with the suffix of
// each half, and the default hint where the hint is empty.
func stringPair(hint string) (rules.Sample, rules.Sample) {
	if hint == "" {
		hint = defaultHint
	}
	return rules.Pair(emit.LiteralString, hint+sampleSuffix, hint+alternateSuffix)
}

// ZeroValue returns the value that a field of the type reads as where the
// response sets nothing, and reports whether one exists.
//
// A nullable type returns null, and a list returns the empty list. Int and
// Float return 0 at their widths, String and ID the empty string, and
// Boolean false. A custom scalar of the table of an integer shape returns 0
// at the width of the shape, and one of bytes the empty string.
//
// ZeroValue reports false for an object type, an input object, an
// interface, a union, a sum and an enum, because GraphQL gives none of them
// a value where a response sets nothing, and for a named type that the view
// does not contain. It reports false for a custom scalar outside the table
// and of a well-known shape, and for a spelling that names no built-in
// scalar.
//
// # Allocation contract
//
// A zero of a scalar and null allocate nothing. The empty list allocates
// the reference to its type, which restates every child of the reference.
func (Rules) ZeroValue(ref *node.TypeRef, v rules.View) (emit.Value, bool) {
	switch {
	case ref == nil:
		return emit.Value{}, false
	case ref.Form == symbol.FormOptional:
		return emit.Literal(emit.LiteralNil, ""), true
	case ref.Form == symbol.FormList:
		return emit.Composite(rules.EmitRef(ref)), true
	case ref.Form != symbol.FormNamed:
		return emit.Value{}, false
	case ref.Target.IsZero():
		switch ref.Spelling {
		case graphql.IntScalar:
			return emit.Number(emit.LiteralInt, zeroNumber, intBits), true
		case graphql.FloatScalar:
			return emit.Number(emit.LiteralFloat, zeroNumber, floatBits), true
		case graphql.BooleanScalar:
			return emit.Literal(emit.LiteralBool, sampleFalse), true
		case graphql.StringScalar, graphql.IDScalar:
			return emit.Literal(emit.LiteralString, ""), true
		default:
			return emit.Value{}, false
		}
	}
	sym, _ := v.Lookup(ref.Target)
	scalar, is := sym.(*node.Alias)
	if !is || scalar.Target != nil {
		return emit.Value{}, false
	}
	shape, tabled := graphql.ScalarShape(scalar.Annotations)
	switch {
	case tabled && shape.Form == symbol.FormScalar:
		return emit.Number(emit.LiteralInt, zeroNumber, shape.Bits), true
	case tabled && shape.Form == symbol.FormBytes:
		return emit.Literal(emit.LiteralString, ""), true
	default:
		return emit.Value{}, false
	}
}

// LiteralFor reads text in the grammar of a constant value of GraphQL as a
// value of the type ref, and reports whether the text is a value of the
// type. A literal is typed as an input of GraphQL is coerced:
//
//   - Int takes an integer inside 32 bits, and Float an integer or a float
//     that is finite at double precision, each stated at its width.
//   - String takes a string or a block string, ID a string or an integer as
//     its text, and Boolean true or false.
//   - A custom scalar of the table of an integer shape takes an integer
//     inside the range of the shape, and one of bytes a string. A custom
//     scalar outside the table takes each literal as its own kind, because
//     the server of the schema decides its coercion.
//   - An enum takes the name of one of its values, converted to the enum
//     type.
//   - An input object takes an object whose fields name fields of the input
//     object once each, every field typed by the type of its field. It
//     takes no object that leaves out a field of a type that is not
//     nullable and has no default.
//   - A nullable type takes null and what its type takes. A list takes a
//     list whose items its element type takes, and a single value of its
//     element type as a list of one item.
//
// An object type, an interface, a union and a sum take nothing, because no
// literal is a value of an output type. Where the reference names a type
// that the view does not contain, or a spelling that names no built-in
// scalar, each literal lifts as its own kind: a list and an object, an enum
// value and null then report false. The scope plays no part, because a
// literal of GraphQL reads the same in every scope.
//
// # Allocation contract
//
// LiteralFor allocates what the parse of the text allocates. A number reads
// through [go/constant] and allocates as [numeric.Int] and [numeric.Float]
// state. A composite allocates the reference to its type and its list of
// fields, and an enum value the reference and the conversion.
func (r Rules) LiteralFor(_ rules.Scope, ref *node.TypeRef, text string, v rules.View) (emit.Value, bool) {
	parsed, err := value.Parse(text)
	if err != nil {
		return emit.Value{}, false
	}
	return r.typed(parsed, ref, v, 0)
}

// typed returns the parsed literal lit as a value of the type ref, and
// reports whether lit is one.
func (r Rules) typed(lit *ast.Value, ref *node.TypeRef, v rules.View, depth int) (emit.Value, bool) {
	switch {
	case ref == nil || depth > deriveDepth:
		return untyped(lit)
	case ref.Form == symbol.FormOptional && len(ref.Elems) == 1:
		if lit.Kind == ast.NullValue {
			return emit.Literal(emit.LiteralNil, ""), true
		}
		return r.typed(lit, ref.Elems[0], v, depth+1)
	case ref.Form == symbol.FormList && len(ref.Elems) == 1:
		if lit.Kind == ast.NullValue {
			return emit.Value{}, false
		}
		items := lit.Children
		if lit.Kind != ast.ListValue {
			items = ast.ChildValueList{{Value: lit}}
		}
		fields := make([]emit.ValueField, 0, len(items))
		for _, item := range items {
			typed, valid := r.typed(item.Value, ref.Elems[0], v, depth+1)
			if !valid {
				return emit.Value{}, false
			}
			fields = append(fields, emit.Element(typed))
		}
		return emit.Composite(rules.EmitRef(ref), fields...), true
	case ref.Form != symbol.FormNamed:
		return emit.Value{}, false
	case ref.Target.IsZero():
		return builtinTyped(lit, ref.Spelling)
	}
	sym, held := v.Lookup(ref.Target)
	if !held {
		return untyped(lit)
	}
	switch d := sym.(type) {
	case *node.Enum:
		named := lit.Kind == ast.EnumValue && slices.ContainsFunc(d.Variants, func(variant *node.EnumVariant) bool {
			return variant.Name == lit.Raw
		})
		if !named {
			return emit.Value{}, false
		}
		return emit.Conversion(rules.EmitRef(ref), emit.Literal(emit.LiteralString, lit.Raw)), true
	case *node.Struct:
		_, input := rules.Fact(v, d.ID, meta.Named[string](graphql.InputKey))
		if !input || lit.Kind != ast.ObjectValue {
			return emit.Value{}, false
		}
		return r.inputTyped(lit, ref, d, v, depth)
	case *node.Alias:
		if d.Target != nil {
			return emit.Value{}, false
		}
		shape, tabled := graphql.ScalarShape(d.Annotations)
		switch {
		case !tabled:
			return untyped(lit)
		case shape.Form == symbol.FormScalar && lit.Kind == ast.IntValue:
			return numeric.Int(number(lit.Raw, token.INT), shape.Class, shape.Bits)
		case shape.Form == symbol.FormBytes && lit.Kind == ast.StringValue:
			return emit.Literal(emit.LiteralString, lit.Raw), true
		default:
			return emit.Value{}, false
		}
	default:
		return emit.Value{}, false
	}
}

// inputTyped returns the object lit as a value of the input object in, whose
// reference is ref. Each field of lit names a field of in once, and every
// field of in whose type is not nullable and that has no default is in lit.
// The fields of the value are in the order of lit.
func (r Rules) inputTyped(
	lit *ast.Value, ref *node.TypeRef, in *node.Struct, v rules.View, depth int,
) (emit.Value, bool) {
	fields := make([]emit.ValueField, 0, len(lit.Children))
	for i, child := range lit.Children {
		at := slices.IndexFunc(in.Fields, func(f *node.Field) bool { return f.Name == child.Name })
		repeated := slices.ContainsFunc(lit.Children[:i], func(earlier *ast.ChildValue) bool {
			return earlier.Name == child.Name
		})
		if at < 0 || repeated {
			return emit.Value{}, false
		}
		typed, valid := r.typed(child.Value, in.Fields[at].Type, v, depth+1)
		if !valid {
			return emit.Value{}, false
		}
		fields = append(fields, emit.NamedField(child.Name, typed))
	}
	for _, f := range in.Fields {
		required := f.Type.Form != symbol.FormOptional && f.Value == ""
		if required && lit.Children.ForName(f.Name) == nil {
			return emit.Value{}, false
		}
	}
	return emit.Composite(rules.EmitRef(ref), fields...), true
}

// builtinTyped returns the literal lit as a value of the built-in scalar
// spelling. A spelling that names no built-in scalar takes each literal as
// its own kind.
func builtinTyped(lit *ast.Value, spelling string) (emit.Value, bool) {
	switch {
	case spelling == graphql.IntScalar && lit.Kind == ast.IntValue:
		return numeric.Int(number(lit.Raw, token.INT), rules.ScalarInt, intBits)
	case spelling == graphql.FloatScalar && (lit.Kind == ast.IntValue || lit.Kind == ast.FloatValue):
		return numeric.Float(number(lit.Raw, token.FLOAT), floatBits)
	case spelling == graphql.StringScalar && (lit.Kind == ast.StringValue || lit.Kind == ast.BlockValue),
		spelling == graphql.IDScalar && (lit.Kind == ast.StringValue || lit.Kind == ast.IntValue):
		return emit.Literal(emit.LiteralString, lit.Raw), true
	case spelling == graphql.BooleanScalar && lit.Kind == ast.BooleanValue:
		return emit.Literal(emit.LiteralBool, lit.Raw), true
	case graphql.IsBuiltin(spelling):
		return emit.Value{}, false
	default:
		return untyped(lit)
	}
}

// untyped returns a scalar literal as its own kind: an integer and a float
// as their exact text, a string and a block string as their content, and
// true and false as a truth value. An enum value, null, a list and an
// object have no kind of their own, so untyped reports false for them.
func untyped(lit *ast.Value) (emit.Value, bool) {
	switch lit.Kind {
	case ast.IntValue:
		return emit.Literal(emit.LiteralInt, lit.Raw), true
	case ast.FloatValue:
		return numeric.Float(number(lit.Raw, token.FLOAT), 0)
	case ast.StringValue, ast.BlockValue:
		return emit.Literal(emit.LiteralString, lit.Raw), true
	case ast.BooleanValue:
		return emit.Literal(emit.LiteralBool, lit.Raw), true
	default:
		return emit.Value{}, false
	}
}

// number returns the exact value of the text of an integer or a float of
// GraphQL, which the lexer writes as decimal digits behind at most one minus
// sign. kind is [token.INT] for an integer and [token.FLOAT] for a float.
func number(text string, kind token.Token) constant.Value {
	digits, negative := strings.CutPrefix(text, minusSign)
	n := constant.MakeFromLiteral(digits, kind, 0)
	if negative {
		n = constant.UnaryOp(token.SUB, n, 0)
	}
	return n
}
