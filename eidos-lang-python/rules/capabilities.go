// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"iter"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The special methods and attributes that the capabilities read: the
// constructor and the allocator of a class, the method that writes the
// message of an exception and the attribute of its cause, equality and the
// hash.
const (
	initMethod = "__init__"
	newMethod  = "__new__"
	strMethod  = "__str__"
	causeField = "__cause__"
	eqMethod   = "__eq__"
	hashMethod = "__hash__"
)

// constraintsOpen opens the spelling of the bound of a type variable with
// constraints, as (int, str), which the frontend loads as one bound of the
// form Union.
const constraintsOpen = "("

// The names of the dataclasses and of typing that the constructors and the
// equality of a class read: the decorator, the function of the options of
// a field, the sentinel that makes the later fields keyword-only, and the
// base of a named tuple. The options of the decorator and of field turn the
// generated __init__ off, make the parameters keyword-only, freeze the
// instances, generate a hash and compare by the fields. A field specifier of
// PEP 681 also takes the option alias, the name of the parameter of its
// field, and factory, the default factory.
const (
	dataclassDecorator = "dataclass"
	fieldFunction      = "field"
	kwOnlySentinel     = "KW_ONLY"
	namedTupleClass    = "NamedTuple"
	initOption         = "init"
	kwOnlyOption       = "kw_only"
	frozenOption       = "frozen"
	unsafeHashOption   = "unsafe_hash"
	eqOption           = "eq"
	defaultOption      = "default"
	factoryOption      = "default_factory"
	factoryShort       = "factory"
	aliasOption        = "alias"
	trueValue          = "True"
	falseValue         = "False"
	optionSep          = "="
)

// The classes of sets without a hash, whose __hash__ is None, beside the
// shapes of mutable lists and maps and bytearray: set of builtins, and Set
// and MutableSet of typing and collections.abc.
const (
	setClass        = "set"
	setAlias        = "Set"
	mutableSetClass = "MutableSet"
)

// The marks of the text of a call that [callOf] splits: the brackets that
// nest, the separator of the arguments, and the escape inside a string.
const (
	callOpen   = "("
	callClose  = ")"
	openers    = "([{"
	closers    = ")]}"
	argSep     = ','
	escapeMark = '\\'
)

// objectWitness is the witness of a type parameter without a bound and
// without a default: the top type of Python.
const objectWitness = objectClass

// ErrorContract returns the contract of a class whose bases reach an
// exception class of builtins, such as Exception or ValueError, through
// classes that the view declares. Message is the method __str__ among the
// members of the class, which inherit the members of its bases, and Cause
// is the field __cause__ among them. Each is nil where only a class of
// builtins declares it, because the class then inherits it from the error of
// the language, as every exception inherits the __str__ and the __cause__ of
// BaseException. Python calls no hook of an exception, so Unwrap, Is and As
// are nil. ErrorContract reports false for every other declaration, and for
// a chain deeper than the default depth of the kernel.
//
// It allocates the binding of the member walk and what the walk allocates.
func (r Rules) ErrorContract(decl symbol.Symbol, v rules.View) (rules.ErrorContract, bool) {
	s, isClass := decl.(*node.Struct)
	if !isClass || s == nil || !r.raises(s, v, 0) {
		return rules.ErrorContract{}, false
	}
	var c rules.ErrorContract
	set, _ := rules.NewBound(r, v, nil).MembersOf(s)
	for _, m := range set.Members {
		if m.Owner.Package == builtinsModule {
			continue
		}
		switch x := m.Symbol.(type) {
		case *node.Method:
			if x.Name == strMethod && c.Message == nil {
				c.Message = x
			}
		case *node.Field:
			if x.Name == causeField && c.Cause == nil {
				c.Cause = x
			}
		}
	}
	return c, true
}

// raises reports whether a base of the class s is an exception class of
// builtins, or a class that the view declares and that raises in turn, to
// the default depth of the kernel at depth. A base without a target names a
// class of builtins where nothing binds its name.
func (r Rules) raises(s *node.Struct, v rules.View, depth int) bool {
	if depth >= rules.DefaultDepth {
		return false
	}
	for _, base := range s.Extends {
		switch {
		case base == nil:
		case base.Target.IsZero():
			module, name, named := builtinName(base)
			if named && module == builtinsModule && python.IsBuiltinException(name) {
				return true
			}
		case base.Target.Package == builtinsModule && base.Target.Owner == "" &&
			python.IsBuiltinException(base.Target.Name):
			return true
		default:
			sym, _ := v.Lookup(base.Target)
			if next, isClass := sym.(*node.Struct); isClass && r.raises(next, v, depth+1) {
				return true
			}
		}
	}
	return false
}

// Properties returns the properties of a class. Each getter of an instance,
// a method under the decorator property, is a property, in the order of the
// member walk, and the setter of the same name is its setter. The type of a
// property is the type of the result of its getter, and nil where the getter
// states none. A setter without a getter is no property, because no code
// reads it, and a deleter is no part of the view.
//
// # Allocation contract
//
// Properties allocates the binding of the member walk, what the walk
// allocates, and the list of properties, sized to the members of the walk.
func (r Rules) Properties(s *node.Struct, v rules.View) []rules.Property {
	set, _ := rules.NewBound(r, v, nil).MembersOf(s)
	out := make([]rules.Property, 0, len(set.Members))
	for _, m := range set.Members {
		get, isMethod := m.Symbol.(*node.Method)
		if !isMethod || get.Accessor != symbol.AccessorGet || get.Level != symbol.LevelInstance {
			continue
		}
		p := rules.Property{Name: get.Name, Getter: get}
		if len(get.Returns) > 0 && get.Returns[0] != nil {
			p.Type = get.Returns[0].Type
		}
		for _, other := range set.Members {
			setter, isSetter := other.Symbol.(*node.Method)
			if isSetter && setter.Accessor == symbol.AccessorSet && setter.Level == symbol.LevelInstance &&
				setter.Name == get.Name {

				p.Setter = setter
				break
			}
		}
		out = append(out, p)
	}
	return out
}

// Constructors returns the signatures that construct an instance of a class,
// one for each overload. The first class of the chain of the class and its
// first bases that states a constructor decides:
//
//   - A dataclass generates __init__ from its fields, unless its decorator
//     states init=False. The fields of a base dataclass come first, and a
//     field that the class declares again keeps its place. A field at type
//     level, a field of field(init=False) and the sentinel KW_ONLY have no
//     parameter. A field with a default is optional, and a field after
//     KW_ONLY, of field(kw_only=True), or of a decorator with kw_only=True is
//     passed by keyword. The keyword parameters follow the others, as a
//     dataclass moves them to the end of the signature. A class that a
//     dataclass_transform of PEP 681 makes a dataclass, as
//     [Rules.dataclass] finds it, generates __init__ the same way under the
//     defaults of the transform, and a field of its field specifiers takes
//     the name of its alias.
//   - A named tuple generates __new__ from its fields, and a field with a
//     default is optional.
//   - A class that declares __init__ constructs through it, and a class
//     without __init__ through its __new__. The overloads of the method are
//     its signatures, and its implementation is not.
//
// A class without a constructor in its chain constructs through the
// __init__ of object, which takes nothing. The list is empty for a chain
// deeper than the default depth of the kernel.
//
// # Allocation contract
//
// Constructors allocates the list of callables and the lists of each
// callable. A dataclass allocates the parameters of its fields and the
// fields of its bases, and reads the scope of each class. A declared
// constructor also allocates the binding that projects it.
func (r Rules) Constructors(s *node.Struct, v rules.View) []rules.Callable {
	for range rules.DefaultDepth {
		form, generated := r.dataclass(s, v)
		switch {
		case generated && option(slices.Values(form.options), initOption) != falseValue:
			return []rules.Callable{{Name: initMethod, Params: r.dataclassParams(s, form, v, 0)}}
		case r.namedTuple(s, v):
			return []rules.Callable{{Name: newMethod, Params: tupleParams(s)}}
		}
		if out, declared := r.declaredConstructors(s, v); declared {
			return out
		}
		base, extends := firstClass(s, v)
		if !extends {
			return []rules.Callable{{Name: initMethod}}
		}
		s = base
	}
	return nil
}

// firstClass returns the first base of the class s that is a class the view
// declares, and reports false for a class without one. A class of Python
// finds an attribute that it does not declare in that base first, unless
// another base declares it. It allocates what the view allocates to record
// its reads.
func firstClass(s *node.Struct, v rules.View) (*node.Struct, bool) {
	for _, ext := range s.Extends {
		if ext == nil {
			continue
		}
		sym, _ := v.Lookup(ext.Target)
		if next, isClass := sym.(*node.Struct); isClass {
			return next, true
		}
	}
	return nil, false
}

// declaredConstructors returns the signatures of the __init__ that the class
// s declares, or of its __new__ where it declares no __init__, and reports
// whether it declares either. The overloads of a method are its signatures,
// and the implementation after them is not.
func (r Rules) declaredConstructors(s *node.Struct, v rules.View) ([]rules.Callable, bool) {
	for _, name := range [2]string{initMethod, newMethod} {
		var methods []*node.Method
		overloaded := false
		for _, m := range s.Methods {
			if m == nil || m.Name != name {
				continue
			}
			if rules.IsImplementation(m, s.Methods) {
				overloaded = true
				continue
			}
			methods = append(methods, m)
		}
		if len(methods) == 0 && !overloaded {
			continue
		}
		b := rules.NewBound(r, v, nil)
		out := make([]rules.Callable, 0, len(methods))
		for _, m := range methods {
			c, _ := b.CallableOf(m)
			c.Receiver = nil
			out = append(out, c)
		}
		return out, true
	}
	return nil, false
}

// namedTuple reports whether a base of the class s is NamedTuple of typing.
func (Rules) namedTuple(s *node.Struct, v rules.View) bool {
	scope := v.ScopeOf(s.ID)
	pkg, _ := v.PackageOf(s.ID)
	for _, base := range s.Extends {
		if base == nil {
			continue
		}
		module, name := base.Target.Package, base.Target.Name
		if base.Target.IsZero() {
			module, name = canonical(pkg, scope.File, base.Spelling)
		}
		if module == typingModule && name == namedTupleClass {
			return true
		}
	}
	return false
}

// dataclassParams returns the parameters of the __init__ that the dataclass
// s generates under its form: the fields of its base dataclasses first, and
// its own fields after them. A field that s declares again replaces the
// field of the base in its place, and the keyword parameters follow the
// others in their order. A field of a field specifier of a transform takes
// the name of its alias.
func (r Rules) dataclassParams(s *node.Struct, form dataclassForm, v rules.View, depth int) []rules.ParamView {
	var params []rules.ParamView
	if depth < rules.DefaultDepth {
		for _, ext := range s.Extends {
			if ext == nil {
				continue
			}
			sym, _ := v.Lookup(ext.Target)
			if base, isClass := sym.(*node.Struct); isClass {
				if baseForm, generated := r.dataclass(base, v); generated {
					params = append(params, r.dataclassParams(base, baseForm, v, depth+1)...)
				}
			}
		}
	}
	params = slices.Grow(params, len(s.Fields))
	scope := v.ScopeOf(s.ID)
	pkg, _ := v.PackageOf(s.ID)
	keyword := option(slices.Values(form.options), kwOnlyOption) == trueValue
	for _, f := range s.Fields {
		if f == nil || f.Level == symbol.LevelType {
			continue
		}
		if f.Type != nil {
			if module, name := canonical(pkg, scope.File, f.Type.Spelling); module == dataclassesModule &&
				name == kwOnlySentinel {

				keyword = true
				continue
			}
		}
		callee, args, called := callOf(f.Value)
		fieldOptions := called && fieldCall(pkg, scope.File, callee, form.specifiers)
		if fieldOptions && option(arguments(args), initOption) == falseValue {
			continue
		}
		p := rules.ParamView{Name: f.Name, Ref: f.Type, Role: rules.ParamInput}
		if alias, quoted := python.Unquote(option(arguments(args), aliasOption)); fieldOptions && quoted {
			p.Name = alias
		}
		p.Optional = f.Value != "" && (!fieldOptions || option(arguments(args), defaultOption) != "" ||
			option(arguments(args), factoryOption) != "" || option(arguments(args), factoryShort) != "")
		if keyword || fieldOptions && option(arguments(args), kwOnlyOption) == trueValue {
			p.Passing = symbol.PassingKeyword
		}
		if at := slices.IndexFunc(params, func(q rules.ParamView) bool { return q.Name == f.Name }); at >= 0 {
			params[at] = p
			continue
		}
		params = append(params, p)
	}
	slices.SortStableFunc(params, func(a, b rules.ParamView) int {
		switch byKeyword := a.Passing == symbol.PassingKeyword; {
		case byKeyword == (b.Passing == symbol.PassingKeyword):
			return 0
		case byKeyword:
			return 1
		default:
			return -1
		}
	})
	return params
}

// fieldCall reports whether the callee of a value, in the file f of the
// package pkg, is field of the module dataclasses or one of the field
// specifiers of a transform.
func fieldCall(pkg *node.Package, f *node.File, callee string, specifiers []qualified) bool {
	module, name := canonical(pkg, f, callee)
	return module == dataclassesModule && name == fieldFunction ||
		slices.Contains(specifiers, qualified{module: module, name: name})
}

// tupleParams returns the parameters of the __new__ that the named tuple s
// generates: one for each field of an instance, in order, and optional where
// the field has a default.
func tupleParams(s *node.Struct) []rules.ParamView {
	params := make([]rules.ParamView, 0, len(s.Fields))
	for _, f := range s.Fields {
		if f == nil || f.Level == symbol.LevelType {
			continue
		}
		p := rules.ParamView{Name: f.Name, Ref: f.Type, Role: rules.ParamInput, Optional: f.Value != ""}
		params = append(params, p)
	}
	return params
}

// option returns the value of the keyword argument name among the arguments
// args of a call, each written as name=value, and the empty value where no
// argument has the name. It allocates nothing.
func option(args iter.Seq[string], name string) string {
	for a := range args {
		key, value, keyed := strings.Cut(a, optionSep)
		if keyed && strings.TrimSpace(key) == name {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// callOf returns the callee and the text of the arguments of the text of a
// call, such as field and default=1 for field(default=1), and reports false
// for a text that is no call. It allocates nothing.
func callOf(text string) (callee, args string, ok bool) {
	text = strings.TrimSpace(text)
	open := strings.Index(text, callOpen)
	if open <= 0 || !strings.HasSuffix(text, callClose) {
		return "", "", false
	}
	return strings.TrimSpace(text[:open]), text[open+len(callOpen) : len(text)-len(callClose)], true
}

// arguments yields the arguments of the text args of the arguments of a
// call, in order. The arguments split at the commas outside brackets and
// strings, and each is trimmed of white space.
func arguments(args string) iter.Seq[string] {
	return func(yield func(string) bool) {
		depth, start := 0, 0
		var quote byte
		for i := 0; i < len(args); i++ {
			c := args[i]
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
			case strings.IndexByte(closers, c) >= 0:
				depth--
			case c == argSep && depth == 0:
				if !yield(strings.TrimSpace(args[start:i])) {
					return
				}
				start = i + 1
			}
		}
		if last := strings.TrimSpace(args[start:]); last != "" {
			yield(last)
		}
	}
}

// Comparable reports whether a value of a type works as a key of a dict and
// as a member of a set, where Python demands a hash, and returns the
// references that break it. A list, a dict, a set and a bytearray have no
// hash, and neither have Set and MutableSet of typing and collections.abc
// and a mutable list or map of the table of [Rules.ShapeOf]. A tuple, a
// union, an optional and an immutable list or map hash where their members
// hash. A class hashes by its identity, unless the first class of its chain
// of first bases that states a hash decides otherwise:
//
//   - a class that declares __eq__ without __hash__, or that sets
//     __hash__ to None, has no hash, and neither has a subclass that
//     states no hash;
//   - a dataclass that compares its fields and is not frozen has no hash,
//     unless its decorator states unsafe_hash=True;
//   - a frozen dataclass, a dataclass of unsafe_hash=True and a named tuple
//     hash their fields, the fields of their bases included, so each field
//     must hash. A field of InitVar is no part of the hash, because an
//     instance does not keep it.
//
// A TypedDict is a dict, so it has no hash, and an enum, a Protocol and the
// top type hash. An alias hashes as its target. A type outside the view
// reports no problem, because its hash is unknown.
//
// # Allocation contract
//
// Comparable allocates the list of problems as it grows, and nothing for a
// type that hashes. A dataclass reads its scope, and a class that hashes its
// fields allocates the member walk.
func (r Rules) Comparable(ref *node.TypeRef, v rules.View) (bool, []*node.TypeRef) {
	var problems []*node.TypeRef
	r.hashable(ref, v, 0, &problems)
	return len(problems) == 0, problems
}

// hashable appends to problems each reference inside ref that has no hash,
// by the rules of [Rules.Comparable], at a depth of the walk.
func (r Rules) hashable(ref *node.TypeRef, v rules.View, depth int, problems *[]*node.TypeRef) {
	if ref == nil || depth > deriveDepth {
		return
	}
	switch ref.Form {
	case symbol.FormOptional, symbol.FormUnion, symbol.FormTuple:
		for _, e := range ref.Elems {
			r.hashable(e, v, depth+1, problems)
		}
		return
	case symbol.FormList:
		if ref.Mutability != symbol.MutabilityImmutable {
			*problems = append(*problems, ref)
			return
		}
		r.hashable(firstElem(ref), v, depth+1, problems)
		return
	case symbol.FormNamed:
	default:
		return
	}
	if ref.Target.IsZero() {
		module, name, _ := builtinName(ref)
		r.hashableNamed(ref, module, name, r.Builtin(ref, v), v, depth, problems)
		return
	}
	sym, held := v.Lookup(ref.Target)
	if !held {
		return
	}
	if s, shaped := r.ShapeOf(ref, sym, v); shaped {
		r.hashableNamed(ref, ref.Target.Package, ref.Target.Name, s, v, depth, problems)
		return
	}
	switch d := sym.(type) {
	case *node.Struct:
		r.hashableClass(ref, d, v, depth, problems)
	case *node.Interface:
		if typed, _ := rules.Fact(v, d.ID, meta.Named[bool](python.TypedDictKey)); typed {
			*problems = append(*problems, ref)
		}
	case *node.Alias:
		r.hashable(rules.Substitute(d.Target, d.TypeParams, ref.Args), v, depth+1, problems)
	}
}

// hashableNamed appends ref to problems where the class of the module and the
// name, whose values have the shape s, has no hash, and walks the arguments
// of a class that hashes where they hash.
func (r Rules) hashableNamed(
	ref *node.TypeRef, module, name string, s rules.TypeShape, v rules.View, depth int, problems *[]*node.TypeRef,
) {
	switch {
	case module == builtinsModule && (name == setClass || name == bytearrayClass),
		module != builtinsModule && (name == setAlias || name == mutableSetClass),
		(s.Form == symbol.FormList || s.Form == symbol.FormMap) && s.Mutability != symbol.MutabilityImmutable:
		*problems = append(*problems, ref)
	case s.Form == symbol.FormList || s.Form == symbol.FormMap:
		for _, a := range ref.Args {
			r.hashable(a, v, depth+1, problems)
		}
	}
}

// hashableClass appends to problems what breaks the hash of the class s,
// whose reference is ref, by the rules of [Rules.Comparable]. The first
// class of the chain of s and its first bases that states a hash decides,
// as Python finds __hash__:
//
//   - a class that declares __hash__ hashes;
//   - a class that sets __hash__ to None, or annotates it as None as
//     typeshed does for list, and a class that declares __eq__ without
//     __hash__, has no hash;
//   - a frozen dataclass, a dataclass of unsafe_hash=True and a named tuple
//     hash their fields, the fields of their bases included, without the
//     fields of InitVar;
//   - a dataclass that compares its fields has no hash, and a dataclass of
//     eq=False states no hash.
//
// A chain without a class that states a hash ends at object, which hashes,
// and so does a chain deeper than the default depth of the kernel.
func (r Rules) hashableClass(ref *node.TypeRef, s *node.Struct, v rules.View, depth int, problems *[]*node.TypeRef) {
	for range rules.DefaultDepth {
		form, generated := r.dataclass(s, v)
		frozen := option(slices.Values(form.options), frozenOption) == trueValue ||
			option(slices.Values(form.options), unsafeHashOption) == trueValue
		switch {
		case slices.ContainsFunc(s.Methods, func(m *node.Method) bool { return m != nil && m.Name == hashMethod }):
			return
		case slices.ContainsFunc(s.Fields, func(f *node.Field) bool {
			return f != nil && f.Name == hashMethod &&
				(strings.TrimSpace(f.Value) == noneName || f.Type != nil && f.Type.Spelling == noneName)
		}),
			slices.ContainsFunc(s.Methods, func(m *node.Method) bool { return m != nil && m.Name == eqMethod }):
			*problems = append(*problems, ref)
			return
		case generated && frozen, r.namedTuple(s, v):
			set, _ := rules.NewBound(r, v, nil).MembersOf(s)
			for _, m := range set.Members {
				f, isField := m.Symbol.(*node.Field)
				if !isField || f.Level == symbol.LevelType {
					continue
				}
				if initVar, _ := rules.Fact(v, f.ID, meta.Named[bool](python.InitVarKey)); !initVar {
					r.hashable(f.Type, v, depth+1, problems)
				}
			}
			return
		case generated && option(slices.Values(form.options), eqOption) != falseValue:
			*problems = append(*problems, ref)
			return
		}
		base, extends := firstClass(s, v)
		if !extends {
			return
		}
		s = base
	}
}

// Derive returns the witness of a type parameter: its default where it has
// one, its first constraint where it has constraints, because a type
// variable with constraints admits each constraint and no union of them, its
// bound otherwise, which every admitted type satisfies, and object without
// either. It allocates the reference to object, one allocation, and nothing
// for a default, a constraint or a bound.
func (Rules) Derive(p *node.TypeParam, _ rules.View) (*node.TypeRef, bool) {
	switch {
	case p == nil:
		return nil, false
	case p.Default != nil:
		return p.Default, true
	case len(p.Bounds) == 0 || p.Bounds[0] == nil:
		return &node.TypeRef{Spelling: objectWitness}, true
	}
	bound := p.Bounds[0]
	if bound.Form == symbol.FormUnion && strings.HasPrefix(bound.Spelling, constraintsOpen) && len(bound.Elems) > 0 {
		return bound.Elems[0], true
	}
	return bound, true
}

// Reified reports false, because Python erases the type arguments of a
// generic class: an instance of list[int] is a list at run time. Reified
// allocates nothing.
func (Rules) Reified() bool { return false }
