---
adr: 0010
title: An export entry has a five-part key and no signature
status: Accepted
date: 2026-10-02
supersedes: none
superseded-by: none
rfc: RFC-0019
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0010: An export entry has a five-part key and no signature

## Status

Accepted

## Context

A dependent plan reads from a producing plan's export the names that the
producer's settle gave its declarations. It never applies the producing
target's naming rules a second time, so it never misses a respell, a
reverted collision or a name override. It finds each declaration by a key
that it builds before the producing plan runs.

The export was first specified with a key of three parts: the origin, the
plugin and the emitted name. Each entry was to record a kind, a canonical
signature and the settled spelling. The kernel conflicts with that
specification:

- Three parts do not identify a generated declaration. One generator can
  emit one name into two families, such as a stub in its primary family and
  one in its test family. A generator that emits a stub and a mock of one
  interface gives both a method from the same origin, under the same name,
  on two receivers.
- The kernel cannot compute a generated declaration's signature. It
  projects source type references into canonical shapes through a source
  language's rules. A generated declaration's types are emit references,
  which no projection reads, and the kernel has no lowering from shapes
  back to a target.

A dependent in the producer's language also needs the import path of the
package that declares an exported name. The layout routes each declaration
to a file after the settle, and the target derives the file's package from
its path.

## Decision

We will key an exported declaration by its origin, plugin, family, host and
emitted name, and record no signature, because a dependent can build that
key before the producing plan runs and the kernel cannot compute a
generated declaration's signature.

- The origin is the source declaration the generated one derives from. The
  plugin is the plugin whose unit declared it, and the family is the one
  the unit was emitted into.
- The host is the emitted name of the declaration a member is declared in,
  or of the type a method's receiver names. It is empty for any other
  declaration at file level.
- Both names are the ones the plugin emitted, before the settle respelled
  them.
- Each entry records the declaration's kind, the spelling the settle gave
  it, the package its file declares and the file's workspace-relative path.
- `Find` returns every entry under one key, in file order.

## Alternatives Considered

### Keep the three-part key

Under this alternative a dependent builds a key from the origin, the plugin
and the emitted name alone, and needs less knowledge of how the producing
plugin routes its output. It lost because two declarations of one generator
then share a key: a stub in the primary family and one in the test family,
and one method of a stub and of a mock of the same interface. No part of
the key tells them apart.

### Record a canonical signature on each entry

With a signature in canonical type shapes on each entry, a dependent in
another target language reads the generated declaration's types from the
export alone. It lost because the kernel has no way to compute that
signature. A generated declaration's types are emit references, and the
kernel's projections read only source type references.

### List the settled store before the layout

Listing the settled declarations before the layout routes them gives each
entry its key, kind and spelling, and leaves out the file and the package.
It lost because a
dependent in the producer's language qualifies each reference by the
import path of the declaration's package. The target derives that package
from the path of the file the layout routes the declaration to.

## Consequences

**Positive:**

- A dependent finds a declaration under the names the producing plugin
  emits. It reads the declaration's name in the producing target from the
  entry, and does not apply that target's naming rules.
- Every declaration of a rendered file has an entry, members and methods
  included. Two entries share a key only where one generator emitted one
  name from one origin into one family more than once.
- A dependent in the producer's language qualifies a reference with the
  entry's package.

**Negative:**

- A dependent builds a key of five parts. A key with a wrong family or host
  returns an empty result from `Find`, the same result as a key for a
  declaration the producer did not emit.
- A binding generator in another target language reads the origin's shape
  through its own reader, and that reader needs the origin's package in its
  plan's scope.
- The settle respells names in place, so it records the emitted name of
  every declaration whose name it changes. A settle that changes no name
  records nothing.
- An entry is 280 bytes on a 64-bit platform. An export of 20,000
  declarations is about 5.6 MB.

**Neutral:**

- The run builds an export only for a plan that a dependent plan or a
  workspace check reads.
- `Find` costs one binary search and allocates nothing.

## References

| What | Where |
|---|---|
| D34, the export key this decision replaces | ../architecture/21-decisions.md |
| D19, plan exports with declared dependencies | ../architecture/21-decisions.md |
