---
adr: 0021
title: The receiver of an implementation avoids the names of its signature
status: Accepted
date: 2026-10-11
supersedes: none
superseded-by: none
rfc: RFC-0025
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0021: The receiver of an implementation avoids the names of its signature

## Status

Accepted

## Context

Each method of an implementation has a receiver, because its body reads the field `next` through
the receiver. The receiver's name is the first letter of the struct's name in lower case, so the
struct `LedgerAudit` gives `l`. The accepted design appended a number to the letter only where a
parameter of the method had the same name.

Go declares the receiver, the parameters, the named results and the type parameters of a method in
one block. With Go 1.27.2, `go vet` reports `l redeclared in this block` for each of these
methods:

```go
func (l LedgerAudit) Post(account string) (l string)
func (l Box[l]) Get() l
func (l Ledger) Map[l any](v l) l
```

## Decision

We will give each receiver a name that differs from each name that the signature of its method
declares, because Go declares the receiver in the same block as those names.

- The letter is the first letter of the struct's name in lower case. A struct whose name does not
  start with a letter gets the letter `r`.
- Where a parameter, a named result, a type parameter of the method or a type parameter of the
  struct has the letter as its name, the loader appends the smallest number from 0 that gives a
  free name, such as `l0`.

## Alternatives Considered

### Numbering against the parameters alone

The loader appends a number only where a parameter has the letter as its name. The accepted design
specified this rule. The receiver's name then does not depend on the results or the type
parameters.

It lost because a method with a named result or a type parameter of that name does not compile in
Go. `go vet` reports `l redeclared in this block` for each of the three methods of the context.

## Consequences

**Positive:**

- An implementation compiles in Go whatever names the signature of its subject declares.

**Negative:**

- The loader does not see the names that a template declares in a body. A body that declares the
  receiver's name at its top level does not compile in Go, because the body shares the block of
  the receiver.
- A rename of a result or a type parameter in the subject can rename the receiver of a generated
  method. The body of that method then changes in the diff too.
- A template reads the receiver's name from `.Decl.Receiver.Name`. A template that writes the letter
  itself generates Go that does not compile for a method whose receiver has a number.

**Neutral:**

- `Ledger` of the conformance fixture does not declare a named result or a type parameter, so the
  receiver of `LedgerAudit.Post` is `l`.

## References

| What | Where |
|---|---|
| The Go specification: declarations and scope | https://go.dev/ref/spec#Declarations_and_scope |
