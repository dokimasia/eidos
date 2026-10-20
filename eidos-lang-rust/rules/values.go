// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"cmp"
	"strings"
	"unicode/utf8"

	"go.dokimi.dev/eidos/lang/numeric"
	"go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// deriveDepth bounds a derivation through a type that refers to itself, such
// as a struct with a field of its own type behind a Box, and a chain of
// aliases and wrappers.
const deriveDepth = 8

// maxArray is the longest array whose every element a value writes. A
// longer array has no value, because the value tree has no repeat
// expression such as [0; 4096].
const maxArray = 64

// The pairs and the zeros of the primitives and of the types of the standard
// library, and the hint of a string where the caller names none. A character
// literal has no literal kind of the value tree, so its values are Rust
// text.
const (
	sampleInt        = "42"
	alternateInt     = "7"
	sampleFloat      = "1.5"
	alternateFloat   = "2.5"
	zeroNumber       = "0"
	sampleChar       = "'a'"
	alternateChar    = "'b'"
	zeroChar         = `'\0'`
	defaultHint      = "sample"
	sampleSuffix     = "-a"
	alternateSuffix  = "-b"
	sampleSeconds    = "1"
	alternateSeconds = "2"
	sampleEpoch      = "1700000000"
	alternateEpoch   = "1700000001"
	secondsBits      = 64
)

// The associated functions, constants and variants of the standard library
// that a value calls or names.
const (
	fromFunction     = "from"
	newFunction      = "new"
	defaultFunction  = "default"
	otherFunction    = "other"
	fromSecsFunction = "from_secs"
	addFunction      = "add"
	zeroConst        = "ZERO"
	epochConst       = "UNIX_EPOCH"
	someVariant      = "Some"
	okVariant        = "Ok"
	noneVariant      = "None"
)

// The types and the modules of the standard library that a value names
// beside the table of [libTypes].
const (
	durationType   = "Duration"
	systemTimeType = "SystemTime"
	mutexType      = "Mutex"
	rwLockType     = "RwLock"
	addTrait       = "Add"
	ioModule       = "io"
	fmtModule      = "fmt"
	opsHome        = "std/ops"
)

// The spellings of the type of an array expression and of a tuple expression
// that a value builds. The backend writes such a composite as its elements
// in brackets or in parentheses, without the type, so the spelling only
// states that the composite has a type.
const (
	arrayExpression = "[_]"
	tupleExpression = "(_)"
)

// The items of the standard library that a value names. A function of the
// prelude has no package, and an item of a module has the package path of
// the module, so a value of io::Error::other imports std::io.
var (
	someCall = symbol.Identity{Lang: rust.Lang, Name: someVariant, Kind: symbol.KindSumVariant}
	okCall   = symbol.Identity{Lang: rust.Lang, Name: okVariant, Kind: symbol.KindSumVariant}
	boxFrom  = symbol.Identity{
		Lang: rust.Lang, Owner: boxType, Name: fromFunction, Kind: symbol.KindMethod,
	}
	durationFromSecs = symbol.Identity{
		Lang: rust.Lang, Package: timeHome, Owner: durationType, Name: fromSecsFunction, Kind: symbol.KindMethod,
	}
	durationZero = symbol.Identity{
		Lang: rust.Lang, Package: timeHome, Owner: durationType, Name: zeroConst, Kind: symbol.KindField,
	}
	unixEpoch = symbol.Identity{
		Lang: rust.Lang, Package: timeHome, Owner: systemTimeType, Name: epochConst, Kind: symbol.KindField,
	}
	addCall = symbol.Identity{
		Lang: rust.Lang, Package: opsHome, Owner: addTrait, Name: addFunction, Kind: symbol.KindMethod,
	}
	ioErrorOther = symbol.Identity{
		Lang: rust.Lang, Package: stdCrate, Owner: ioModule + symbol.OwnerSep + errorTrait, Name: otherFunction,
		Kind: symbol.KindMethod,
	}
	fmtError = symbol.Identity{
		Lang: rust.Lang, Package: stdCrate, Owner: fmtModule, Name: errorTrait, Kind: symbol.KindStruct,
	}
)

// DeriveSamples derives two values of a type that differ, for a check that
// must tell one from the other, in Rust's grammar of expressions:
//
//   - an integer takes 42 and 7, a float 1.5 and 2.5, bool true and false,
//     char 'a' and 'b', and &str the hint with the suffixes -a and -b;
//   - String takes String::from of the two strings, OsString, PathBuf and
//     Cow<str> the from of their types, and &OsStr and &Path the new of
//     theirs;
//   - the unit type takes (), and refuses the second value;
//   - Option<T> takes Some of each value of T, and a Result<T, E> Ok of each
//     value of T;
//   - Vec<T> takes vec![x], VecDeque, LinkedList, BinaryHeap, HashSet and
//     BTreeSet take the from of an array of one element, and HashMap and
//     BTreeMap take the from of an array of one pair, which differ in the
//     key;
//   - an array [T; N] takes N elements, a slice behind a borrow an array of
//     one element, and a tuple a tuple of one value per member;
//   - Box, Rc and Arc take the new of each value of their type, and the from
//     of the value of a reference to a str, a slice, a Path or an OsStr.
//     Cell, RefCell, Mutex and RwLock take the new of each value;
//   - a shared borrow &T takes the address of each value of T;
//   - Duration takes Duration::from_secs(1) and Duration::from_secs(2), and
//     SystemTime the sum of UNIX_EPOCH and 1,700,000,000 seconds, and one
//     second later, through Add::add;
//   - io::Error takes io::Error::other of the two strings, a Box of a trait
//     object of the error trait Box::from of them, and fmt::Error itself
//     without a second value;
//   - an enum takes its first two variants, as Color::Red;
//   - a data enum takes its first variant that a value builds: a unit
//     variant, a tuple variant called with a value of each field, or a
//     struct variant with a value of each field. A unit variant takes the
//     next such variant as its second value, and a variant with fields its
//     second values;
//   - a struct takes a call of its first construction of [Rules.Constructors]
//     that returns the struct itself and has parameters that derive, and the
//     second value sets the first parameter to its second value. A
//     construction without parameters takes one value;
//   - an alias takes the pair of its target, with its arguments
//     substituted.
//
// A part of a value takes the values that an author stated for it first,
// through [rules.View.Authored], and the derivation completes each half that
// no author stated. A parameter of a construction reads the values of the
// field of its name, and a field of a variant its own.
//
// Both halves refuse together, with one of these reasons:
//
//   - [rules.RefusedUnresolved]: a named type that the view does not
//     contain, and a type of another crate.
//   - [rules.RefusedNoLiteral]: a type without two values that a check can
//     write, such as a mutable borrow, a type parameter, a trait object, an
//     iterator, a function, the never type, CString, Instant, an empty array,
//     and a struct without a construction whose parameters derive.
//   - [rules.RefusedDepth]: a type that refers to itself past the budget of
//     the walk.
//
// # Allocation contract
//
// A pair of a number, a bool and a character allocates nothing, and a pair
// of a string allocates the two hinted texts. A pair of a composite
// allocates the reference to its type and the list of entries of each
// composite, and a call allocates the list of its arguments. A pair of a
// struct also allocates its constructions and the binding that projects
// each associated function.
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
	case symbol.FormBorrow:
		return r.borrowPair(ref, hint, v, depth)
	case symbol.FormList:
		return r.arrayPair(firstElem(ref), 1, hint, v, depth)
	case symbol.FormArray:
		return r.arrayPair(firstElem(ref), ref.Length, hint, v, depth)
	case symbol.FormTuple:
		return r.tuplePair(ref, hint, v, depth)
	case symbol.FormNamed:
		if !ref.Target.IsZero() {
			return r.declaredPair(ref, hint, v, depth)
		}
		return r.libPair(ref, hint, v, depth)
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

// borrowPair derives the pair of a borrow. A borrow of a text without a size,
// such as &str and &Path, takes the pair of the text, whose values are
// references already, and a borrow of a slice the address of an array.
// Every other shared borrow takes the address of each value of its type. A
// mutable borrow refuses, because the value tree has no mutable address.
func (r Rules) borrowPair(ref *node.TypeRef, hint string, v rules.View, depth int) (rules.Sample, rules.Sample) {
	if ref.Mutability == symbol.MutabilityMutable {
		return rules.RefusedPair(rules.RefusedNoLiteral)
	}
	elem := firstElem(ref)
	sample, alternate := r.partPair(symbol.Identity{}, elem, hint, v, depth+1)
	if sizeOf(elem) == sizelessText {
		return sample, alternate
	}
	return rules.Lift(sample, emit.Address), rules.Lift(alternate, emit.Address)
}

// arrayPair returns the pair of an array of n elements of the type elem: n
// samples, and n alternates. An array without elements has one value, and
// an array longer than [maxArray] has no value that a check writes.
func (r Rules) arrayPair(elem *node.TypeRef, n int, hint string, v rules.View, depth int) (rules.Sample, rules.Sample) {
	if n <= 0 || n > maxArray {
		return rules.RefusedPair(rules.RefusedNoLiteral)
	}
	sample, alternate := r.partPair(symbol.Identity{}, elem, hint, v, depth+1)
	if !sample.OK() || !alternate.OK() {
		return rules.RefusedPair(rules.FirstRefusal(sample, alternate))
	}
	t := &emit.TypeRef{Spelling: arrayExpression, Form: symbol.FormArray, Length: n}
	fields := make([]emit.ValueField, 2*n)
	for i := range n {
		fields[i], fields[n+i] = emit.Element(sample.Value), emit.Element(alternate.Value)
	}
	return rules.Of(emit.Composite(t, fields[:n:n]...)), rules.Of(emit.Composite(t, fields[n:]...))
}

// tuplePair returns the pair of a tuple: a tuple of the samples of its
// members, and a tuple of their alternates.
func (r Rules) tuplePair(ref *node.TypeRef, hint string, v rules.View, depth int) (rules.Sample, rules.Sample) {
	n := len(ref.Elems)
	if n == 0 {
		return rules.RefusedPair(rules.RefusedNoLiteral)
	}
	fields := make([]emit.ValueField, 2*n)
	for i, e := range ref.Elems {
		sample, alternate := r.partPair(symbol.Identity{}, e, hint, v, depth+1)
		if !sample.OK() || !alternate.OK() {
			return rules.RefusedPair(rules.FirstRefusal(sample, alternate))
		}
		fields[i], fields[n+i] = emit.Element(sample.Value), emit.Element(alternate.Value)
	}
	t := &emit.TypeRef{Spelling: tupleExpression, Form: symbol.FormTuple}
	return rules.Of(emit.Composite(t, fields[:n:n]...)), rules.Of(emit.Composite(t, fields[n:]...))
}

// declaredPair derives the pair of a named type that the view contains: a
// struct, an enum, a data enum and an alias.
func (r Rules) declaredPair(ref *node.TypeRef, hint string, v rules.View, depth int) (rules.Sample, rules.Sample) {
	sym, held := v.Lookup(ref.Target)
	if !held {
		return rules.RefusedPair(rules.RefusedUnresolved)
	}
	switch d := sym.(type) {
	case *node.Struct:
		return r.structPair(ref, d, v, depth)
	case *node.Enum:
		return enumPair(d)
	case *node.Sum:
		return r.sumPair(ref, d, hint, v, depth)
	case *node.Alias:
		if d.Target == nil {
			return rules.RefusedPair(rules.RefusedNoLiteral)
		}
		return r.derive(rules.Substitute(d.Target, d.TypeParams, typeArgsFor(d.TypeParams, ref)), hint, v, depth+1)
	default:
		return rules.RefusedPair(rules.RefusedNoLiteral)
	}
}

// structPair returns the pair of a struct through its first construction
// that returns the struct itself and has parameters that derive. Each
// parameter takes the values that an author stated for the field of its
// name, with the type arguments of ref substituted, and the second value sets
// the first parameter to its second value. A construction without
// parameters takes one value where no construction with parameters derives.
func (r Rules) structPair(ref *node.TypeRef, s *node.Struct, v rules.View, depth int) (rules.Sample, rules.Sample) {
	args := typeArgsFor(s.TypeParams, ref)
	why := rules.RefusedNone
	var single rules.Sample
	for _, c := range r.constructions(s, v) {
		n := len(c.callable.Params)
		switch {
		case !c.direct:
			continue
		case n == 0:
			if !single.OK() {
				single = rules.Of(c.value(s, nil))
			}
			continue
		}
		values := make([]emit.Value, 2*n)
		derived := true
		for i, p := range c.callable.Params {
			sample, other := r.partPair(fieldID(s, p.Name), rules.Substitute(p.Ref, s.TypeParams, args), p.Name, v,
				depth+1)
			if !sample.OK() || !other.OK() {
				if why == rules.RefusedNone {
					why = rules.FirstRefusal(sample, other)
				}
				derived = false
				break
			}
			values[i], values[n+i] = sample.Value, sample.Value
			if i == 0 {
				values[n] = other.Value
			}
		}
		if derived {
			return rules.Of(c.value(s, values[:n:n])), rules.Of(c.value(s, values[n:]))
		}
	}
	if single.OK() {
		return single, rules.Refused(rules.RefusedNoLiteral)
	}
	if why == rules.RefusedNone {
		why = rules.RefusedNoLiteral
	}
	return rules.RefusedPair(why)
}

// enumPair returns the pair of an enum: the names of its first two variants,
// such as Color::Red and Color::Green. An enum of one variant has one value.
func enumPair(e *node.Enum) (rules.Sample, rules.Sample) {
	var sample rules.Sample
	for _, variant := range e.Variants {
		switch {
		case variant == nil:
		case sample.OK():
			return sample, rules.Of(emit.Name(variant.ID))
		default:
			sample = rules.Of(emit.Name(variant.ID))
		}
	}
	if sample.OK() {
		return sample, rules.Refused(rules.RefusedNoLiteral)
	}
	return rules.RefusedPair(rules.RefusedNoLiteral)
}

// sumPair returns the pair of a data enum: its first variant that a value
// builds, as [Rules.DeriveSamples] states, with the type arguments of ref
// substituted in the types of its fields.
func (r Rules) sumPair(
	ref *node.TypeRef, d *node.Sum, hint string, v rules.View, depth int,
) (rules.Sample, rules.Sample) {
	args := typeArgsFor(d.TypeParams, ref)
	why, failed := rules.RefusedNoLiteral, false
	var sample rules.Sample
	for _, variant := range d.Variants {
		s, a := rules.RefusedPair(rules.RefusedNoLiteral)
		if variant != nil {
			s, a = r.variantPair(d, variant, args, hint, v, depth)
		}
		switch {
		case !s.OK():
			if !failed {
				why, failed = s.Refusal, true
			}
		case sample.OK():
			return sample, s
		case a.OK():
			return s, a
		default:
			sample = s
		}
	}
	if sample.OK() {
		return sample, rules.Refused(rules.RefusedNoLiteral)
	}
	return rules.RefusedPair(why)
}

// variantPair returns the pair of one variant of the data enum d: the name
// of a unit variant without a second value, the call of a tuple variant
// with a value of each field, and a struct variant with a value of each
// field. The second value sets the first field to its second value. A nil
// field has no value.
func (r Rules) variantPair(
	d *node.Sum, variant *node.SumVariant, args []*node.TypeRef, hint string, v rules.View, depth int,
) (rules.Sample, rules.Sample) {
	n := len(variant.Fields)
	if n == 0 {
		return rules.Of(emit.Name(variant.ID)), rules.Refused(rules.RefusedNoLiteral)
	}
	values := make([]emit.Value, 2*n)
	for i, f := range variant.Fields {
		var (
			id  symbol.Identity
			typ *node.TypeRef
		)
		name := hint
		if f != nil {
			id, typ = f.ID, f.Type
			name = cmp.Or(f.Name, hint)
		}
		sample, alternate := r.partPair(id, rules.Substitute(typ, d.TypeParams, args), name, v, depth+1)
		if !sample.OK() || !alternate.OK() {
			return rules.RefusedPair(rules.FirstRefusal(sample, alternate))
		}
		values[i], values[n+i] = sample.Value, sample.Value
		if i == 0 {
			values[n] = alternate.Value
		}
	}
	if variant.Fields[0].Name == "" {
		return rules.Of(emit.Call(variant.ID, values[:n:n]...)), rules.Of(emit.Call(variant.ID, values[n:]...))
	}
	fields := make([]emit.ValueField, 2*n)
	for i, f := range variant.Fields {
		fields[i], fields[n+i] = emit.NamedField(f.Name, values[i]), emit.NamedField(f.Name, values[n+i])
	}
	t := &emit.TypeRef{Spelling: d.Name + pathSep + variant.Name, Target: variant.ID}
	return rules.Of(emit.Composite(t, fields[:n:n]...)), rules.Of(emit.Composite(t, fields[n:]...))
}

// libPair derives the pair of a reference ref without a target: a type of
// the standard library, a result, and the unit type, by the rules of
// [Rules.DeriveSamples].
func (r Rules) libPair(ref *node.TypeRef, hint string, v rules.View, depth int) (rules.Sample, rules.Sample) {
	if ref.Spelling == unitType {
		return rules.Of(emit.Composite(&emit.TypeRef{Spelling: unitType, Form: symbol.FormTuple})),
			rules.Refused(rules.RefusedNoLiteral)
	}
	if value, _, result := r.Result(ref, v); result {
		if value == nil {
			unit := emit.Composite(&emit.TypeRef{Spelling: unitType, Form: symbol.FormTuple})
			return rules.Of(emit.Call(okCall, unit)), rules.Refused(rules.RefusedNoLiteral)
		}
		sample, alternate := r.partPair(symbol.Identity{}, value, hint, v, depth+1)
		return calledPair(okCall, sample, alternate)
	}
	t, name, known := libTypeOf(ref)
	if !known {
		return rules.RefusedPair(unknownRefusal(ref))
	}
	from := symbol.Identity{Lang: rust.Lang, Package: t.home, Owner: name, Name: fromFunction, Kind: symbol.KindMethod}
	build := from
	build.Name = newFunction
	elem := typeArg(ref, 0)
	switch {
	case t.form == symbol.FormScalar && t.class == rules.ScalarFloat:
		return rules.NumberPair(emit.LiteralFloat, sampleFloat, alternateFloat, t.bits)
	case t.form == symbol.FormScalar:
		return rules.NumberPair(emit.LiteralInt, sampleInt, alternateInt, t.bits)
	case t.form == symbol.FormBool:
		return rules.Pair(emit.LiteralBool, trueText, falseText)
	case name == charType:
		return rules.Of(emit.Raw(rust.Lang, sampleChar)), rules.Of(emit.Raw(rust.Lang, alternateChar))
	case name == strType:
		return textPair(hint)
	case t.form == symbol.FormText && t.kind == libNoValue:
		return rules.RefusedPair(rules.RefusedNoLiteral)
	case t.form == symbol.FormText && t.kind == libUnsized:
		sample, alternate := textPair(hint)
		return calledPair(build, sample, alternate)
	case t.form == symbol.FormText:
		sample, alternate := textPair(hint)
		return calledPair(from, sample, alternate)
	case t.form == symbol.FormOptional:
		sample, alternate := r.partPair(symbol.Identity{}, elem, hint, v, depth+1)
		return calledPair(someCall, sample, alternate)
	case name == vecType:
		sample, alternate := r.partPair(symbol.Identity{}, elem, hint, v, depth+1)
		vec := &emit.TypeRef{Spelling: vecType, Form: symbol.FormList}
		list := func(x emit.Value) emit.Value { return emit.Composite(vec, emit.Element(x)) }
		return rules.Lift(sample, list), rules.Lift(alternate, list)
	case t.form == symbol.FormList, t.kind == libSet:
		sample, alternate := r.arrayPair(elem, 1, hint, v, depth)
		return calledPair(from, sample, alternate)
	case t.form == symbol.FormMap:
		return r.mapPair(&emit.TypeRef{Spelling: name, Package: t.home}, elem, typeArg(ref, 1), hint, v, depth)
	case name == boxType && errorObject(ref):
		sample, alternate := textPair(hint)
		return calledPair(boxFrom, sample, alternate)
	case t.form == symbol.FormBorrow:
		sample, alternate := r.partPair(symbol.Identity{}, elem, hint, v, depth+1)
		if t.kind == libPointer && sizeOf(elem) != sized {
			return calledPair(from, sample, alternate)
		}
		return calledPair(build, sample, alternate)
	case t.wellKnown == rules.WellKnownDuration:
		return rules.Of(emit.Call(durationFromSecs, emit.Number(emit.LiteralInt, sampleSeconds, secondsBits))),
			rules.Of(emit.Call(durationFromSecs, emit.Number(emit.LiteralInt, alternateSeconds, secondsBits)))
	case t.wellKnown == rules.WellKnownTimestamp:
		epoch := emit.Name(unixEpoch)
		return rules.Of(emit.Call(addCall, epoch,
				emit.Call(durationFromSecs, emit.Number(emit.LiteralInt, sampleEpoch, secondsBits)))),
			rules.Of(emit.Call(addCall, epoch,
				emit.Call(durationFromSecs, emit.Number(emit.LiteralInt, alternateEpoch, secondsBits))))
	case t.wellKnown == rules.WellKnownError && libModule(ref) == ioModule:
		sample, alternate := textPair(hint)
		return calledPair(ioErrorOther, sample, alternate)
	case t.wellKnown == rules.WellKnownError && libModule(ref) == fmtModule:
		return rules.Of(emit.Name(fmtError)), rules.Refused(rules.RefusedNoLiteral)
	default:
		return rules.RefusedPair(rules.RefusedNoLiteral)
	}
}

// mapPair returns the pair of a map of the type t: a map of one entry, and
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

// calledPair returns the pair of the calls of the function fn with each
// value of a pair as its one argument, and a half that has no value
// unchanged with its reason.
func calledPair(fn symbol.Identity, sample, alternate rules.Sample) (rules.Sample, rules.Sample) {
	call := func(x emit.Value) emit.Value { return emit.Call(fn, x) }
	return rules.Lift(sample, call), rules.Lift(alternate, call)
}

// textPair returns the pair of a string: the hint with the suffix of each
// half, and the default hint where the hint is empty.
func textPair(hint string) (rules.Sample, rules.Sample) {
	if hint == "" {
		hint = defaultHint
	}
	return rules.Pair(emit.LiteralString, hint+sampleSuffix, hint+alternateSuffix)
}

// ZeroValue returns the value of a type that reads as nothing set:
//
//   - 0 for an integer and a float, false for bool, '\0' for char and the
//     empty string for &str;
//   - String::new(), and the new of OsString and PathBuf, Cow::from("") for
//     Cow<str>, and the new of the empty string for &OsStr and &Path;
//   - () for the unit type, None for an option, Vec::new() for a vector, and
//     the new of VecDeque, LinkedList, BinaryHeap, HashMap, BTreeMap, HashSet
//     and BTreeSet;
//   - an array of N zeros for [T; N], the address of the empty array for a
//     borrow of a slice, and the tuple of the zeros of its members for a
//     tuple;
//   - the new of the zero of its type for Box, Rc, Arc, Cell, RefCell, Mutex
//     and RwLock, and the from of the zero of a reference for a pointer to a
//     str, a slice, a Path and an OsStr;
//   - Duration::ZERO for Duration, and SystemTime::UNIX_EPOCH for
//     SystemTime;
//   - the variant that #[default] marks for an enum that derives Default,
//     and T::default() for a struct and a data enum that derive Default, and
//     for a struct that a trait impl implements Default for;
//   - the zero of its target for an alias.
//
// Every other type has no zero value, and ZeroValue reports false: a
// reference other than a borrow of a text or a slice, a result, an error, a
// type parameter, a trait object, and a type outside the view.
//
// # Allocation contract
//
// The zero of a number, a bool, a character and a string allocates nothing.
// A call allocates the list of its arguments, and a composite the reference
// to its type and the list of its entries.
func (r Rules) ZeroValue(ref *node.TypeRef, v rules.View) (emit.Value, bool) {
	return r.zero(ref, v, 0)
}

// zero is [Rules.ZeroValue] at a depth of the walk.
func (r Rules) zero(ref *node.TypeRef, v rules.View, depth int) (emit.Value, bool) {
	if ref == nil || depth > deriveDepth {
		return emit.Value{}, false
	}
	switch ref.Form {
	case symbol.FormBorrow:
		elem := firstElem(ref)
		size := sizeOf(elem)
		if ref.Mutability == symbol.MutabilityMutable || size == sized {
			return emit.Value{}, false
		}
		x, ok := r.zero(elem, v, depth+1)
		if !ok || size == sizelessText {
			return x, ok
		}
		return emit.Address(x), true
	case symbol.FormList:
		return emit.Composite(&emit.TypeRef{Spelling: arrayExpression, Form: symbol.FormArray}), true
	case symbol.FormArray:
		if ref.Length <= 0 || ref.Length > maxArray {
			return emit.Value{}, false
		}
		x, ok := r.zero(firstElem(ref), v, depth+1)
		if !ok {
			return emit.Value{}, false
		}
		fields := make([]emit.ValueField, ref.Length)
		for i := range fields {
			fields[i] = emit.Element(x)
		}
		return emit.Composite(&emit.TypeRef{Spelling: arrayExpression, Form: symbol.FormArray, Length: ref.Length},
			fields...), true
	case symbol.FormTuple:
		fields := make([]emit.ValueField, 0, len(ref.Elems))
		for _, e := range ref.Elems {
			x, ok := r.zero(e, v, depth+1)
			if !ok {
				return emit.Value{}, false
			}
			fields = append(fields, emit.Element(x))
		}
		return emit.Composite(&emit.TypeRef{Spelling: tupleExpression, Form: symbol.FormTuple}, fields...), true
	case symbol.FormNamed:
		if !ref.Target.IsZero() {
			return r.declaredZero(ref, v, depth)
		}
		return r.libZero(ref, v, depth)
	default:
		return emit.Value{}, false
	}
}

// declaredZero returns the zero of a named type that the view contains, by
// the rules of [Rules.ZeroValue].
func (r Rules) declaredZero(ref *node.TypeRef, v rules.View, depth int) (emit.Value, bool) {
	sym, held := v.Lookup(ref.Target)
	if !held {
		return emit.Value{}, false
	}
	switch d := sym.(type) {
	case *node.Enum:
		info := r.EnumOf(d, v)
		for _, variant := range d.Variants {
			if variant != nil && info.Zero != "" && variant.Name == info.Zero {
				return emit.Name(variant.ID), true
			}
		}
		return emit.Value{}, false
	case *node.Struct, *node.Sum:
		if !hasTrait(sym, defaultTrait) {
			return emit.Value{}, false
		}
		return emit.Call(symbol.Identity{
			Lang: rust.Lang, Package: ref.Target.Package, Owner: ref.Target.Name, Name: defaultFunction,
			Kind: symbol.KindMethod,
		}), true
	case *node.Alias:
		return r.zero(rules.Substitute(d.Target, d.TypeParams, typeArgsFor(d.TypeParams, ref)), v, depth+1)
	default:
		return emit.Value{}, false
	}
}

// libZero returns the zero of a reference ref without a target, by the
// rules of [Rules.ZeroValue].
func (r Rules) libZero(ref *node.TypeRef, v rules.View, depth int) (emit.Value, bool) {
	if ref.Spelling == unitType {
		return emit.Composite(&emit.TypeRef{Spelling: unitType, Form: symbol.FormTuple}), true
	}
	t, name, known := libTypeOf(ref)
	if !known {
		return emit.Value{}, false
	}
	build := symbol.Identity{Lang: rust.Lang, Package: t.home, Owner: name, Name: newFunction, Kind: symbol.KindMethod}
	from := build
	from.Name = fromFunction
	elem := typeArg(ref, 0)
	switch {
	case t.form == symbol.FormScalar && t.class == rules.ScalarFloat:
		return emit.Number(emit.LiteralFloat, zeroNumber, t.bits), true
	case t.form == symbol.FormScalar:
		return emit.Number(emit.LiteralInt, zeroNumber, t.bits), true
	case t.form == symbol.FormBool:
		return emit.Literal(emit.LiteralBool, falseText), true
	case name == charType:
		return emit.Raw(rust.Lang, zeroChar), true
	case name == strType:
		return emit.Literal(emit.LiteralString, ""), true
	case t.form == symbol.FormText && t.kind == libNoValue:
		return emit.Value{}, false
	case t.form == symbol.FormText && t.kind == libUnsized:
		return emit.Call(build, emit.Literal(emit.LiteralString, "")), true
	case name == cowType:
		return emit.Call(from, emit.Literal(emit.LiteralString, "")), true
	case t.form == symbol.FormOptional:
		return emit.Literal(emit.LiteralNil, ""), true
	case t.form == symbol.FormText, t.form == symbol.FormList, t.form == symbol.FormMap, t.kind == libSet:
		return emit.Call(build), true
	case name == boxType && errorObject(ref):
		return emit.Value{}, false
	case t.form == symbol.FormBorrow:
		x, ok := r.zero(elem, v, depth+1)
		if !ok {
			return emit.Value{}, false
		}
		if t.kind == libPointer && sizeOf(elem) != sized {
			return emit.Call(from, x), true
		}
		return emit.Call(build, x), true
	case t.wellKnown == rules.WellKnownDuration:
		return emit.Name(durationZero), true
	case t.wellKnown == rules.WellKnownTimestamp:
		return emit.Name(unixEpoch), true
	default:
		return emit.Value{}, false
	}
}

// LiteralFor reads text in the grammar of the literals of Rust as a value of
// the type ref, and reports whether the text is a value of the type:
//
//   - an integer takes an integer literal of any radix, and a float an
//     integer or a float literal, each with a minus sign and with the type
//     suffix of the type or none, inside the range of the type;
//   - bool takes true or false, char a character literal, &str a string
//     literal or a raw string literal, and a borrow of a slice of u8 a byte
//     string;
//   - String, OsString, PathBuf and Cow<str> take the from of a string, and
//     &OsStr and &Path the new of a string;
//   - an option takes None, and Some of a value of its type;
//   - Vec<u8> takes the from of a byte string;
//   - Box, Rc, Arc, Cell, RefCell, Mutex and RwLock take the value of their
//     type inside the call that [Rules.DeriveSamples] writes for them, and a
//     shared borrow the address of a value of its type;
//   - an enum and a data enum take the name or the path of a unit variant,
//     as Red or Color::Red, which becomes the name of the variant;
//   - an alias takes what its target takes;
//   - every type takes the name or the path of a constant or a static of the
//     scope s of its type.
//
// A nil reference takes every literal but a path, as a value of its own
// kind. An integer is its decimal text, and a float the shortest decimal
// text that reads back to it at the precision of its type. Every other type
// takes no literal.
//
// # Allocation contract
//
// LiteralFor allocates what [scanLiteral] allocates, the decimal text of a
// number, and the list of the argument of each call. The name of a constant
// allocates what [Rules.Resolve] allocates.
func (r Rules) LiteralFor(s rules.Scope, ref *node.TypeRef, text string, v rules.View) (emit.Value, bool) {
	lit, scanned := scanLiteral(text)
	if !scanned {
		return emit.Value{}, false
	}
	return r.typed(s, lit, ref, v, 0)
}

// Text returns the value of a text type of Rust whose content is text, for a
// text that an author states:
//
//   - str and a borrow of it take the string literal of text;
//   - String, OsString, PathBuf, Cow<str> and a pointer of the standard
//     library to a text take the from of the string literal, and Path and
//     OsStr the new of it;
//   - char takes the character literal of a text of one character.
//
// Every other text type, CString and CStr among them, and a char of a text of
// another length take the string literal. An alias takes what its target
// takes.
//
// # Allocation contract
//
// Text allocates the argument list of a call, and the character literal of a
// char. A string literal allocates nothing.
func (r Rules) Text(ref *node.TypeRef, text string, v rules.View) emit.Value {
	if value, typed := r.typed(rules.Scope{}, literal{kind: literalString, text: text}, ref, v, 0); typed {
		return value
	}
	char, size := utf8.DecodeRuneInString(text)
	t := rules.Unaliased(ref, v)
	if char != utf8.RuneError && size == len(text) && t != nil && t.Form == symbol.FormNamed &&
		t.Target.IsZero() && t.Spelling == charType {

		return emit.Raw(rust.Lang, quoteChar(char))
	}
	return emit.Literal(emit.LiteralString, text)
}

// typed returns the literal lit as a value of the type ref by the rules of
// [Rules.LiteralFor], at a depth of the walk.
func (r Rules) typed(s rules.Scope, lit literal, ref *node.TypeRef, v rules.View, depth int) (emit.Value, bool) {
	switch {
	case depth > deriveDepth:
		return emit.Value{}, false
	case ref == nil:
		return lit.value()
	case lit.kind == literalPath:
		return r.named(s, lit.text, ref, v, depth)
	}
	switch ref.Form {
	case symbol.FormBorrow:
		elem := firstElem(ref)
		switch size := sizeOf(elem); {
		case ref.Mutability == symbol.MutabilityMutable || elem == nil:
			return emit.Value{}, false
		case size == sizelessSlice:
			if lit.kind != literalBytes || !byteType(firstElem(elem)) {
				return emit.Value{}, false
			}
			return emit.Raw(rust.Lang, lit.text), true
		case size == sizelessText:
			return r.typed(s, lit, elem, v, depth+1)
		}
		x, ok := r.typed(s, lit, elem, v, depth+1)
		if !ok {
			return emit.Value{}, false
		}
		return emit.Address(x), true
	case symbol.FormNamed:
	default:
		return emit.Value{}, false
	}
	if !ref.Target.IsZero() {
		sym, _ := v.Lookup(ref.Target)
		alias, isAlias := sym.(*node.Alias)
		if !isAlias || alias.Target == nil {
			return emit.Value{}, false
		}
		return r.typed(s, lit, rules.Substitute(alias.Target, alias.TypeParams, typeArgsFor(alias.TypeParams, ref)), v,
			depth+1)
	}
	return r.typedLib(s, lit, ref, v, depth)
}

// typedLib returns the literal lit as a value of a reference ref without a
// target, by the rules of [Rules.LiteralFor].
func (r Rules) typedLib(s rules.Scope, lit literal, ref *node.TypeRef, v rules.View, depth int) (emit.Value, bool) {
	if ref.Spelling == unitType {
		if lit.kind != literalUnit {
			return emit.Value{}, false
		}
		return lit.value()
	}
	t, name, known := libTypeOf(ref)
	if !known {
		return emit.Value{}, false
	}
	build := symbol.Identity{Lang: rust.Lang, Package: t.home, Owner: name, Name: newFunction, Kind: symbol.KindMethod}
	from := build
	from.Name = fromFunction
	elem := typeArg(ref, 0)
	switch {
	case t.form == symbol.FormScalar:
		if lit.suffix != "" && lit.suffix != name {
			return emit.Value{}, false
		}
		switch {
		case t.class == rules.ScalarFloat && (lit.kind == literalFloat || lit.kind == literalInt):
			f, ok := lit.float(t.bits)
			if !ok {
				return emit.Value{}, false
			}
			return emit.Number(emit.LiteralFloat, numeric.Decimal(f, t.bits), t.bits), true
		case t.class != rules.ScalarFloat && lit.kind == literalInt:
			n, ok := lit.integer()
			if !ok {
				return emit.Value{}, false
			}
			return numeric.Int(n, t.class, t.bits)
		}
		return emit.Value{}, false
	case t.form == symbol.FormBool && lit.kind == literalBool,
		name == charType && lit.kind == literalChar,
		name == strType && lit.kind == literalString:
		return lit.value()
	case t.form == symbol.FormText && (t.kind == libNoValue || lit.kind != literalString || name == charType):
		return emit.Value{}, false
	case t.form == symbol.FormText && t.kind == libUnsized:
		return emit.Call(build, emit.Literal(emit.LiteralString, lit.text)), true
	case t.form == symbol.FormText:
		return emit.Call(from, emit.Literal(emit.LiteralString, lit.text)), true
	case t.form == symbol.FormOptional && lit.kind == literalSome:
		inner, scanned := scanLiteral(lit.text)
		if !scanned {
			return emit.Value{}, false
		}
		x, ok := r.typed(s, inner, elem, v, depth+1)
		if !ok {
			return emit.Value{}, false
		}
		return emit.Call(someCall, x), true
	case name == vecType && lit.kind == literalBytes && byteType(elem):
		return emit.Call(from, emit.Raw(rust.Lang, lit.text)), true
	case t.form == symbol.FormBorrow && (name != boxType || !errorObject(ref)):
		x, ok := r.typed(s, lit, elem, v, depth+1)
		if !ok {
			return emit.Value{}, false
		}
		if t.kind == libPointer && sizeOf(elem) != sized {
			return emit.Call(from, x), true
		}
		return emit.Call(build, x), true
	default:
		return emit.Value{}, false
	}
}

// named returns the path text as a value of the type ref: None for an
// option, a unit variant of the enum or the data enum that ref refers to, as
// Red or Color::Red, and else a constant or a static that text names in the
// scope s whose type is ref. An alias reads as its target.
func (r Rules) named(s rules.Scope, text string, ref *node.TypeRef, v rules.View, depth int) (emit.Value, bool) {
	if ref != nil && ref.Form == symbol.FormNamed && text == noneVariant {
		if t, _, known := libTypeOf(ref); known && ref.Target.IsZero() && t.form == symbol.FormOptional {
			return emit.Literal(emit.LiteralNil, ""), true
		}
	}
	if ref != nil && ref.Form == symbol.FormNamed && !ref.Target.IsZero() {
		owner, name, qualified := cutPath(text)
		sym, _ := v.Lookup(ref.Target)
		switch d := sym.(type) {
		case *node.Enum:
			for _, variant := range d.Variants {
				if variant != nil && variant.Name == name && (!qualified || owner == d.Name) {
					return emit.Name(variant.ID), true
				}
			}
		case *node.Sum:
			for _, variant := range d.Variants {
				unit := variant != nil && len(variant.Fields) == 0
				if unit && variant.Name == name && (!qualified || owner == d.Name) {
					return emit.Name(variant.ID), true
				}
			}
		case *node.Alias:
			if d.Target != nil && depth < deriveDepth {
				target := rules.Substitute(d.Target, d.TypeParams, typeArgsFor(d.TypeParams, ref))
				if x, ok := r.named(s, text, target, v, depth+1); ok {
					return x, true
				}
			}
		}
	}
	sym, err := r.Resolve(s, text, directive.ResolvePackageVar, v)
	if err != nil {
		return emit.Value{}, false
	}
	var typ *node.TypeRef
	switch d := sym.(type) {
	case *node.Constant:
		typ = d.Type
	case *node.Variable:
		typ = d.Type
	}
	decl, declared := sym.(node.Declaration)
	if !declared || ref != nil && !sameType(typ, ref) {
		return emit.Value{}, false
	}
	return emit.Name(decl.Identity()), true
}

// value returns the literal as a value of its own kind: an integer as its
// decimal text, a float as the shortest decimal text that reads back to it
// at double precision, a string, a truth value, the unit value, and a
// character literal and a byte string as Rust text. A path and a literal of
// Some refer to a type that only a reference resolves, so value reports
// false for them.
func (lit literal) value() (emit.Value, bool) {
	switch lit.kind {
	case literalInt:
		n, ok := lit.integer()
		if !ok {
			return emit.Value{}, false
		}
		return emit.Number(emit.LiteralInt, n.ExactString(), 0), true
	case literalFloat:
		f, ok := lit.float(doubleBits)
		if !ok {
			return emit.Value{}, false
		}
		return emit.Number(emit.LiteralFloat, numeric.Decimal(f, doubleBits), 0), true
	case literalString:
		return emit.Literal(emit.LiteralString, lit.text), true
	case literalBool:
		return emit.Literal(emit.LiteralBool, lit.text), true
	case literalUnit:
		return emit.Composite(&emit.TypeRef{Spelling: unitType, Form: symbol.FormTuple}), true
	case literalChar, literalBytes:
		return emit.Raw(rust.Lang, lit.text), true
	default:
		return emit.Value{}, false
	}
}

// unknownRefusal returns the refusal of a reference without a target that
// is not a type of the table of [libTypes]: [rules.RefusedNoLiteral] for a
// path of the standard library and for the never type, whose types the rules
// know without a value, and [rules.RefusedUnresolved] for a type of another
// crate and for a bare name that nothing binds.
func unknownRefusal(ref *node.TypeRef) rules.Refusal {
	if _, std := stdName(ref); ref.Spelling == neverType || std && ref.Package != "" {
		return rules.RefusedNoLiteral
	}
	return rules.RefusedUnresolved
}

// libModule returns the module of a reference ref to a type of the standard
// library below its crate, such as io for std::io::Error, and the empty
// module for a name of the prelude. It allocates nothing.
func libModule(ref *node.TypeRef) string {
	_, module, _ := strings.Cut(ref.Package, packageSep)
	return module
}

// sizeless is the class of a type without a size, which a value states
// through a reference, and which a pointer of the standard library builds
// through its from.
type sizeless uint8

const (
	// sized is a type with a size.
	sized sizeless = iota
	// sizelessText is a text of the standard library: str, OsStr and Path.
	// Its values are references already, as a string literal is a &str.
	sizelessText
	// sizelessSlice is a slice [T], whose value is an array that a borrow
	// of the slice takes the address of.
	sizelessSlice
)

// sizeOf returns the class of the size of the type ref. It allocates
// nothing.
func sizeOf(ref *node.TypeRef) sizeless {
	switch {
	case ref == nil:
		return sized
	case ref.Form == symbol.FormList:
		return sizelessSlice
	case ref.Form != symbol.FormNamed || !ref.Target.IsZero():
		return sized
	}
	if t, _, known := libTypeOf(ref); known && t.form == symbol.FormText && t.kind == libUnsized {
		return sizelessText
	}
	return sized
}

// byteType reports whether ref is the primitive u8. It allocates nothing.
func byteType(ref *node.TypeRef) bool {
	if ref == nil || ref.Form != symbol.FormNamed || !ref.Target.IsZero() {
		return false
	}
	t, _, known := libTypeOf(ref)
	return known && t.form == symbol.FormScalar && t.class == rules.ScalarUint && t.bits == 8
}

// fieldID returns the identity of the field of the struct s with the name,
// whose authored values a parameter of a construction of s reads, and the
// zero identity where s has no such field.
func fieldID(s *node.Struct, name string) symbol.Identity {
	for _, f := range s.Fields {
		if f != nil && f.Name == name && name != "" {
			return f.ID
		}
	}
	return symbol.Identity{}
}

// cutPath returns the path of text without its last segment, and the last
// segment, and reports whether text has more than one segment. A path
// through modules keeps only the segment before the last as its owner, as
// models::Color::Red has the owner Color. It allocates nothing.
func cutPath(text string) (owner, name string, qualified bool) {
	owner, name, qualified = strings.CutLast(text, pathSep)
	if !qualified {
		return "", text, false
	}
	if _, last, nested := strings.CutLast(owner, pathSep); nested {
		owner = last
	}
	return owner, name, true
}

// sameType reports whether the references a and b denote one type: one form,
// mutability and length, one target, one spelling for a named reference
// without a target, and children and arguments of one type in order. It
// allocates nothing.
func sameType(a, b *node.TypeRef) bool {
	switch {
	case a == nil || b == nil:
		return a == b
	case a.Form != b.Form || a.Target != b.Target || a.Mutability != b.Mutability || a.Length != b.Length,
		a.Form == symbol.FormNamed && a.Target.IsZero() && a.Spelling != b.Spelling,
		len(a.Elems) != len(b.Elems) || len(a.Args) != len(b.Args):
		return false
	}
	for i := range a.Elems {
		if !sameType(a.Elems[i], b.Elems[i]) {
			return false
		}
	}
	for i := range a.Args {
		if !sameType(a.Args[i], b.Args[i]) {
			return false
		}
	}
	return true
}

// firstElem returns the first child of a structural reference, the type of a
// borrow, a slice, an array or an option, and nil where the reference has no
// child.
func firstElem(ref *node.TypeRef) *node.TypeRef {
	if ref != nil && len(ref.Elems) > 0 {
		return ref.Elems[0]
	}
	return nil
}
