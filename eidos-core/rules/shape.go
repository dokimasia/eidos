// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"strconv"

	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// ScalarClass names which number a Scalar is.
type ScalarClass uint8

const (
	// ScalarInt is a signed integer.
	ScalarInt ScalarClass = iota + 1
	// ScalarUint is an unsigned integer.
	ScalarUint
	// ScalarFloat is a floating-point number.
	ScalarFloat
)

// String returns the class's spelling, and the decimal number of an
// undeclared class. It allocates nothing for a declared class.
func (c ScalarClass) String() string {
	switch c {
	case ScalarInt:
		return "int"
	case ScalarUint:
		return "uint"
	case ScalarFloat:
		return "float"
	default:
		return strconv.Itoa(int(c))
	}
}

// TypeShape is the canonical shape of a type, which every conversion
// between languages reads. Form names its structure from the one closed
// enum: a structural form contains its children folded, and a leaf form
// is classified.
//
// # Allocation contract
//
// A TypeShape is a value of 200 bytes. The constructors return it by
// value, and allocate only the list of arguments a reference keeps.
type TypeShape struct {
	Form     symbol.TypeForm
	Spelling string          // the source spelling, on every form
	Class    ScalarClass     // Scalar
	Bits     int             // Scalar: 0 for the platform width
	Length   int             // Array
	Split    int             // Func: the index in Elems where the returns begin
	Variance symbol.Variance // Wildcard
	Async    bool            // Stream
	Elems    []TypeShape     // the form's children, in the form's order
	Ref      symbol.Identity // Reference and Sum
	Args     []TypeShape     // Reference type arguments
}

// Opaque returns the shape of a reference the projection cannot
// classify: [symbol.FormOpaque] with the reference's spelling, and
// without a spelling for a nil reference. A backend spells the type and
// derives nothing from it. Opaque allocates nothing.
func Opaque(ref *node.TypeRef) TypeShape {
	s := TypeShape{Form: symbol.FormOpaque}
	if ref != nil {
		s.Spelling = ref.Spelling
	}
	return s
}

// Scalar returns a number shape of a class and a width in bits, 0 for
// the platform width. It allocates nothing.
func Scalar(spelling string, class ScalarClass, bits int) TypeShape {
	return TypeShape{Form: symbol.FormScalar, Spelling: spelling, Class: class, Bits: bits}
}

// Leaf returns a childless leaf shape: Bool, Text or Bytes. It
// allocates nothing.
func Leaf(form symbol.TypeForm, spelling string) TypeShape {
	return TypeShape{Form: form, Spelling: spelling}
}

// Reference returns the shape of a reference to a declaration, with its
// type arguments in order. The shape keeps args as its Args, so a call
// with arguments allocates their list at the call site, one allocation.
// A call without arguments allocates nothing.
func Reference(spelling string, id symbol.Identity, args ...TypeShape) TypeShape {
	return TypeShape{Form: symbol.FormReference, Spelling: spelling, Ref: id, Args: args}
}

// A language's Builtin maps its own spelling of a well-known type onto
// one of these blessed reference identities, so a Go time.Time and a
// proto Timestamp project to one shape. The registry contains these
// two, and growing it only adds entries.
var (
	// WellKnownTimestamp is a point in time.
	WellKnownTimestamp = wellKnown("timestamp")
	// WellKnownDuration is a span of time.
	WellKnownDuration = wellKnown("duration")
)

// No frontend declares the registry's own language and package, so no
// declaration's identity collides with a well-known one.
const (
	wellKnownLang    symbol.Lang = "gen"
	wellKnownPackage string      = "wellknown"
)

// wellKnown returns the registry's identity of a name.
func wellKnown(name string) symbol.Identity {
	return symbol.Identity{Lang: wellKnownLang, Package: wellKnownPackage, Name: name, Kind: symbol.KindAlias}
}

// IsWellKnown reports whether an identity is one the registry
// blesses. It allocates nothing.
func IsWellKnown(id symbol.Identity) bool {
	return id == WellKnownTimestamp || id == WellKnownDuration
}

// typeOf is the kernel's fold: a structural form folds into the
// same form with its children folded, a named reference to a type
// parameter folds to Opaque, a named reference with a target the
// view contains classifies by the declaration it names, and a named
// reference without a target goes to the language's Builtin. Where the
// Builtin returns a structural form without children for a reference
// with type arguments, the folded arguments become the form's children.
// It is total and never panics. A reference in the memo returns its
// shape and allocates nothing. A first fold allocates what the
// allocation contract of [Bound.TypeOf] lists.
func (b Bound) typeOf(ref *node.TypeRef) TypeShape {
	if ref == nil {
		return Opaque(nil)
	}
	if b.memo != nil {
		if s, held := b.memo[ref]; held {
			return s
		}
	}
	s := b.fold(ref)
	if b.memo != nil {
		b.memo[ref] = s
	}
	return s
}

// fold computes one reference's shape. A structural shape allocates the
// list of its children, and Bytes allocates none.
func (b Bound) fold(ref *node.TypeRef) TypeShape {
	if ref.Form.Structural() && ref.Form != symbol.FormNamed {
		return b.children(TypeShape{
			Form: ref.Form, Spelling: ref.Spelling,
			Length: ref.Length, Split: ref.Split, Variance: ref.Variance,
		}, ref.Elems)
	}
	if ref.Target.Kind == symbol.KindTypeParam {
		// A type parameter's shape is its argument's, which a use of
		// the parameter does not state, so no read would decide it.
		return Opaque(ref)
	}
	if !ref.Target.IsZero() {
		if decl, held := b.view.Lookup(ref.Target); held {
			return b.classify(ref, decl)
		}
		// A target outside the view's scope is as unreachable as
		// one the graph never contained, and the read is recorded.
		return Opaque(ref)
	}
	s := b.source.Builtin(ref, b.view)
	if len(ref.Args) == 0 || len(s.Elems) > 0 || !s.Form.Structural() {
		return s
	}
	if !takes(s.Form, len(ref.Args)) {
		return Opaque(ref)
	}
	return b.children(s, ref.Args)
}

// children returns s with each of refs folded as its children, in order.
// A list whose one child folds to the eight-bit unsigned scalar folds to
// Bytes, the one rule beyond structure, so Go's []byte, Java's byte[] and
// Rust's Vec<u8> project alike. A shape without refs has no list of
// children.
func (b Bound) children(s TypeShape, refs []*node.TypeRef) TypeShape {
	if s.Form == symbol.FormList && len(refs) == 1 && isByte(b.typeOf(refs[0])) {
		return TypeShape{Form: symbol.FormBytes, Spelling: s.Spelling}
	}
	if len(refs) > 0 {
		s.Elems = make([]TypeShape, 0, len(refs))
		for _, child := range refs {
			s.Elems = append(s.Elems, b.typeOf(child))
		}
	}
	return s
}

// classify returns the shape of a reference to a declaration the
// view contains.
func (b Bound) classify(ref *node.TypeRef, decl symbol.Symbol) TypeShape {
	form := symbol.FormReference
	if decl.Kind() == symbol.KindSum {
		form = symbol.FormSum
	}
	s := TypeShape{Form: form, Spelling: ref.Spelling, Ref: ref.Target}
	if len(ref.Args) > 0 {
		s.Args = make([]TypeShape, 0, len(ref.Args))
		for _, arg := range ref.Args {
			s.Args = append(s.Args, b.typeOf(arg))
		}
	}
	return s
}

// isByte reports whether a shape is the eight-bit unsigned scalar.
func isByte(s TypeShape) bool {
	return s.Form == symbol.FormScalar && s.Class == ScalarUint && s.Bits == 8
}

// takes reports whether a structural form takes n type arguments as its
// children. An optional, a list, an array, a stream and a borrow take
// one, and a map takes two. A tuple, a union and an intersection take
// every argument as a member. A function, a wildcard, an inline form and
// a named form take none.
func takes(f symbol.TypeForm, n int) bool {
	switch f {
	case symbol.FormOptional, symbol.FormList, symbol.FormArray, symbol.FormStream, symbol.FormBorrow:
		return n == 1
	case symbol.FormMap:
		return n == 2
	case symbol.FormTuple, symbol.FormUnion, symbol.FormIntersection:
		return true
	default:
		return false
	}
}
