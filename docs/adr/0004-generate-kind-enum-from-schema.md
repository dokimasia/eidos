---
adr: 0004
title: Generate the Kind enum from the schema
status: Accepted
date: 2026-08-30
supersedes: none
superseded-by: none
---

# ADR-0004: Generate the Kind enum from the schema

## Status

Accepted

## Context

Two statements in the architecture specification pull against each
other. The kernel map lists "one Kind enum" among the hand-written
contents of `symbol/`. The symbol model's own law says that adding a
kind is "a schema edit and nothing else". Both cannot hold: a
hand-written enum makes every new kind a two-place edit, the schema
struct plus the constant, and the two copies can drift until
something notices.

The constants must match the schema structs one to one, because every
generated `Kind()` method returns one and every consumer switches on
them. RFC-0002 specifies the generator this decision relies on.

## Decision

We will generate the `Kind` constants and their `String` method into
`symbol/kind.gen.go` from `symbol/schema`, because the enum's content
is exactly the schema's struct list and a second hand-written copy
breaks the schema-edit-only law. The `Kind` type itself stays
hand-written in `symbol/`.

## Alternatives Considered

### Hand-write the enum and let the mirror guard check parity

This is the letter of the kernel map: `symbol/` stays fully
hand-written, and a generated test asserts the constants match the
schema. It lost because it keeps the two-place edit and adds a guard
whose only job is catching the second place being forgotten. The
guard proves the copies match; generation removes the second copy.

### Generate the constants, use stringer for String

`golang.org/x/tools/cmd/stringer` is the conventional tool for
`String()` on const blocks. It lost because the const block is
already coming out of our own generator, so `String` is one more
stanza in a template that already runs. Wiring stringer in costs
either a `tool` directive, which puts `x/tools` into the
zero-dependency kernel's `go.mod` that consumers audit, or an
unpinned `go run ...@version` fetch at generate time. The full
argument is in RFC-0002.

## Consequences

**Positive:**

- Adding a kind is one edit and one regeneration, which is what the
  symbol model's law promises.
- `String` can never disagree with the constants, because both render
  from the same list in the same pass.

**Negative:**

- `symbol/` is no longer purely hand-written: one generated file
  lives in the vocabulary package, and a reader has to know
  `kind.gen.go` comes from the schema.
- The generator becomes load-bearing for the kernel's most central
  type. The mirror guard covers drift, but a generator bug now
  reaches further than the models.

**Neutral:**

- Constant values follow schema declaration order, so reordering the
  schema renumbers them. Nothing durable stores the numeric values:
  the JSON codec encodes kind names, and the sealed state carries its
  own format version.

## References

- [01-repos-and-kernel.md](../architecture/01-repos-and-kernel.md),
  the kernel map that lists the enum as hand-written
- [02-symbol-model.md](../architecture/02-symbol-model.md), the
  schema-edit-only law
- [RFC-0002](../rfc/0002-model-generator.md), the model generator
