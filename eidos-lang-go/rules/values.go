// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"go/constant"
	"go/scanner"
	"go/token"
	"math"
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
// a struct with a pointer to itself derives its field's value
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
	sampleZero       = "0"
	sampleSeconds    = "1700000000"
	alternateSeconds = "1700000001"
	defaultHint      = "sample"
	sampleSuffix     = "-a"
	alternateSuffix  = "-b"
	timePackage      = "time"
	timeUnix         = "Unix"
	// stringQuote opens the exact value of a string constant.
	stringQuote = `"`
)

// Go's keyword literals: the absent value and the two truth values.
const (
	nilSpelling   = "nil"
	trueSpelling  = "true"
	falseSpelling = "false"
)

// The widths in bits the literal text is written at: a
// platform-sized number states none, and a float then writes at
// float64's precision.
const (
	platformWidth = 0
	float32Width  = 32
	float64Width  = 64
)

// SamplesOf derives a type's two distinguishable values: a builtin's
// pair from the fixed table, a defined type's as a conversion of
// its underlying type's, a struct's as a composite setting its
// first settable exported field, a slice's and an array's as a
// one-element composite differing in the element, a map's as a
// one-entry composite differing in the key, a pointer's as the
// address of its inner composite, time.Time as a call and
// time.Duration as a conversion. A number states the width of its
// builtin. An interface, a function type, a channel, an inline body,
// a predeclared type without a literal and a pointer to anything but
// a composite refuse with no literal, because Go takes the address
// of a composite literal alone. A named type outside the view
// refuses as unresolved.
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
		return addressed(sample), addressed(alternate)
	case symbol.FormList, symbol.FormArray:
		sample, alternate := r.derive(child(ref, 0), hint, v, depth+1)
		return lift(sample, elements(ref)), lift(alternate, elements(ref))
	case symbol.FormMap:
		key, otherKey := r.derive(child(ref, 0), hint, v, depth+1)
		value, _ := r.derive(child(ref, 1), hint, v, depth+1)
		if !key.OK() || !otherKey.OK() || !value.OK() {
			return refused(firstRefusal(key, otherKey, value))
		}
		return rules.Of(entry(ref, key.Value, value.Value)),
			rules.Of(entry(ref, otherKey.Value, value.Value))
	case symbol.FormNamed:
		if ref.Target.IsZero() {
			return r.builtinPair(ref, hint)
		}
		return r.declaredPair(ref, hint, v, depth)
	default:
		return refused(rules.RefusedNoLiteral)
	}
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
			return rules.Of(emit.Composite(t, emit.NamedField(f.Name, sample.Value))),
				rules.Of(emit.Composite(t, emit.NamedField(f.Name, alternate.Value)))
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

// builtinPair derives the pair of a spelling the resolution step
// left without a target, through the builtin table: a number's pair
// at the builtin's width, small for byte and rune, a boolean's, a
// string's with the hint, time.Time's as a call and
// time.Duration's as a conversion. A predeclared type the table
// places nowhere refuses with no literal, and any other spelling
// refuses as unresolved.
func (r Rules) builtinPair(ref *node.TypeRef, hint string) (rules.Sample, rules.Sample) {
	shape := r.Builtin(ref, rules.View{})
	switch shape.Form {
	case symbol.FormScalar:
		spelling := named(ref)
		switch {
		case shape.Class == rules.ScalarFloat:
			return numbers(emit.LiteralFloat, sampleFloat, alternateFloat, shape.Bits)
		case spelling == spellByte || spelling == spellUint8 || spelling == spellRune:
			return numbers(emit.LiteralInt, sampleSmall, alternateSmall, shape.Bits)
		default:
			return numbers(emit.LiteralInt, sampleInt, alternateInt, shape.Bits)
		}
	case symbol.FormBool:
		return pair(emit.LiteralBool, trueSpelling, falseSpelling)
	case symbol.FormText:
		if hint == "" {
			hint = defaultHint
		}
		return pair(emit.LiteralString, hint+sampleSuffix, hint+alternateSuffix)
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
	}
	if golang.Predeclared(named(ref)) || named(ref) == spellUnsafePointer {
		return refused(rules.RefusedNoLiteral)
	}
	return refused(rules.RefusedUnresolved)
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
			return emit.Raw(golang.Lang, texts[0]), emit.Raw(golang.Lang, texts[1]), true
		}
	}
	return emit.Value{}, emit.Value{}, false
}

// enumZero returns the zero of an enumeration's underlying type,
// read off the first variant value the frontend stamped: the empty
// string where that value is a string, zero where it is a number,
// and false where no variant has a stamped value.
func (r Rules) enumZero(e *node.Enum, v rules.View) (emit.Value, bool) {
	key, held := r.constKey(v)
	if !held {
		return emit.Value{}, false
	}
	for _, variant := range e.Variants {
		if variant == nil {
			continue
		}
		text, stamped := rules.Fact(v, variant.ID, key)
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

// ZeroValue returns the zero value of a type as Go spells it: nil
// for a pointer, a slice, a map, a function, a channel, an interface
// and unsafe.Pointer, an empty composite for a struct and an array,
// the builtin zeros at the builtin's width, a conversion of the
// underlying zero for a defined type and for an enumeration whose
// variants' values name its underlying type, and false for a
// spelling the rules cannot place.
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

// builtinZero returns the zero value of a spelling the resolution
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
	}
	switch named(ref) {
	case spellAny, errorSpelling, spellUnsafePointer:
		return emit.Literal(emit.LiteralNil, ""), true
	default:
		return emit.Value{}, false
	}
}

// LiteralFor lifts a Go literal into the value tree, typed by the
// reference: nil, the two booleans, an integer in any base Go reads,
// a float, a rune as its integer value, and a quoted or raw string.
// Go's own scanner reads the text, and the value is written back as
// canonical decimal text, which every target reads alike. Where the
// reference names a builtin, directly or through aliases in the
// view, the value must be one of the builtin: an integer type takes
// an integral value inside its width, a float type a finite value
// inside its width, bool a truth value and string a string, and a
// number states the builtin's width. A value type takes no nil. Any
// other text is no literal. The file plays no part, because a Go
// literal reads the same in every scope.
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

// literal is one scanned Go literal: its constant value, and none
// for nil.
type literal struct {
	value constant.Value
}

// scanLiteral reads one Go literal: a keyword among nil, true and
// false, a number behind at most one sign, a rune or a string. Text
// with anything else, a comment included, is no literal.
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
			continue // the scanner's own line-end semicolon
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
		}
		return emit.Literal(emit.LiteralNil, ""), true
	}
	switch shape.Form {
	case symbol.FormScalar:
		if shape.Class == rules.ScalarFloat {
			return floatValue(l.value, shape.Bits)
		}
		return intValue(l.value, shape.Class, shape.Bits)
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
	}
	switch l.value.Kind() {
	case constant.Bool:
		return emit.Literal(emit.LiteralBool, l.value.ExactString()), true
	case constant.String:
		return emit.Literal(emit.LiteralString, constant.StringVal(l.value)), true
	case constant.Int:
		return emit.Literal(emit.LiteralInt, l.value.ExactString()), true
	case constant.Float:
		return floatValue(l.value, platformWidth)
	default:
		return emit.Value{}, false
	}
}

// intValue returns an integral value inside an integer builtin's
// range as decimal text. The platform width bounds as 64 bits.
func intValue(value constant.Value, class rules.ScalarClass, bits int) (emit.Value, bool) {
	n := constant.ToInt(value)
	if n.Kind() != constant.Int {
		return emit.Value{}, false
	}
	width := bits
	if width == platformWidth {
		width = float64Width
	}
	one := constant.MakeInt64(1)
	var lo, hi constant.Value
	if class == rules.ScalarUint {
		lo = constant.MakeInt64(0)
		hi = constant.BinaryOp(constant.Shift(one, token.SHL, uint(width)), token.SUB, one)
	} else {
		half := constant.Shift(one, token.SHL, uint(width-1))
		lo = constant.UnaryOp(token.SUB, half, 0)
		hi = constant.BinaryOp(half, token.SUB, one)
	}
	if constant.Compare(n, token.LSS, lo) || constant.Compare(n, token.GTR, hi) {
		return emit.Value{}, false
	}
	return emit.Number(emit.LiteralInt, n.ExactString(), bits), true
}

// floatValue returns a finite value inside a float builtin's range
// as decimal text at the builtin's precision.
func floatValue(value constant.Value, bits int) (emit.Value, bool) {
	x := constant.ToFloat(value)
	if x.Kind() != constant.Float {
		return emit.Value{}, false
	}
	width := float64Width
	f, _ := constant.Float64Val(x)
	if bits == float32Width {
		width = float32Width
		narrow, _ := constant.Float32Val(x)
		f = float64(narrow)
	}
	if math.IsInf(f, 0) {
		return emit.Value{}, false
	}
	return emit.Number(emit.LiteralFloat, decimal(f, width), bits), true
}

// decimal writes a float as the shortest decimal text that reads
// back to it at a precision: positional notation from 1e-6 up to
// 1e21, exponent notation outside it with the exponent unpadded,
// the rule encoding/json writes floats by, cutoffs compared at the
// same precision.
func decimal(f float64, bits int) string {
	format := byte('f')
	if abs := math.Abs(f); abs != 0 {
		narrow := float32(abs)
		if bits == float64Width && (abs < 1e-6 || abs >= 1e21) ||
			bits == float32Width && (narrow < 1e-6 || narrow >= 1e21) {
			format = 'e'
		}
	}
	b := strconv.AppendFloat(nil, f, format, -1, bits)
	if n := len(b); format == 'e' && n >= 4 && b[n-4] == 'e' && b[n-3] == '-' && b[n-2] == '0' {
		b[n-2] = b[n-1]
		b = b[:n-1]
	}
	return string(b)
}

// literalShape returns the shape a literal is typed against: a
// builtin's through the table, a workspace alias's through its
// target, and a structural reference's form. A reference the rules
// cannot place, and no reference, return the zero shape, which types
// nothing.
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
		return emit.Composite(t, emit.Element(inner))
	}
}

// entry returns a map composite with one keyed entry.
func entry(ref *node.TypeRef, key, value emit.Value) emit.Value {
	return emit.Composite(rules.EmitRef(ref), emit.KeyedEntry(key, value))
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
