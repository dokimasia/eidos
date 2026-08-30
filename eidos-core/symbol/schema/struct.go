// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package schema

import (
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// Struct is a type values can be made of: a Go struct, a Java,
// Kotlin, TypeScript or Python class, a Rust struct.
//
// Struct and [Interface] differ by whether a language can
// instantiate them, never by which members they may declare. Both
// carry fields and methods, and one source construct maps to one
// kind whatever its body holds. Abstract is what keeps that line
// intact for a Java or Kotlin abstract class: it holds state and
// constructors like a class, so it is a Struct, and the flag says
// no value of it can be made directly.
//
// Final says the language forbids subclassing. It is the first
// question a doubling generator asks, and it is not a rare case:
// Kotlin classes are final unless declared open.
//
// Three supertype fields stay separate because they answer
// different questions. Embeds is compositional promotion, which Go
// fills. Extends and Implements are nominal, which the JVM
// languages fill; Python fills Extends in method-resolution order.
// A language fills what it has.
//
// Types holds declarations nested inside this one, which Java,
// Kotlin, C#, TypeScript and Python all allow. Without it the walk
// never reaches an inner class and no generator can see one.
//
//eidos:subject
type Struct struct {
	ID         symbol.Identity   `eidos:"node"`
	Origin     symbol.Identity   `eidos:"emit"`
	Pos        position.Pos      `eidos:"node"`
	Doc        []string          `eidos:"both"`
	Name       string            `eidos:"both"`
	Visibility symbol.Visibility `eidos:"both"`
	Abstract   bool              `eidos:"both"` // no value of it can be made directly
	Final      bool              `eidos:"both"` // subclassing is forbidden
	TypeParams []*TypeParam      `eidos:"both,walk"`
	Fields     []*Field          `eidos:"both,walk,slot=fields"`
	Methods    []*Method         `eidos:"both,walk,slot=methods"`
	Types      []Symbol          `eidos:"both,walk,slot=types"` // nested declarations
	Embeds     []*Embed          `eidos:"both,walk"`            // compositional promotion
	Extends    []*TypeRef        `eidos:"both,walk"`            // nominal supertypes
	Implements []*TypeRef        `eidos:"both,walk"`
}

// Interface is a shape values are checked against: a Go or Java
// interface, a TypeScript interface, a Rust trait, a Swift
// protocol.
//
// It carries Fields because interfaces in several languages are
// mostly properties, and a TypeScript interface body usually holds
// them. A plugin asking what contracts exist finds every interface
// by kind, never by filtering another kind on a metadata key.
//
// Interfaces have no Implements: an interface that names another
// interface is widening its own contract, which is Extends. They
// need no Abstract either, since nothing instantiates one.
//
// Types holds nested declarations. An [Alias] in there with a nil
// Target is an associated type, which is how a Rust trait's "type
// Item;" and a Swift associatedtype project: a name the
// implementation supplies, rather than a parameter the caller
// chooses.
//
//eidos:subject
type Interface struct {
	ID         symbol.Identity   `eidos:"node"`
	Origin     symbol.Identity   `eidos:"emit"`
	Pos        position.Pos      `eidos:"node"`
	Doc        []string          `eidos:"both"`
	Name       string            `eidos:"both"`
	Visibility symbol.Visibility `eidos:"both"`
	TypeParams []*TypeParam      `eidos:"both,walk"`
	Fields     []*Field          `eidos:"both,walk,slot=fields"` // properties, not just methods
	Methods    []*Method         `eidos:"both,walk,slot=methods"`
	Types      []Symbol          `eidos:"both,walk,slot=types"` // nested declarations and associated types
	Embeds     []*Embed          `eidos:"both,walk"`
	Extends    []*TypeRef        `eidos:"both,walk"`
}

// Alias is a name for another type: a Go type alias or defined
// type, a TypeScript type alias, a Rust type alias.
//
// Target carries the aliased type as written, and is nil for an
// associated type declared inside an [Interface], where the
// implementation supplies it. An alias may be generic, so a
// language that parameterizes aliases fills TypeParams and the
// target references them.
//
//eidos:subject
type Alias struct {
	ID         symbol.Identity   `eidos:"node"`
	Origin     symbol.Identity   `eidos:"emit"`
	Pos        position.Pos      `eidos:"node"`
	Doc        []string          `eidos:"both"`
	Name       string            `eidos:"both"`
	Visibility symbol.Visibility `eidos:"both"`
	TypeParams []*TypeParam      `eidos:"both,walk"`
	Target     *TypeRef          `eidos:"both,walk"` // nil for an associated type
}
