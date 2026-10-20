---
adr: 0071
title: The kernel splits a return of a result type
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

# ADR-0071: The kernel splits a return of a result type

## Status

Accepted

## Context

A Rust function reports a failure in the type of its return:

```rust
pub fn open(path: &str) -> Result<Session, StoreError>;
```

The kernel names the result type as an error model, `ErrorsResultType`, and the role `ReturnError`
for the failure of that model. No rules produced either. Under the absent rules, the callable view
of `open` has the error model `none`, and its one return is the `Result` itself. Every detector of
the shape catalog reads the value and the failure of a callable through the roles of its returns, so
no detector found a shape on a Rust function that can fail.

## Decision

We will split a return of a result type in `Bound.CallableOf` through the optional capability
`ResultRules`, because every consumer of a callable reads the value and the failure of its returns
through their roles.

- `ResultRules.Result(ref, v)` returns the reference of the value and the reference of the failure
  of a result type, and reports false for a reference of another type. The value is nil for a
  result without a value, such as `Result<(), E>`.
- Under `ErrorsResultType`, a return of a result type has two views: the value with the role that
  `ReturnRoles` gave the return, where the value is not nil, and then the failure with the role
  `ReturnError`. Both views have the name of the return.
- Every other return keeps one view, and a language without the capability keeps the views that it
  had.
- The model keeps the signature as the source states it.

## Alternatives Considered

### The result in the frontend

The frontend would load a return of `Result<T, E>` as two returns, `T` and `E`. The backend would then
write a function of two results, and a Rust mirror would not equal its source.

### A return role of its own

`ReturnRoles` would give a result type a role `ReturnResult`, and the consumer would unwrap it. Every
detector of the shape catalog reads the value and the failure through their roles, so each detector
would repeat the unwrap for Rust.

## Consequences

**Positive:**

- A detector finds the failure channel of a Rust function through the role `ReturnError`, as it
  finds the error result of a Go function.
- A Rust mirror writes the `Result` of its source, because the model is unchanged.

**Negative:**

- The kernel has one more optional capability and a split in `CallableOf`, and the SDK facade
  restates the interface.
- A consumer that counts the returns of a callable view counts two for one `Result` of the source.
- A `Result` outside the return of a callable, in a field or a function type, has no split and
  folds to Opaque.

## References

| What | Where |
|---|---|
| RFC-0076, the design of this decision | ../rfc/0076-the-projection-rules-of-rust.md |
| D189, the row of this decision | ../architecture/21-decisions.md |
| The Rust standard library, `std::result` | <https://doc.rust-lang.org/std/result/index.html> |
