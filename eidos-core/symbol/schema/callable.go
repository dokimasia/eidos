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
// declared throws leaves it empty, and the error model stays the
// projection's neutral question.
//
// The node model carries the signature and never a body, because
// parsed bodies are out of scope entirely. The emit model carries
// what a generated body holds in its Body field: the standard
// slots, and one content form.
//
//eidos:subject
type Function struct {
	ID          symbol.Identity   `eidos:"node"`
	Origin      symbol.Identity   `eidos:"emit"`
	Pos         position.Pos      `eidos:"node"`
	Doc         []string          `eidos:"both"`
	Name        string            `eidos:"both,name"`
	Visibility  symbol.Visibility `eidos:"both,fact=Visibility"`
	Async       bool              `eidos:"both,fact=Async"`
	TypeParams  []*TypeParam      `eidos:"both,walk,fact=TypeParams"`
	Params      []*Param          `eidos:"both,walk"`
	Returns     []*Return         `eidos:"both,walk,fact=MultiReturn"`
	Throws      []*TypeRef        `eidos:"both,walk,fact=Throws"`
	Annotations Annotations       `eidos:"emit,fact=Annotations"`
	Body        Body              `eidos:"emit"`
}

// Method is a callable attached to a type.
//
// Receiver holds the explicit receiver where the language writes
// one, as Go and Rust do, and stays nil where the receiver is
// implicit. Level says whether the method belongs to instances or
// to the type itself, which covers JVM statics and Kotlin
// companions.
//
// Receives carries the type a method attaches to when it is
// declared outside that type's own declaration: a Kotlin extension
// function, a Swift extension member, a C# extension method, a Rust
// impl for a type from another crate. Host stays the declaration
// that contains the method, so the two answer different questions
// and neither has to lie. A method declared inside its type leaves
// Receives nil.
//
// Abstract marks a member with no body that a subtype must supply.
// Final forbids overriding. Override marks a member that replaces a
// supertype's, which Kotlin, C#, Swift and TypeScript spell as a
// keyword the emitted code has to carry; Java spells it as an
// annotation instead, so a Java frontend leaves the field false.
//
// HasDefault marks an interface method that carries a body: a Java
// default method, a Kotlin interface method, a Rust default impl.
// It differs from Abstract's inverse, because a class method with a
// body is ordinary rather than a default.
//
// Async and Throws carry what [Function]'s carry: the
// asynchronous result in the language's own form, and the failure
// types the declaration announces.
//
// The node model carries the signature and never a body; the emit
// model carries what a generated body holds in its Body field.
//
//eidos:subject
type Method struct {
	ID          symbol.Identity   `eidos:"node"`
	Origin      symbol.Identity   `eidos:"emit"`
	Pos         position.Pos      `eidos:"node"`
	Doc         []string          `eidos:"both"`
	Name        string            `eidos:"both,name"`
	Visibility  symbol.Visibility `eidos:"both,fact=Visibility"`
	Level       symbol.Level      `eidos:"both,fact=Level"`
	Abstract    bool              `eidos:"both,fact=Abstract"`    // no body; a subtype must supply one
	Final       bool              `eidos:"both,fact=Final"`       // overriding is forbidden
	Override    bool              `eidos:"both,fact=Override"`    // replaces a supertype's member
	HasDefault  bool              `eidos:"both,fact=DefaultBody"` // an interface method with a body
	Async       bool              `eidos:"both,fact=Async"`
	Accessor    symbol.Accessor   `eidos:"both,fact=Accessor"`   // a get or set property accessor
	Indexer     bool              `eidos:"both,fact=Indexer"`    // an index signature: one key parameter, one result
	Constructs  bool              `eidos:"both,fact=Constructs"` // a construct signature on an interface
	Hard        bool              `eidos:"both,fact=HardPrivate"` // runtime-private: TypeScript's # names
	Receiver    *Param            `eidos:"both,walk"` // nil where the receiver is implicit
	Receives    *TypeRef          `eidos:"both,walk"` // set when declared outside the type it attaches to
	TypeParams  []*TypeParam      `eidos:"both,walk,fact=TypeParams"`
	Params      []*Param          `eidos:"both,walk"`
	Returns     []*Return         `eidos:"both,walk,fact=MultiReturn"`
	Throws      []*TypeRef        `eidos:"both,walk,fact=Throws"`
	Annotations Annotations       `eidos:"emit,fact=Annotations"`
	Body        Body              `eidos:"emit"`
	Host        symbol.Identity   `eidos:"node"`
}

// Param is one parameter of a callable, or its receiver.
//
// Name is empty where the language allows an unnamed parameter, as
// Go and Java interfaces do. Label is the caller-facing name where
// a language gives a parameter two, which Swift and Objective-C do:
// in "func greet(person name: String)" the label is "person" and
// the name is "name".
//
// Default holds the source spelling of the default value,
// unevaluated, and is empty when the parameter has none. A
// generator that drops a default changes the callee's contract, so
// the spelling is carried with the parameter rather than living in
// metadata.
//
// Variadic distinguishes the positional and keyword forms, because
// Python, Ruby and PHP have both. It is legal on the trailing
// parameters only, and frontends enforce that rather than the
// model.
type Param struct {
	ID          symbol.Identity `eidos:"node"`
	Pos         position.Pos    `eidos:"node"`
	Name        string          `eidos:"both,name"`       // "" when unnamed
	Label       string          `eidos:"both,fact=Label"` // caller-facing name; Swift and Objective-C
	Type        *TypeRef        `eidos:"both,walk"`
	Default     string          `eidos:"both,fact=ParamDefault"` // source spelling, unevaluated; "" when none
	Variadic    symbol.Variadic `eidos:"both,fact=Variadic"`     // positional or keyword
	Annotations Annotations     `eidos:"emit,fact=Annotations"`
}

// Return is one result of a callable.
//
// The list is a slice because Go returns several values. A language
// with one result fills one entry, and a language with none fills
// none. Name carries a Go named result and is empty elsewhere.
type Return struct {
	ID   symbol.Identity `eidos:"node"`
	Pos  position.Pos    `eidos:"node"`
	Name string          `eidos:"both,name,fact=NamedReturn"` // Go named results; "" elsewhere
	Type *TypeRef        `eidos:"both,walk"`
}
