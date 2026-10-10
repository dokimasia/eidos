// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"slices"
	"strings"

	"go.dokimi.dev/eidos/lang/numeric"
	typescript "go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// deriveDepth bounds a derivation through self-referential types. An
// interface with a field of its own type derives this many levels deep,
// and refuses below that level.
const deriveDepth = 8

// The fixed pairs and zeros of the builtin types, and the hint of a
// string when the caller gives none. A Date and a Uint8Array are values
// that a constructor returns, so they are TypeScript text.
const (
	sampleNumber    = "1.5"
	alternateNumber = "2.5"
	sampleBigInt    = "42n"
	alternateBigInt = "7n"
	sampleDate      = "new Date(1700000000000)"
	alternateDate   = "new Date(1700000001000)"
	sampleBytes     = "new Uint8Array([1])"
	alternateBytes  = "new Uint8Array([2])"
	zeroNumber      = "0"
	defaultHint     = "sample"
	sampleSuffix    = "-a"
	alternateSuffix = "-b"
)

// openers and closers are the brackets of a type spelling. arrowHead is
// the arrow of a function type, whose > closes no bracket. unionSep
// separates the members of a union.
const (
	openers   = "(<[{"
	closers   = ")>]}"
	arrowHead = "=>"
	unionSep  = '|'
)

// keywordTypes lists the keyword types of TypeScript. The builtin table
// classifies them as Opaque, and SamplesOf derives no value for them.
var keywordTypes = []string{
	"any", "unknown", "never", "object", "symbol", "void", spellNull, spellUndefined, "this",
}

// SamplesOf derives two distinguishable values of a type:
//
//   - number, string and boolean take a fixed pair, and the pair of a
//     string contains the hint;
//   - bigint, Date and Uint8Array take a fixed pair of TypeScript text;
//   - a literal type takes its one value, and refuses the second value;
//   - a list, an Array and a ReadonlyArray take an array of one element,
//     and the two arrays differ in the element;
//   - a tuple takes an array of one value per member;
//   - a map of an index signature and a Record take an object of one
//     entry, and the two objects differ in the key;
//   - an optional takes the pair of its type;
//   - a union takes the first two distinct values that its members
//     derive, in member order;
//   - a class and an interface take an object literal that sets one
//     property. The literal is asserted to the type, because it sets no
//     other property;
//   - an enum takes its first two members with distinct values. Each
//     value is asserted to the enum's type;
//   - an alias takes the pair of its target, with the arguments
//     substituted.
//
// The key of a map is a string or a number. The property of an object
// literal is the first required property, or the first property when
// the type has no required one.
//
// The values that an author stated come first for an element, a member,
// the value of a key and a property. SamplesOf reads them through
// [rules.View.Authored], and derives each value without an author's
// statement. A Map, a ReadonlyMap, a stream, a function type, an
// intersection and an inline object type refuse with no literal,
// because the value model has no constructor call and no merge. A
// keyword type, such as unknown, refuses with no literal too. A named
// type outside the view refuses as unresolved.
//
// # Allocation contract
//
//   - A builtin's pair allocates the hinted texts of a string, and
//     nothing else.
//   - A composite's pair allocates the reference to its type and the list
//     of each composite.
//   - An object's pair allocates the member walk and the assertion.
//   - An enum's pair allocates its list of members and the assertion of
//     each value.
func (r Rules) SamplesOf(ref *node.TypeRef, hint string, v rules.View) (rules.Sample, rules.Sample) {
	return r.derive(ref, hint, v, 0)
}

// derive derives the pair of [Rules.SamplesOf] at a depth of the walk.
func (r Rules) derive(ref *node.TypeRef, hint string, v rules.View, depth int) (rules.Sample, rules.Sample) {
	if ref == nil {
		return rules.RefusedPair(rules.RefusedNoLiteral)
	}
	if depth > deriveDepth {
		return rules.RefusedPair(rules.RefusedDepth)
	}
	switch ref.Form {
	case symbol.FormOptional:
		return r.derive(child(ref, 0), hint, v, depth+1)
	case symbol.FormList:
		return r.listPair(rules.EmitRef(ref), child(ref, 0), hint, v, depth)
	case symbol.FormMap:
		return r.mapPair(rules.EmitRef(ref), child(ref, 0), child(ref, 1), hint, v, depth)
	case symbol.FormTuple:
		return r.tuplePair(ref, hint, v, depth)
	case symbol.FormUnion:
		return r.unionPair(ref, hint, v, depth)
	case symbol.FormNamed:
		if ref.Target.IsZero() {
			return r.builtinPair(ref, hint, v, depth)
		}
		return r.declaredPair(ref, hint, v, depth)
	default:
		return rules.RefusedPair(rules.RefusedNoLiteral)
	}
}

// partPair returns the pair of one part of a value. The values that an
// author stated on the part's declaration and on its type come first,
// and partPair derives each half without an author's statement. An
// element, a member of a tuple or a union, and the value of a key have
// no declaration of their own, so subject is zero for them.
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

// listPair returns the pair of an array of the type t. Each array has
// one element, from the pair of the element type elem.
func (r Rules) listPair(
	t *emit.TypeRef, elem *node.TypeRef, hint string, v rules.View, depth int,
) (rules.Sample, rules.Sample) {
	sample, alternate := r.partPair(symbol.Identity{}, elem, hint, v, depth+1)
	if !sample.OK() || !alternate.OK() {
		return rules.RefusedPair(rules.FirstRefusal(sample, alternate))
	}
	return rules.Of(emit.Composite(t, emit.Element(sample.Value))),
		rules.Of(emit.Composite(t, emit.Element(alternate.Value)))
}

// mapPair returns the pair of an object of the type t, which maps keys
// of the type key to values of the type value. Each object has one
// entry, and the two entries differ in the key. A key other than a
// string or a number refuses with no literal. An object of another key
// type, such as a Record over an enum, needs an entry for every member.
func (r Rules) mapPair(
	t *emit.TypeRef, key, value *node.TypeRef, hint string, v rules.View, depth int,
) (rules.Sample, rules.Sample) {
	if key == nil {
		return rules.RefusedPair(rules.RefusedNoLiteral)
	}
	if form := r.Builtin(unaliased(key, v), v).Form; form != symbol.FormText && form != symbol.FormScalar {
		return rules.RefusedPair(rules.RefusedNoLiteral)
	}
	k, otherKey := r.derive(key, hint, v, depth+1)
	val, _ := r.partPair(symbol.Identity{}, value, hint, v, depth+1)
	if !k.OK() || !otherKey.OK() || !val.OK() {
		return rules.RefusedPair(rules.FirstRefusal(k, otherKey, val))
	}
	return rules.Of(emit.Composite(t, emit.KeyedEntry(k.Value, val.Value))),
		rules.Of(emit.Composite(t, emit.KeyedEntry(otherKey.Value, val.Value)))
}

// tuplePair returns the pair of a tuple. The sample is an array of the
// samples of the members, and the alternate is an array of their
// alternates.
func (r Rules) tuplePair(ref *node.TypeRef, hint string, v rules.View, depth int) (rules.Sample, rules.Sample) {
	if len(ref.Elems) == 0 {
		return rules.RefusedPair(rules.RefusedNoLiteral)
	}
	samples := make([]emit.ValueField, 0, len(ref.Elems))
	alternates := make([]emit.ValueField, 0, len(ref.Elems))
	for _, e := range ref.Elems {
		sample, alternate := r.partPair(symbol.Identity{}, e, hint, v, depth+1)
		if !sample.OK() || !alternate.OK() {
			return rules.RefusedPair(rules.FirstRefusal(sample, alternate))
		}
		samples = append(samples, emit.Element(sample.Value))
		alternates = append(alternates, emit.Element(alternate.Value))
	}
	t := rules.EmitRef(ref)
	return rules.Of(emit.Composite(t, samples...)), rules.Of(emit.Composite(t, alternates...))
}

// unionPair returns the first two distinct values that the members of a
// union derive, in member order. The values of a member are the halves
// of its pair, or the one value of a literal type. unionPair refuses with
// the first member's refusal when the members derive fewer than two
// values.
func (r Rules) unionPair(ref *node.TypeRef, hint string, v rules.View, depth int) (rules.Sample, rules.Sample) {
	var sample rules.Sample
	why := rules.RefusedNoLiteral
	for i, m := range ref.Elems {
		s, a := r.partPair(symbol.Identity{}, m, hint, v, depth+1)
		if i == 0 {
			why = rules.FirstRefusal(s, a)
		}
		for _, c := range [2]rules.Sample{s, a} {
			switch {
			case !c.OK():
			case !sample.OK():
				sample = c
			case c.Value.Kind != emit.ValueLiteral || c.Value.Literal != sample.Value.Literal ||
				c.Value.Text != sample.Value.Text:
				return sample, c
			}
		}
	}
	return rules.RefusedPair(why)
}

// builtinPair derives the pair of a named reference without a target
// through the builtin table.
func (r Rules) builtinPair(ref *node.TypeRef, hint string, v rules.View, depth int) (rules.Sample, rules.Sample) {
	switch shape := r.Builtin(ref, v); shape.Form {
	case symbol.FormScalar:
		return rules.NumberPair(emit.LiteralFloat, sampleNumber, alternateNumber, numberBits)
	case symbol.FormBool:
		return rules.Pair(emit.LiteralBool, spellTrue, spellFalse)
	case symbol.FormText:
		if hint == "" {
			hint = defaultHint
		}
		return rules.Pair(emit.LiteralString, hint+sampleSuffix, hint+alternateSuffix)
	case symbol.FormBytes:
		return rules.Of(emit.Raw(typescript.Lang, sampleBytes)), rules.Of(emit.Raw(typescript.Lang, alternateBytes))
	case symbol.FormReference:
		return rules.Of(emit.Raw(typescript.Lang, sampleDate)), rules.Of(emit.Raw(typescript.Lang, alternateDate))
	case symbol.FormList:
		return r.listPair(rules.EmitRef(ref), ref.Args[0], hint, v, depth)
	case symbol.FormMap:
		if ref.Spelling != spellRecord || len(ref.Args) != 2 {
			return rules.RefusedPair(rules.RefusedNoLiteral)
		}
		return r.mapPair(rules.EmitRef(ref), ref.Args[0], ref.Args[1], hint, v, depth)
	case symbol.FormStream:
		return rules.RefusedPair(rules.RefusedNoLiteral)
	}
	switch {
	case ref.Package == "" && ref.Spelling == spellBigInt:
		return rules.Of(emit.Raw(typescript.Lang, sampleBigInt)), rules.Of(emit.Raw(typescript.Lang, alternateBigInt))
	case ref.Package == "" && slices.Contains(keywordTypes, ref.Spelling):
		return rules.RefusedPair(rules.RefusedNoLiteral)
	}
	if lit, scanned := scanLiteral(ref.Spelling); scanned && lit.kind != literalPath {
		value, _ := lit.value()
		return rules.Of(value), rules.Refused(rules.RefusedNoLiteral)
	}
	return rules.RefusedPair(rules.RefusedUnresolved)
}

// declaredPair derives the pair of a named type in the view.
func (r Rules) declaredPair(ref *node.TypeRef, hint string, v rules.View, depth int) (rules.Sample, rules.Sample) {
	sym, held := v.Lookup(ref.Target)
	if !held {
		return rules.RefusedPair(rules.RefusedUnresolved)
	}
	switch d := sym.(type) {
	case *node.Struct, *node.Interface:
		decl, _ := sym.(node.Declaration)
		return r.objectPair(ref, decl, v, depth)
	case *node.Enum:
		t := rules.EmitRef(ref)
		ms := members(d)
		for i, m := range ms {
			if !m.known {
				continue
			}
			for _, other := range ms[i+1:] {
				if other.known && other.value != m.value {
					return rules.Of(emit.Conversion(t, m.value.literal())),
						rules.Of(emit.Conversion(t, other.value.literal()))
				}
			}
			break
		}
		return rules.RefusedPair(rules.RefusedNoLiteral)
	case *node.Alias:
		// derive refuses an alias without a target as it refuses a nil
		// reference.
		return r.derive(r.Substitute(d.Target, d.TypeParams, ref.Args), hint, v, depth+1)
	default:
		return rules.RefusedPair(rules.RefusedNoLiteral)
	}
}

// objectPair derives the pair of a class or an interface. Each value is
// an object literal that sets one public instance property, and is
// asserted to the type. objectPair tries the required properties first
// and then the optional ones, each in the order of the member walk. The
// first property whose pair derives sets the pair.
func (r Rules) objectPair(
	ref *node.TypeRef, decl node.Declaration, v rules.View, depth int,
) (rules.Sample, rules.Sample) {
	set, _ := rules.NewBound(r, v, nil).MembersOf(decl)
	for _, optional := range [2]bool{false, true} {
		for _, m := range set.Members {
			f, isField := m.Symbol.(*node.Field)
			if !isField || f.Type == nil || f.Name == "" || f.Optional != optional ||
				f.Level == symbol.LevelType || f.Visibility != symbol.VisibilityPublic {
				continue
			}
			sample, alternate := r.partPair(f.ID, f.Type, f.Name, v, depth+1)
			if !sample.OK() || !alternate.OK() {
				continue
			}
			t := rules.EmitRef(ref)
			return rules.Of(emit.Conversion(t, emit.Composite(t, emit.NamedField(f.Name, sample.Value)))),
				rules.Of(emit.Conversion(t, emit.Composite(t, emit.NamedField(f.Name, alternate.Value))))
		}
	}
	return rules.RefusedPair(rules.RefusedNoLiteral)
}

// ZeroValue returns the zero value of a type in TypeScript:
//
//   - 0 for number, the empty string for string, false for boolean and 0n
//     for bigint;
//   - the empty array for a list, an Array and a ReadonlyArray;
//   - null for an optional whose union lists null and not undefined, and
//     undefined for every other optional;
//   - the zero of its target for an alias.
//
// The optional of an optional member, such as the a?: T of a tuple,
// lists neither null nor undefined, so its zero is undefined. Every
// other type has no zero, and ZeroValue reports false. A builtin's zero
// allocates nothing, and the empty array allocates the reference to its
// type.
func (r Rules) ZeroValue(ref *node.TypeRef, v rules.View) (emit.Value, bool) {
	for range deriveDepth {
		switch {
		case ref == nil:
			return emit.Value{}, false
		case ref.Form == symbol.FormOptional:
			if lists(ref.Spelling, spellNull) && !lists(ref.Spelling, spellUndefined) {
				return emit.Literal(emit.LiteralNil, ""), true
			}
			return emit.Raw(typescript.Lang, spellUndefined), true
		case ref.Form == symbol.FormList:
			return emit.Composite(rules.EmitRef(ref)), true
		case ref.Form != symbol.FormNamed:
			return emit.Value{}, false
		case ref.Target.IsZero():
			return r.builtinZero(ref, v)
		}
		sym, _ := v.Lookup(ref.Target)
		alias, isAlias := sym.(*node.Alias)
		if !isAlias || alias.Target == nil {
			return emit.Value{}, false
		}
		ref = r.Substitute(alias.Target, alias.TypeParams, ref.Args)
	}
	return emit.Value{}, false
}

// builtinZero returns the zero value of a named reference without a
// target through the builtin table.
func (r Rules) builtinZero(ref *node.TypeRef, v rules.View) (emit.Value, bool) {
	switch r.Builtin(ref, v).Form {
	case symbol.FormScalar:
		return emit.Number(emit.LiteralFloat, zeroNumber, numberBits), true
	case symbol.FormBool:
		return emit.Literal(emit.LiteralBool, spellFalse), true
	case symbol.FormText:
		return emit.Literal(emit.LiteralString, ""), true
	case symbol.FormList:
		return emit.Composite(rules.EmitRef(ref)), true
	}
	if ref.Package == "" && ref.Spelling == spellBigInt {
		return emit.Raw(typescript.Lang, zeroNumber+bigintMark), true
	}
	return emit.Value{}, false
}

// LiteralFor lifts a TypeScript literal into the value tree, as a value
// of the referenced type. LiteralFor reads these literals:
//
//   - a number in any radix of TypeScript, with separators;
//   - a bigint;
//   - a string, and a template literal without a substitution;
//   - true and false, null and undefined;
//   - a member of an enum, written as Member or Enum.Member.
//
// LiteralFor writes a number as the shortest decimal text that parses
// back to its double, and a bigint as decimal text with its n.
//
// Each type takes these literals:
//
//   - number, string, boolean and bigint each take a literal of their
//     own kind;
//   - a literal type takes its own value, null takes null, and undefined
//     and void take undefined;
//   - an enum takes a member whose value is known, and LiteralFor
//     asserts the member to the enum's type;
//   - an optional takes null when its union lists null, and undefined
//     when its union lists undefined or does not list null;
//   - an optional also takes every literal that its type takes;
//   - a union takes a literal that one of its members takes, as the first
//     such member types it;
//   - an alias takes what its target takes;
//   - any, unknown and a nil reference take every literal except a
//     member, as a value of the literal's own kind.
//
// Every other type does not take a literal. That includes a class, an
// interface and a type outside the view. The file plays no part, because a
// TypeScript literal has the same value in every module.
//
// # Allocation contract
//
// LiteralFor allocates:
//
//   - the decimal text of a number;
//   - the content of a string with an escape;
//   - the digits of a bigint with its n;
//   - for a member of an enum, the enum's list of members, the reference
//     to the enum and the assertion.
//
// A truth value, null, undefined and a string without an escape allocate
// nothing.
func (r Rules) LiteralFor(_ *node.File, ref *node.TypeRef, text string, v rules.View) (emit.Value, bool) {
	lit, scanned := scanLiteral(text)
	if !scanned {
		return emit.Value{}, false
	}
	return r.typed(lit, ref, v, 0)
}

// typed implements [Rules.LiteralFor] at a depth of the walk.
func (r Rules) typed(lit literal, ref *node.TypeRef, v rules.View, depth int) (emit.Value, bool) {
	switch {
	case ref == nil:
		return lit.value()
	case depth > deriveDepth:
		return emit.Value{}, false
	case ref.Form == symbol.FormOptional && lit.kind == literalNull:
		if !lists(ref.Spelling, spellNull) {
			return emit.Value{}, false
		}
		return lit.value()
	case ref.Form == symbol.FormOptional && lit.kind == literalUndefined:
		if lists(ref.Spelling, spellNull) && !lists(ref.Spelling, spellUndefined) {
			return emit.Value{}, false
		}
		return lit.value()
	case ref.Form == symbol.FormOptional:
		return r.typed(lit, child(ref, 0), v, depth+1)
	case ref.Form == symbol.FormUnion:
		for _, m := range ref.Elems {
			if value, typed := r.typed(lit, m, v, depth+1); typed {
				return value, true
			}
		}
		return emit.Value{}, false
	case ref.Form != symbol.FormNamed:
		return emit.Value{}, false
	case ref.Target.IsZero():
		return r.typedBuiltin(lit, ref, v)
	}
	sym, _ := v.Lookup(ref.Target)
	enum, isEnum := sym.(*node.Enum)
	alias, isAlias := sym.(*node.Alias)
	switch {
	case isEnum && lit.kind == literalPath:
		qualifier, name, qualified := strings.Cut(lit.text, pathSep)
		if !qualified {
			name = qualifier
		} else if qualifier != enum.Name {
			return emit.Value{}, false
		}
		for _, m := range members(enum) {
			if m.name == name && m.known {
				return emit.Conversion(rules.EmitRef(ref), m.value.literal()), true
			}
		}
		return emit.Value{}, false
	case isAlias && alias.Target != nil:
		return r.typed(lit, r.Substitute(alias.Target, alias.TypeParams, ref.Args), v, depth+1)
	default:
		return emit.Value{}, false
	}
}

// typedBuiltin returns a literal as a value of a named reference without
// a target.
func (r Rules) typedBuiltin(lit literal, ref *node.TypeRef, v rules.View) (emit.Value, bool) {
	var takes literalKind
	switch r.Builtin(ref, v).Form {
	case symbol.FormScalar:
		takes = literalNumber
	case symbol.FormBool:
		takes = literalBool
	case symbol.FormText:
		takes = literalString
	}
	switch {
	case takes != 0:
	case ref.Package != "":
		return emit.Value{}, false
	case ref.Spelling == spellBigInt:
		takes = literalBigInt
	case ref.Spelling == spellNull:
		takes = literalNull
	case ref.Spelling == spellUndefined || ref.Spelling == "void":
		takes = literalUndefined
	case ref.Spelling == "any" || ref.Spelling == "unknown":
		return lit.value()
	default:
		own, scanned := scanLiteral(ref.Spelling)
		if !scanned || own.kind == literalPath || own != lit {
			return emit.Value{}, false
		}
		return lit.value()
	}
	if lit.kind != takes {
		return emit.Value{}, false
	}
	return lit.value()
}

// lists reports whether the union of a type spelling has a member at its
// top level, outside every bracket and every quoted literal. A type that
// is not a union has one member, the type itself.
func lists(spelling, member string) bool {
	depth, start := 0, 0
	var quote byte
	for i := 0; i < len(spelling); i++ {
		c := spelling[i]
		switch {
		case quote != 0 && c == escapeMark:
			i++
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case strings.IndexByte(quotes, c) >= 0:
			quote = c
		case strings.IndexByte(openers, c) >= 0:
			depth++
		case strings.IndexByte(closers, c) >= 0 && !strings.HasSuffix(spelling[:i+1], arrowHead):
			depth--
		case c == unionSep && depth == 0:
			if strings.TrimSpace(spelling[start:i]) == member {
				return true
			}
			start = i + 1
		}
	}
	return strings.TrimSpace(spelling[start:]) == member
}

// child returns the child at index i of a structural reference, or nil
// when the reference has no such child.
func child(ref *node.TypeRef, i int) *node.TypeRef {
	if i < len(ref.Elems) {
		return ref.Elems[i]
	}
	return nil
}

// value returns the literal as a value of its own kind. A number has
// TypeScript's width, and a bigint and undefined are TypeScript text. A
// path refers to a member of an enum, which only the enum's type can
// resolve, so value reports false for a path.
func (lit literal) value() (emit.Value, bool) {
	switch lit.kind {
	case literalNumber:
		return emit.Number(emit.LiteralFloat, numeric.Decimal(lit.number, numberBits), numberBits), true
	case literalBigInt:
		return emit.Raw(typescript.Lang, lit.text+bigintMark), true
	case literalString:
		return emit.Literal(emit.LiteralString, lit.text), true
	case literalBool:
		return emit.Literal(emit.LiteralBool, lit.text), true
	case literalNull:
		return emit.Literal(emit.LiteralNil, ""), true
	case literalUndefined:
		return emit.Raw(typescript.Lang, spellUndefined), true
	default:
		return emit.Value{}, false
	}
}

// literal returns the value of an enum member as a literal. A text is a
// string, and any other value is a number of TypeScript's width.
func (v enumValue) literal() emit.Value {
	if v.isText {
		return emit.Literal(emit.LiteralString, v.text)
	}
	return emit.Number(emit.LiteralFloat, numeric.Decimal(v.number, numberBits), numberBits)
}
