// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"slices"
	"strings"

	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The names that the error contract reads: the field of the cause by the
// convention of std::error::Error::source, the attributes of thiserror that
// mark the cause, and the crate of thiserror, whose derive of Error makes a
// type an error.
const (
	sourceName     = "source"
	fromAttr       = "from"
	thiserrorCrate = "thiserror"
)

// sizedMark relaxes the bound Sized, as T: ?Sized does. It constrains no
// witness.
const sizedMark = "?Sized"

// The traits of the standard library that the unit type () implements, and
// those that String implements. The witness of a type parameter is the first
// of the two types that implements every bound of the parameter.
var (
	unitTraits = []string{
		"Clone", "Copy", "Debug", defaultTrait, eqTrait, "Hash", ordTrait, partialEqTrait, partialOrd,
		sendTrait, syncTrait, "Sized", unpinTrait, "Any", "UnwindSafe", "RefUnwindSafe",
	}
	stringTraits = []string{
		"Clone", "Debug", defaultTrait, "Display", eqTrait, "Hash", ordTrait, partialEqTrait, partialOrd,
		sendTrait, syncTrait, "Sized", unpinTrait, "Any", "UnwindSafe", "RefUnwindSafe", "ToString", "FromStr",
	}
)

// stringBounds maps the traits with one type argument that String
// implements to the spelling of the argument: AsRef<str>, Borrow<str> and
// Into<String>.
var stringBounds = map[string]string{"AsRef": strType, "Borrow": strType, "Into": stringType}

// comparableTraits are the traits whose bound on a type parameter gives the
// parameter the operator ==, because each of them extends PartialEq.
var comparableTraits = []string{partialEqTrait, eqTrait, partialOrd, ordTrait}

// construction is one way that a value of a struct builds: the call of an
// associated function of the struct, or the literal of the struct where
// method is nil. callable has the parameters of the call, or the fields of
// the literal, and direct reports that the call returns the struct itself,
// and not a result or an option of it.
type construction struct {
	method   *node.Method
	callable rules.Callable
	direct   bool
}

// value returns the value that the construction c of the struct s builds
// from the values args of its parameters: the call of the associated
// function, the literal of a struct with named fields, as Order { id: 1 },
// and the literal of a tuple struct, as Id(1).
func (c construction) value(s *node.Struct, args []emit.Value) emit.Value {
	if c.method != nil {
		return emit.Call(c.method.ID, args...)
	}
	var fields []emit.ValueField
	if len(args) > 0 {
		fields = make([]emit.ValueField, 0, len(args))
	}
	for i, x := range args {
		if c.callable.Params[i].Passing == symbol.PassingPositional {
			fields = append(fields, emit.Element(x))
			continue
		}
		fields = append(fields, emit.NamedField(c.callable.Params[i].Name, x))
	}
	return emit.Composite(&emit.TypeRef{Spelling: s.Name, Target: s.ID}, fields...)
}

// Constructors returns the callables that construct a struct, in this order:
//
//  1. each associated function of the struct, a method at the level of the
//     type that is not private, whose return is Self or the struct, also
//     inside a result and an option, as Result<Self, Error>, in declaration
//     order;
//  2. the literal of the struct, where no field is private. The literal has
//     the name of the struct, and a parameter for each field: by keyword for
//     a named field and by position for a field of a tuple struct.
//
// A nil struct has no constructor.
//
// # Allocation contract
//
// Constructors allocates the list of callables, the binding that projects
// the associated functions, and the lists of each callable.
func (r Rules) Constructors(s *node.Struct, v rules.View) []rules.Callable {
	if s == nil {
		return nil
	}
	cs := r.constructions(s, v)
	out := make([]rules.Callable, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.callable)
	}
	return out
}

// constructions returns the constructions of the struct s in the order of
// [Rules.Constructors].
func (r Rules) constructions(s *node.Struct, v rules.View) []construction {
	var out []construction
	var b rules.Bound
	for _, m := range s.Methods {
		if m == nil || m.Level != symbol.LevelType || m.Visibility == symbol.VisibilityPrivate ||
			len(m.Returns) != 1 || m.Returns[0] == nil {

			continue
		}
		direct, builds := r.builds(m.Returns[0].Type, s, v, 0)
		if !builds {
			continue
		}
		if b.View().IsZero() {
			b = rules.NewBound(r, v, nil)
		}
		c, _ := b.CallableOf(m)
		out = append(out, construction{method: m, callable: c, direct: direct})
	}
	literal := rules.Callable{Name: s.Name}
	for _, f := range s.Fields {
		switch {
		case f == nil || f.Level == symbol.LevelType:
			continue
		case f.Visibility == symbol.VisibilityPrivate:
			return out
		}
		p := rules.ParamView{Name: f.Name, Ref: f.Type, Role: rules.ParamInput, Passing: symbol.PassingKeyword}
		if f.Name == "" {
			p.Passing = symbol.PassingPositional
		}
		literal.Params = append(literal.Params, p)
	}
	return append(out, construction{callable: literal, direct: true})
}

// builds reports whether a function of the struct s with the return type ret
// builds the struct, and whether it returns the struct itself: a reference to
// s returns it, as Self does, which the load resolves to the struct of the
// impl block. A result or an option of such a type builds it inside the
// wrapper. It reports false past [deriveDepth] wrappers.
func (r Rules) builds(ret *node.TypeRef, s *node.Struct, v rules.View, depth int) (direct, builds bool) {
	switch {
	case ret == nil || ret.Form != symbol.FormNamed || depth > deriveDepth:
		return false, false
	case !ret.Target.IsZero() && ret.Target == s.ID:
		return true, true
	}
	if value, _, ok := r.Result(ret, v); ok {
		_, builds = r.builds(value, s, v, depth+1)
		return false, builds
	}
	if t, _, known := libTypeOf(ret); known && ret.Target.IsZero() && t.form == symbol.FormOptional {
		_, builds = r.builds(typeArg(ret, 0), s, v, depth+1)
		return false, builds
	}
	return false, false
}

// ErrorContract returns the contract of an error type: a struct, an enum or
// a data enum whose trait impls implement std::error::Error, or that derives
// Error of thiserror, as #[derive(thiserror::Error)] or #[derive(Error)]
// beside use thiserror::Error.
//
// Cause is the field of a struct that the attribute #[source] or #[from]
// marks, and else the field named source, which std::error::Error::source
// returns by convention. The message comes from an impl of Display, whose
// methods the frontend does not add to the type, so Message is nil. Rust
// calls no hook of an error type, so Unwrap, Is and As are nil, and a value
// of the type is the error, so Pointer is false. ErrorContract reports false
// for every other declaration.
//
// # Allocation contract
//
// ErrorContract allocates what the view allocates to record the read of the
// scope of a type that derives Error.
func (Rules) ErrorContract(decl symbol.Symbol, v rules.View) (rules.ErrorContract, bool) {
	var (
		id          symbol.Identity
		annotations symbol.Annotations
		implements  []*node.TypeRef
		fields      []*node.Field
	)
	switch d := decl.(type) {
	case *node.Struct:
		if d == nil {
			return rules.ErrorContract{}, false
		}
		id, annotations, implements, fields = d.ID, d.Annotations, d.Implements, d.Fields
	case *node.Enum:
		if d == nil {
			return rules.ErrorContract{}, false
		}
		id, annotations, implements = d.ID, d.Annotations, d.Implements
	case *node.Sum:
		if d == nil {
			return rules.ErrorContract{}, false
		}
		id, annotations, implements = d.ID, d.Annotations, d.Implements
	default:
		return rules.ErrorContract{}, false
	}
	if !implementsTrait(implements, errorTrait) && !derivesThiserror(annotations, v.ScopeOf(id).File) {
		return rules.ErrorContract{}, false
	}
	var c rules.ErrorContract
	for _, f := range fields {
		if f == nil || f.Level == symbol.LevelType {
			continue
		}
		marked := slices.ContainsFunc(f.Annotations, func(an symbol.Annotation) bool {
			return an.Name == sourceName || an.Name == fromAttr
		})
		if marked {
			c.Cause = f
			return c, true
		}
		if f.Name == sourceName && c.Cause == nil {
			c.Cause = f
		}
	}
	return c, true
}

// derivesThiserror reports whether a derive attribute among the annotations
// names Error of thiserror: the path thiserror::Error, or the name Error
// where a use declaration of the file f binds it from the root of thiserror,
// which re-exports the derive. It allocates nothing.
func derivesThiserror(annotations symbol.Annotations, f *node.File) bool {
	for _, an := range annotations {
		if an.Name != deriveAttr {
			continue
		}
		for _, arg := range an.Args {
			arg = strings.TrimSpace(arg)
			module, item, imported := bindingOf(f, arg)
			if arg == thiserrorCrate+pathSep+errorTrait ||
				arg == errorTrait && imported && item == errorTrait && module == thiserrorCrate {

				return true
			}
		}
	}
	return false
}

// Ownership returns how a call passes a parameter: a shared borrow &T, as the
// receiver &self, lends the value, a mutable borrow &mut T, as the receiver
// &mut self, lends it for mutation, and every other parameter moves or copies
// the value, as self and Box<Self> do. It allocates nothing.
func (Rules) Ownership(p rules.ParamView) rules.Ownership {
	switch ref := p.Ref; {
	case ref == nil || ref.Form != symbol.FormBorrow:
		return rules.OwnByValue
	case ref.Mutability == symbol.MutabilityMutable:
		return rules.OwnBorrowMut
	default:
		return rules.OwnBorrow
	}
}

// Comparable reports whether a value of a type works with the operator ==,
// which Rust gives a type that implements PartialEq, and returns the
// references that break it:
//
//   - a primitive, a text, the unit type, Duration, SystemTime, Instant and
//     fmt::Error compare;
//   - a borrow, an option, a slice, an array, a tuple, a vector, a map, a
//     set, Box, Rc, Arc, Cell, RefCell and a result compare where their
//     members and type arguments compare;
//   - a struct, an enum and a data enum compare where they derive PartialEq,
//     and a struct also where a trait impl implements PartialEq for it, and
//     each of its type arguments compares;
//   - a type parameter compares where a bound extends PartialEq;
//   - an alias compares as its target;
//   - Mutex, RwLock, io::Error, a function, a trait object, an iterator and
//     a trait do not compare.
//
// A float compares, because PartialEq admits it. A type of another crate
// and a type outside the view report no problem, because their equality is
// unknown.
//
// # Allocation contract
//
// Comparable allocates the list of problems as it grows, and nothing for a
// type that compares. Each read allocates what the view allocates to record
// it.
func (r Rules) Comparable(ref *node.TypeRef, v rules.View) (bool, []*node.TypeRef) {
	var problems []*node.TypeRef
	r.comparable(ref, v, 0, &problems)
	return len(problems) == 0, problems
}

// comparable appends to problems each reference inside ref that does not
// compare, by the rules of [Rules.Comparable], at a depth of the walk.
func (r Rules) comparable(ref *node.TypeRef, v rules.View, depth int, problems *[]*node.TypeRef) {
	if ref == nil || depth > deriveDepth {
		return
	}
	switch ref.Form {
	case symbol.FormBorrow, symbol.FormOptional, symbol.FormList, symbol.FormArray, symbol.FormTuple:
		for _, e := range ref.Elems {
			r.comparable(e, v, depth+1, problems)
		}
		return
	case symbol.FormFunc, symbol.FormIntersection:
		*problems = append(*problems, ref)
		return
	case symbol.FormNamed:
	default:
		return
	}
	if ref.Target.IsZero() {
		r.comparableLib(ref, v, depth, problems)
		return
	}
	sym, held := v.Lookup(ref.Target)
	if !held {
		return
	}
	switch d := sym.(type) {
	case *node.Struct, *node.Enum, *node.Sum:
		if !hasTrait(sym, partialEqTrait) {
			*problems = append(*problems, ref)
			return
		}
		r.comparableArgs(ref, v, depth, problems)
	case *node.Alias:
		if d.Target != nil {
			r.comparable(rules.Substitute(d.Target, d.TypeParams, typeArgsFor(d.TypeParams, ref)), v, depth+1, problems)
		}
	case *node.TypeParam:
		compares := false
		for _, trait := range comparableTraits {
			compares = compares || implementsTrait(d.Bounds, trait)
		}
		if !compares {
			*problems = append(*problems, ref)
		}
	case *node.Interface:
		*problems = append(*problems, ref)
	}
}

// comparableLib appends to problems what breaks the equality of a reference
// ref without a target: the reference itself for Mutex, RwLock, an error
// other than fmt::Error, an iterator and a stream, and what breaks the
// equality of a type argument of every other type of the standard library.
func (r Rules) comparableLib(ref *node.TypeRef, v rules.View, depth int, problems *[]*node.TypeRef) {
	if _, _, result := r.Result(ref, v); result {
		r.comparableArgs(ref, v, depth, problems)
		return
	}
	t, name, known := libTypeOf(ref)
	switch {
	case !known:
	case name == mutexType, name == rwLockType, t.form == symbol.FormStream,
		t.wellKnown == rules.WellKnownError && libModule(ref) != fmtModule:

		*problems = append(*problems, ref)
	default:
		r.comparableArgs(ref, v, depth, problems)
	}
}

// comparableArgs appends to problems what breaks the equality of each type
// argument of ref, past its lifetimes.
func (r Rules) comparableArgs(ref *node.TypeRef, v rules.View, depth int, problems *[]*node.TypeRef) {
	for _, a := range ref.Args {
		if a != nil && !strings.HasPrefix(a.Spelling, lifetimeMark) {
			r.comparable(a, v, depth+1, problems)
		}
	}
}

// Derive returns the witness of a type parameter: its default where it has
// one, and else the unit type () where every bound is a trait of the
// standard library that () implements, such as Debug and Clone, and String
// where every bound is a trait that String implements, such as Display and
// AsRef<str>. The relaxation ?Sized constrains neither. A const parameter
// takes a value and no type, and a parameter with another bound, such as a
// trait of the crate, has no witness, so Derive reports false for both.
//
// # Allocation contract
//
// Derive allocates the reference to () or to String, one allocation, and
// nothing for a default.
func (Rules) Derive(p *node.TypeParam, _ rules.View) (*node.TypeRef, bool) {
	switch {
	case p == nil || p.Const:
		return nil, false
	case p.Default != nil:
		return p.Default, true
	case satisfies(p.Bounds, unitTraits, nil):
		return &node.TypeRef{Spelling: unitType}, true
	case satisfies(p.Bounds, stringTraits, stringBounds):
		return &node.TypeRef{Spelling: stringType}, true
	default:
		return nil, false
	}
}

// satisfies reports whether a type that implements the traits of the
// standard library traits, and the traits of one type argument of withArg
// with the spelling of the argument that withArg maps them to, satisfies
// every bound. A bound is a trait of the standard library without a target,
// or the relaxation ?Sized. It allocates nothing.
func satisfies(bounds []*node.TypeRef, traits []string, withArg map[string]string) bool {
	for _, b := range bounds {
		if b == nil || b.Spelling == sizedMark {
			continue
		}
		name, std := stdName(b)
		switch {
		case !std:
			return false
		case len(b.Args) == 0 && slices.Contains(traits, name):
		case len(b.Args) == 1 && b.Args[0] != nil && withArg[name] != "" && b.Args[0].Spelling == withArg[name]:
		default:
			return false
		}
	}
	return true
}

// Reified reports true, because Rust compiles each instantiation of a generic
// declaration apart, so a value keeps its type arguments. Reified allocates
// nothing.
func (Rules) Reified() bool { return true }
