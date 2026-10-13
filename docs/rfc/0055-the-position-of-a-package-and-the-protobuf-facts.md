---
rfc: 0055
title: The position of a package and the protobuf facts
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-12
updated: 2026-10-12
discussion: none
supersedes: none
superseded-by: none
produces-adr: ADR-0036
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0055: The position of a package and the protobuf facts

## Summary

No frontend sets the position of a package, so a finding on a package has no position. We propose
that the load places a package without a position at the position of its first file. The previous
version also had four protobuf facts that eidos does not have: the default JSON name of a field,
typed option values, a Go type for one field, and the Go spelling of the well-known types. We
propose the verdict out for the first three and the confirmation that the fourth is intended.

## Motivation

- `GraphBuilder.Package` sets the identity and the path of a package, and no frontend of the Go,
  TypeScript, protobuf, Java and Rust satellites sets a position. A directive on a package that the
  kernel refuses reports at no file and no line.
- The protobuf frontend stamps a stated `json_name` alone. protoc derives a default for every field.
- The frontend stamps each option as text, and no rule types the value.
- An author can rename a field for one language, and cannot choose its Go type.
- The Go spoke spells `google.protobuf.Timestamp` as `time.Time` and `Duration` as `time.Duration`,
  which differs from the types that protobuf-go generates.

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-core/frontend/load` | The splice places a package without a position at the position of its first file |

### Invariants

1. Every package with a file has a position, so a finding on it has a file and a line.

### The position of a package

The splice of the load reads the packages that the units built. Where a package has no position
after the splice, the splice gives it the position of the first file of the package, in the order
of the units and of their files. A frontend that sets the position of a package keeps it. The
position of a file is where its frontend placed it, such as the package clause of a Go file.

### Verdicts

| Capability | Verdict | Reason | Reverses the verdict |
|---|---|---|---|
| The default JSON name of every field | out | No generator reads the JSON name of a protobuf field, the stated one included. A generator derives the default from the name of the field by the rule of protoc where it needs one | A generator that writes or reads the JSON form of a message |
| Typed option values | out | No generator reads the value of an option, and the stamp keeps each option as text in a list, one entry for each option | A generator that reads the value of an option |
| A Go type for one field | out | No generator writes a protobuf field in Go, and a type override is an option of the generator that writes it | A generator of Go messages from protobuf |
| The Go spelling of the well-known types | intended | eidos writes the declarations of its neutral model, and the Go spoke spells a well-known type as the type that Go code uses for it. No generator writes protobuf-go messages, so wire compatibility with protobuf-go does not apply | A generator of protobuf-go messages |

### Failures

| Condition | Report |
|---|---|
| A package without files | None. The package keeps no position |

### Migration

1. Place a package without a position in the splice.

## Alternatives

### A position from each frontend

Each frontend could set the position of the packages that it builds. Every frontend would repeat the
rule, and a frontend that leaves it out would leave a package without a position.

## Drawbacks

- The position of a package depends on the order of its files, which the order of the units
  decides.

## Open questions

None.

## Unresolved and future work

None.

## References

- D112 in [21](../architecture/21-decisions.md), the facts of protobuf.
