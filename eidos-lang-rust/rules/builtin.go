// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"slices"
	"strings"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The spellings that the rules read: the separator of the segments of a
// path, the separator of a package path, the separator of the name and the
// type of an associated type binding, the unit type, the never type and the
// mark that opens a lifetime.
const (
	pathSep      = "::"
	packageSep   = "/"
	bindingSep   = "="
	unitType     = "()"
	neverType    = "!"
	lifetimeMark = "'"
)

// The crates of the standard library, whose modules re-export one another's
// items, so core::time::Duration is std::time::Duration.
const (
	stdCrate   = "std"
	coreCrate  = "core"
	allocCrate = "alloc"
)

// The names of the types of the standard library that the rules read beside
// the table of [libTypes].
const (
	stringType = "String"
	strType    = "str"
	charType   = "char"
	cowType    = "Cow"
	vecType    = "Vec"
	boxType    = "Box"
	errorTrait = "Error"
	pathType   = "Path"
	osStrType  = "OsStr"
	selfType   = "Self"
	itemName   = "Item"
)

// The auto traits, which a trait object can name beside its principal trait,
// as in dyn Error + Send + Sync, and which the unit type and String implement.
const (
	sendTrait  = "Send"
	syncTrait  = "Sync"
	unpinTrait = "Unpin"
)

// autoTraits are the auto traits that a trait object can name beside its
// principal trait. A bound of an auto trait adds no method, so the rules read
// past it.
var autoTraits = []string{sendTrait, syncTrait, unpinTrait}

// streamCrates are the crates of the asynchronous Stream trait: the futures
// crates, and the crates that re-export it.
var streamCrates = []string{"futures", "futures_core", "futures_util", "tokio_stream", "async_std"}

// collectionsModule is the module of the collections of the standard
// library, whose child modules declare them and whose root re-exports them.
const collectionsModule = "collections"

// The modules below the crates of the standard library that declare or
// re-export a type of the table. Every table entry shares its list, and the
// rules only read it.
var (
	primitiveModules = []string{"primitive"}
	stringModules    = []string{"string"}
	ffiModules       = []string{"ffi", "ffi/os_str", "ffi/c_str"}
	pathModules      = []string{"path"}
	borrowModules    = []string{"borrow"}
	optionModules    = []string{"option"}
	vecModules       = []string{"vec"}
	vecDequeModules  = []string{collectionsModule, collectionsModule + packageSep + "vec_deque"}
	listModules      = []string{collectionsModule, collectionsModule + packageSep + "linked_list"}
	heapModules      = []string{collectionsModule, collectionsModule + packageSep + "binary_heap"}
	hashMapModules   = []string{collectionsModule, collectionsModule + packageSep + "hash_map"}
	btreeMapModules  = []string{collectionsModule, collectionsModule + packageSep + "btree_map"}
	hashSetModules   = []string{collectionsModule, collectionsModule + packageSep + "hash_set"}
	btreeSetModules  = []string{collectionsModule, collectionsModule + packageSep + "btree_set"}
	boxModules       = []string{"boxed"}
	rcModules        = []string{"rc"}
	syncModules      = []string{"sync"}
	cellModules      = []string{"cell"}
	timeModules      = []string{"time"}
	errorModules     = []string{"io", "error", "fmt"}
	iterModules      = []string{"iter"}
)

// The package paths from which a value imports a type of the table. The
// prelude of Rust has the other types of the table in scope.
const (
	ffiHome         = "std/ffi"
	pathHome        = "std/path"
	borrowHome      = "std/borrow"
	collectionsHome = stdCrate + packageSep + collectionsModule
	rcHome          = "std/rc"
	syncHome        = "std/sync"
	cellHome        = "std/cell"
	timeHome        = "std/time"
)

// opaqueElem is the child of an iterator whose reference states no Item, a
// stream of values that the signature does not state. Every shape shares
// the list, and the fold of the kernel only reads it.
var opaqueElem = []rules.TypeShape{{Form: symbol.FormOpaque}}

// libClass is the class of a type of the standard library beside the form
// of its values. A class decides the values of a type whose form does not,
// and the zero class is a type whose form decides its values.
type libClass uint8

const (
	// libUnsized is a text that a value states only behind a pointer, such
	// as str and Path.
	libUnsized libClass = iota + 1
	// libSet is a set, for which the model has no form.
	libSet
	// libNoValue is a type that the rules fold or know and that has no value
	// that a check can write, such as CString and Instant.
	libNoValue
	// libPointer is a pointer that owns its value, Box, Rc and Arc, which
	// builds a value without a size through its from.
	libPointer
)

// libType is one type of the standard library that the rules fold.
type libType struct {
	// wellKnown is the well-known identity of a reference form.
	wellKnown symbol.Identity
	// modules are the modules below std, core and alloc whose path names
	// the type. home is the package path from which a value imports the
	// type, and the empty path for a type that the prelude has in scope.
	modules []string
	home    string
	// bits is the width of a number, 0 for the width of the platform.
	bits int
	// params is the number of type parameters that a reference states.
	params int
	// form is the form of its values, and Opaque for a type without one.
	form symbol.TypeForm
	// class is the class of a number.
	class rules.ScalarClass
	// mutability is the mutability of a list, a map and a borrow.
	mutability symbol.Mutability
	// kind is the class of the type beside its form.
	kind libClass
	// async marks an asynchronous stream.
	async bool
}

// libTypes maps the name of each type of the standard library that the rules
// fold to its entry. The prelude has bool, the numbers, char, str, String,
// Option, Vec, Box and the iterator traits in scope, so their references need
// no path.
var libTypes = map[string]libType{
	"bool":  {form: symbol.FormBool, modules: primitiveModules},
	"i8":    {form: symbol.FormScalar, class: rules.ScalarInt, bits: 8, modules: primitiveModules},
	"i16":   {form: symbol.FormScalar, class: rules.ScalarInt, bits: 16, modules: primitiveModules},
	"i32":   {form: symbol.FormScalar, class: rules.ScalarInt, bits: 32, modules: primitiveModules},
	"i64":   {form: symbol.FormScalar, class: rules.ScalarInt, bits: 64, modules: primitiveModules},
	"i128":  {form: symbol.FormScalar, class: rules.ScalarInt, bits: 128, modules: primitiveModules},
	"isize": {form: symbol.FormScalar, class: rules.ScalarInt, modules: primitiveModules},
	"u8":    {form: symbol.FormScalar, class: rules.ScalarUint, bits: 8, modules: primitiveModules},
	"u16":   {form: symbol.FormScalar, class: rules.ScalarUint, bits: 16, modules: primitiveModules},
	"u32":   {form: symbol.FormScalar, class: rules.ScalarUint, bits: 32, modules: primitiveModules},
	"u64":   {form: symbol.FormScalar, class: rules.ScalarUint, bits: 64, modules: primitiveModules},
	"u128":  {form: symbol.FormScalar, class: rules.ScalarUint, bits: 128, modules: primitiveModules},
	"usize": {form: symbol.FormScalar, class: rules.ScalarUint, modules: primitiveModules},
	"f32":   {form: symbol.FormScalar, class: rules.ScalarFloat, bits: 32, modules: primitiveModules},
	"f64":   {form: symbol.FormScalar, class: rules.ScalarFloat, bits: 64, modules: primitiveModules},

	charType:   {form: symbol.FormText, modules: primitiveModules},
	strType:    {form: symbol.FormText, kind: libUnsized, modules: primitiveModules},
	stringType: {form: symbol.FormText, modules: stringModules},
	"OsString": {form: symbol.FormText, modules: ffiModules, home: ffiHome},
	osStrType:  {form: symbol.FormText, kind: libUnsized, modules: ffiModules, home: ffiHome},
	"PathBuf":  {form: symbol.FormText, modules: pathModules, home: pathHome},
	pathType:   {form: symbol.FormText, kind: libUnsized, modules: pathModules, home: pathHome},
	"CString":  {form: symbol.FormText, kind: libNoValue, modules: ffiModules, home: ffiHome},
	"CStr":     {form: symbol.FormText, kind: libNoValue, modules: ffiModules, home: ffiHome},
	cowType:    {form: symbol.FormText, params: 1, modules: borrowModules, home: borrowHome},

	"Option": {form: symbol.FormOptional, params: 1, modules: optionModules},
	vecType: {
		form: symbol.FormList, mutability: symbol.MutabilityMutable, params: 1, modules: vecModules,
	},
	"VecDeque": {
		form: symbol.FormList, mutability: symbol.MutabilityMutable, params: 1,
		modules: vecDequeModules, home: collectionsHome,
	},
	"LinkedList": {
		form: symbol.FormList, mutability: symbol.MutabilityMutable, params: 1,
		modules: listModules, home: collectionsHome,
	},
	"BinaryHeap": {
		form: symbol.FormList, mutability: symbol.MutabilityMutable, params: 1,
		modules: heapModules, home: collectionsHome,
	},
	"HashMap": {
		form: symbol.FormMap, mutability: symbol.MutabilityMutable, params: 2,
		modules: hashMapModules, home: collectionsHome,
	},
	"BTreeMap": {
		form: symbol.FormMap, mutability: symbol.MutabilityMutable, params: 2,
		modules: btreeMapModules, home: collectionsHome,
	},
	"HashSet": {
		form: symbol.FormOpaque, params: 1, kind: libSet, modules: hashSetModules, home: collectionsHome,
	},
	"BTreeSet": {
		form: symbol.FormOpaque, params: 1, kind: libSet, modules: btreeSetModules, home: collectionsHome,
	},

	boxType: {
		form: symbol.FormBorrow, mutability: symbol.MutabilityMutable, params: 1, kind: libPointer,
		modules: boxModules,
	},
	"Rc": {
		form: symbol.FormBorrow, mutability: symbol.MutabilityImmutable, params: 1, kind: libPointer,
		modules: rcModules, home: rcHome,
	},
	"Arc": {
		form: symbol.FormBorrow, mutability: symbol.MutabilityImmutable, params: 1, kind: libPointer,
		modules: syncModules, home: syncHome,
	},
	"Cell": {
		form: symbol.FormBorrow, mutability: symbol.MutabilityMutable, params: 1, modules: cellModules, home: cellHome,
	},
	"RefCell": {
		form: symbol.FormBorrow, mutability: symbol.MutabilityMutable, params: 1, modules: cellModules, home: cellHome,
	},
	"Mutex": {
		form: symbol.FormBorrow, mutability: symbol.MutabilityMutable, params: 1,
		modules: syncModules, home: syncHome,
	},
	"RwLock": {
		form: symbol.FormBorrow, mutability: symbol.MutabilityMutable, params: 1,
		modules: syncModules, home: syncHome,
	},

	"Duration": {
		form: symbol.FormReference, wellKnown: rules.WellKnownDuration, modules: timeModules, home: timeHome,
	},
	"SystemTime": {
		form: symbol.FormReference, wellKnown: rules.WellKnownTimestamp, modules: timeModules, home: timeHome,
	},
	"Instant":  {form: symbol.FormOpaque, kind: libNoValue, modules: timeModules, home: timeHome},
	errorTrait: {form: symbol.FormReference, wellKnown: rules.WellKnownError, modules: errorModules},

	"Iterator":            {form: symbol.FormStream, modules: iterModules},
	"IntoIterator":        {form: symbol.FormStream, modules: iterModules},
	"DoubleEndedIterator": {form: symbol.FormStream, modules: iterModules},
	"ExactSizeIterator":   {form: symbol.FormStream, modules: iterModules},
	"Stream":              {form: symbol.FormStream, async: true},
}

// Builtin classifies a reference that the resolution step left without a
// target, by the last segment of its spelling and the module of its package.
// A name of the prelude, and a name that a glob import imports, have no
// module. A path of the standard library starts with std, core or alloc,
// which re-export one another's items, and the rest of the path is one of the
// modules that declare the type:
//
//   - bool is a bool, and the integers and the floats are numbers of their
//     widths. isize and usize have the width of the platform;
//   - char, str, String, OsString, OsStr, PathBuf, Path, CString, CStr and
//     Cow<str> are text;
//   - the unit type () is the well-known empty value, and the never type ! is
//     a union without members;
//   - Option<T> is an optional, Vec, VecDeque, LinkedList and BinaryHeap are
//     mutable lists, and HashMap and BTreeMap are mutable maps. The fold of
//     the kernel takes their type arguments as the children, so Vec<u8>
//     folds to bytes;
//   - Box, Cell, RefCell, Mutex and RwLock are a mutable borrow of their
//     argument, and Rc and Arc an immutable one. A Box of a trait object of
//     the error trait, as Box<dyn Error + Send + Sync>, is the well-known
//     error;
//   - Duration and SystemTime of time are the well-known duration and
//     timestamp, and Error of io, error and fmt is the well-known error;
//   - Iterator, IntoIterator, DoubleEndedIterator and ExactSizeIterator are
//     a stream of the type of their binding Item, and Stream of the futures
//     crates is an asynchronous one. An iterator without a binding Item
//     yields an Opaque value.
//
// Every other reference is Opaque: an associated type binding, HashSet and
// BTreeSet, because the model has no form for a set, Instant, which is no
// point of a calendar, a function trait, a type of another crate, and a
// generic type whose reference does not state each of its type arguments.
//
// # Allocation contract
//
// Builtin allocates nothing for a type without an iterator. An iterator with
// a binding Item allocates the list of its child and what the fold of the
// type of the binding allocates in a binding of its own: the memo of the
// binding, its first group and the shape, four allocations for an Item of a
// primitive.
func (r Rules) Builtin(ref *node.TypeRef, v rules.View) rules.TypeShape {
	if ref == nil || ref.Form != symbol.FormNamed || !ref.Target.IsZero() {
		return rules.Opaque(ref)
	}
	switch ref.Spelling {
	case unitType:
		return rules.Reference(ref.Spelling, rules.WellKnownEmpty)
	case neverType:
		return rules.TypeShape{Form: symbol.FormUnion, Spelling: ref.Spelling}
	}
	t, name, known := libTypeOf(ref)
	switch {
	case !known:
		return rules.Opaque(ref)
	case t.form == symbol.FormStream:
		s := rules.TypeShape{Form: symbol.FormStream, Spelling: ref.Spelling, Async: t.async, Elems: opaqueElem}
		if item := binding(ref, itemName); item != nil {
			s.Elems = []rules.TypeShape{rules.NewBound(r, v, nil).TypeOf(item)}
		}
		return s
	case name == cowType:
		if arg := typeArg(ref, 0); arg != nil && arg.Form == symbol.FormNamed && arg.Spelling == strType {
			return rules.Leaf(symbol.FormText, ref.Spelling)
		}
		return rules.Opaque(ref)
	case name == boxType && errorObject(ref):
		return rules.Reference(ref.Spelling, rules.WellKnownError)
	case t.params != len(ref.Args):
		return rules.Opaque(ref)
	}
	switch t.form {
	case symbol.FormScalar:
		return rules.Scalar(ref.Spelling, t.class, t.bits)
	case symbol.FormReference:
		return rules.Reference(ref.Spelling, t.wellKnown)
	case symbol.FormBool, symbol.FormText:
		return rules.Leaf(t.form, ref.Spelling)
	case symbol.FormOptional, symbol.FormList, symbol.FormMap, symbol.FormBorrow:
		return rules.TypeShape{Form: t.form, Spelling: ref.Spelling, Mutability: t.mutability}
	default:
		return rules.Opaque(ref)
	}
}

// libTypeOf returns the entry of [libTypes] that the named reference ref
// without a target names, and its name. The name is the last segment of the
// spelling of ref. The package of ref is empty, or a path of the standard
// library whose module below its crate is one of the modules of the entry.
// Stream takes a package of one of the [streamCrates] in place of a path of
// the standard library. libTypeOf reports false for every other reference.
// It allocates nothing.
func libTypeOf(ref *node.TypeRef) (libType, string, bool) {
	name := ref.Spelling
	if _, last, qualified := strings.CutLast(name, pathSep); qualified {
		name = last
	}
	t, known := libTypes[name]
	if !known {
		return libType{}, "", false
	}
	if ref.Package == "" {
		return t, name, true
	}
	head, module, _ := strings.Cut(ref.Package, packageSep)
	switch {
	case t.async:
		return t, name, slices.Contains(streamCrates, head)
	case head != stdCrate && head != coreCrate && head != allocCrate:
		return libType{}, "", false
	case module == "" || slices.Contains(t.modules, module):
		return t, name, true
	default:
		return libType{}, "", false
	}
}

// binding returns the type that the associated type binding of the name
// binds among the type arguments of ref, such as the T of Item = T, and nil
// where ref has no such binding. The frontend lowers a binding as a named
// reference whose spelling is the name, an equals sign and the type, with
// the type as its one argument. It allocates nothing.
func binding(ref *node.TypeRef, name string) *node.TypeRef {
	for _, a := range ref.Args {
		if a == nil || len(a.Args) != 1 {
			continue
		}
		if bound, _, binds := strings.Cut(a.Spelling, bindingSep); binds && bound == name {
			return a.Args[0]
		}
	}
	return nil
}

// typeArg returns the type argument of ref at index i, counting past the
// lifetimes, and nil where ref has no such argument. It allocates nothing.
func typeArg(ref *node.TypeRef, i int) *node.TypeRef {
	for _, a := range ref.Args {
		switch {
		case a == nil:
		case strings.HasPrefix(a.Spelling, lifetimeMark):
			continue
		case i == 0:
			return a
		}
		i--
	}
	return nil
}

// errorObject reports whether the one type argument of the Box reference ref
// is a trait object of the error trait whose other bounds are auto traits,
// as dyn Error + Send + Sync. It allocates nothing.
func errorObject(ref *node.TypeRef) bool {
	obj := typeArg(ref, 0)
	if obj == nil || obj.Form != symbol.FormIntersection {
		return false
	}
	principal := false
	for _, bound := range obj.Elems {
		name, std := stdName(bound)
		if !std {
			return false
		}
		t, _, known := libTypeOf(bound)
		switch {
		case known && t.wellKnown == rules.WellKnownError && !principal:
			principal = true
		case !slices.Contains(autoTraits, name):
			return false
		}
	}
	return principal
}
