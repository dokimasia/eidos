// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// TypeRef is a type as a declaration mentions it.
//
// A type reference is not a declaration, so it has no identity of its
// own. Its Target is the identity of the declaration it resolves to.
//
// Spelling is the source text verbatim. Target is the canonical
// identity that the resolution step finds for the spelling. It is zero
// until the resolution step runs, and permanently for builtins and
// types outside the workspace. That is legitimate degradation, so a
// consumer checks the target before relying on it.
//
// Target is an identity and not a pointer. A key can be stored,
// compared and kept across runs. A pointer cannot, and following
// one would make the walk cyclic.
//
// Package is the package that the import of the reference names. It
// is empty for a builtin, for a structural form and for a declaration
// that the file uses without an import. The named children of a
// structural form record their own package. Go records an import path,
// TypeScript a module specifier, and Java and Rust a slash-separated
// package or module path. A backend writes such a path with the
// separator of its language. Each is the form that a declaration
// identity of the language uses for its package, so a backend reads
// the package of a target and of a reference alike. A proto import
// names a file and not a package, so protobuf records the path of the
// imported file that declares the type. A backend imports a type that
// the workspace never loaded through Package, because such a reference
// has no target.
//
// Args lists the type arguments of an instantiation, so
// Map[string, User] has two. A reference with Args has the bare name
// in Spelling. A backend writes the argument list in the brackets of
// its language. The Go backend writes Map[K, V], and the Java backend
// writes Map<K, V>.
//
// Form and Elems record the structure that the frontend parsed, with
// the children of each form in its fixed order:
//
//   - Optional, List, Array, Stream and Borrow: one child.
//   - Map: the key, then the value.
//   - Func: the parameters, then the returns, which begin at Split.
//   - Tuple, Union and Intersection: the members.
//   - Wildcard: the bound, with Variance, and none for an unbounded
//     one.
//   - Inline and Named: none.
//
// No element of Elems is nil. A frontend records a child that the source
// does not state as an empty reference, so the form keeps its arity.
//
// The spelling is kept verbatim beside them, so a backend writes the
// spelling that the frontend read. The resolution step visits the
// named types inside a composite through the children. A structural
// reference has no target of its own, and its Named children have
// targets. Length is a fixed array length that the source writes as a
// literal. It is 0 when the length is an expression in the spelling.
//
// Async marks a Stream whose elements arrive asynchronously, such as a
// protobuf stream over a connection. A frontend sets it from the syntax
// of the language, and the fold of the projection copies it into the
// shape of the stream. It is false on every other form.
//
// Mutability states for a List, an Array, a Tuple, a Map and a Borrow
// whether a holder can write through the reference. TypeScript states
// Immutable for readonly T[] and Mutable for T[], and Rust states
// Immutable for &T and Mutable for &mut T. A language that does not
// distinguish the two leaves it unknown, and the fold copies it into
// the shape.
//
// Absent lists the members of the source union that an Optional folds
// away because they denote absence, in source order: TypeScript's null
// and undefined in T | null. It is empty for an Optional that the
// source states with a mark, such as Go's *T or TypeScript's T?. Only
// the node model has it, because a backend writes the spelling.
//
// Fields and Methods are the members of an Inline reference. They are
// the properties and methods of a TypeScript object type, the fields of
// a Go inline struct and the methods of a Go inline interface. They are
// the same nodes as the members of a declaration, without identities.
// No carrier and no stamp can address them. The resolution step
// resolves the references among them. Only the node model has them. A
// backend writes an inline body from the spelling of the reference.
type TypeRef struct {
	Pos        position.Pos      `eidos:"node"`
	Spelling   string            `eidos:"both"`      // source text, verbatim
	Target     symbol.Identity   `eidos:"both"`      // zero until resolution, and for builtins, externals and structural forms
	Package    string            `eidos:"both"`      // what the import names; "" for a builtin, a structural form and a name needing no import
	Form       symbol.TypeForm   `eidos:"both"`      // the structure; FormNamed by default
	Variance   symbol.Variance   `eidos:"both"`      // FormWildcard: In for a lower bound, Out for an upper one
	Async      bool              `eidos:"both"`      // FormStream: the elements arrive asynchronously
	Mutability symbol.Mutability `eidos:"both"`      // List, Array, Tuple, Map and Borrow: whether a holder can write through it
	Elems      []*TypeRef        `eidos:"both,walk"` // the form's children, in the form's fixed order
	Split      int               `eidos:"both"`      // FormFunc: the index in Elems where the returns begin
	Length     int               `eidos:"both"`      // FormArray: the literal length; 0 when the spelling keeps an expression
	Args       []*TypeRef        `eidos:"both,walk"` // the type arguments of an instantiation
	Absent     []*TypeRef        `eidos:"node"`      // FormOptional: the members of the source union that denote absence
	Fields     []*Field          `eidos:"node,walk"` // FormInline: the body's fields, without identities
	Methods    []*Method         `eidos:"node,walk"` // FormInline: the body's methods, without identities
}

// TypeParam is one parameter of a generic declaration.
//
// Variance is Invariant for Go and Rust, and the declared variance
// for Kotlin, C# and Java wildcards. Bounds lists the constraint as
// type references, such as a Go constraint interface, a Java or Kotlin
// upper bound, or a Rust trait bound. Language metadata records what a
// bound cannot express in references, such as the full type set of a
// Go constraint.
//
// Default is the type argument used when a caller supplies none,
// which TypeScript writes as "<T = string>". It is nil where the
// language has no such form.
//
// Const marks a parameter whose argument is a value and not a type,
// which Rust writes as "<const N: usize>". Type is then the value's
// type and DefaultValue its default spelling. Both are empty for an
// ordinary type parameter.
type TypeParam struct {
	ID           symbol.Identity `eidos:"node"`
	Pos          position.Pos    `eidos:"node"`
	Name         string          `eidos:"both,name"`
	Variance     symbol.Variance `eidos:"both,fact=Variance"`
	Const        bool            `eidos:"both,fact=ConstParam"` // the argument is a value, not a type
	Bounds       []*TypeRef      `eidos:"both,walk"`
	Default      *TypeRef        `eidos:"both,walk,fact=TypeParamDefault"` // default type argument; nil when none
	Type         *TypeRef        `eidos:"both,walk"`                       // the value's type, when Const
	DefaultValue string          `eidos:"both"`                            // default value spelling, when Const
}

// Embed is one embedded type in a declaration that promotes members,
// such as a Go embedded field, a Go embedded interface or a PHP trait
// use.
//
// Embedding differs from nominal supertyping because the members are
// promoted and not inherited. The set of members that a type has
// across its embeds is a rule of the language and not of the model.
//
// An embed is a declaration in its own right. It has the
// documentation, trailing comment, tag and annotations of an embedded
// field. Its identity has the bare name of the embedded type, and a
// directive attaches to that identity.
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

// TagEntry is one entry of the struct tag of an emitted field: a key and
// its value. A target joins the entries of a field into its tag syntax,
// such as json:"name,omitempty" in Go. A target without tags refuses a
// field with entries.
type TagEntry struct {
	Key   string `eidos:"emit"`
	Value string `eidos:"emit"`
}
