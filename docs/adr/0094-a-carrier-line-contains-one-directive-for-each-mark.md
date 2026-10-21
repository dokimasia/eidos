---
adr: 0094
title: A carrier line contains one directive for each mark
status: Accepted
date: 2026-10-21
supersedes: none
superseded-by: none
rfc: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0094: A carrier line contains one directive for each mark

## Status

Accepted

## Context

A carrier line in a comment opens with a mark of the brand. The text after the mark was the payload
of one directive. The grammar reads a payload as a name followed by arguments, and a bare argument is
positional. A callable with three mixins needed three directive lines.

- In a corpus of fixtures for a generator of test doubles, 75 callables of the Go corpus have a
  mixin. Four of them have two or three mixins. Their directives take 10 lines.
- A bare value may contain `+`, `-` and `:`, so a second mark on a line parsed as a positional
  argument. Validation refused it under `ExtraPositional`. In the place of a variant, validation
  refused it under `UnknownVariant`.
- Issue 63 asked for a list of variants on one line. In `mixin idempotent concurrent`, each word
  after the name is a variant. The issue also asked for each keyed param to apply to every variant of
  the line that declares its key.
- The keys `read`, `version`, `fn`, `observe`, `sentinel`, `write`, `axis`, `close`, `duration` and
  `unsafe` belong to more than one mixin of the catalog. Of these, `duration`, `fn`, `sentinel` and
  `write` mean different things in different mixins.

## Decision

We will read a carrier line as one or more directives. Each directive opens with a mark of the
brand.

- A later directive opens with `acme:`, `+acme:` or `-acme:` after a blank. A letter must follow
  the mark. The start of a line has the same rule.
- The cut reads the arguments of the line at the top level of the grammar. A quoted value or a list
  is one argument, so a mark inside it is text. An author quotes a positional value that begins with
  a mark.
- Each directive has its own arguments and its own polarity. A key belongs to the directive that it
  follows. `-acme:` negates one directive of the line.
- The first directive keeps the position of the carrier. A later directive has the line of its mark.
  Its column is the column of the comment advanced by the bytes before its mark on that line.
- The store orders the instances of a subject by position, so validation numbers the instances of
  one line in line order. A finding points at its own directive.
- The cut runs on the payload after the join of the continuation lines, so a later directive can
  start on a continuation line.
- Every directive of a line has the tool-directive shape of the line.
- `directive.Cut` cuts a payload before the first argument that opens another directive.
  `directive.Locate` maps an offset of a joined payload to its line.
- Each frontend that reads its carriers through the unit reads the form, because
  `SourceUnit.Comment` returns one carrier for each directive.
- Decorators, attributes and the other markers of the syntax of a language do not change.

## Alternatives Considered

### A list of variants on one line

A schema opts in to the form. `mixin idempotent concurrent side-effect observe=List` is then one
instance of each variant. Each keyed param applies to every variant of the line that declares its
key.

- On `mixin timeout ttl duration=5s`, the timeout also becomes the lifetime of every entry.
- On `mixin order-after validates fn=Check`, the validator also fills the required predecessor of
  `order-after`.
- A refusal of a key that two variants declare keeps `scope partition` on two lines. The form also
  applies to the variants of one schema alone.

### Directives without a mark after the first

Only a lookup in the registry finds that `mixin` opens a directive in
`//acme:mixin idempotent mixin concurrent`, because a bare word after a directive is a positional
argument. A frontend parses its carriers without the registry.

### A separator between directives

A semicolon or a comma separates the directives of a line. The grammar reads a semicolon as a byte
of a bare value, so an existing value would change its meaning. A comma separates the elements of a
list. The mark of the brand already opens a directive at the start of a line.

### One position for every directive of a line

Every directive of a line has the position of the carrier. The store orders the instances of a
subject by position, then by name and arguments. The instances of one line would then number by
name and not by their order on the line.

## Consequences

**Positive:**

- Each of the four callables of the corpus fits on one line. Each param belongs to one directive.
- The form applies to the directives of the kernel, of the targets and of the plugins.
- Only text that validation refused changes its meaning. The one exception is a positional value
  that begins with a mark.

**Negative:**

- A positional value that begins with a mark of the brand opens a directive. Its author quotes such
  a value.
- A declaration can have its directives on one line or on a line each. A text search for one
  layout misses the other.
- A block comment keeps the indentation of its later lines. The column of a later directive on such
  a line counts that indentation from the column of the comment.

## References

| What | Where |
|---|---|
| D212, the row of this decision | ../architecture/21-decisions.md |
| The carrier, the grammar and the schema of a directive | ../architecture/05-directives.md |
| RFC-0005, the grammar of a payload | ../rfc/0005-directive-grammar.md |
| RFC-0022, which keeps one variant for each instance | ../rfc/0022-the-shape-catalog.md |
| Issue 63, several mixins on one directive line | <https://github.com/dokimasia/eidos/issues/63> |
