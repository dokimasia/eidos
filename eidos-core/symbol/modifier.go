// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package symbol

// Visibility is the normalized five-way visibility of a
// declaration.
//
// The zero value is [VisibilityUnknown]: emptiness never claims
// anything, so a frontend that has not returned has not silently
// returned public. The raw source spelling stays in language
// metadata, because normalizing loses which of several spellings a
// language used.
type Visibility uint8

const (
	// VisibilityUnknown means no frontend has returned.
	VisibilityUnknown Visibility = iota
	// VisibilityPublic is visible to every consumer.
	VisibilityPublic
	// VisibilityPackage is visible within the declaring namespace:
	// Go's lowercase, Java's default access.
	VisibilityPackage
	// VisibilityProtected is visible to the declaring type and its
	// subtypes.
	VisibilityProtected
	// VisibilityPrivate is visible to the declaring type only.
	VisibilityPrivate
	// VisibilityInternal is visible within the declaring module:
	// Kotlin and C# internal, Rust pub(crate).
	VisibilityInternal
)

// Level says whether a member belongs to instances of a type or to
// the type itself.
//
// The zero value is [LevelInstance], which every Go member returns
// and which is the common case everywhere.
type Level uint8

const (
	// LevelInstance belongs to each value of the type.
	LevelInstance Level = iota
	// LevelType belongs to the type: JVM statics, Kotlin
	// companions, TypeScript static members.
	LevelType
)

// Accessor says whether a callable is a property accessor rather
// than an ordinary method.
//
// The zero value is [AccessorNone], which every ordinary method
// carries. TypeScript, C# and Kotlin state accessors in syntax;
// languages spelling properties as conventions never set it.
type Accessor uint8

const (
	// AccessorNone is an ordinary method.
	AccessorNone Accessor = iota
	// AccessorGet reads the property: no parameters, one result.
	AccessorGet
	// AccessorSet writes the property: one parameter, no result.
	AccessorSet
)

// Variance is the variance of a type parameter.
//
// The zero value is [VarianceInvariant], which Go and Rust always
// carry. Kotlin, C# and Java wildcards carry the other two.
type Variance uint8

const (
	// VarianceInvariant admits the parameter's type only.
	VarianceInvariant Variance = iota
	// VarianceIn is contravariant: Kotlin in, C# in, Java super.
	VarianceIn
	// VarianceOut is covariant: Kotlin out, C# out, Java extends.
	VarianceOut
)

// Mutability says whether a binding may be reassigned after
// initialization.
//
// The zero value is [MutabilityUnknown], because a language that
// does not distinguish the two has not returned immutable. Kotlin
// val against var, TypeScript readonly and Java final are the
// distinction; a Kotlin const val is a Constant instead, since it
// is fixed at compile time.
type Mutability uint8

const (
	// MutabilityUnknown means the language draws no distinction, or
	// no frontend has returned.
	MutabilityUnknown Mutability = iota
	// MutabilityMutable may be reassigned.
	MutabilityMutable
	// MutabilityImmutable is fixed after initialization.
	MutabilityImmutable
)

// TypeForm names the structure of a type reference and of a
// projected shape, from one closed set.
//
// The structural forms are a frontend's to set from syntax, beside
// the reference's verbatim spelling, with the children in a fixed
// order per form. The leaf forms belong to the projection's fold
// alone: a frontend never sets one, and the fold never returns a
// structural form unfolded. The zero value is [FormNamed], a name
// resolved or not, which is what a frontend that decomposes
// nothing leaves on every reference.
type TypeForm uint8

const (
	// FormNamed is a name, resolved or not: the default.
	FormNamed TypeForm = iota
	// FormOptional has one child: Go *T, Kotlin T?, TypeScript
	// T | undefined.
	FormOptional
	// FormList has one child: a slice, an array of open length, a
	// repeated field.
	FormList
	// FormArray has one child and a fixed length the reference
	// records.
	FormArray
	// FormMap has two children, the key then the value.
	FormMap
	// FormFunc has the parameters then the returns as children,
	// the returns from the index the reference records.
	FormFunc
	// FormTuple has its members as children, in order.
	FormTuple
	// FormUnion has its members as children, in order, untagged.
	FormUnion
	// FormStream has one child: a channel, an async iterator.
	FormStream
	// FormBorrow has one child: a Rust reference, a C++ reference.
	FormBorrow
	// FormWildcard has one child, the bound, and the reference
	// records the variance.
	FormWildcard
	// FormInline has no children: an inline struct, interface or
	// object body.
	FormInline

	// FormScalar is a number; the shape carries its class and
	// width.
	FormScalar
	// FormBool is a truth value.
	FormBool
	// FormText is a string.
	FormText
	// FormBytes is a byte sequence.
	FormBytes
	// FormReference names a declaration the graph holds.
	FormReference
	// FormSum names a Sum declaration.
	FormSum
	// FormOpaque is representable and not projectable; the shape
	// carries the spelling.
	FormOpaque
)

// Structural reports whether a form is one a frontend sets from
// syntax, as opposed to a leaf the fold returns.
func (f TypeForm) Structural() bool { return f <= FormInline }

// Variadic says how a parameter accepts a variable number of
// arguments.
//
// The zero value is [VariadicNone]. Positional and keyword forms
// stay separate because Python, Ruby and PHP have both, and a
// generator spelling a delegating call has to reproduce the right
// one.
type Variadic uint8

const (
	// VariadicNone takes exactly one argument.
	VariadicNone Variadic = iota
	// VariadicPositional collects the remaining positional
	// arguments: Go and Java varargs, TypeScript rest parameters,
	// Python *args.
	VariadicPositional
	// VariadicKeyword collects the remaining keyword arguments:
	// Python **kwargs, Ruby double splat, PHP named spread.
	VariadicKeyword
)
