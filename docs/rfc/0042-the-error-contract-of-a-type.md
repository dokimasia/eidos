---
rfc: 0042
title: The error contract of a type
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-12
updated: 2026-10-12
discussion: none
supersedes: none
superseded-by: none
produces-adr: ADR-0030
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0042: The error contract of a type

## Summary

The rules compose the name of an error value and recognise such a name. They return neither the base
of a recognised name nor the error contract of a type. That contract consists of the member that
returns the message, the field that stores the cause, and the hooks that the error functions of the
language call. The fold maps Go's `error` to Opaque, so a field of that type has no classification.
The sentinel plugin derives these facts from Go spellings. This proposal adds the base of a sentinel
name and a well-known identity for the error value. It also adds an optional capability that states
the error contract of a type.

## Motivation

- `ErrorValueRules` composes `ErrNotFound` from the base `not found`, and recognises `ErrNotFound`
  as a sentinel name. It does not return the base of a recognised name, so a plugin that writes a
  message for `ErrNotFound` cuts the prefix itself.
- The kernel's fold maps Go's predeclared `error` to Opaque. A field, a list element or a type
  argument of that type has no classification, and every spoke refuses to translate it.
- No projection states the contract of an error type. The contract has the member that returns the
  message, the hooks `Unwrap`, `Is` and `As` that Go's `errors` package calls, and the field that
  stores the cause. It also states whether the error is the type or a pointer to it. The sentinel
  plugin finds the cause as the first field spelled `error` in a struct whose method `Unwrap`
  returns a result spelled `error`.

A context is a parameter of a call. Go's rules classify a parameter of type `context.Context`, and
TypeScript's rules a parameter of type `AbortSignal`, as the context of the call. The plugins of the
repository read a context only as a parameter, so this proposal does not classify the context on any
other reference.

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-core/rules` | `ErrorValueRules.SentinelBase` and `Bound.SentinelBase` in place of `IsSentinelName`. `WellKnownError`. `ErrorTypeRules`, `ErrorContract` and `Bound.ErrorContract` |
| Go satellite | the rules implement `SentinelBase` and `ErrorTypeRules`, `Builtin` maps `error` to `WellKnownError`, and the spoke spells it. `MethodSet` and `NullaryString` move from the annotator into the rules, and the annotator stamps `golang.satisfiesError` through `ErrorContract` |
| TypeScript satellite | the rules implement `ErrorTypeRules`, `Builtin` maps the global `Error` to `WellKnownError`, and the spoke spells it |
| Java and Rust satellites | the spokes spell `WellKnownError` |
| `eidos-conformance` | the hub has a row of the error value |
| `eidos-reference/sentinel` | stamps through `SentinelBase` and takes the cause from `ErrorContract` |
| `eidos-sdk` | the facade re-exports the new declarations |
| `docs/architecture` | the projection document names the capability and the well-known error |

### Invariants

1. Recognition round-trips: `SentinelBase(SentinelName(b))` reports true for every base `b` that
   opens with a letter.
2. `ErrorContract` reads the members that the member walk of the language returns, so it applies the
   member policy of the language, promotion included.
3. A language maps its own spelling of the error value to `WellKnownError` in its `Builtin`, as it
   maps its spelling of a timestamp.
4. The Go annotator and the Go rules decide with one rule whether a type is an error, so the stamp
   `golang.satisfiesError` and the projected contract agree.

### The base of a sentinel name

```go
type ErrorValueRules interface {
    // SentinelName returns the name of the error value that a generator
    // derives from a base name.
    SentinelName(base string) string
    // SentinelBase returns the base that SentinelName composes ident
    // from, and reports false where ident is not a name that SentinelName
    // returns. The Go rules return NotFound for ErrNotFound.
    SentinelBase(ident string) (string, bool)
}

// SentinelBase returns the base of the sentinel name ident. It reports
// false where ident is not a sentinel name, and where the language does
// not implement ErrorValueRules.
func (b Bound) SentinelBase(ident string) (string, bool)
```

### The error value

```go
// WellKnownError is the value of a failure, such as Go's error and
// TypeScript's Error.
WellKnownError = wellKnown("error")
```

Go's `Builtin` maps the predeclared `error`, and TypeScript's `Builtin` maps the global `Error`. The
spokes spell the identity as `error` in Go, `Error` in TypeScript, `Exception` in Java and
`Box<dyn std::error::Error>` in Rust. The Go rules keep the zero value `nil` and refuse a sample of
the error value, as they do now.

### The error contract

```go
// ErrorTypeRules is the optional capability of a language whose types
// can be errors. [Bound.ErrorContract] calls it.
type ErrorTypeRules interface {
    // ErrorContract returns the contract of decl as an error type, and
    // reports false where decl is not one. It reads the members of decl
    // through v.
    ErrorContract(decl symbol.Symbol, v View) (ErrorContract, bool)
}

// ErrorContract is what a language states about an error type.
type ErrorContract struct {
    // Message is the member that returns the message of the error: the
    // method Error of a Go type, and the field message where a
    // TypeScript class declares one. It is nil where the type inherits
    // the member from the error of its language.
    Message symbol.Symbol
    // Cause is the field that stores the error that caused this one, and
    // nil where the type has none.
    Cause *node.Field
    // Unwrap, Is and As are the hooks that the error functions of the
    // language call, and nil where the type does not declare one.
    Unwrap, Is, As *node.Method
    // Pointer reports that a pointer to the type is the error and a
    // value of the type is not, as a Go method Error with a pointer
    // receiver states.
    Pointer bool
}

// ErrorContract returns the contract of decl as an error type, through
// the rules of the language of decl. It reports false where decl is not
// an error type, and where those rules do not implement ErrorTypeRules.
func (b Bound) ErrorContract(decl symbol.Symbol) (ErrorContract, bool)
```

The Go rules implement the capability for a struct, an enumeration, an interface and a defined type
whose method set has the method `Error() string`:

- The method set is the members that the member walk returns, and for a defined type the file-level
  methods of its package whose receiver names it. `MethodSet` in the Go rules returns it, and the Go
  annotator reads it there.
- `Pointer` reads the mark `golang.receiverIsPointer` of the method `Error`.
- `Unwrap` is `Unwrap() error` or `Unwrap() []error`, `Is` is `Is(error) bool`, and `As` is
  `As(any) bool`.
- `Cause` is the first field of the type `error` where the type declares `Unwrap() error`.

The TypeScript rules implement the capability for a class whose `extends` chain reaches a global
error class, such as `Error` or `TypeError`. `Message` and `Cause` are the fields `message` and
`cause` where the class declares them. TypeScript has no hooks, so `Unwrap`, `Is` and `As` are nil.

### Failures

The queries do not report a finding. `ErrorContract` reports false for a nil declaration, and
`SentinelBase` for an empty identifier.

### Migration

1. Replace `IsSentinelName` with `SentinelBase` in the rules, the Go rules and the sentinel plugin.
2. Add `WellKnownError`, map it in the two `Builtin` functions, spell it in the four spokes, and add
   the row of the hub.
3. Add `ErrorTypeRules`, implement it in the Go and TypeScript rules, move `MethodSet` and
   `NullaryString` into the Go rules, and take the cause of the sentinel plugin from the contract.
4. Name the capability and the identity in the projection document.

## Alternatives

### A classification of error and context as their own forms

The fold could have forms of its own for the error value and the context. A form changes the closed
enum of the forms that every spoke and every plugin switches over, while a well-known identity adds
one case to the spokes alone. The context is a parameter role in both languages that have one.

### An error contract from facts of the Go annotator

The Go annotator stamps `golang.satisfiesError`, and the rules could read the stamp. A rule that
reads the stamp reports false before the annotator has run, so an annotator of another plugin would
not see a contract. The rules compute the contract from the method set, and the annotator stamps
from the same computation.

### Keeping IsSentinelName beside SentinelBase

A second method of the same recognition has two implementations that can disagree. `SentinelBase`
returns the base and reports the recognition in one call.

## Drawbacks

- `ErrorValueRules` changes a method, so every implementation and every caller of
  `IsSentinelName` changes: the Go rules, the kernel's fixture rules and the sentinel plugin.
- A field of type `error` now folds to a reference, so a plugin that switched on Opaque for it reads
  the well-known identity instead. The Java and Rust spokes translate the error value into a type of
  their own language, where they refused it before.
- The rules gain one capability, one type and one well-known identity, and the Go rules two exported
  functions.

## Open questions

None.

## Unresolved and future work

None.

## References

- The Go blog on errors in Go 1.13, which describes `Unwrap`, `Is` and `As`:
  <https://go.dev/blog/go1.13-errors>.
- The cause of an error in ECMAScript: <https://tc39.es/ecma262/#sec-installerrorcause>.
