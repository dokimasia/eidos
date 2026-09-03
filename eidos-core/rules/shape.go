// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"strconv"

	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/symbol"
)

// ScalarClass says which number a Scalar is.
type ScalarClass uint8

const (
	// ScalarInt is a signed integer.
	ScalarInt ScalarClass = iota + 1
	// ScalarUint is an unsigned integer.
	ScalarUint
	// ScalarFloat is a floating-point number.
	ScalarFloat
)

// String returns the class's spelling.
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

// TypeShape is the canonical shape of a type: the hub every
// cross-language conversion turns on. Form names its structure
// from the one closed enum, the structural forms with their
// children folded and the leaves classified.
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
// hold: representable, not projectable, carrying the spelling.
func Opaque(ref *node.TypeRef) TypeShape {
	s := TypeShape{Form: symbol.FormOpaque}
	if ref != nil {
		s.Spelling = ref.Spelling
	}
	return s
}

// Scalar returns a number shape of a class and width.
func Scalar(spelling string, class ScalarClass, bits int) TypeShape {
	return TypeShape{Form: symbol.FormScalar, Spelling: spelling, Class: class, Bits: bits}
}

// Leaf returns a childless leaf shape: Bool, Text or Bytes.
func Leaf(form symbol.TypeForm, spelling string) TypeShape {
	return TypeShape{Form: form, Spelling: spelling}
}

// Reference returns the shape of a reference to a declaration.
func Reference(spelling string, id symbol.Identity, args ...TypeShape) TypeShape {
	return TypeShape{Form: symbol.FormReference, Spelling: spelling, Ref: id, Args: args}
}

// The well-known types: blessed reference identities a language's
// Builtin maps its own spelling onto, so a Go time.Time and a proto
// Timestamp project to one shape. The registry holds these two and
// no others until a second consumer needs an entry; growing it
// only adds.
var (
	// WellKnownTimestamp is a point in time.
	WellKnownTimestamp = wellKnown("timestamp")
	// WellKnownDuration is a span of time.
	WellKnownDuration = wellKnown("duration")
)

// The well-known registry's own language and package, which no
// frontend declares, so nothing collides with them.
const (
	wellKnownLang    symbol.Lang = "gen"
	wellKnownPackage string      = "wellknown"
)

// wellKnown mints one registry identity.
func wellKnown(name string) symbol.Identity {
	return symbol.Identity{Lang: wellKnownLang, Package: wellKnownPackage, Name: name, Kind: symbol.KindAlias}
}

// IsWellKnown reports whether an identity is one the registry
// blesses.
func IsWellKnown(id symbol.Identity) bool {
	return id == WellKnownTimestamp || id == WellKnownDuration
}

// typeOf is the kernel's fold: a structural form folds into the
// same form with its children folded, a named reference with a
// target the view holds classifies by the declaration it names,
// and a named reference without one goes to the language's
// Builtin. It is total and never panics.
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

// fold computes one reference's shape.
func (b Bound) fold(ref *node.TypeRef) TypeShape {
	if ref.Form.Structural() && ref.Form != symbol.FormNamed {
		s := TypeShape{
			Form: ref.Form, Spelling: ref.Spelling,
			Length: ref.Length, Split: ref.Split, Variance: ref.Variance,
		}
		if len(ref.Elems) > 0 {
			s.Elems = make([]TypeShape, 0, len(ref.Elems))
			for _, child := range ref.Elems {
				s.Elems = append(s.Elems, b.typeOf(child))
			}
		}
		if s.Form == symbol.FormList && len(s.Elems) == 1 && isByte(s.Elems[0]) {
			// The one rule beyond structure: a list of eight-bit
			// unsigned scalars is Bytes, so Go's []byte, Java's
			// byte[] and Rust's Vec<u8> project alike.
			return TypeShape{Form: symbol.FormBytes, Spelling: ref.Spelling}
		}
		return s
	}
	if !ref.Target.IsZero() {
		if decl, held := b.view.Lookup(ref.Target); held {
			return b.classify(ref, decl)
		}
		// A target outside the view's scope is as unreachable as
		// one the graph never held, and the read is recorded.
		return Opaque(ref)
	}
	return b.source.Builtin(ref, b.view)
}

// classify returns the shape of a reference to a held declaration.
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
