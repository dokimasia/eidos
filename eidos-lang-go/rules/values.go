// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"go/constant"
	"go/scanner"
	"go/token"
	"slices"
	"strings"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/numeric"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// deriveDepth bounds a derivation through self-referential types. A
// struct with a pointer to itself derives the value of its field this
// many levels down, and refuses below that.
const deriveDepth = 8

// The fixed pairs that the builtin table derives, and the hint of a
// string where the caller passes none.
const (
	sampleInt        = "42"
	alternateInt     = "7"
	sampleFloat      = "1.5"
	alternateFloat   = "2.5"
	sampleSmall      = "1"
	alternateSmall   = "2"
	sampleZero       = "0"
	sampleSeconds    = "1700000000"
	alternateSeconds = "1700000001"
	defaultHint      = "sample"
	sampleSuffix     = "-a"
	alternateSuffix  = "-b"
	timeUnix         = "Unix"
	// stringQuote opens the exact value of a string constant.
	stringQuote = `"`
)

// The keyword literals of Go. They are the absent value and the two
// truth values.
const (
	nilSpelling   = "nil"
	trueSpelling  = "true"
	falseSpelling = "false"
)

// platformWidth is the width of a platform-sized number, which has none.
// A float of that width writes at the precision of float64.
const platformWidth = 0

// DeriveSamples derives the two distinguishable values of a type:
//   - the pair of a builtin from the fixed table
//   - the pair of a defined type as a conversion of the pair of its
//     underlying type
//   - the pair of a struct as a composite that sets its first exported
//     field whose pair derives
//   - the pair of a slice or an array as a one-element composite that
//     differs in the element
//   - the pair of a map as a one-entry composite that differs in the key
//   - the pair of a pointer as the address of its inner composite
//   - time.Time as a call, and time.Duration as a conversion
//
// A number states the width of its builtin. An interface, a function
// type, a channel, an inline body, a predeclared type without a literal
// and a pointer to anything but a composite refuse with no literal,
// because Go takes the address of a composite literal alone. A named type
// outside the view refuses as unresolved.
//
// The element of a slice or an array, the key and the value of a map, and
// the field of a struct take the values that an author stated for them
// first, which [rules.View.Authored] reads. The derivation completes each
// half that no author stated.
//
// # Allocation contract
//
// The pair of a builtin allocates nothing beyond the hinted text of a
// string. The pair of a composite allocates the reference to its type and
// the list of each composite, and a conversion allocates its wrapped
// value.
func (r Rules) DeriveSamples(
	ref *node.TypeRef,
	hint string,
	v rules.View,
) (rules.Sample, rules.Sample) {
	return r.derive(ref, hint, v, 0)
}

// derive is [Rules.DeriveSamples] with the depth walked so far.
func (r Rules) derive(
	ref *node.TypeRef,
	hint string,
	v rules.View,
	depth int,
) (rules.Sample, rules.Sample) {
	if ref == nil {
		return rules.RefusedPair(rules.RefusedNoLiteral)
	}
	if depth > deriveDepth {
		return rules.RefusedPair(rules.RefusedDepth)
	}
	switch ref.Form {
	case symbol.FormOptional:
		sample, alternate := r.derive(child(ref, 0), hint, v, depth+1)
		return addressed(sample), addressed(alternate)
	case symbol.FormList, symbol.FormArray:
		sample, alternate := r.partPair(symbol.Identity{}, child(ref, 0), hint, v, depth+1)
		t := rules.EmitRef(ref)
		return rules.Lift(sample, elements(t)), rules.Lift(alternate, elements(t))
	case symbol.FormMap:
		key, otherKey := r.partPair(symbol.Identity{}, child(ref, 0), hint, v, depth+1)
		value, _ := r.partPair(symbol.Identity{}, child(ref, 1), hint, v, depth+1)
		if !key.OK() || !otherKey.OK() || !value.OK() {
			return rules.RefusedPair(rules.FirstRefusal(key, otherKey, value))
		}
		t := rules.EmitRef(ref)
		return rules.Of(emit.Composite(t, emit.KeyedEntry(key.Value, value.Value))),
			rules.Of(emit.Composite(t, emit.KeyedEntry(otherKey.Value, value.Value)))
	case symbol.FormNamed:
		if ref.Target.IsZero() {
			return r.builtinPair(ref, hint)
		}
		return r.declaredPair(ref, hint, v, depth)
	default:
		return rules.RefusedPair(rules.RefusedNoLiteral)
	}
}

// partPair returns the pair of one part of a composite. It takes the
// values that an author stated on the declaration that has the part and
// on its type, and derives each half that no author stated. A list
// element and the key and the value of a map have no declaration of their
// own, so subject is zero for them.
func (r Rules) partPair(
	subject symbol.Identity,
	ref *node.TypeRef,
	hint string,
	v rules.View,
	depth int,
) (rules.Sample, rules.Sample) {
	sample, alternate := v.Authored(r, subject, ref)
	if sample.OK() && alternate.OK() {
		return sample, alternate
	}
	derived, derivedAlternate := r.derive(ref, hint, v, depth)
	return rules.Complete(sample, alternate, derived, derivedAlternate)
}

// declaredPair derives the pair of a named type in the view.
func (r Rules) declaredPair(
	ref *node.TypeRef,
	hint string,
	v rules.View,
	depth int,
) (rules.Sample, rules.Sample) {
	sym, held := v.Lookup(ref.Target)
	if !held {
		return rules.RefusedPair(rules.RefusedUnresolved)
	}
	switch d := sym.(type) {
	case *node.Struct:
		// Without an exported field, every value of the type is one
		// value, and a check needs two. Where no field derives, the
		// refusal of the first field is the reason.
		why, refused := rules.RefusedNoLiteral, false
		for _, f := range d.Fields {
			if f == nil || f.Type == nil || !token.IsExported(f.Name) {
				continue
			}
			sample, alternate := r.partPair(f.ID, f.Type, f.Name, v, depth+1)
			if !sample.OK() || !alternate.OK() {
				if !refused {
					why, refused = rules.FirstRefusal(sample, alternate), true
				}
				continue
			}
			t := rules.EmitRef(ref)
			return rules.Of(emit.Composite(t, emit.NamedField(f.Name, sample.Value))),
				rules.Of(emit.Composite(t, emit.NamedField(f.Name, alternate.Value)))
		}
		return rules.RefusedPair(why)
	case *node.Alias:
		if d.Target == nil {
			return rules.RefusedPair(rules.RefusedNoLiteral)
		}
		sample, alternate := r.derive(d.Target, hint, v, depth+1)
		if !d.Defined {
			return sample, alternate
		}
		t := rules.EmitRef(ref)
		return rules.Lift(sample, converted(t)), rules.Lift(alternate, converted(t))
	case *node.Enum:
		t := rules.EmitRef(ref)
		if first, second, exact := r.twoVariantValues(d, v); exact {
			return rules.Of(emit.Conversion(t, first)), rules.Of(emit.Conversion(t, second))
		}
		return rules.Of(emit.Conversion(t, emit.Literal(emit.LiteralInt, sampleSmall))),
			rules.Of(emit.Conversion(t, emit.Literal(emit.LiteralInt, alternateSmall)))
	default:
		return rules.RefusedPair(rules.RefusedNoLiteral)
	}
}

// builtinPair derives the pair of a spelling that the resolution step
// left without a target, through the builtin table. A number has a pair
// at the width of its builtin, with small values for byte and rune. A
// boolean, a string with the hint, time.Time as a call and time.Duration
// as a conversion have pairs too. A predeclared type that the table does
// not place refuses with no literal, and any other spelling refuses as
// unresolved.
func (r Rules) builtinPair(ref *node.TypeRef, hint string) (rules.Sample, rules.Sample) {
	shape := r.Builtin(ref, rules.View{})
	switch shape.Form {
	case symbol.FormScalar:
		spelling := named(ref)
		switch {
		case shape.Class == rules.ScalarFloat:
			return rules.NumberPair(emit.LiteralFloat, sampleFloat, alternateFloat, shape.Bits)
		case spelling == spellByte || spelling == spellUint8 || spelling == spellRune:
			return rules.NumberPair(emit.LiteralInt, sampleSmall, alternateSmall, shape.Bits)
		default:
			return rules.NumberPair(emit.LiteralInt, sampleInt, alternateInt, shape.Bits)
		}
	case symbol.FormBool:
		return rules.Pair(emit.LiteralBool, trueSpelling, falseSpelling)
	case symbol.FormText:
		if hint == "" {
			hint = defaultHint
		}
		return rules.Pair(emit.LiteralString, hint+sampleSuffix, hint+alternateSuffix)
	case symbol.FormReference:
		switch shape.Ref {
		case rules.WellKnownTimestamp:
			unix := symbol.Identity{
				Lang:    golang.Lang,
				Package: timePackage,
				Name:    timeUnix,
				Kind:    symbol.KindFunction,
			}
			zero := emit.Literal(emit.LiteralInt, sampleZero)
			return rules.Of(emit.Call(unix, emit.Literal(emit.LiteralInt, sampleSeconds), zero)),
				rules.Of(emit.Call(unix, emit.Literal(emit.LiteralInt, alternateSeconds), zero))
		case rules.WellKnownDuration:
			t := rules.EmitRef(ref)
			return rules.Of(emit.Conversion(t, emit.Literal(emit.LiteralInt, sampleSmall))),
				rules.Of(emit.Conversion(t, emit.Literal(emit.LiteralInt, alternateSmall)))
		}
	default:
		// Every other form refuses below.
	}
	if t := nameOf(ref); t.pkg == "" && golang.Predeclared(t.name) || t == unsafePointerType {
		return rules.RefusedPair(rules.RefusedNoLiteral)
	}
	return rules.RefusedPair(rules.RefusedUnresolved)
}

// twoVariantValues returns the exact values of the first two variants of
// an enumeration, where the frontend stamped both and they differ.
func (Rules) twoVariantValues(e *node.Enum, v rules.View) (emit.Value, emit.Value, bool) {
	var texts []string
	for _, variant := range e.Variants {
		if variant == nil {
			continue
		}
		if text, stamped := rules.Fact(v, variant.ID, constValue); stamped &&
			!slices.Contains(texts, text) {

			texts = append(texts, text)
		}
		if len(texts) == 2 {
			return emit.Raw(golang.Lang, texts[0]), emit.Raw(golang.Lang, texts[1]), true
		}
	}
	return emit.Value{}, emit.Value{}, false
}

// enumZero returns the zero of the underlying type of an enumeration. It
// reads the first variant value that the frontend stamped, and returns
// the empty string where that value is a string and zero where it is a
// number. It reports false where no variant has a stamped value.
func (Rules) enumZero(e *node.Enum, v rules.View) (emit.Value, bool) {
	for _, variant := range e.Variants {
		if variant == nil {
			continue
		}
		text, stamped := rules.Fact(v, variant.ID, constValue)
		if !stamped {
			continue
		}
		if strings.HasPrefix(text, stringQuote) {
			return emit.Literal(emit.LiteralString, ""), true
		}
		return emit.Literal(emit.LiteralInt, sampleZero), true
	}
	return emit.Value{}, false
}

// ZeroValue returns the zero value of a type as Go spells it:
//   - nil for a pointer, a slice, a map, a function, a channel, an
//     interface and unsafe.Pointer
//   - an empty composite for a struct and an array
//   - the builtin zeros at the width of the builtin
//   - a conversion of the underlying zero for a defined type, and for an
//     enumeration whose variant values have its underlying type
//
// It reports false for a spelling that the rules cannot place. The zero
// of a builtin allocates nothing, and a composite or a conversion
// allocates the reference to its type and its wrapped value.
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
		return r.builtinZero(ref)
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
		inner, placed := r.enumZero(d, v)
		if !placed {
			return emit.Value{}, false
		}
		return emit.Conversion(rules.EmitRef(ref), inner), true
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

// builtinZero returns the zero value of a spelling that the resolution
// step left without a target, through the builtin table.
func (r Rules) builtinZero(ref *node.TypeRef) (emit.Value, bool) {
	shape := r.Builtin(ref, rules.View{})
	switch shape.Form {
	case symbol.FormScalar:
		if shape.Class == rules.ScalarFloat {
			return emit.Number(emit.LiteralFloat, sampleZero, shape.Bits), true
		}
		return emit.Number(emit.LiteralInt, sampleZero, shape.Bits), true
	case symbol.FormBool:
		return emit.Literal(emit.LiteralBool, falseSpelling), true
	case symbol.FormText:
		return emit.Literal(emit.LiteralString, ""), true
	case symbol.FormReference:
		switch shape.Ref {
		case rules.WellKnownTimestamp:
			return emit.Composite(rules.EmitRef(ref)), true
		case rules.WellKnownDuration:
			return emit.Conversion(rules.EmitRef(ref), emit.Literal(emit.LiteralInt, sampleZero)), true
		}
	default:
		// Every other form takes the zero of its name below.
	}
	switch nameOf(ref) {
	case anyType, errorType, unsafePointerType:
		return emit.Literal(emit.LiteralNil, ""), true
	default:
		return emit.Value{}, false
	}
}

// LiteralFor lifts a Go literal into the value tree, typed by the
// reference. A literal is nil, one of the two booleans, an integer in any
// base that Go reads, a float, a rune as its integer value, or a quoted
// or raw string. The scanner of Go reads the text, and LiteralFor writes
// the value back as canonical decimal text, which every target reads
// alike.
//
// Where the reference is a builtin, directly or through aliases in the
// view, the value must be a value of the builtin:
//   - An integer type takes an integral value inside its width.
//   - A float type takes a finite value inside its width.
//   - bool takes a truth value, and string takes a string.
//   - A number states the width of the builtin.
//
// A value type takes no nil. Any other text is no literal. The file has
// no effect, because a Go literal reads the same in every scope.
//
// # Allocation contract
//
// LiteralFor allocates the state of the scanner, the file set and the
// file, the lists of the tokens that it scans, the constant and the
// canonical text. A decimal integer costs six allocations.
func (r Rules) LiteralFor(
	_ *node.File,
	ref *node.TypeRef,
	text string,
	v rules.View,
) (emit.Value, bool) {
	lit, scanned := scanLiteral(strings.TrimSpace(text))
	if !scanned {
		return emit.Value{}, false
	}
	return lit.typed(r.literalShape(ref, v, 0))
}

// literal is one scanned Go literal. value is its constant value, and nil
// for the literal nil.
type literal struct {
	value constant.Value
}

// scanLiteral reads one Go literal. A literal is one of the keywords nil,
// true and false, a number after at most one sign, a rune or a string.
// Text with anything else, a comment included, is no literal.
func scanLiteral(text string) (literal, bool) {
	src := []byte(text)
	fset := token.NewFileSet()
	var s scanner.Scanner
	s.Init(fset.AddFile("", fset.Base(), len(src)), src, nil, scanner.ScanComments)
	var toks []token.Token
	var lits []string
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.SEMICOLON && lit == "\n" {
			continue // the semicolon that the scanner adds at a line end
		}
		toks = append(toks, tok)
		lits = append(lits, lit)
	}
	if s.ErrorCount > 0 {
		return literal{}, false
	}
	negate := false
	if len(toks) == 2 && (toks[0] == token.SUB || toks[0] == token.ADD) &&
		(toks[1] == token.INT || toks[1] == token.FLOAT) {

		negate = toks[0] == token.SUB
		toks, lits = toks[1:], lits[1:]
	}
	if len(toks) != 1 {
		return literal{}, false
	}
	switch toks[0] {
	case token.IDENT:
		switch lits[0] {
		case nilSpelling:
			return literal{}, true
		case trueSpelling, falseSpelling:
			return literal{value: constant.MakeBool(lits[0] == trueSpelling)}, true
		}
	case token.INT, token.FLOAT, token.CHAR, token.STRING:
		value := constant.MakeFromLiteral(lits[0], toks[0], 0)
		if value.Kind() == constant.Unknown {
			return literal{}, false
		}
		if negate {
			value = constant.UnaryOp(token.SUB, value, 0)
		}
		return literal{value: value}, true
	default:
		// Any other token is no literal.
	}
	return literal{}, false
}

// typed returns the literal as a value of a shape. A scalar, a
// boolean and a text shape take only their own values, and the zero
// shape, or any other, takes each literal as its own kind.
func (l literal) typed(shape rules.TypeShape) (emit.Value, bool) {
	if l.value == nil {
		switch shape.Form {
		case symbol.FormScalar, symbol.FormBool, symbol.FormText:
			return emit.Value{}, false
		default:
			return emit.Literal(emit.LiteralNil, ""), true
		}
	}
	switch shape.Form {
	case symbol.FormScalar:
		if shape.Class == rules.ScalarFloat {
			return numeric.Float(l.value, shape.Bits)
		}
		return numeric.Int(l.value, shape.Class, shape.Bits)
	case symbol.FormBool:
		if l.value.Kind() != constant.Bool {
			return emit.Value{}, false
		}
		return emit.Literal(emit.LiteralBool, l.value.ExactString()), true
	case symbol.FormText:
		if l.value.Kind() != constant.String {
			return emit.Value{}, false
		}
		return emit.Literal(emit.LiteralString, constant.StringVal(l.value)), true
	default:
		// Any other shape takes each literal as its own kind below.
	}
	switch l.value.Kind() {
	case constant.Bool:
		return emit.Literal(emit.LiteralBool, l.value.ExactString()), true
	case constant.String:
		return emit.Literal(emit.LiteralString, constant.StringVal(l.value)), true
	case constant.Int:
		return emit.Literal(emit.LiteralInt, l.value.ExactString()), true
	case constant.Float:
		return numeric.Float(l.value, platformWidth)
	default:
		return emit.Value{}, false
	}
}

// literalShape returns the shape against which a literal is typed. A
// builtin has the shape of the table, a workspace alias has the shape of
// its target, and a structural reference has its form. A reference that
// the rules cannot place, and a nil reference, return the zero shape,
// which types nothing.
func (r Rules) literalShape(ref *node.TypeRef, v rules.View, depth int) rules.TypeShape {
	switch {
	case ref == nil || depth > deriveDepth:
		return rules.TypeShape{}
	case ref.Form != symbol.FormNamed:
		return rules.TypeShape{Form: ref.Form}
	case ref.Target.IsZero():
		return r.Builtin(ref, v)
	}
	sym, held := v.Lookup(ref.Target)
	if a, is := sym.(*node.Alias); held && is && a.Target != nil {
		return r.literalShape(a.Target, v, depth+1)
	}
	return rules.TypeShape{}
}

// addressed takes the address of a derived composite, and refuses
// any other value with no literal, because Go takes the address of
// a composite literal alone: &42 does not compile. A refusal passes
// through.
func addressed(s rules.Sample) rules.Sample {
	switch {
	case !s.OK():
		return s
	case s.Value.Kind != emit.ValueComposite:
		return rules.Refused(rules.RefusedNoLiteral)
	default:
		return rules.Of(emit.Address(s.Value))
	}
}

// converted returns the wrap that converts a value to a type.
func converted(t *emit.TypeRef) func(emit.Value) emit.Value {
	return func(inner emit.Value) emit.Value { return emit.Conversion(t, inner) }
}

// elements returns the wrap that places one value as the one element of a
// slice or an array of a type.
func elements(t *emit.TypeRef) func(emit.Value) emit.Value {
	return func(inner emit.Value) emit.Value {
		return emit.Composite(t, emit.Element(inner))
	}
}

// child returns the child at index i of a structural reference, or nil.
func child(ref *node.TypeRef, i int) *node.TypeRef {
	if i < len(ref.Elems) {
		return ref.Elems[i]
	}
	return nil
}
