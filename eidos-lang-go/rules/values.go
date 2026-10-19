// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"go/constant"
	"go/scanner"
	"go/token"
	"math/big"
	"slices"
	"strconv"
	"strings"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/numeric"
	"go.dokimi.dev/eidos/sdk/directive"
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
	// stringQuote opens the exact value of a string constant, and
	// fractionSep separates the numerator and the denominator of the exact
	// value of a fractional float constant, such as 3/2.
	stringQuote = `"`
	fractionSep = "/"
	// structKeyword opens an inline struct body.
	structKeyword = "struct"
	// callParens closes the call of a function without arguments.
	callParens = "()"
)

// The kinds of the declarations that a name and a call of [Rules.LiteralFor]
// refer to. The variant of an enumeration is a constant of Go.
var (
	valueKinds    = []symbol.Kind{symbol.KindConstant, symbol.KindVariable, symbol.KindEnumVariant}
	functionKinds = []symbol.Kind{symbol.KindFunction}
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
//   - the pair of a builtin from the fixed table, and two floats for a
//     complex number, which Go converts to it
//   - the pair of a defined type as a conversion of the pair of its
//     underlying type
//   - the pair of an enumeration as conversions of the exact values of
//     its first two variants with distinct values, each typed as a literal
//     of its underlying type, or else as conversions of the pair of its
//     underlying type
//   - the pair of a struct as a composite that sets its first exported
//     field whose pair derives
//   - the pair of a slice or an array as a one-element composite that
//     differs in the element
//   - the pair of a map as a one-entry composite that differs in the key
//   - the pair of a pointer as the address of its inner composite
//   - time.Time as a call, and time.Duration as a conversion, with or
//     without a target, because both are well-known types
//   - the one value of a struct without fields, struct{}{}, beside an
//     alternate that refuses with no literal
//
// A map of empty structs, the set of Go, derives its pair from the pair of
// its keys with struct{}{} as the value of each entry. A slice of empty
// structs derives no alternate, because the halves of its pair differ in
// the element.
//
// The type arguments of a reference to a generic type replace its type
// parameters in the type of each field and in the target of a defined
// type, through [rules.Substitute], so a field of Box[int] that T types
// derives the pair of int. A number states the width of its builtin. An
// interface, a function type, a channel, an inline body with members, a
// predeclared type without a literal and a pointer to anything but a
// composite refuse with no literal, because Go takes the address of a
// composite literal alone. A named type outside the view refuses as
// unresolved.
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
		if _, known := r.WellKnown(ref.Target); ref.Target.IsZero() || known {
			return r.builtinPair(ref, hint)
		}
		return r.declaredPair(ref, hint, v, depth)
	case symbol.FormInline:
		if len(ref.Fields) > 0 || len(ref.Methods) > 0 || !strings.HasPrefix(named(ref), structKeyword) {
			return rules.RefusedPair(rules.RefusedNoLiteral)
		}
		// Every value of a struct without fields is the one value struct{}{}.
		return rules.Of(emit.Composite(rules.EmitRef(ref))), rules.Refused(rules.RefusedNoLiteral)
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
			typ := rules.Substitute(f.Type, d.TypeParams, ref.Args)
			sample, alternate := r.partPair(f.ID, typ, f.Name, v, depth+1)
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
		sample, alternate := r.derive(rules.Substitute(d.Target, d.TypeParams, ref.Args), hint, v, depth+1)
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
		sample, alternate := r.derive(d.Underlying, hint, v, depth+1)
		return rules.Lift(sample, converted(t)), rules.Lift(alternate, converted(t))
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
	if t := nameOf(ref); t == complex64Type || t == complex128Type {
		return rules.Pair(emit.LiteralFloat, sampleFloat, alternateFloat)
	}
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
// an enumeration of the view whose exact values differ, each typed as a
// literal of the underlying type of the enumeration. It reports false
// where fewer than two variants have distinct exact values of that type.
// A loaded enumeration has no nil variant. It allocates the rational
// number of a fraction.
func (r Rules) twoVariantValues(e *node.Enum, v rules.View) (emit.Value, emit.Value, bool) {
	shape := r.literalShape(e.Underlying, v, 0)
	var values [2]emit.Value
	var texts [2]string
	found := 0
	for _, variant := range e.Variants {
		text, stamped := rules.Fact(v, variant.ID, constValue)
		if !stamped || found == 1 && text == texts[0] {
			continue
		}
		exact, read := exactConstant(text)
		if !read {
			continue
		}
		value, typed := literal{value: exact}.typed(shape)
		if !typed {
			continue
		}
		values[found], texts[found] = value, text
		if found++; found == len(values) {
			return values[0], values[1], true
		}
	}
	return emit.Value{}, emit.Value{}, false
}

// exactConstant reads an exact value that the frontend stamps as a
// constant: a quoted string, a truth value, an integer, or a fraction such
// as 3/2 for a float. It reports false for any other text, and allocates
// the rational number of a fraction.
func exactConstant(text string) (constant.Value, bool) {
	switch {
	case strings.HasPrefix(text, stringQuote):
		s, err := strconv.Unquote(text)
		return constant.MakeString(s), err == nil
	case text == trueSpelling || text == falseSpelling:
		return constant.MakeBool(text == trueSpelling), true
	case !strings.Contains(text, fractionSep):
		value := constant.MakeFromLiteral(text, token.INT, 0)
		return value, value.Kind() == constant.Int
	}
	rat, read := new(big.Rat).SetString(text)
	if !read {
		return nil, false
	}
	return constant.Make(rat), true
}

// ZeroValue returns the zero value of a type as Go spells it:
//   - nil for a pointer, a slice, a map, a function, a channel, an
//     interface, an inline interface included, and unsafe.Pointer
//   - an empty composite for a struct, an inline struct and an array
//   - the builtin zeros at the width of the builtin, and 0 for a complex
//     number
//   - the zeros of time.Time and time.Duration from the builtin table,
//     with or without a target
//   - a conversion of the underlying zero for a defined type and for an
//     enumeration
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
	case symbol.FormInline:
		switch spelling := named(ref); {
		case strings.HasPrefix(spelling, interfaceKeyword):
			return emit.Literal(emit.LiteralNil, ""), true
		case strings.HasPrefix(spelling, structKeyword):
			return emit.Composite(rules.EmitRef(ref)), true
		default:
			return emit.Value{}, false
		}
	case symbol.FormNamed:
	default:
		return emit.Value{}, false
	}
	if _, known := r.WellKnown(ref.Target); ref.Target.IsZero() || known {
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
		inner, placed := r.ZeroValue(d.Underlying, v)
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
	if t := nameOf(ref); t == complex64Type || t == complex128Type {
		return emit.Literal(emit.LiteralInt, sampleZero), true
	}
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

// LiteralFor lifts Go text into the value tree. It reads three forms of
// text in this order:
//   - A literal, typed by the reference. A literal is nil, one of the two
//     booleans, an integer in any base that Go reads, a float, a rune as
//     its integer value, or a quoted or raw string.
//   - The name of a constant or a variable in the scope s, bare or
//     qualified, as [emit.Name] of the declaration. The variant of an
//     enumeration is a constant.
//   - A call of a function in the scope s without arguments, such as New()
//     or store.New(), as [emit.Call] of the function.
//
// The scanner of Go reads a literal, and LiteralFor writes it back as
// canonical decimal text, which every target reads alike. Where the
// reference is a builtin, directly or through aliases in the view, the
// literal must be a value of the builtin:
//   - An integer type takes an integral value inside its width.
//   - A float type takes a finite value inside its width.
//   - bool takes a truth value, and string takes a string.
//   - A number states the width of the builtin.
//
// A value type takes no nil. A name resolves as [directive.ResolvePackageVar]
// resolves it, and a call as [directive.ResolveCallableInScope] resolves
// it, from the subject of s. A call of a method does not resolve, because
// it needs a receiver. LiteralFor does not check the type of a name or a
// call. A qualified name or call whose qualifier the file of s imports
// from a package outside the view is a stand-in under the import path,
// with the kind of a constant or a function. Any other text is no value.
//
// # Allocation contract
//
// A literal allocates the state of the scanner, the file set and the
// file, the lists of the tokens that it scans, the constant and the
// canonical text. A decimal integer costs six allocations. A name and a
// call allocate no scanner. A qualified name or call allocates the
// bindings of the imports of the file, three allocations for a file of up
// to eight imports without a dot import, which is the whole cost of a
// stand-in. A name or a call that resolves in the view allocates the
// bindings and the list of candidates of the resolution, four
// allocations, and a qualified one the bindings a second time.
func (r Rules) LiteralFor(
	s rules.Scope,
	ref *node.TypeRef,
	text string,
	v rules.View,
) (emit.Value, bool) {
	text = strings.TrimSpace(text)
	name, call := strings.CutSuffix(text, callParens)
	if !goName(name) || !call && (name == nilSpelling || name == trueSpelling || name == falseSpelling) {
		lit, scanned := scanLiteral(text)
		if !scanned {
			return emit.Value{}, false
		}
		return lit.typed(r.literalShape(ref, v, 0))
	}
	kind, kinds, standIn := directive.ResolvePackageVar, valueKinds, symbol.KindConstant
	if call {
		kind, kinds, standIn = directive.ResolveCallableInScope, functionKinds, symbol.KindFunction
	}
	// A qualifier that binds a package outside the view resolves to no
	// declaration of the view, so the stand-in comes first.
	id, found := outsideView(s, name, standIn, v)
	if !found {
		id, found = r.declaredValue(s, name, kind, kinds, v)
	}
	switch {
	case !found:
		return emit.Value{}, false
	case call:
		return emit.Call(id), true
	default:
		return emit.Name(id), true
	}
}

// declaredValue returns the identity of the declaration that name refers
// to in the scope at a resolution kind. It reports false where the name
// resolves to nothing, or to a declaration of none of the kinds.
func (r Rules) declaredValue(
	s rules.Scope, name string, kind directive.ResolutionKind, kinds []symbol.Kind, v rules.View,
) (symbol.Identity, bool) {
	sym, err := r.Resolve(s, name, kind, v)
	if err != nil {
		return symbol.Identity{}, false
	}
	decl, declares := sym.(node.Declaration)
	if !declares || !slices.Contains(kinds, decl.Identity().Kind) {
		return symbol.Identity{}, false
	}
	return decl.Identity(), true
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
// builtin has the shape of the table, an alias or a defined type of the
// view has the shape of its target, an enumeration of the view has the
// shape of its underlying type, and a structural reference has its form.
// A reference that the rules cannot place, and a nil reference, return the
// zero shape, which types nothing.
func (r Rules) literalShape(ref *node.TypeRef, v rules.View, depth int) rules.TypeShape {
	switch {
	case ref == nil || depth > deriveDepth:
		return rules.TypeShape{}
	case ref.Form != symbol.FormNamed:
		return rules.TypeShape{Form: ref.Form}
	case ref.Target.IsZero():
		return r.Builtin(ref, v)
	}
	sym, _ := v.Lookup(ref.Target)
	switch d := sym.(type) {
	case *node.Alias:
		return r.literalShape(d.Target, v, depth+1)
	case *node.Enum:
		return r.literalShape(d.Underlying, v, depth+1)
	default:
		return rules.TypeShape{}
	}
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
