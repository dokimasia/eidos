// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"slices"
	"strings"

	"go.dokimi.dev/eidos/lang/numeric"
	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// deriveDepth bounds a derivation through a type that refers to itself, such
// as a dataclass with a field of its own type.
const deriveDepth = 8

// The pairs and the zeros of the builtins, and the hint of a string where
// the caller names none. A bytes literal, a datetime and a timedelta have
// no literal of the value tree, so their pairs are Python text or
// constructions.
const (
	sampleInt       = "42"
	alternateInt    = "7"
	sampleFloat     = "1.5"
	alternateFloat  = "2.5"
	sampleBytes     = `b"a"`
	alternateBytes  = `b"b"`
	zeroBytes       = `b""`
	sampleYear      = "2023"
	sampleMonth     = "11"
	sampleDay       = "14"
	alternateDay    = "15"
	sampleDays      = "1"
	alternateDays   = "2"
	zeroNumber      = "0"
	defaultHint     = "sample"
	sampleSuffix    = "-a"
	alternateSuffix = "-b"
)

// DeriveSamples derives two values of a type that differ, for a check that
// must tell one from the other:
//
//   - int takes 42 and 7, float 1.5 and 2.5, bool True and False, and str
//     the hint with the suffixes -a and -b. bytes takes b"a" and b"b", and
//     bytearray their constructions bytearray(b"a") and bytearray(b"b");
//   - a datetime constructs 2023-11-14 and 2023-11-15, a timedelta one day
//     and two days, and an exception class the hinted strings;
//   - None takes None, and refuses the second value;
//   - a Literal takes its first two values, and its one value without a
//     second;
//   - a list and a sequence take a list of one element, and a dict and a
//     mapping take a dict of one entry, which differ in the key;
//   - a tuple takes a tuple of one value per member, and an optional takes
//     the pair of its type;
//   - a union takes the first two distinct values that its members derive;
//   - a class takes a call of its constructor with its required
//     parameters, and a TypedDict a dict of its required keys. The two calls
//     differ in the first parameter. A constructor without a required
//     parameter takes its first optional one;
//   - an enum takes its first two members of distinct values, each as the
//     name of the member, such as Color.RED;
//   - an alias takes the pair of its target, with its arguments
//     substituted, and a NewType the conversion of the pair of its base.
//
// A part of a value takes the values that an author stated for it first,
// through [rules.View.Authored], and the derivation completes each half
// that no author stated. A parameter of a constructor reads the values of
// the field of its name.
//
// Both halves refuse together, with one of these reasons:
//
//   - [rules.RefusedUnresolved]: a named type that the view does not
//     contain, and a name without a target outside the modules of the table
//     of [Rules.Builtin].
//   - [rules.RefusedNoLiteral]: a type without two values that a check can
//     write, such as a Protocol, a function type, a stream, the top type, a
//     set, a class of the modules of the table without a form, and a class
//     without a constructor whose parameters derive.
//   - [rules.RefusedDepth]: a type that refers to itself past the budget of
//     the walk.
//
// # Allocation contract
//
// A pair of a scalar allocates nothing beyond the two hinted texts of a
// string. A pair of a composite allocates the reference to its type and the
// list of fields of each composite. A pair of a constructor also allocates
// the constructors of the class and what the member walk allocates. A pair of
// an enum allocates its list of members.
func (r Rules) DeriveSamples(ref *node.TypeRef, hint string, v rules.View) (rules.Sample, rules.Sample) {
	return r.derive(ref, hint, v, 0)
}

// derive is [Rules.DeriveSamples] at a depth of the walk.
func (r Rules) derive(ref *node.TypeRef, hint string, v rules.View, depth int) (rules.Sample, rules.Sample) {
	switch {
	case ref == nil:
		return rules.RefusedPair(rules.RefusedNoLiteral)
	case depth > deriveDepth:
		return rules.RefusedPair(rules.RefusedDepth)
	}
	switch ref.Form {
	case symbol.FormOptional:
		return r.derive(firstElem(ref), hint, v, depth+1)
	case symbol.FormList:
		return r.listPair(rules.EmitRef(ref), firstElem(ref), hint, v, depth)
	case symbol.FormTuple:
		return r.tuplePair(ref, hint, v, depth)
	case symbol.FormUnion:
		return r.unionPair(ref, hint, v, depth)
	case symbol.FormNamed:
		if !ref.Target.IsZero() {
			return r.declaredPair(ref, hint, v, depth)
		}
		module, name, _ := builtinName(ref)
		s := r.Builtin(ref, v)
		if _, known := classified(module, name, ref); s.Form == symbol.FormOpaque && known {
			return rules.RefusedPair(rules.RefusedNoLiteral)
		}
		if s.Form == symbol.FormOpaque {
			return rules.RefusedPair(rules.RefusedUnresolved)
		}
		return r.namedPair(ref, module, name, s, hint, v, depth)
	default:
		return rules.RefusedPair(rules.RefusedNoLiteral)
	}
}

// partPair returns the pair of one part of a value: the values that an
// author stated for the part and for its type, and the derivation for each
// half that no author stated. An element, a member and an entry have no
// declaration of their own, so subject is zero for them.
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

// declaredPair derives the pair of a named type that the view contains: a
// well-known type, a class of the table of typeshed, a class, a TypedDict,
// an enum and an alias.
func (r Rules) declaredPair(ref *node.TypeRef, hint string, v rules.View, depth int) (rules.Sample, rules.Sample) {
	sym, held := v.Lookup(ref.Target)
	if !held {
		return rules.RefusedPair(rules.RefusedUnresolved)
	}
	if id, blessed := r.WellKnown(ref.Target); blessed {
		return wellKnownPair(ref, id, hint)
	}
	if s, shaped := r.ShapeOf(ref, sym, v); shaped {
		return r.namedPair(ref, ref.Target.Package, ref.Target.Name, s, hint, v, depth)
	}
	switch d := sym.(type) {
	case *node.Struct:
		return r.constructedPair(ref, d, v, depth)
	case *node.Interface:
		if typed, _ := rules.Fact(v, d.ID, meta.Named[bool](python.TypedDictKey)); !typed {
			return rules.RefusedPair(rules.RefusedNoLiteral)
		}
		return r.dictPair(ref, d, v, depth)
	case *node.Enum:
		return r.enumPair(d, v)
	case *node.Alias:
		target := rules.Substitute(d.Target, d.TypeParams, ref.Args)
		if !d.Defined {
			return r.derive(target, hint, v, depth+1)
		}
		t := rules.EmitRef(ref)
		wrap := func(x emit.Value) emit.Value { return emit.Conversion(t, x) }
		sample, alternate := r.derive(target, hint, v, depth+1)
		return rules.Lift(sample, wrap), rules.Lift(alternate, wrap)
	default:
		return rules.RefusedPair(rules.RefusedNoLiteral)
	}
}

// namedPair derives the pair of a reference ref to the name of the module
// whose values have the shape s: a builtin or a class of the table of
// typeshed, where the resolution step resolved it or not.
func (r Rules) namedPair(
	ref *node.TypeRef, module, name string, s rules.TypeShape, hint string, v rules.View, depth int,
) (rules.Sample, rules.Sample) {
	if name == literalForm && (module == typingModule || module == typingExtensions) {
		return literalPair(ref)
	}
	switch s.Form {
	case symbol.FormScalar:
		if s.Class == rules.ScalarFloat {
			return rules.NumberPair(emit.LiteralFloat, sampleFloat, alternateFloat, floatBits)
		}
		return rules.NumberPair(emit.LiteralInt, sampleInt, alternateInt, 0)
	case symbol.FormBool:
		return rules.Pair(emit.LiteralBool, trueText, falseText)
	case symbol.FormText:
		return textPair(hint)
	case symbol.FormBytes:
		sample, alternate := emit.Raw(python.Lang, sampleBytes), emit.Raw(python.Lang, alternateBytes)
		if module == builtinsModule && name == bytearrayClass {
			t := rules.EmitRef(ref)
			return rules.Of(emit.Construction(t, sample)), rules.Of(emit.Construction(t, alternate))
		}
		return rules.Of(sample), rules.Of(alternate)
	case symbol.FormList:
		return r.listPair(rules.EmitRef(ref), arg(ref, 0), hint, v, depth)
	case symbol.FormMap:
		return r.mapPair(rules.EmitRef(ref), arg(ref, 0), arg(ref, 1), hint, v, depth)
	case symbol.FormReference:
		return wellKnownPair(ref, s.Ref, hint)
	default:
		return rules.RefusedPair(rules.RefusedNoLiteral)
	}
}

// wellKnownPair derives the pair of a reference ref to a well-known type id:
// two constructions of a datetime and of a timedelta, two exceptions with
// the hinted strings, and None alone for the empty value.
func wellKnownPair(ref *node.TypeRef, id symbol.Identity, hint string) (rules.Sample, rules.Sample) {
	t := rules.EmitRef(ref)
	switch id {
	case rules.WellKnownTimestamp:
		year, month := emit.Number(emit.LiteralInt, sampleYear, 0), emit.Number(emit.LiteralInt, sampleMonth, 0)
		return rules.Of(emit.Construction(t, year, month, emit.Number(emit.LiteralInt, sampleDay, 0))),
			rules.Of(emit.Construction(t, year, month, emit.Number(emit.LiteralInt, alternateDay, 0)))
	case rules.WellKnownDuration:
		return rules.Of(emit.Construction(t, emit.Number(emit.LiteralInt, sampleDays, 0))),
			rules.Of(emit.Construction(t, emit.Number(emit.LiteralInt, alternateDays, 0)))
	case rules.WellKnownError:
		sample, alternate := textPair(hint)
		return rules.Of(emit.Construction(t, sample.Value)), rules.Of(emit.Construction(t, alternate.Value))
	default:
		return rules.Of(emit.Literal(emit.LiteralNil, "")), rules.Refused(rules.RefusedNoLiteral)
	}
}

// textPair returns the pair of a string: the hint with the suffix of each
// half, and the default hint where the hint is empty.
func textPair(hint string) (rules.Sample, rules.Sample) {
	if hint == "" {
		hint = defaultHint
	}
	return rules.Pair(emit.LiteralString, hint+sampleSuffix, hint+alternateSuffix)
}

// literalPair returns the pair of Literal[...]: its first two values that
// differ, and its one value without a second. Each value is the literal of
// its own kind, and the member of an enum is its name. The fold of Literal
// takes only the values that literalValue reads, so every value reads, and
// a Literal of the fold has at least one value.
func literalPair(ref *node.TypeRef) (rules.Sample, rules.Sample) {
	var sample rules.Sample
	for _, a := range ref.Args {
		value, _ := literalValue(a)
		switch {
		case !sample.OK():
			sample = rules.Of(value)
		case !sameValue(value, sample.Value):
			return sample, rules.Of(value)
		}
	}
	return sample, rules.Refused(rules.RefusedNoLiteral)
}

// literalValue returns the value of one argument a of Literal, which is not
// nil: the literal of its own kind, and the name of the member of an enum
// that the resolution step resolved. It reports false for every other
// argument.
func literalValue(a *node.TypeRef) (emit.Value, bool) {
	if a.Target.Kind == symbol.KindEnumVariant {
		return emit.Name(a.Target), true
	}
	lit, scanned := scanLiteral(a.Spelling)
	if !scanned {
		return emit.Value{}, false
	}
	return lit.value()
}

// listPair returns the pair of a list of the type t: a list of one element,
// from the pair of the element type elem.
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

// mapPair returns the pair of a dict of the type t: a dict of one entry, and
// the two entries differ in the key. The key and the value take the pairs of
// their types.
func (r Rules) mapPair(
	t *emit.TypeRef, key, value *node.TypeRef, hint string, v rules.View, depth int,
) (rules.Sample, rules.Sample) {
	k, otherKey := r.partPair(symbol.Identity{}, key, hint, v, depth+1)
	val, _ := r.partPair(symbol.Identity{}, value, hint, v, depth+1)
	if !k.OK() || !otherKey.OK() || !val.OK() {
		return rules.RefusedPair(rules.FirstRefusal(k, otherKey, val))
	}
	return rules.Of(emit.Composite(t, emit.KeyedEntry(k.Value, val.Value))),
		rules.Of(emit.Composite(t, emit.KeyedEntry(otherKey.Value, val.Value)))
}

// tuplePair returns the pair of a tuple: a tuple of the samples of its
// members, and a tuple of their alternates. A tuple without members has one
// value, which tells no two values apart.
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
// union derive, in member order. It refuses with the refusal of the first
// member where the members derive fewer than two values.
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
			case !sameValue(c.Value, sample.Value):
				return sample, c
			}
		}
	}
	return rules.RefusedPair(why)
}

// constructedPair returns the pair of a class: two calls of its first
// constructor that set its required parameters, by keyword where a
// parameter takes a keyword and by position otherwise. The alternate sets
// the first parameter with its alternate value. A constructor without a
// required parameter sets its first optional parameter whose pair derives.
// A parameter takes the values that an author stated for the field of its
// name.
func (r Rules) constructedPair(
	ref *node.TypeRef, s *node.Struct, v rules.View, depth int,
) (rules.Sample, rules.Sample) {
	cs := r.Constructors(s, v)
	if len(cs) == 0 {
		return rules.RefusedPair(rules.RefusedNoLiteral)
	}
	fields := map[string]symbol.Identity{}
	if set, membered := rules.NewBound(r, v, nil).MembersOf(s); membered {
		for _, m := range set.Members {
			if f, isField := m.Symbol.(*node.Field); isField {
				fields[f.Name] = f.ID
			}
		}
	}
	params := cs[0].Params
	optional := !slices.ContainsFunc(params, func(p rules.ParamView) bool { return !p.Optional && !p.Variadic })
	t := rules.EmitRef(ref)
	calls := make([]emit.ValueField, 2*len(params))
	samples, alternates := calls[:0:len(params)], calls[len(params):len(params)]
	for _, p := range params {
		if p.Variadic || p.Optional && !optional {
			continue
		}
		sample, alternate := r.partPair(fields[p.Name], p.Ref, p.Name, v, depth+1)
		if !sample.OK() || !alternate.OK() {
			if optional {
				continue
			}
			return rules.RefusedPair(rules.FirstRefusal(sample, alternate))
		}
		if len(alternates) > 0 {
			alternate = sample
		}
		samples = append(samples, argument(p, sample.Value))
		alternates = append(alternates, argument(p, alternate.Value))
		if optional {
			break
		}
	}
	if len(samples) == 0 {
		return rules.RefusedPair(rules.RefusedNoLiteral)
	}
	return rules.Of(emit.Composite(t, samples...)), rules.Of(emit.Composite(t, alternates...))
}

// dictPair returns the pair of a TypedDict: two dicts of its required keys,
// and the alternate sets the first key with its alternate value. A
// TypedDict without a required key sets its first key whose pair derives.
func (r Rules) dictPair(ref *node.TypeRef, d *node.Interface, v rules.View, depth int) (rules.Sample, rules.Sample) {
	set, _ := rules.NewBound(r, v, nil).MembersOf(d)
	t := rules.EmitRef(ref)
	for _, optional := range [2]bool{false, true} {
		var samples, alternates []emit.ValueField
		for _, m := range set.Members {
			f, isField := m.Symbol.(*node.Field)
			if !isField || f.Optional != optional || f.Level == symbol.LevelType {
				continue
			}
			sample, alternate := r.partPair(f.ID, f.Type, f.Name, v, depth+1)
			switch {
			case !sample.OK() || !alternate.OK():
				if optional {
					continue
				}
				return rules.RefusedPair(rules.FirstRefusal(sample, alternate))
			case len(alternates) > 0:
				alternate = sample
			}
			samples = append(samples, emit.NamedField(f.Name, sample.Value))
			alternates = append(alternates, emit.NamedField(f.Name, alternate.Value))
			if optional {
				break
			}
		}
		if len(samples) > 0 {
			return rules.Of(emit.Composite(t, samples...)), rules.Of(emit.Composite(t, alternates...))
		}
	}
	return rules.RefusedPair(rules.RefusedNoLiteral)
}

// enumPair returns the pair of an enum: the names of its first two members
// of distinct known values, such as Color.RED and Color.GREEN.
func (r Rules) enumPair(e *node.Enum, v rules.View) (rules.Sample, rules.Sample) {
	ms := r.members(e, r.kindOf(e, v, 0), v)
	for i, m := range ms {
		if !m.known {
			continue
		}
		for _, other := range ms[i+1:] {
			if other.known && (other.isText != m.isText || other.text != m.text) {
				return rules.Of(emit.Name(m.variant.ID)), rules.Of(emit.Name(other.variant.ID))
			}
		}
		break
	}
	return rules.RefusedPair(rules.RefusedNoLiteral)
}

// argument returns the value x as the argument of a call for the parameter
// p: by keyword where p takes a keyword, and by position for a parameter
// before the slash of its signature.
func argument(p rules.ParamView, x emit.Value) emit.ValueField {
	if p.Passing == symbol.PassingPositional {
		return emit.Element(x)
	}
	return emit.NamedField(p.Name, x)
}

// ZeroValue returns the value of a type that reads as nothing set:
//
//   - None for an optional, for None, for the top type and for a Literal
//     with None among its values;
//   - 0 for int and float, False for bool, the empty string for str, b""
//     for bytes, and the construction bytearray() for bytearray;
//   - the empty list for a list and a sequence, the empty dict for a dict
//     and a mapping, and the empty tuple for a tuple without members and for
//     a tuple of one type of any length;
//   - the zero of its target for an alias, and the conversion of the zero of
//     its base for a NewType.
//
// Every other type has no zero value, and ZeroValue reports false: a tuple
// of members, a class, a TypedDict, an enum, a datetime and a type outside
// the view.
//
// # Allocation contract
//
// The zero of a builtin allocates nothing. An empty composite allocates the
// reference to its type, and a conversion its inner value.
func (r Rules) ZeroValue(ref *node.TypeRef, v rules.View) (emit.Value, bool) {
	var convert []*emit.TypeRef
	for range deriveDepth {
		switch {
		case ref == nil:
			return emit.Value{}, false
		case ref.Form == symbol.FormOptional:
			return converted(convert, emit.Literal(emit.LiteralNil, "")), true
		case ref.Form == symbol.FormList, ref.Form == symbol.FormTuple && len(ref.Elems) == 0:
			return converted(convert, emit.Composite(rules.EmitRef(ref))), true
		case ref.Form != symbol.FormNamed:
			return emit.Value{}, false
		case ref.Target.IsZero():
			module, name, _ := builtinName(ref)
			return r.namedZero(convert, ref, module, name, r.Builtin(ref, v))
		}
		sym, held := v.Lookup(ref.Target)
		if !held {
			return emit.Value{}, false
		}
		if id, blessed := r.WellKnown(ref.Target); blessed {
			return r.namedZero(convert, ref, "", "", rules.Reference(ref.Spelling, id))
		}
		if s, shaped := r.ShapeOf(ref, sym, v); shaped {
			return r.namedZero(convert, ref, ref.Target.Package, ref.Target.Name, s)
		}
		alias, isAlias := sym.(*node.Alias)
		if !isAlias || alias.Target == nil {
			return emit.Value{}, false
		}
		if alias.Defined {
			convert = append(convert, rules.EmitRef(ref))
		}
		ref = rules.Substitute(alias.Target, alias.TypeParams, ref.Args)
	}
	return emit.Value{}, false
}

// namedZero returns the zero of a reference ref to the name of the module
// whose values have the shape s, converted through the types of convert
// from the last to the first.
func (Rules) namedZero(
	convert []*emit.TypeRef, ref *node.TypeRef, module, name string, s rules.TypeShape,
) (emit.Value, bool) {
	var zero emit.Value
	switch {
	case s.Form == symbol.FormDynamic, s.Form == symbol.FormReference && s.Ref == rules.WellKnownEmpty,
		s.Form == symbol.FormOptional:
		zero = emit.Literal(emit.LiteralNil, "")
	case name == literalForm && (module == typingModule || module == typingExtensions):
		return emit.Value{}, false
	case s.Form == symbol.FormScalar && s.Class == rules.ScalarFloat:
		zero = emit.Number(emit.LiteralFloat, zeroNumber, floatBits)
	case s.Form == symbol.FormScalar:
		zero = emit.Number(emit.LiteralInt, zeroNumber, 0)
	case s.Form == symbol.FormBool:
		zero = emit.Literal(emit.LiteralBool, falseText)
	case s.Form == symbol.FormText:
		zero = emit.Literal(emit.LiteralString, "")
	case s.Form == symbol.FormBytes && module == builtinsModule && name == bytearrayClass:
		zero = emit.Construction(rules.EmitRef(ref))
	case s.Form == symbol.FormBytes:
		zero = emit.Raw(python.Lang, zeroBytes)
	case s.Form == symbol.FormList || s.Form == symbol.FormMap:
		zero = emit.Composite(rules.EmitRef(ref))
	default:
		return emit.Value{}, false
	}
	return converted(convert, zero), true
}

// converted returns the value x converted through the types of convert,
// from the last to the first, as UserId(0) converts the zero of the base of
// the NewType UserId.
func converted(convert []*emit.TypeRef, x emit.Value) emit.Value {
	for _, t := range slices.Backward(convert) {
		x = emit.Conversion(t, x)
	}
	return x
}

// LiteralFor reads text in the grammar of the literals of Python as a value
// of the type ref, and reports whether the text is a value of the type:
//
//   - int takes an integer, float an integer or a float, bool True or
//     False, str a string, bytes a bytes literal, and bytearray the
//     conversion of a bytes literal, as bytearray(b"a");
//   - None and an optional take None, and an optional takes what its type
//     takes;
//   - a Literal takes one of its values, and the name of one of its members
//     of an enum, as Status.NEW for Literal[Status.NEW];
//   - an enum takes the name of one of its members, as RED or Color.RED,
//     which becomes the name of the member;
//   - a union takes what one of its members takes, as the first such member
//     types it;
//   - an alias takes what its target takes, and a NewType the conversion of
//     what its base takes;
//   - the top type and a nil reference take every literal but a name, as a
//     value of its own kind.
//
// An integer is its decimal text, and a float the shortest decimal text that
// reads back to it. Every other type takes no literal, a class and a type
// outside the view among them. The scope plays no part, because a literal
// of Python has one value in every module.
//
// # Allocation contract
//
// LiteralFor allocates what [scanLiteral] allocates, the decimal text of a
// float, and the conversion of a NewType. The member of an enum allocates
// the list of the members of the enum.
func (r Rules) LiteralFor(_ rules.Scope, ref *node.TypeRef, text string, v rules.View) (emit.Value, bool) {
	lit, scanned := scanLiteral(text)
	if !scanned {
		return emit.Value{}, false
	}
	return r.typed(lit, ref, v, 0)
}

// typed returns the literal lit as a value of the type ref by the rules of
// [Rules.LiteralFor], at a depth of the walk.
func (r Rules) typed(lit literal, ref *node.TypeRef, v rules.View, depth int) (emit.Value, bool) {
	switch {
	case ref == nil:
		return lit.value()
	case depth > deriveDepth:
		return emit.Value{}, false
	case ref.Form == symbol.FormOptional && lit.kind == literalNone:
		return emit.Literal(emit.LiteralNil, ""), true
	case ref.Form == symbol.FormOptional:
		return r.typed(lit, firstElem(ref), v, depth+1)
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
		module, name, _ := builtinName(ref)
		return typedNamed(lit, ref, module, name, r.Builtin(ref, v))
	}
	sym, held := v.Lookup(ref.Target)
	if !held {
		return emit.Value{}, false
	}
	if id, blessed := r.WellKnown(ref.Target); blessed {
		return typedNamed(lit, ref, "", "", rules.Reference(ref.Spelling, id))
	}
	if s, shaped := r.ShapeOf(ref, sym, v); shaped {
		return typedNamed(lit, ref, ref.Target.Package, ref.Target.Name, s)
	}
	switch d := sym.(type) {
	case *node.Enum:
		name, named := memberOf(lit, d.Name)
		if !named {
			return emit.Value{}, false
		}
		for _, m := range r.members(d, r.kindOf(d, v, 0), v) {
			if m.name == name {
				return emit.Name(m.variant.ID), true
			}
		}
		return emit.Value{}, false
	case *node.Alias:
		target := rules.Substitute(d.Target, d.TypeParams, ref.Args)
		value, typed := r.typed(lit, target, v, depth+1)
		if !typed || !d.Defined {
			return value, typed
		}
		return emit.Conversion(rules.EmitRef(ref), value), true
	default:
		return emit.Value{}, false
	}
}

// typedNamed returns the literal lit as a value of a reference ref to the
// name of the module whose values have the shape s. A value of Literal is
// one of its arguments: a literal of the same value, or a member of an enum
// that the name of lit names.
func typedNamed(lit literal, ref *node.TypeRef, module, name string, s rules.TypeShape) (emit.Value, bool) {
	if name == literalForm && (module == typingModule || module == typingExtensions) {
		own, valued := lit.value()
		for _, a := range ref.Args {
			if a == nil {
				continue
			}
			if a.Target.Kind == symbol.KindEnumVariant {
				if member, named := memberOf(lit, enumOfVariant(a.Target).Name); named && member == a.Target.Name {
					return emit.Name(a.Target), true
				}
				continue
			}
			if value, isValue := literalValue(a); valued && isValue && sameValue(own, value) {
				return value, true
			}
		}
		return emit.Value{}, false
	}
	switch {
	case s.Form == symbol.FormScalar && s.Class == rules.ScalarFloat && lit.kind == literalInt:
		return emit.Number(emit.LiteralFloat, lit.text, floatBits), true
	case s.Form == symbol.FormScalar && s.Class == rules.ScalarFloat && lit.kind == literalFloat:
		return emit.Number(emit.LiteralFloat, numeric.Decimal(lit.number, floatBits), floatBits), true
	case s.Form == symbol.FormBytes && lit.kind == literalBytes && module == builtinsModule && name == bytearrayClass:
		return emit.Conversion(rules.EmitRef(ref), emit.Raw(python.Lang, lit.text)), true
	case s.Form == symbol.FormScalar && lit.kind == literalInt,
		s.Form == symbol.FormBool && lit.kind == literalBool,
		s.Form == symbol.FormText && lit.kind == literalString,
		s.Form == symbol.FormBytes && lit.kind == literalBytes,
		s.Form == symbol.FormReference && s.Ref == rules.WellKnownEmpty && lit.kind == literalNone,
		s.Form == symbol.FormDynamic && lit.kind != literalName:
		return lit.value()
	default:
		return emit.Value{}, false
	}
}

// memberOf returns the name of the member of the enum named enum that the
// name lit names: the member alone, as RED, or the member after the name of
// the enum, as Color.RED, also behind the modules of the enum, as
// models.Color.RED. It reports false for a literal that is no name, and for
// a name of a member of another enum. It allocates nothing.
func memberOf(lit literal, enum string) (string, bool) {
	if lit.kind != literalName {
		return "", false
	}
	owner, name, qualified := strings.CutLast(lit.text, python.ModuleSep)
	if !qualified {
		return lit.text, true
	}
	if _, last, dotted := strings.CutLast(owner, python.ModuleSep); dotted {
		owner = last
	}
	return name, owner == enum
}

// value returns the literal as a value of its own kind: an integer as its
// decimal text, a float as the shortest decimal text that reads back to it,
// a string, a truth value, None, and a bytes literal as Python text. A name
// refers to a declaration that only a type resolves, so value reports false
// for a name.
func (lit literal) value() (emit.Value, bool) {
	switch lit.kind {
	case literalInt:
		return emit.Number(emit.LiteralInt, lit.text, 0), true
	case literalFloat:
		return emit.Number(emit.LiteralFloat, numeric.Decimal(lit.number, floatBits), floatBits), true
	case literalString:
		return emit.Literal(emit.LiteralString, lit.text), true
	case literalBool:
		return emit.Literal(emit.LiteralBool, lit.text), true
	case literalNone:
		return emit.Literal(emit.LiteralNil, ""), true
	case literalBytes:
		return emit.Raw(python.Lang, lit.text), true
	default:
		return emit.Value{}, false
	}
}

// sameValue reports whether a and b are one literal or one name: literals of
// one kind and one text, or names of one declaration. A value of another
// kind has parts that sameValue does not compare, so it reports false for
// one.
func sameValue(a, b emit.Value) bool {
	switch {
	case a.Kind != b.Kind:
		return false
	case a.Kind == emit.ValueLiteral:
		return a.Literal == b.Literal && a.Text == b.Text && a.Lang == b.Lang
	case a.Kind == emit.ValueName:
		return a.Callee == b.Callee
	default:
		return false
	}
}

// firstElem returns the first child of a structural reference, the type of
// an optional or of a list, and nil where the reference has no child.
func firstElem(ref *node.TypeRef) *node.TypeRef {
	if len(ref.Elems) > 0 {
		return ref.Elems[0]
	}
	return nil
}

// arg returns the type argument at index i of a named reference, and nil
// where the reference has no such argument.
func arg(ref *node.TypeRef, i int) *node.TypeRef {
	if i < len(ref.Args) {
		return ref.Args[i]
	}
	return nil
}
