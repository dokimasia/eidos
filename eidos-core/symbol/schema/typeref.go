// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// TypeRef is a type as a declaration mentions it.
//
// Spelling is the source text verbatim. Target is the canonical
// identity the spelling resolves to. It is zero until the
// resolution step runs, and permanently for builtins and types
// outside the workspace. That is legitimate degradation, so a
// consumer checks the target before relying on it.
//
// Target is an identity and not a pointer. A key can be stored,
// compared and kept across runs. A pointer cannot, and following
// one would make the walk cyclic.
//
// Package is what the reference's import names. It is empty for a
// builtin, for a structural form, whose named children record their
// own, and for a declaration the file uses without an import. Go
// records an import path, TypeScript a module specifier, and Java
// and Rust a slash-separated package or module path, which a backend
// writes with the language's own separator. Each is the form a
// declaration identity of the language uses for its package, so a
// backend reads a target's package and a reference's alike. A proto
// import names a file and not a package, so protobuf records the
// path of the imported file that declares the type. A backend
// imports a type the workspace never loaded through Package,
// because such a reference has no target.
//
// Args lists the type arguments of an instantiation, so
// Map[string, User] has two. A reference with Args has the bare
// name in Spelling, and a target writes the argument list in its
// own brackets, so the one instantiation spells Map[K, V] in Go and
// Map<K, V> in Java.
//
// Form and Elems record the structure the frontend parsed, each
// form's children in its fixed order:
//
//   - Optional, List, Array, Stream and Borrow: one child.
//   - Map: the key, then the value.
//   - Func: the parameters, then the returns, which begin at Split.
//   - Tuple, Union and Intersection: the members.
//   - Wildcard: the bound, with Variance, and none for an unbounded
//     one.
//   - Inline and Named: none.
//
// The spelling is kept verbatim beside them, so a backend spells what
// it read, and the resolution step visits the named types inside a
// composite through the children. A structural reference has no
// target of its own, and its Named children do. Length is a fixed
// array length written as a literal, and 0 where the length is an
// expression the spelling keeps.
//
// Fields and Methods are an Inline reference's members: the
// properties and methods of a TypeScript object type, and the fields
// of a Go inline struct and the methods of a Go inline interface.
// They are the nodes a declaration's members are, without
// identities, so no carrier and no stamp can address them, and the
// resolution step resolves the references among them. They are the
// node model's alone: a backend spells an inline body from the
// reference's spelling.
type TypeRef struct {
	ID       symbol.Identity `eidos:"node"`
	Pos      position.Pos    `eidos:"node"`
	Spelling string          `eidos:"both"`      // source text, verbatim
	Target   symbol.Identity `eidos:"both"`      // zero until resolution, and for builtins, externals and structural forms
	Package  string          `eidos:"both"`      // what the import names; "" for a builtin, a structural form and a name needing no import
	Form     symbol.TypeForm `eidos:"both"`      // the structure; FormNamed by default
	Elems    []*TypeRef      `eidos:"both,walk"` // the form's children, in the form's fixed order
	Split    int             `eidos:"both"`      // FormFunc: the index in Elems where the returns begin
	Length   int             `eidos:"both"`      // FormArray: the literal length; 0 when the spelling keeps an expression
	Variance symbol.Variance `eidos:"both"`      // FormWildcard: In for a lower bound, Out for an upper one
	Args     []*TypeRef      `eidos:"both,walk"` // the type arguments of an instantiation
	Fields   []*Field        `eidos:"node,walk"` // FormInline: the body's fields, without identities
	Methods  []*Method       `eidos:"node,walk"` // FormInline: the body's methods, without identities
}

// TypeParam is one parameter of a generic declaration.
//
// Variance is Invariant for Go and Rust, and the declared variance
// for Kotlin, C# and Java wildcards. Bounds lists the constraint as
// type references: a Go constraint interface, a Java or Kotlin
// upper bound, a Rust trait bound. What a bound cannot express in
// references, such as a Go constraint's full type set, is language
// metadata.
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
// arrive promoted and not inherited. Which members a type has
// across its embeds is a language rule and not a model one.
//
// An embed is a declaration in its own right: it has the
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
