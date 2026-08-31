// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package schema

import (
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// TypeRef is a type as a declaration mentions it.
//
// Spelling holds the source text verbatim. Target holds the
// canonical identity the spelling resolves to, and it stays zero
// until the resolution step runs, and permanently for builtins and
// types outside the workspace. That is legitimate degradation, so a
// consumer asks before relying on a target.
//
// Target is an identity rather than a pointer. A key can be stored,
// compared and carried across runs; a pointer cannot, and following
// one would make the walk cyclic.
//
// Args holds the type arguments of an instantiation, so
// Map[string, User] carries two. A reference carrying Args holds
// the bare name in Spelling, and a target writes the argument
// list in its own brackets, so the one instantiation spells
// Map[K, V] in Go and Map<K, V> in Java.
type TypeRef struct {
	ID       symbol.Identity `eidos:"node"`
	Pos      position.Pos    `eidos:"node"`
	Spelling string          `eidos:"both"` // source text, verbatim
	Target   symbol.Identity `eidos:"both"` // zero until resolution, and for builtins and externals
	Args     []*TypeRef      `eidos:"both,walk"`
}

// TypeParam is one parameter of a generic declaration.
//
// Variance is Invariant for Go and Rust, and carries the declared
// variance for Kotlin, C# and Java wildcards. Bounds holds the
// constraint as type references: a Go constraint interface, a Java
// or Kotlin upper bound, a Rust trait bound. What a bound cannot
// express in references, such as a Go constraint's full type set,
// stays in language metadata.
//
// Default is the type argument used when a caller supplies none,
// which TypeScript writes as "<T = string>". It is nil where the
// language has no such form.
//
// Const marks a parameter whose argument is a value rather than a
// type, which Rust writes as "<const N: usize>". Type then carries
// the value's type and DefaultValue its default spelling; both stay
// empty for an ordinary type parameter.
type TypeParam struct {
	ID           symbol.Identity `eidos:"node"`
	Pos          position.Pos    `eidos:"node"`
	Name         string          `eidos:"both"`
	Variance     symbol.Variance `eidos:"both"`
	Bounds       []*TypeRef      `eidos:"both,walk"`
	Default      *TypeRef        `eidos:"both,walk"` // default type argument; nil when none
	Const        bool            `eidos:"both"`      // the argument is a value, not a type
	Type         *TypeRef        `eidos:"both,walk"` // the value's type, when Const
	DefaultValue string          `eidos:"both"`      // default value spelling, when Const
}

// Constraint is a named, reusable bound: a Go constraint interface
// declared for reuse, a Rust trait bound alias.
//
// Terms holds the projectable members of the type set. A term the
// projection cannot hold, such as an approximation element or a
// union of underlying types, stays in language metadata, and the
// declaration still projects with the terms that survive.
type Constraint struct {
	ID    symbol.Identity `eidos:"node"`
	Pos   position.Pos    `eidos:"node"`
	Terms []*TypeRef      `eidos:"both,walk"` // the projectable terms
}

// Embed is one embedded type in a declaration that promotes
// members: a Go embedded field or embedded interface, a PHP trait
// use.
//
// Embedding differs from nominal supertyping because the members
// arrive promoted rather than inherited, and resolving what a type
// effectively holds across embeds is a language rule rather than a
// model one.
type Embed struct {
	ID   symbol.Identity `eidos:"node"`
	Pos  position.Pos    `eidos:"node"`
	Ref  *TypeRef        `eidos:"both,walk"`
	Host symbol.Identity `eidos:"node"`
}
