// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"strings"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The modules of the classes and the forms that the rules classify, as
// package paths. typing_extensions declares the forms of typing for the
// versions of Python before each form.
const (
	builtinsModule    = "builtins"
	typingModule      = "typing"
	typingExtensions  = "typing_extensions"
	abcModule         = "collections/abc"
	collectionsModule = "collections"
	datetimeModule    = "datetime"
	enumModule        = "enum"
	dataclassesModule = "dataclasses"
)

// The classes of the builtins that the rules classify, and None, which no
// class declares.
const (
	intClass           = "int"
	floatClass         = "float"
	boolClass          = "bool"
	strClass           = "str"
	bytesClass         = "bytes"
	bytearrayClass     = "bytearray"
	objectClass        = "object"
	listClass          = "list"
	dictClass          = "dict"
	frozendictClass    = "frozendict"
	tupleClass         = "tuple"
	exceptionClass     = "Exception"
	baseExceptionClass = "BaseException"
	noneName           = "None"
)

// The forms and the classes of typing, collections.abc and collections that
// the rules classify.
const (
	anyForm           = "Any"
	literalForm       = "Literal"
	literalStringForm = "LiteralString"
	typeGuardForm     = "TypeGuard"
	typeIsForm        = "TypeIs"
	textAlias         = "Text"
	sequenceClass     = "Sequence"
	collectionClass   = "Collection"
	iterableClass     = "Iterable"
	mutableSequence   = "MutableSequence"
	listAlias         = "List"
	dequeAlias        = "Deque"
	dequeClass        = "deque"
	mappingClass      = "Mapping"
	mutableMapping    = "MutableMapping"
	dictAlias         = "Dict"
	defaultDictAlias  = "DefaultDict"
	defaultdictClass  = "defaultdict"
	orderedDictClass  = "OrderedDict"
	chainMapClass     = "ChainMap"
	iteratorClass     = "Iterator"
	generatorClass    = "Generator"
	asyncIterator     = "AsyncIterator"
	asyncIterable     = "AsyncIterable"
	asyncGenerator    = "AsyncGenerator"
	datetimeClass     = "datetime"
	timedeltaClass    = "timedelta"
)

// floatBits is the width of a float of Python, a float of double precision.
const floatBits = 64

// The children of a generic class that a reference states no argument for:
// the top type for each, as a type checker reads a missing argument as Any.
// Every shape shares the lists, and the fold of the kernel only reads them.
var (
	anyChild = []rules.TypeShape{rules.Leaf(symbol.FormDynamic, anyForm)}
	anyPair  = []rules.TypeShape{rules.Leaf(symbol.FormDynamic, anyForm), rules.Leaf(symbol.FormDynamic, anyForm)}
)

// Builtin classifies a name that the resolution step left without a target,
// by its module and its name. The module of a name that an import binds is
// the package of the reference, and the name is the last name of the
// spelling. A name without a dot that nothing binds is in builtins. The
// names classify as [Rules.ShapeOf] classifies the classes of the table of
// typeshed, and as [Rules.WellKnown] blesses datetime, timedelta, Exception
// and BaseException. None is the well-known empty value. A name that an
// import binds under another name, such as Seq after from typing import
// Sequence as Seq, is outside the table, because the reference does not
// keep the name of the import. Every other spelling is Opaque, and so is a
// dotted name whose first name nothing binds.
//
// # Allocation contract
//
// Builtin allocates the list of the child of an optional Literal, and
// nothing otherwise.
func (Rules) Builtin(ref *node.TypeRef, _ rules.View) rules.TypeShape {
	if ref == nil || ref.Form != symbol.FormNamed || !ref.Target.IsZero() {
		return rules.Opaque(ref)
	}
	module, name, named := builtinName(ref)
	if !named {
		return rules.Opaque(ref)
	}
	if module == builtinsModule && name == noneName {
		return rules.Reference(ref.Spelling, rules.WellKnownEmpty)
	}
	if id, blessed := wellKnown(module, name); blessed {
		return rules.Reference(ref.Spelling, id)
	}
	if s, known := classified(module, name, ref); known {
		return s
	}
	return rules.Opaque(ref)
}

// ShapeOf returns the shape of the values of a reference ref to a class or a
// form of the table of typeshed, which the module and the name of decl
// select:
//
//   - int is a signed integer of the platform's width, because an int of
//     Python has no bound, and float is a float of 64 bits;
//   - bool is a bool, str and Text and LiteralString are text, bytes and
//     bytearray are bytes, and object and Any are the top type;
//   - TypeGuard and TypeIs are a bool, the value that the narrowing
//     function returns;
//   - list, MutableSequence, List, Deque and deque are a mutable list, and
//     Sequence, Collection and Iterable are an immutable list;
//   - dict, MutableMapping, Dict, DefaultDict, defaultdict, OrderedDict and
//     ChainMap are a mutable map, and Mapping and frozendict are an
//     immutable map;
//   - Iterator and Generator are a stream, and AsyncIterator, AsyncIterable
//     and AsyncGenerator are an asynchronous stream;
//   - Literal is the shape of its values, as [literalShape] states.
//
// A generic class without type arguments has the top type as each child, as
// a type checker reads list as list[Any]. The fold takes the type arguments
// of every other generic class as its children, and the first type argument
// of a stream, such as the int of Generator[int, None, None]. Every other
// class and form of builtins, typing, typing_extensions, collections.abc,
// collections and datetime is Opaque, because the model has no form for it:
// set, frozenset, complex, type, Callable without an exact list of
// parameters, and date among them. A declaration of every other module
// reports false, so its references fold to references to it.
//
// # Allocation contract
//
// ShapeOf allocates the list of the child of an optional Literal, and
// nothing otherwise.
func (Rules) ShapeOf(ref *node.TypeRef, decl symbol.Symbol, _ rules.View) (rules.TypeShape, bool) {
	d, named := decl.(node.Declaration)
	if !named || ref == nil {
		return rules.TypeShape{}, false
	}
	id := d.Identity()
	if id.Lang != python.Lang || id.Owner != "" {
		return rules.TypeShape{}, false
	}
	return classified(id.Package, id.Name, ref)
}

// WellKnown returns the well-known identity of a class of typeshed:
// datetime of the module datetime is the timestamp, timedelta of the module
// datetime is the duration, and Exception and BaseException of builtins are
// the error. It reports false for every other declaration. It allocates
// nothing.
func (Rules) WellKnown(id symbol.Identity) (symbol.Identity, bool) {
	if id.Lang != python.Lang || id.Owner != "" {
		return symbol.Identity{}, false
	}
	return wellKnown(id.Package, id.Name)
}

// classified returns the shape of the values of a reference ref to the name
// in the module, by the table of [Rules.ShapeOf], and reports false for a
// module outside the table.
func classified(module, name string, ref *node.TypeRef) (rules.TypeShape, bool) {
	spelling := ref.Spelling
	switch module {
	case builtinsModule:
		switch name {
		case intClass:
			return rules.Scalar(spelling, rules.ScalarInt, 0), true
		case floatClass:
			return rules.Scalar(spelling, rules.ScalarFloat, floatBits), true
		case boolClass:
			return rules.Leaf(symbol.FormBool, spelling), true
		case strClass:
			return rules.Leaf(symbol.FormText, spelling), true
		case bytesClass, bytearrayClass:
			return rules.Leaf(symbol.FormBytes, spelling), true
		case objectClass:
			return rules.Leaf(symbol.FormDynamic, spelling), true
		case listClass:
			return generic(symbol.FormList, symbol.MutabilityMutable, ref), true
		case tupleClass:
			return generic(symbol.FormList, symbol.MutabilityImmutable, ref), true
		case dictClass:
			return generic(symbol.FormMap, symbol.MutabilityMutable, ref), true
		case frozendictClass:
			return generic(symbol.FormMap, symbol.MutabilityImmutable, ref), true
		default:
			return rules.Opaque(ref), true
		}
	case typingModule, typingExtensions, abcModule, collectionsModule:
		switch name {
		case anyForm:
			return rules.Leaf(symbol.FormDynamic, spelling), true
		case textAlias, literalStringForm:
			return rules.Leaf(symbol.FormText, spelling), true
		case typeGuardForm, typeIsForm:
			return rules.Leaf(symbol.FormBool, spelling), true
		case literalForm:
			return literalShape(spelling, ref.Args), true
		case sequenceClass, collectionClass, iterableClass:
			return generic(symbol.FormList, symbol.MutabilityImmutable, ref), true
		case mutableSequence, listAlias, dequeAlias, dequeClass:
			return generic(symbol.FormList, symbol.MutabilityMutable, ref), true
		case mappingClass:
			return generic(symbol.FormMap, symbol.MutabilityImmutable, ref), true
		case mutableMapping, dictAlias, defaultDictAlias, defaultdictClass, orderedDictClass, chainMapClass:
			return generic(symbol.FormMap, symbol.MutabilityMutable, ref), true
		case iteratorClass, generatorClass:
			return stream(false, ref), true
		case asyncIterator, asyncIterable, asyncGenerator:
			return stream(true, ref), true
		default:
			return rules.Opaque(ref), true
		}
	case datetimeModule:
		return rules.Opaque(ref), true
	default:
		return rules.TypeShape{}, false
	}
}

// stream returns the shape of a reference ref to a class of streams, an
// asynchronous one where async is set. A stream without a type argument has
// the top type as its child, and the fold takes the first type argument of
// every other stream as its child, the type of the values that it yields.
func stream(async bool, ref *node.TypeRef) rules.TypeShape {
	s := rules.TypeShape{Form: symbol.FormStream, Spelling: ref.Spelling, Async: async}
	if len(ref.Args) == 0 {
		s.Elems = anyChild
	}
	return s
}

// generic returns the shape of a reference ref to a generic class of the
// form, a list or a map, at the mutability. A reference without type
// arguments has the top type as each child, and the fold takes the type
// arguments of every other reference as the children of the form.
func generic(form symbol.TypeForm, mutability symbol.Mutability, ref *node.TypeRef) rules.TypeShape {
	s := rules.TypeShape{Form: form, Spelling: ref.Spelling, Mutability: mutability}
	switch {
	case len(ref.Args) > 0:
	case form == symbol.FormMap:
		s.Elems = anyPair
	default:
		s.Elems = anyChild
	}
	return s
}

// builtinName returns the module and the name of a named reference ref
// without a target, as [Rules.Builtin] reads them: the package of the
// reference and the last name of its spelling, or builtins and the spelling
// for a name without a dot that nothing binds. It reports false for a
// dotted name whose first name nothing binds. It allocates nothing.
func builtinName(ref *node.TypeRef) (module, name string, ok bool) {
	module, name = ref.Package, ref.Spelling
	switch {
	case module == "" && strings.Contains(name, python.ModuleSep):
		return "", name, false
	case module == "":
		return builtinsModule, name, true
	}
	if _, last, dotted := strings.CutLast(name, python.ModuleSep); dotted {
		name = last
	}
	return module, name, true
}

// wellKnown returns the well-known identity of the name in the module, by
// the table of [Rules.WellKnown], and reports false for every other name.
func wellKnown(module, name string) (symbol.Identity, bool) {
	switch {
	case module == datetimeModule && name == datetimeClass:
		return rules.WellKnownTimestamp, true
	case module == datetimeModule && name == timedeltaClass:
		return rules.WellKnownDuration, true
	case module == builtinsModule && (name == exceptionClass || name == baseExceptionClass):
		return rules.WellKnownError, true
	default:
		return symbol.Identity{}, false
	}
}

// literalShape returns the shape of the values of Literal[args], whose
// spelling is spelling. The values of one kind decide the shape: strings
// are text, integers a signed integer of the platform's width, True and
// False a bool, and bytes literals bytes. Members of one enum are a
// reference to the enum, where the resolution step resolved each member.
// None beside other values makes the shape optional, and None alone is the
// well-known empty value. literalShape returns Opaque for a literal without
// values, for values of two kinds or of two enums, and for a value of
// another kind. It allocates the list of the child of an optional, and
// nothing otherwise.
func literalShape(spelling string, args []*node.TypeRef) rules.TypeShape {
	var (
		kind     literalKind
		enum     symbol.Identity
		optional bool
	)
	for _, a := range args {
		if a == nil {
			return rules.TypeShape{Form: symbol.FormOpaque, Spelling: spelling}
		}
		lit, scanned := scanLiteral(a.Spelling)
		member := symbol.Identity{}
		switch {
		case scanned && lit.kind == literalNone:
			optional = true
			continue
		case a.Target.Kind == symbol.KindEnumVariant:
			member = enumOfVariant(a.Target)
			lit.kind = literalName
		case !scanned || lit.kind == literalName || lit.kind == literalFloat:
			return rules.TypeShape{Form: symbol.FormOpaque, Spelling: spelling}
		}
		switch {
		case kind == 0:
			kind, enum = lit.kind, member
		case lit.kind != kind || member != enum:
			return rules.TypeShape{Form: symbol.FormOpaque, Spelling: spelling}
		}
	}
	var s rules.TypeShape
	switch kind {
	case literalString:
		s = rules.Leaf(symbol.FormText, spelling)
	case literalInt:
		s = rules.Scalar(spelling, rules.ScalarInt, 0)
	case literalBool:
		s = rules.Leaf(symbol.FormBool, spelling)
	case literalBytes:
		s = rules.Leaf(symbol.FormBytes, spelling)
	case literalName:
		s = rules.Reference(spelling, enum)
	default:
		if optional {
			return rules.Reference(spelling, rules.WellKnownEmpty)
		}
		return rules.TypeShape{Form: symbol.FormOpaque, Spelling: spelling}
	}
	if optional {
		return rules.TypeShape{Form: symbol.FormOptional, Spelling: spelling, Elems: []rules.TypeShape{s}}
	}
	return s
}

// enumOfVariant returns the identity of the enum that declares the variant
// id: the last name of the owner chain of the variant in the package of the
// variant, under the rest of the chain.
func enumOfVariant(id symbol.Identity) symbol.Identity {
	owner, name, nested := strings.CutLast(id.Owner, symbol.OwnerSep)
	if !nested {
		owner, name = "", id.Owner
	}
	return symbol.Identity{Lang: id.Lang, Package: id.Package, Owner: owner, Name: name, Kind: symbol.KindEnum}
}
