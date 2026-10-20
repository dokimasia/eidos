---
adr: 0067
title: A parameter states how a caller passes its argument
status: Accepted
date: 2026-10-20
supersedes: none
superseded-by: none
rfc: RFC-0073
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0067: A parameter states how a caller passes its argument

## Status

Accepted

## Context

A Python parameter before the slash of its signature takes its argument by position alone, and a
parameter after the star or after `*args` takes it by keyword alone. The `timeout` of
`def get(url, *, timeout: float)` accepts no positional argument. A generator that writes a
delegating call, a stub or a client of such a callable has to know which form each parameter
accepts. A generator that writes a keyword parameter as an ordinary one changes the calls that the
callee accepts.

`Param` had no field for the way a caller passes the argument. `Param.Label` is the name that a
caller writes for a parameter with two names, the outer name of a Swift parameter, which is a
separate fact.

## Decision

We will give `Param` the field `Passing` of the type `symbol.Passing`, with the fact `Passing`,
because the way a caller passes the argument changes the calls that a callable accepts, as its
default does.

- `PassingDefault` is the zero value: the way of the language of the callable. `PassingPositional`
  passes the argument by position alone, and `PassingKeyword` by keyword alone.
- The Python frontend sets the passing of each parameter, and the Python backend writes the slash
  and the star of a signature from it.
- `rules.ParamView` reports the passing, so a projection reads it.
- The discriminator of an overloaded callable spells a keyword parameter as its name and `=`
  before its type, and a positional parameter with a `/` after its type, so two overloads that
  differ only in passing have two identities.
- Every other backend takes a stance on the fact. Go, Java, TypeScript and Rust take `Holds`,
  because their callers pass every argument by position. GraphQL takes `Holds`, because an argument
  always has its name, and protobuf takes `Holds`, because a method takes one request.

## Alternatives Considered

### The label of a parameter

`Param.Label` would mark a keyword parameter with its own name. A label is the outer name that a
call writes, which Python does not have, and a positional-only parameter has no label to state.

### A stamp of the Python frontend

A stamp `python.passing` on each parameter would leave the model unchanged. A generator into
another language reads the model and not the keys of Python, so it would write a keyword parameter
as an ordinary one.

## Consequences

**Positive:**

- A generator over Python sources writes the signatures of its sources with their slashes and
  stars.
- A projection of a callable tells a generator that a delegating call names a parameter.

**Negative:**

- The schema gains a field with a fact, so every sealed state runs cold once, and the coverage suite
  fails a released backend that takes no stance on the fact.
- The backends of languages without keyword arguments state a verdict for a fact that no source of
  theirs sets.

## References

| What | Where |
|---|---|
| RFC-0073, the design of this decision | ../rfc/0073-the-python-satellite.md |
| D179, the row of this decision | ../architecture/21-decisions.md |
| PEP 570, positional-only parameters | <https://peps.python.org/pep-0570/> |
| PEP 3102, keyword-only arguments | <https://peps.python.org/pep-3102/> |
