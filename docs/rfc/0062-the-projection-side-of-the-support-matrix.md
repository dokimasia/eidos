---
rfc: 0062
title: The projection side of the support matrix
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-13
updated: 2026-10-13
discussion: none
supersedes: none
superseded-by: none
produces-adr: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0062: The projection side of the support matrix

## Summary

The support matrix states what each language reads, what each target renders, how each target
spells each canonical form and which languages lower markers. It does not state which optional
capabilities of the projection vocabulary the rules of each language implement. We propose a fifth
table of the matrix: a row for each optional capability, a column for each language, and a cell that
the matrix finds by asserting the capability on the rules of the language. We also decide against
a plugin that exists only to call `PropertyRules`, `ConstructRules`, `PromotionRules` or
`EqualityRules`.

## Motivation

- The rules of Go implement nine optional capabilities, the rules of TypeScript six and the rules of
  protobuf two. No rules package implements `ThrowsRules` or `OwnershipRules`.
- No production code asserts `PropertyRules`, `ConstructRules`, `PromotionRules` or
  `EqualityRules`. The Go annotator calls the method `Comparable` of the Go rules directly.
- The table of the optional capabilities in the projection document lists the languages whose
  constructs motivate each capability, and leaves `WellKnownRules` and `SatisfactionRules` out. No
  document states which languages implement a capability.
- A proposal gives every implemented capability a production caller in the reference ensemble or in
  the conformance matrix. RFC-0061 decides the rules of Java and Rust.

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-conformance/matrix` | `Markdown` writes a fifth table, the projection side, from the rules package of the SDK |
| `docs/architecture/03-projection.md` | The contract block and the table of the optional capabilities list `WellKnownRules` and `SatisfactionRules`, and the section states that the support matrix lists the languages that implement each capability |

### Invariants

1. The projection side has one row for each optional capability of package rules: each exported
   interface whose name ends in `Rules`, other than `SourceRules`. The test of the matrix derives
   that list from the source of the package.
2. The cell of a language states `implements` where the rules of the corpus of the language satisfy
   the capability, `absent` where they do not, and `none` for a corpus without rules.

### The projection side

A consumer finds an optional capability by asserting it on the rules of a language, and the matrix
finds it the same way. For each capability, the matrix asserts the interface on the `Rules` of the
corpus of each entry. The table follows the read side and has a column for each entry in the order of
the entries, so Java and Rust have columns with `none` in each cell. The section opens with a sentence
that states what a cell contains, as the other sections do.

The matrix states the capabilities in the order of their declaration in package rules. Its test
parses the files of package rules, lists each exported interface whose name ends in `Rules` other
than `SourceRules`, and compares the list with the rows of the table. A capability that package rules
declares and the matrix does not list fails the test with its name.

### Verdicts

| Item | Verdict | Reason | Reverses the verdict |
|---|---|---|---|
| The matrix states the capabilities that the rules of each language implement | in | A consumer asserts a capability, and the matrix asserts each capability of each language. The table states the support of each language from the code | none |
| A plugin that calls `PropertyRules`, `ConstructRules`, `PromotionRules` or `EqualityRules` | out | No plugin of the reference ensemble needs one of the four. A plugin built to call one has no consumer, and the tests of the rules of each language check each implementation | A plugin that needs one of the four capabilities |
| The Go annotator calls `Comparable` of the Go rules directly | in as built | The annotator stamps a fact of the Go satellite, and the Go rules compute it. A neutral consumer asserts `EqualityRules` instead | A neutral consumer that reads the comparability of a type of Go |

### Failures

| Condition | Report |
|---|---|
| Package rules declares an optional capability that the matrix does not list | The test of the matrix fails and names the capability |
| The rules of a language implement a capability in part | None. A Go type satisfies an interface only with every method, so the assertion fails and the cell states `absent` |

### Migration

1. Add the projection side to `Markdown`, and its rows to the test of the matrix.
2. Add the two capabilities to the table of the projection document.

## Alternatives

### A plugin for each capability

The reference ensemble could gain a plugin that calls each of the four capabilities, such as a
builder for a struct of another package through `PromotionRules`. Each plugin would exist for its
call. The builder of the ensemble writes its file into the package of its struct, and RFC-0060 keeps
that scope.

### A list of capabilities without a check

The matrix could list the capabilities by hand, and its test could compare the table with the same
list. A capability that a later change adds to package rules would then have no row, and nothing
would report it.

## Drawbacks

- The matrix imports the rules package of the SDK and gets one more table.
- The test of the matrix reads the source of package rules.

## Open questions

None.

## Unresolved and future work

None.

## References

- [RFC-0060](0060-the-reference-ensemble-after-its-review.md), the scope of the builder.
- [RFC-0061](0061-the-declared-name-of-an-imported-go-package.md), the rules of Java and Rust.
- [03](../architecture/03-projection.md), the optional capabilities.
