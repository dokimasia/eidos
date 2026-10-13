---
rfc: 0048
title: Unique names
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-12
updated: 2026-10-12
discussion: none
supersedes: none
superseded-by: none
produces-adr: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0048: Unique names

## Summary

The Go satellite and the doubles of the reference ensemble each choose a name that no other name of a
method uses, with two rules and two candidate orders. The double package gives an unnamed parameter
the name of its position without a check, so the name can repeat the name of another parameter. We
propose one
function in the naming package that returns the first candidate that a set of names does not
contain, and the rules of both callers on it. We refuse the case styles `ScreamingKebab`, `Dot` and
`Title`, and a repair of free text into an identifier.

## Motivation

- `PointerReceiver` of the Go satellite tries the first letter of the host, its first two letters,
  `recv`, and then `recv0`, `recv1` and upward. The doubles try the first letter, or `d` for a host
  that does not start with a letter, and then the letter with `0`, `1` and upward.
- The double package gives the parameter at position i without a name the name `a<i>`, and adds the
  names to the set as it goes. A method `Put(string, a1 int)` gets two parameters named `a1`, which Go
  refuses.
- The naming package lacks the case styles `ScreamingKebab`, `Dot` and `Title` and a repair of free
  text into an identifier.

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-lang/naming` | `Unique` |
| `eidos-lang-go` | the receiver rule of `PointerReceiver` on `Unique` |
| `eidos-reference/internal/double` | the receiver and the positional names on `Unique`, with the named parameters taken first |

### Invariants

1. `Unique` returns a name that the set of taken names does not contain.
2. A name that a method declares is never the name of another parameter, result or receiver of the
   method, after the Go satellite or the doubles complete it.

### The function

```go
// Unique returns the first of candidates that taken does not contain.
// Where taken contains every candidate, it returns the last candidate
// followed by the smallest number from 0 that gives a name that taken
// does not contain. It allocates only for a name with a number.
//
// Unique panics without a candidate, which is a defect of the caller.
func Unique(taken map[string]bool, candidates ...string) string
```

### The callers

- `PointerReceiver` keeps its order. It returns the first letter of the host where no name takes it,
  without an allocation, and otherwise calls `Unique` with the first two letters and `recv`.
- The double package takes the names of every named parameter and result first. It then gives each
  unnamed parameter the name that `Unique` returns over the positional name, and the receiver the
  name that `Unique` returns over its letter.

The candidate orders of both callers stay as they are, so a generated file changes only where two of
its names collided.

### Verdicts out

| Capability | Reason | Reverses the verdict |
|---|---|---|
| The case styles `ScreamingKebab`, `Dot` and `Title` | No target spells a name in these styles, and no plugin of the ensemble writes one | A target or a plugin that spells a name in one of the styles |
| A repair of free text into an identifier | A backend refuses a name outside the identifier shape with a finding, and no plugin derives an identifier from free text | A plugin that derives identifiers from free text, such as the variants of an enumeration from its string values |

### Failures

| Condition | Report |
|---|---|
| `Unique` without a candidate | a panic, because the caller passes the candidates as constants |

### Migration

1. Add `Unique`.
2. Move the receiver rule of the Go satellite and the names of the doubles onto it.

## Alternatives

### A taken set as a function

`Unique` could take a function that reports whether a name is taken. Both callers keep a map of the
names of a method, and a function would need a closure at each call.

### One candidate order for both callers

The double package could use the order of the Go satellite. The canary ring compares the generated
doubles across releases, so a new order would change the receiver of every double.

## Drawbacks

- The naming package gains a function that the satellites and the plugins call, so its signature
  is part of the API of the module.

## Open questions

None.

## Unresolved and future work

None.

## References

None.
