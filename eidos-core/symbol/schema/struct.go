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
// never reaches an inner class and no generator can see one. Level
// says whether a nested type binds to an enclosing instance, which
// a Java inner class and a Kotlin inner class do, or stands at
// type level, which Java writes as static; a file-level type
// leaves it unstated.
//
// Sealed says the direct subtypes are enumerated, which Java and
// Kotlin declare; Permits carries the enumeration where the
// language states one beyond the file, as Java's permits clause
// does.
//
//eidos:subject
type Struct struct {
	ID          symbol.Identity   `eidos:"node"`
	Origin      symbol.Identity   `eidos:"emit"`
	Pos         position.Pos      `eidos:"node"`
	Doc         []string          `eidos:"both"`
	Name        string            `eidos:"both,name"`
	Visibility  symbol.Visibility `eidos:"both,fact=Visibility"`
	Level       symbol.Level      `eidos:"both,fact=Level"`    // a nested type's binding
	Abstract    bool              `eidos:"both,fact=Abstract"` // no value of it can be made directly
	Final       bool              `eidos:"both,fact=Final"`    // subclassing is forbidden
	Sealed      bool              `eidos:"both,fact=Sealed"`   // the direct subtypes are enumerated
	TypeParams  []*TypeParam      `eidos:"both,walk,fact=TypeParams"`
	Fields      []*Field          `eidos:"both,walk,slot=fields"`
	Methods     []*Method         `eidos:"both,walk,slot=methods"`
	Types       []Symbol          `eidos:"both,walk,slot=types,fact=Types"` // nested declarations
	Embeds      []*Embed          `eidos:"both,walk,fact=Embeds"`           // compositional promotion
	Extends     []*TypeRef        `eidos:"both,walk,fact=Extends"`          // nominal supertypes
	Implements  []*TypeRef        `eidos:"both,walk,fact=Implements"`
	Permits     []*TypeRef        `eidos:"both,walk,fact=Permits"` // the enumerated subtypes
	Annotations Annotations       `eidos:"emit,fact=Annotations"`
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
// Sealed and Permits carry what [Struct]'s carry: the direct
// subtypes are enumerated, and the enumeration where the language
// states one.
//
//eidos:subject
type Interface struct {
	ID          symbol.Identity   `eidos:"node"`
	Origin      symbol.Identity   `eidos:"emit"`
	Pos         position.Pos      `eidos:"node"`
	Doc         []string          `eidos:"both"`
	Name        string            `eidos:"both,name"`
	Visibility  symbol.Visibility `eidos:"both,fact=Visibility"`
	Sealed      bool              `eidos:"both,fact=Sealed"` // the direct subtypes are enumerated
	TypeParams  []*TypeParam      `eidos:"both,walk,fact=TypeParams"`
	Fields      []*Field          `eidos:"both,walk,slot=fields,fact=Properties"` // properties, not just methods
	Methods     []*Method         `eidos:"both,walk,slot=methods"`
	Types       []Symbol          `eidos:"both,walk,slot=types,fact=Types"` // nested declarations and associated types
	Embeds      []*Embed          `eidos:"both,walk,fact=Embeds"`
	Extends     []*TypeRef        `eidos:"both,walk,fact=Extends"`
	Permits     []*TypeRef        `eidos:"both,walk,fact=Permits"` // the enumerated subtypes
	Annotations Annotations       `eidos:"emit,fact=Annotations"`
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
// Defined marks a distinct type rather than a transparent alias:
// a Go defined type, which attaches methods and converts
// explicitly, against the alias form that is only another
// spelling. A language without the distinction leaves it false,
// and one whose aliases are transparent alone refuses it.
//
//eidos:subject
type Alias struct {
	ID          symbol.Identity   `eidos:"node"`
	Origin      symbol.Identity   `eidos:"emit"`
	Pos         position.Pos      `eidos:"node"`
	Doc         []string          `eidos:"both"`
	Name        string            `eidos:"both,name"`
	Visibility  symbol.Visibility `eidos:"both,fact=Visibility"`
	Defined     bool              `eidos:"both,fact=Defined"` // a distinct type, not a transparent alias
	TypeParams  []*TypeParam      `eidos:"both,walk,fact=TypeParams"`
	Target      *TypeRef          `eidos:"both,walk"` // nil for an associated type
	Annotations Annotations       `eidos:"emit,fact=Annotations"`
}
