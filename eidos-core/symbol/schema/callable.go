// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// Function is a callable declared outside any type: a Go or Rust
// free function, a Python module-level def, a TypeScript exported
// function.
//
// Async marks a result that arrives asynchronously in the
// language's own form: a TypeScript async function, a Rust async
// fn, a Python async def. Go leaves it false, because its
// concurrency is caller-side and invisible in signatures.
//
// Throws lists the failure types the declaration announces: Java
// checked exceptions, Swift typed throws. A language without
// declared throws leaves it empty, and the error model remains the
// projection's neutral question.
//
// The node model records the signature and never a body, because
// parsed bodies are out of scope entirely. The emit model records a
// generated body in its Body field: the standard slots, and one
// content form.
//
//eidos:subject
type Function struct {
	ID          symbol.Identity    `eidos:"node"`
	Origin      symbol.Identity    `eidos:"emit"`
	Pos         position.Pos       `eidos:"node"`
	Doc         []string           `eidos:"both"`
	Comment     string             `eidos:"both,fact=Comment"` // trailing line comment; "" when none
	Name        string             `eidos:"both,name"`
	Visibility  symbol.Visibility  `eidos:"both,fact=Visibility"`
	Async       bool               `eidos:"both,fact=Async"`
	TypeParams  []*TypeParam       `eidos:"both,walk,fact=TypeParams"`
	Params      []*Param           `eidos:"both,walk"`
	Returns     []*Return          `eidos:"both,walk,fact=MultiReturn"`
	Throws      []*TypeRef         `eidos:"both,walk,fact=Throws"`
	Annotations symbol.Annotations `eidos:"both,fact=Annotations"`
	Body        Body               `eidos:"emit"`
}

// Method is a callable attached to a type.
//
// Receiver is the explicit receiver where the language writes one,
// as Go and Rust do, and nil where the receiver is implicit. Level
// states whether the method belongs to instances or to the type
// itself, which covers JVM statics and Kotlin companions.
//
// Receives names the type a method attaches to when it is declared
// outside that type's own declaration: a Kotlin extension function,
// a Swift extension member, a C# extension method, a Rust impl for
// a type from another crate. Host remains the declaration that
// contains the method, so each field records a different fact and
// both are true. A method declared inside its type leaves Receives
// nil.
//
// Abstract marks a member with no body that a subtype must supply.
// Final forbids overriding. Override marks a member that replaces a
// supertype's, which Kotlin, C#, Swift and TypeScript spell as a
// keyword the emitted code must write. Java spells it as an
// annotation instead, so a Java frontend leaves the field false.
//
// HasDefault marks an interface method with a body: a Java default
// method, a Kotlin interface method, a Rust default impl. It differs
// from Abstract's inverse, because a class method with a body is an
// ordinary method, not a default.
//
// Async and Throws record what [Function]'s record: the
// asynchronous result in the language's own form, and the failure
// types the declaration announces.
//
// The node model records the signature and never a body, and the
// emit model records a generated body in its Body field.
//
//eidos:subject
type Method struct {
	ID          symbol.Identity    `eidos:"node"`
	Origin      symbol.Identity    `eidos:"emit"`
	Pos         position.Pos       `eidos:"node"`
	Doc         []string           `eidos:"both"`
	Comment     string             `eidos:"both,fact=Comment"` // trailing line comment; "" when none
	Name        string             `eidos:"both,name"`
	Visibility  symbol.Visibility  `eidos:"both,fact=Visibility"`
	Level       symbol.Level       `eidos:"both,fact=Level"`
	Abstract    bool               `eidos:"both,fact=Abstract"`    // no body; a subtype must supply one
	Final       bool               `eidos:"both,fact=Final"`       // overriding is forbidden
	Override    bool               `eidos:"both,fact=Override"`    // replaces a supertype's member
	HasDefault  bool               `eidos:"both,fact=DefaultBody"` // an interface method with a body
	Async       bool               `eidos:"both,fact=Async"`
	Accessor    symbol.Accessor    `eidos:"both,fact=Accessor"`    // a get or set property accessor
	Indexer     bool               `eidos:"both,fact=Indexer"`     // an index signature: one key parameter, one result
	Constructs  bool               `eidos:"both,fact=Constructs"`  // a construct signature on an interface
	Hard        bool               `eidos:"both,fact=HardPrivate"` // runtime-private: TypeScript's # names
	Receiver    *Param             `eidos:"both,walk"`             // nil where the receiver is implicit
	Receives    *TypeRef           `eidos:"both,walk"`             // set when declared outside the type it attaches to
	TypeParams  []*TypeParam       `eidos:"both,walk,fact=TypeParams"`
	Params      []*Param           `eidos:"both,walk"`
	Returns     []*Return          `eidos:"both,walk,fact=MultiReturn"`
	Throws      []*TypeRef         `eidos:"both,walk,fact=Throws"`
	Annotations symbol.Annotations `eidos:"both,fact=Annotations"`
	Body        Body               `eidos:"emit"`
	Host        symbol.Identity    `eidos:"node"`
}

// Param is one parameter of a callable, or its receiver.
//
// Name is empty where the language allows an unnamed parameter, as
// Go and Java interfaces do. Label is the caller-facing name where
// a language gives a parameter two, which Swift and Objective-C do:
// in "func greet(person name: String)" the label is "person" and
// the name is "name".
//
// Default is the source spelling of the default value, unevaluated,
// and is empty when the parameter has none. A generator that drops a
// default changes the callee's contract, so the spelling is a field
// of the parameter and not metadata.
//
// Variadic distinguishes the positional and keyword forms, because
// Python, Ruby and PHP have both. It is legal on the trailing
// parameters only, and the frontends enforce that, not the model. A
// variadic parameter's Type is the type of one argument it takes:
// int for Go's ...int and Java's int..., and number for
// TypeScript's ...xs: number[].
//
// A parameter is a subject: an authored value attaches to it in a
// language whose comments can address it, and its identity is the
// host's chain, its name or its position, and the host's
// discriminator.
//
//eidos:subject
type Param struct {
	ID          symbol.Identity    `eidos:"node"`
	Pos         position.Pos       `eidos:"node"`
	Comment     string             `eidos:"both,fact=Comment"` // trailing line comment; "" when none
	Name        string             `eidos:"both,name"`         // "" when unnamed
	Label       string             `eidos:"both,fact=Label"`   // caller-facing name; Swift and Objective-C
	Type        *TypeRef           `eidos:"both,walk"`
	Default     string             `eidos:"both,fact=ParamDefault"` // source spelling, unevaluated; "" when none
	Optional    bool               `eidos:"both,fact=Optional"`     // present-or-absent: TypeScript's ?, Swift's defaulted trailing
	Variadic    symbol.Variadic    `eidos:"both,fact=Variadic"`     // positional or keyword
	Annotations symbol.Annotations `eidos:"both,fact=Annotations"`
}

// Return is one result of a callable.
//
// The list is a slice because Go returns several values. A language
// with one result fills one entry, and a language with none fills
// none. Name is a Go named result's name and is empty elsewhere.
//
// A return is a subject the way a parameter is, named by its
// position where the language leaves it unnamed.
//
//eidos:subject
type Return struct {
	ID      symbol.Identity `eidos:"node"`
	Pos     position.Pos    `eidos:"node"`
	Comment string          `eidos:"both,fact=Comment"`          // trailing line comment; "" when none
	Name    string          `eidos:"both,name,fact=NamedReturn"` // Go named results; "" elsewhere
	Type    *TypeRef        `eidos:"both,walk"`
}
