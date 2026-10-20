---
adr: 0073
title: The Rust rules fold the standard library through a table
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

# ADR-0073: The Rust rules fold the standard library through a table

## Status

Accepted

## Context

A Rust signature names the types of the standard library through the prelude, through a use
declaration and through a path of `std`, `core` or `alloc`:

```rust
use std::collections::HashMap;

pub struct Index {
    names: HashMap<String, Vec<u32>>,
    ttl: std::time::Duration,
}
```

The load of a crate does not read a crate of the standard library, so every such reference has no
target.
Under the absent rules every one folds to Opaque, and `String`, `Vec<T>` and `Option<T>` translate
into no other language.

## Decision

We will fold a reference to a type of the standard library through a table of builtins in the Rust
rules, because the canonical forms of the hub are the forms that a signature needs from the standard
library.

- The table maps the primitives and the texts, `Option`, the lists, the maps, the pointers and the
  cells, `Duration`, `SystemTime`, the error types, and the iterator and stream traits to their forms.
- The rules match the last segment of a spelling and the module of its package against the table.
- A path of `std`, `core` and `alloc` names the same entry, because the three crates re-export one
  another's items. A name of the prelude does not need a path.
- A type that the table lacks folds to Opaque. That includes the sets, `Instant`, a type of another
  crate and a `Result` outside the return of a callable.

## Alternatives Considered

### The standard library as a store

The frontend would load the standard library of Rust from a pinned copy of its source, as the Python
frontend loads typeshed. The types of `std` state their structure through traits and private fields,
so `Vec<T>` would fold to a reference to a struct and not to a list. The rules would still need a
table to give it the form of a list.

## Consequences

**Positive:**

- A Rust field of `HashMap<String, Vec<u32>>` folds to a map from text to a list of numbers, which
  every target spells.
- A load of a crate does not read a copy of the standard library, so its time and its memory do not
  grow with it.

**Negative:**

- A new type of the standard library folds to Opaque until the table has its row.
- A type of another crate, such as `chrono::DateTime`, folds to Opaque, because the load reads no
  dependency.

## References

| What | Where |
|---|---|
| RFC-0076, the design of this decision | ../rfc/0076-the-projection-rules-of-rust.md |
| D191, the row of this decision | ../architecture/21-decisions.md |
| The standard library prelude | <https://doc.rust-lang.org/std/prelude/index.html> |
