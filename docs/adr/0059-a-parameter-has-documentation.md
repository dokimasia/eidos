---
adr: 0059
title: A parameter has documentation
status: Accepted
date: 2026-10-19
supersedes: none
superseded-by: none
rfc: RFC-0068
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0059: A parameter has documentation

## Status

Accepted

## Context

GraphQL documents each argument of a field and each variable of an operation with a description of
its own. `Param` had no documentation: the languages of the hub document a parameter in the
documentation of its callable, as a Javadoc `@param` tag does.

## Decision

We will give `Param` the field `Doc`, without a fact, because GraphQL documents each argument and
each variable with a description of its own.

- The GraphQL frontend loads the description of an argument and of a variable into it, and the
  GraphQL backend writes it as the description.
- A frontend of a language that documents a parameter in the documentation of its callable leaves it
  empty.
- A generator into such a language writes the documentation of a parameter into the documentation of
  its callable.

## Alternatives Considered

### Argument descriptions as markers

The description could be a marker on the parameter, without a change of the model. A description is
documentation, which a generator into another language writes into the documentation of its
callable, and a marker would leave it among the directives of the parameter.

## Consequences

**Positive:**

- A schema and an operation keep the descriptions of their arguments and variables at load.

**Negative:**

- The schema gains a field, so every sealed state runs cold once.
- The Go, TypeScript, Java and Rust backends do not write the documentation of a parameter, and no
  fact makes them state a verdict for it.

## References

| What | Where |
|---|---|
| RFC-0068, the design of this decision | ../rfc/0068-the-graphql-satellite.md |
| D169, the row of this decision | ../architecture/21-decisions.md |
