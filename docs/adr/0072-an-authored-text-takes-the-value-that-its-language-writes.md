---
adr: 0072
title: An authored text takes the value that its language writes
status: Accepted
date: 2026-10-20
supersedes: none
superseded-by: none
rfc: RFC-0076
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0072: An authored text takes the value that its language writes

## Status

Accepted

## Context

An author states a sample of a field with the directive of the kernel, and `View.Authored` lifts the
text of a text type to a string literal by the shape of the type. Rust writes a string literal as a
`&str`, so the literal does not type-check as a `String`:

```rust
let order = Order { sku: "stated" };               // rustc refuses: expected `String`
let order = Order { sku: String::from("stated") }; // the value of a String
```

A `char` takes a character literal, as `'a'`, and a `PathBuf` takes `PathBuf::from("a")`. The rules of
Rust already write these values where they derive a sample, and an authored sample bypassed them.

## Decision

We will ask the optional capability `TextRules` of a language for the value of an authored text of a
text type in `View.Authored`, because a language can write a text of a text type through a call or
through another literal.

- `TextRules.Text(ref, text, v)` returns the value of the type `ref` whose content is `text`, and the
  string literal of `text` where the type takes the literal itself.
- `View.Authored` asks the capability for every authored text of a type that folds to text. A
  language without the capability keeps the string literal.
- The Rust rules give `str` the string literal, `String` and the other owned texts the call of
  `from`, `Path` and `OsStr` the call of `new`, and a `char` the character literal of a text of one
  character.

## Alternatives Considered

### An authored text through LiteralFor

`View.Authored` would quote the text in the syntax of the language and read it back through
`LiteralFor`, which already writes `String::from` for a string literal. The kernel would need the
quoting of every language. A text with a quote, a backslash or a control character would also need the
escapes of each language before the read.

## Consequences

**Positive:**

- An authored sample of a Rust `String` writes `String::from("stated")`, and the generated test
  compiles.
- The backend quotes the text, so the kernel does not keep a rule of the syntax of any language.

**Negative:**

- The kernel has one more optional capability, and the SDK facade restates it.
- A language whose text types need a conversion writes code that does not compile until its rules
  implement the capability.

## References

| What | Where |
|---|---|
| RFC-0076, the design of this decision | ../rfc/0076-the-projection-rules-of-rust.md |
| D190, the row of this decision | ../architecture/21-decisions.md |
| The Rust reference, literal expressions | <https://doc.rust-lang.org/reference/expressions/literal-expr.html> |
