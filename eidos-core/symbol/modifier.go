// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

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
