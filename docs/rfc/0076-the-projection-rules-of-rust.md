---
rfc: 0076
title: The projection rules of Rust
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-20
updated: 2026-10-20
discussion: https://github.com/dokimasia/eidos/issues/4
supersedes: none
superseded-by: none
produces-adr: ADR-0071, ADR-0072, ADR-0073
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0076: The projection rules of Rust

## Summary

We propose the package `lang/rust/rules`, which satisfies the source-rules contract of the kernel for
Rust. A callable that returns `Result<T, E>` reports its failure under the result-type error model,
and the kernel splits the return into a view of `T` and a view of `E` through a new optional
capability, `ResultRules`. A second new capability, `TextRules`, gives an authored text the value
that Rust writes for its type, as `String::from("a")`. The rules fold the primitive types and the
types of the standard library that a crate names without loading the standard library, resolve the
spelling of a directive along the scope order of Rust, and derive samples, zeros and literals in
Rust's grammar. They implement the capabilities of enums, error types, results, texts, ownership,
equality, generics and constructors. The frontend keeps the type of an associated type binding, so
`impl Iterator<Item = T>` folds to a stream of `T`, and it resolves `Self` to the type of its impl
block. The Rust backend writes the values that the rules derive: a map, a tuple, a tuple struct, a
variant and an associated function.

## Motivation

eidos reads and writes Rust, and every walk over a Rust declaration runs under the absent rules of
the kernel. The rules of a language decide what a plugin reads of a callable and a type:

- Every parameter is an input, and every return is a value under no error model. A Rust function
  that returns `Result<Session, StoreError>` has the error model `none`, and its value is the
  `Result` itself.
- Every builtin is Opaque, so `String`, `Vec<T>` and `Option<T>` translate into no other language.
- A directive reference does not resolve, and every sample refuses with `RefusedNoRules`.
- No optional capability applies, so no enum, error type or constructor projects.

A composition of the Rust frontend and the shape catalog of `eidos-plugin-shape` ran over a trait of
three methods. The detectors found no shape on any of them, because each reads the callable that the
rules project. A test generator over Rust cannot derive a check from the failure channel of a
callable, and it cannot write a value of a Rust type.

The kernel already names the result type as an error model, `ErrorsResultType`, and the role
`ReturnError` for the failure of that model. No rules produce either, and the kernel has no way to
state which part of a `Result` is the value and which the failure. That part belongs in the kernel,
because every consumer of a callable reads the views of its returns.

## Detailed design

### Components

| Component | Package | Change |
|---|---|---|
| Kernel | `eidos-core/rules` | `ResultRules` and the split of a result type in `Bound.CallableOf`, and `TextRules`, which `View.Authored` asks for the value of a text |
| SDK | `eidos-sdk/rules` | the regenerated facade with `ResultRules` and `TextRules` |
| Frontend | `eidos-lang-rust/frontend` | the type of an associated type binding as the argument of its reference, `Self` resolved to the owner of its scope, and a receiver `&self` as a borrow of `Self` |
| Rules | `eidos-lang-rust/rules` | new: the source rules and eight optional capabilities |
| Backend | `eidos-lang-rust/backend` | the values of a map, a tuple, a tuple struct, a variant and an associated function, and the use of a path that starts at a module |
| Binary | `cmd/eidos` | the Rust rules among the rules of the stock composition |
| Conformance | `eidos-conformance/lang/rust` | the corpus with its rules, and a projection level for each feature |
| Conformance | `eidos-conformance/matrix` | the rows of `ResultRules` and `TextRules` on the projection side |

### Invariants

1. Every type that a Rust signature can state folds to a shape: a primitive, a type of the standard
   library in the table of builtins, a declaration of the graph, or Opaque for a type that no
   canonical form has, such as a set and a `Result` outside a return.
2. A callable that returns a result type has the error model `result-type`, a view of its failure
   with the role `error`, and a view of its value where the value is not the unit type.
3. A sample, a zero and a literal that the rules derive are values that the Rust backend writes.
4. A read of a rule records on the view that the rule receives.

### The split of a result type

```go
package rules

// ResultRules is the optional capability of a language whose callable
// reports a failure in the type of its return, as a Rust function returns
// Result<T, E>. [Bound.CallableOf] asks it for each return of a callable
// whose error model is [ErrorsResultType], and splits a return of a result
// type into the view of its value and the view of its failure.
type ResultRules interface {
    // Result returns the reference of the value and the reference of the
    // failure of the result type ref, and reports false for a reference of
    // another type. value is nil for a result without a value, such as
    // Result<(), E>. Result reads through v, so each read records.
    Result(ref *node.TypeRef, v View) (value, failure *node.TypeRef, ok bool)
}
```

- `Bound.CallableOf` gives each return of a result type two views under `ErrorsResultType`: the value
  with the role that `ReturnRoles` gave the return, where the value is not nil, and then the failure
  with the role `ReturnError`. Both views have the name of the return.
- Every other return keeps one view, and a language without the capability keeps the views of today.
- `ReturnRoles` states the role of the value of a result, so a `Result` of an iterator is a stream
  with a failure.

### The value of an authored text

`View.Authored` lifts a text that an author stated for a text type to a string literal. Rust writes a
string literal as a `&str`, so a `String` takes `String::from("a")`, and a `char` takes a character
literal.

```go
package rules

// TextRules is the optional capability of a language whose text types take
// a string literal through a conversion, as a String of Rust takes
// String::from("a"). [View.Authored] asks it for the value of a text that an
// author stated for a type that folds to text.
type TextRules interface {
    // Text returns the value of the type ref whose content is text, and the
    // string literal of text where the type takes the literal itself. Text
    // reads through v, so each read records.
    Text(ref *node.TypeRef, text string, v View) emit.Value
}
```

- `View.Authored` asks `TextRules` for every authored text of a type that folds to text, and a
  language without the capability keeps the string literal.
- The Rust rules give `str` and a borrow of it the string literal, `String`, `OsString`, `PathBuf`,
  `Cow<str>` and a pointer to a text the call of `from`, and `Path` and `OsStr` the call of `new`.
- A `char` takes the character literal of a text of one character, with the escapes of Rust for a
  quote, a backslash and a control character. A text of another length keeps the string literal.

### The member walk

```go
func (Rules) Members() rules.MemberPolicy {
    return rules.MemberPolicy{
        Contributes: []rules.Contribution{rules.ContributesExtends, rules.ContributesImplements},
        Shadowing:   rules.ShadowPromote,
    }
}
```

- A trait inherits the methods of its supertraits, which the frontend loads into `Extends`.
- A struct has the methods of the traits that a trait impl adds to its `Implements`. The model has
  no `Implements` on an enum and a sum, so they have their declared methods alone. The frontend
  folds the methods of an inherent impl onto the type, so they are the declared members of the type.
- A declared member hides the inherited members of its name, as an inherent method hides a trait
  method in the method resolution of Rust. Two traits that declare one name at one depth cancel each
  other, as Rust refuses the call with error E0034 unless the caller names the trait.

### Roles

- Every parameter is an input. A caller cancels a future of Rust when it drops it, so no parameter
  has the role context.
- A return is a stream where its type, or the value of its result, is `impl` or `dyn` of `Iterator`,
  `IntoIterator`, `DoubleEndedIterator`, `ExactSizeIterator` or `Stream`, also behind `Box`. Every
  other return is a value.
- The error model is `ErrorsResultType` where a return is a result type, and `ErrorsNone`
  otherwise. A panic is no failure channel of a signature.

A result type is one of these references:

| Reference | Value | Failure |
|---|---|---|
| `Result<T, E>`, `std::result::Result<T, E>` and the paths of `core` and `std` to it | `T` | `E` |
| a reference to an alias of the crate, such as `type Result<T> = std::result::Result<T, Error>` | the value of its target, with the arguments substituted | the failure of its target |
| `<path>::Result<T>` without a target, such as `io::Result<T>` and `anyhow::Result<T>` | `T` | `<path>::Error` |
| `<path>::Result` without a target and without arguments, such as `fmt::Result` | none | `<path>::Error` |

The value is none where it is the unit type `()`.

### Builtins

`Builtin` classifies a reference that the resolution step left without a target by its last segment
and the module of its path. A name of the prelude has no module, and a path of the standard library
starts with `std`, `core` or `alloc`, which re-export one another's items.

| Spelling | Shape |
|---|---|
| `bool` | Bool |
| `i8`, `i16`, `i32`, `i64`, `i128`, and `u8` to `u128` | a signed or an unsigned integer of the width |
| `isize`, `usize` | a signed or an unsigned integer of the platform's width |
| `f32`, `f64` | a float of the width |
| `char`, `str`, `String`, `Cow<str>`, and `OsString`, `OsStr`, `PathBuf`, `Path`, `CString` and `CStr` of their modules | Text |
| `()` | the well-known empty value |
| `!` | a union without members |
| `Option<T>` | Optional of `T` |
| `Vec<T>`, `VecDeque<T>`, `LinkedList<T>`, `BinaryHeap<T>` | a mutable list. `Vec<u8>` folds to Bytes, as the kernel folds every list of bytes |
| `HashMap<K, V>`, `BTreeMap<K, V>` | a mutable map |
| `Box<T>`, `Cell<T>`, `RefCell<T>`, `Mutex<T>`, `RwLock<T>` | a mutable Borrow of `T` |
| `Box<dyn Error>`, also with the bounds `Send`, `Sync` and `Unpin` | the well-known error |
| `Rc<T>`, `Arc<T>` | an immutable Borrow of `T` |
| `Duration` of `time` | the well-known duration |
| `SystemTime` of `time` | the well-known timestamp |
| `Error` of `io`, of `error` and of `fmt` | the well-known error |
| `Iterator`, `IntoIterator`, `DoubleEndedIterator`, `ExactSizeIterator` with `Item = T` | a stream of `T` |
| `Stream` of `futures`, `futures_core`, `futures_util`, `tokio_stream` and `async_std`, with `Item = T` | an asynchronous stream of `T` |

Every other spelling is Opaque. That includes `HashSet` and `BTreeSet`, because the kernel has no
form for a set, `Instant`, which is no point in time of a calendar, the function traits, a type of a
crate that the load does not read, and a generic type whose reference does not state each of its type
arguments. A `Result` outside the return of a callable is Opaque too, because the kernel has no form
of a tagged union of two types. A union of the kernel has no tags, so `Result<String, String>` would
fold to a union whose members cannot tell the value from the failure.

### The frontend

The frontend lowers the binding `Item = T` of `Iterator<Item = T>` as a named reference with its
spelling `Item=T` and the type `T` as its one argument. The spelling of the binding stays, so no
identity changes. `Builtin` gives an iterator the stream of the folded argument of its binding
`Item`, and a binding of another name folds to Opaque.

`Self` resolves to the owner of the scope of its reference: the struct, the enum or the trait whose
definition, impl block or trait body contains it. A path of one item from `Self`, as `Self::Key`,
resolves to the associated item of that name of the owner. A receiver `&self` lowers as an immutable
borrow of `Self`, and `&mut self` as a mutable one, each with the spelling `&Self` or `&mut Self` and
the lifetime of the parameter. `self` and `mut self` lower as `Self`. A trait method and an inherent
method with a receiver then project whole, and `Ownership` reads the borrow of the receiver from its
form.

### Resolution

`Resolve` returns the declaration that the spelling of a directive names in the scope of its subject,
in the order of the frontend:

1. A bare name is an item of the module of the subject or the target of its use binding, in one
   tier, and then a name that a glob import of the module imports.
2. `crate`, `self` and `super` start a path at the crate root, at the module and at its parent.
3. Any other path starts at a child module, at the target of the use binding of its first segment,
   or at an extern crate.
4. A path to an associated item, as `Store::get`, is a member of the type that its other segments
   name.
5. `Self` is the type of the subject: the type that declares a member, and a type itself. A path from
   `Self`, as `Self::new`, is a path to an associated item of that type.

The items of a module are not in scope in its child modules. A spelling that names nothing returns an
error that names the spelling and the scope.

### Values

The rules derive values in Rust's grammar of expressions:

| Type | Sample and alternate | Zero |
|---|---|---|
| an integer | `42` and `7` | `0` |
| a float | `1.5` and `2.5` | `0.0` |
| `bool` | `true` and `false` | `false` |
| `char` | `'a'` and `'b'` | `'\0'` |
| `&str`, `str` | `"<hint>-a"` and `"<hint>-b"` | `""` |
| `String`, `OsString`, `PathBuf`, `Cow<str>` | `String::from("<hint>-a")` and `-b`, and the `from` of their types | `String::new()`, the `new` of `OsString` and `PathBuf`, and `Cow::from("")` |
| `&Path`, `&OsStr` | `Path::new("<hint>-a")` and `-b` | `Path::new("")` |
| `()` | `()`, and no alternate | `()` |
| `Option<T>` | `Some(x)` of each value of `T` | `None` |
| `Result<T, E>` | `Ok(x)` of each value of `T`, and `Ok(())` without an alternate where `T` is `()` | none |
| `Vec<T>` | `vec![x]` | `Vec::new()` |
| `VecDeque<T>`, `LinkedList<T>`, `BinaryHeap<T>`, `HashSet<T>`, `BTreeSet<T>` | `VecDeque::from([x])` | `VecDeque::new()` |
| `[T; N]` | `[x, …]` with N elements, for N from 1 to 64 | the array of N zeros, where `T` has a zero |
| `&[T]` | `&[x]` | `&[]` |
| a tuple | `(x, y)` of each member | the tuple of zeros |
| `HashMap<K, V>`, `BTreeMap<K, V>` | `HashMap::from([(k, v)])`, with another key for the alternate | `HashMap::new()` |
| `Box<T>`, `Rc<T>`, `Arc<T>`, `Cell<T>`, `RefCell<T>`, `Mutex<T>`, `RwLock<T>` | `Box::new(x)` | the `new` of the zero of `T` |
| `Box<str>`, `Rc<[T]>` and every pointer to a type without a size | `Box::from(x)` | the `from` of the zero of `T` |
| `Box<dyn Error>` | `Box::from("<hint>-a")` and `-b` | none |
| `&T` | `&x` | none |
| `Duration` | `Duration::from_secs(1)` and `2` | `Duration::ZERO` |
| `SystemTime` | `Add::add(UNIX_EPOCH, Duration::from_secs(1700000000))` and one second later | `UNIX_EPOCH` |
| `io::Error` | `io::Error::other("<hint>-a")` and `-b` | none |
| `fmt::Error` | `fmt::Error`, and no alternate | none |
| a unit-only enum | the first two variants, as `Color::Red` | the variant of `#[default]` where the enum derives `Default` |
| a struct | the call of its first construction that returns the struct itself, with a sample of each parameter, and the alternate with the second value of the first parameter | `T::default()` where the struct derives or implements `Default` |
| a sum | the first variant that the rules can build: a unit variant, `E::V(x)` or `E::V { f: x }` | `T::default()` where the sum derives `Default` |
| `&mut T`, a type parameter, a function, a trait object, an iterator, the never type, `CString`, `CStr`, `Instant` | refused | none |

A mutable borrow refuses, because the value tree has no mutable address. `SystemTime` takes the call of
`Add::add`, because the value tree has no operator.

- `LiteralFor` reads a decimal, hexadecimal, octal and binary integer with underscores and a type
  suffix, a float with an exponent and a suffix, `true` and `false`, a character literal, a string
  literal and a raw string, and a byte string for bytes. It reads `None` and `Some(x)` for an option,
  the value of a pointer inside the call that the samples write, a variant by its name or its path,
  and the name of a constant or a static of the scope.
- A sample of a struct calls the first constructor of `ConstructRules`, and a struct without one
  builds its literal.

The Rust backend writes each value. It gains these forms:

- A keyed composite of a map type is a call of `from` of the type with an array of pairs, as
  `HashMap::<String, i32>::from([("a".to_string(), 1)])`.
- A positional composite of a tuple is a tuple expression, and one of a declared struct is the
  literal of a tuple struct, as `Point(1, 2)`.
- The name of a variant and of an associated item spells its owner, as `Color::Red` and
  `Duration::from_secs`, and the use declaration binds the owner.

The use declarations of a file follow the module of each reference:

- A path whose module part names the module of its reference uses that module from its parent. The
  module part `io` of `io::Error` names the module `std/io`, so the file uses `std::io`.
- A path from a crate takes no use, as `std::collections::HashMap` does. A path from `crate`, `self`,
  `super` and `Self` takes none either.
- Every other path starts at the item that the module declares, and the use binds that item. The
  trait `Named` is that item in `Named::Key`.

### Capabilities

| Capability | Rust |
|---|---|
| `EnumRules` | an enum without payloads. Each variant has its name as its text. The zero is the variant of `#[default]` where the enum derives `Default`. No variant is a duplicate, because rustc refuses two variants of one discriminant with error E0081 |
| `ErrorTypeRules` | a struct whose `Implements` contains `std::error::Error`, and a struct, an enum or a sum that derives `Error` of thiserror. The cause is the field of `#[source]` or `#[from]`, or the field `source`. The message comes from an impl of `Display`, which the frontend does not fold onto the type, so it is nil |
| `ResultRules` | the result types of the table of roles |
| `TextRules` | the call of `from` or `new` of an owned text and of a pointer to a text, and the character literal of a `char` |
| `OwnershipRules` | a shared borrow, as the receiver `&self`, borrows, a mutable borrow, as `&mut self`, borrows mutably, and every other parameter passes by value |
| `EqualityRules` | the types of `==`: a primitive, a text, the unit type, `Duration`, `SystemTime`, `Instant` and `fmt::Error`, and a list, a map, a set, an option, a result, a tuple, an array, a borrow, a cell and a pointer of comparable members, and a declared type that derives `PartialEq`, or a struct that implements it. A float compares, because `PartialEq` admits it. A mutex, `io::Error`, a function, a trait object, an iterator and a trait do not |
| `GenericsRules` | the witness of a type parameter: its default, else `()` where every bound is a trait that `()` implements, else `String` where every bound is one that `String` implements. `Reified` reports true, because Rust compiles each instantiation apart and keeps its type arguments |
| `ConstructRules` | each associated function of a struct that is not private and returns `Self` or the struct, also inside `Result` and `Option`, in declaration order, and then the literal of the struct where no field is private |

The rules implement no other capability. Rust has no properties, no checked exceptions, no struct
tags and no sentinel error values. A struct of Rust promotes no members. The load reads no crate of
the standard library, so no declaration of it is a target that `WellKnownRules` or `ShapeRules`
would bless.

### Composition

```go
// New returns the Rust rules as the [rules.SourceRules] that a composition
// registers. The value is [Rules], so a caller that needs a capability
// asserts its interface on it. It allocates nothing.
func New() rules.SourceRules
```

The stock binary registers `rustrules.New()` beside the rules of Go, TypeScript, protobuf, GraphQL
and Python. The corpus of Rust in the conformance module states its rules, so each feature states a
projection level, and the rules suite of the kernel runs over the corpus.

### Failures

| Failure | Effect |
|---|---|
| A spelling that names nothing in the scope | `Resolve` returns an error that names the spelling and the scope |
| A sample of a type without a value of its own | a refusal with its reason, such as `RefusedNoLiteral` for a set and `RefusedUnresolved` for a target outside the view |
| A value that the Rust backend cannot spell | `UnspeltValue` of the render |

### Migration

`ResultRules` and `TextRules` are new optional capabilities, so the rules of every other language keep
their views and their authored texts. The frontend adds an argument to the reference of a binding,
gives `Self` a target and lowers a borrowed receiver as a borrow, and the model fingerprint of the
Rust frontend changes with its version. A plugin that read the `Result` of a Rust return as its value
reads its value and its failure after the change. A plugin that read the spelling of a receiver reads
the same spelling, and its form is a borrow.

## Alternatives

### The result in the frontend

The frontend could load a return of `Result<T, E>` as two returns, `T` and `E`. The backend would then
write two results, and a Rust mirror would not equal its source. The model keeps the signature as the
source states it, and the rules interpret it.

### A return role of its own

`ReturnRoles` could give a result type a role `ReturnResult`, and the consumer would unwrap it. Every
detector of the shape catalog reads the value and the failure of a callable through their roles, so
each would repeat the unwrap for Rust.

### The standard library as a store

The frontend could load the standard library of Rust from a pinned copy of its source, as the Python
frontend loads typeshed. The types of `std` state their structure through traits and private fields,
so `Vec<T>` would fold to a reference to a struct and not to a list, and the rules would still need a
table to give it the shape of a list. A table of builtins folds the types that a signature names to
the canonical forms of the hub without the copy.

### The nearest arrival

`ShadowOverride` would take the first trait of two that declare one name. Rust refuses the call, so
the member set would contain a method that no caller can call without naming its trait.

### An authored text through LiteralFor

`View.Authored` could quote an authored text in the syntax of the language and read it back through
`LiteralFor`, which already writes `String::from` for a string literal. The kernel would then need the
quoting of every language, and a text with a quote, a backslash or a control character would need the
escapes of each one. `TextRules` takes the text as a value and leaves the quoting to the backend.

### A union for a result outside a return

`Builtin` could fold a `Result<T, E>` in a field or a function type to the union of `T` and `E`. A
union of the kernel has no tags, so a `Result<String, String>` would fold to a union of two texts, and
a target would write a type whose values do not tell a value from a failure. The return of a callable
splits through `ResultRules`, and every other `Result` is Opaque.

## Drawbacks

- The kernel grows by two interfaces of one method each, by the split in `CallableOf` and by one
  assertion in `View.Authored`. The SDK facade gains both interfaces.
- The rules package is a new package of 3,459 lines, with 4,226 lines of specs.
- `Builtin` keeps a table of the standard library by its paths. A type that the table lacks folds to
  Opaque, and a new type of the standard library needs a row.
- A crate that names a type of another crate, such as `chrono::DateTime`, folds it to Opaque, because
  the load reads no dependency.
- A `Result` in a field or in a function type folds to Opaque, so a feature that states one projects
  partly.
- The message of an error type is nil, because the frontend does not fold the methods of a trait impl
  onto the type.
- The model has no `Implements` on an enum and a sum. A trait impl for an enum does not make it an
  error type, and the member set of the enum does not contain the methods of the trait. An impl of
  `Default` does not give the enum a zero.
- A function that returns `impl Future<Output = T>` without the keyword `async` is not `Async`,
  because the frontend reads the asynchrony from the syntax.
- A path through a module that a use declaration renames, as `coll::HashMap` after
  `use std::collections as coll`, has a module part that does not name its module. The backend binds
  its first segment as an item of the module, because it does not read the use declarations of the
  source.

## Open questions

None.

## Unresolved and future work

- A kernel form for sets, which the Python satellite names too.
- A kernel form of a tagged union of two types, which a `Result` outside a return needs.
- `Implements` on an enum and a sum, which issue 44 asks for.
- The dependencies of a crate as stores of the frontend.

## References

| What | Where |
|---|---|
| Issue 4, the projection rules of Rust | <https://github.com/dokimasia/eidos/issues/4> |
| Issue 44, `Implements` on an enum and a sum | <https://github.com/dokimasia/eidos/issues/44> |
| Rust error E0081, a discriminant that two variants share | <https://doc.rust-lang.org/error_codes/E0081.html> |
| The Rust reference, method call expressions | <https://doc.rust-lang.org/reference/expressions/method-call-expr.html> |
| Rust error E0034, multiple applicable items in scope | <https://doc.rust-lang.org/error_codes/E0034.html> |
| The Rust reference, literal expressions | <https://doc.rust-lang.org/reference/expressions/literal-expr.html> |
| The Rust reference, paths | <https://doc.rust-lang.org/reference/paths.html> |
| The standard library prelude | <https://doc.rust-lang.org/std/prelude/index.html> |
| thiserror, the derive of `Error` | <https://docs.rs/thiserror/latest/thiserror/> |
| tree-sitter-rust v0.24.2, the node `type_binding` | <https://github.com/tree-sitter/tree-sitter-rust/tree/v0.24.2> |
