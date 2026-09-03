// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

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
//
// Form and Elems carry the structure the frontend parsed, in the
// form's fixed child order: one child for Optional, List, Array,
// Stream and Borrow; the key then the value for Map; the
// parameters then the returns for Func, the returns from Split;
// the members for Tuple and Union; the bound for Wildcard, with
// Variance; none for Inline and Named. The spelling stays verbatim
// beside them, so a backend spells what it read, and the
// resolution step reaches the named types inside a composite
// through the children. A structural reference carries no target
// of its own; its Named children do. Length holds a fixed array
// length written as a literal, and 0 where the length is an
// expression the spelling keeps.
type TypeRef struct {
	ID       symbol.Identity `eidos:"node"`
	Pos      position.Pos    `eidos:"node"`
	Spelling string          `eidos:"both"`      // source text, verbatim
	Target   symbol.Identity `eidos:"both"`      // zero until resolution, and for builtins, externals and structural forms
	Form     symbol.TypeForm `eidos:"both"`      // the structure; FormNamed by default
	Elems    []*TypeRef      `eidos:"both,walk"` // the form's children, in the form's fixed order
	Split    int             `eidos:"both"`      // FormFunc: the index in Elems where the returns begin
	Length   int             `eidos:"both"`      // FormArray: the literal length; 0 when the spelling keeps an expression
	Variance symbol.Variance `eidos:"both"`      // FormWildcard: In for a lower bound, Out for an upper one
	Args     []*TypeRef      `eidos:"both,walk"` // the type arguments of an instantiation
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
	Name         string          `eidos:"both,name"`
	Variance     symbol.Variance `eidos:"both,fact=Variance"`
	Bounds       []*TypeRef      `eidos:"both,walk"`
	Default      *TypeRef        `eidos:"both,walk,fact=TypeParamDefault"` // default type argument; nil when none
	Const        bool            `eidos:"both,fact=ConstParam"`            // the argument is a value, not a type
	Type         *TypeRef        `eidos:"both,walk"`                       // the value's type, when Const
	DefaultValue string          `eidos:"both"`                            // default value spelling, when Const
}

// Embed is one embedded type in a declaration that promotes
// members: a Go embedded field or embedded interface, a PHP trait
// use.
//
// Embedding differs from nominal supertyping because the members
// arrive promoted rather than inherited, and resolving what a type
// effectively holds across embeds is a language rule rather than a
// model one.
//
// An embed is a declaration in its own right: it carries the
// documentation, trailing comment, tag and annotations an embedded
// field takes like any field, and its identity, named by the
// embedded type's bare name, is what a directive attaches to.
type Embed struct {
	ID          symbol.Identity    `eidos:"node"`
	Pos         position.Pos       `eidos:"node"`
	Doc         []string           `eidos:"both"`
	Comment     string             `eidos:"both,fact=Comment"` // trailing line comment; "" when none
	Ref         *TypeRef           `eidos:"both,walk"`
	Tag         string             `eidos:"both,fact=Tag"` // tag text without delimiters, "" when none
	Annotations symbol.Annotations `eidos:"both,fact=Annotations"`
	Host        symbol.Identity    `eidos:"node"`
}
