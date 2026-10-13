---
rfc: 0052
title: Go parameters as subjects and the legacy build line
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-12
updated: 2026-10-12
discussion: none
supersedes: none
superseded-by: none
produces-adr: ADR-0035
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0052: Go parameters as subjects and the legacy build line

## Summary

The Go frontend refuses a carrier on a parameter or a result under GOLANG-0003. The model makes both
of them subjects, and the kernel key `gen.sample` admits both. The frontend also reads no
`// +build` line, so a file that the go command leaves out of every build loads its declarations
into the package. We propose two changes. A carrier on a parameter or a result attaches to it. A
`// +build` line constrains a file without a `//go:build` line, as go/build reads it. The text of a
leading comment of a parameter gets the verdict out.

## Motivation

- The frontend refuses the carrier on `id` in the following function, although the schema of a
  parameter states that an authored value attaches to it in a language whose comments can address
  it.

  ```go
  func Load(
      ctx context.Context,
      id int, // +sample value=7
  ) (*Row, error)
  ```

- A generator program in a library directory, with `// +build ignore` and `package main`, loads
  into the package of the directory. The go command leaves the file out, and the frontend reports
  two package names and loads the declarations of the program.
- A comment on the line above a parameter in a list with one parameter per line is dropped. The
  trailing comment of a parameter is its `Comment`.

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-lang-go/frontend` | A carrier on a parameter or a result attaches to it, and a `// +build` line constrains a file |

### Invariants

1. The frontend loads a file where the go command builds it under the tag set of the load.
2. A carrier on a declaration that has an identity attaches to the declaration, or the kernel refuses
   it as misplaced.

### Carriers on parameters and results

The frontend attaches the carriers of the doc and of the trailing comment of a parameter field to
each parameter that the field binds, and so for a result field. The trailing comment of a field is
shared by every name of the field, and so are its carriers. The kernel validates the directive as it
validates a directive on a field, so a directive whose rules take no parameter is misplaced. This
decides D-6 of the improvement plan.

The parameters of a function type inside a type expression have no identity, and neither has the
receiver of a method, so a carrier on either remains refused under GOLANG-0003.

### The legacy build line

go/build of Go 1.27 reads a `// +build` line where a file has no `//go:build` line. The frontend
reads it in the same way:

- A `// +build` line in a comment group before the package clause constrains the file, except in
  the doc of the package, which no blank line separates from the clause.
- Every such line of a file must hold. A line holds where one of its options holds, and an option
  holds where each of its tags holds.
- `go/build/constraint.Parse` reads a line, and a line that does not parse constrains nothing, as in
  go/build.
- The stamp `golang.constraint` has the line that excludes the file.

A `//go:build` line decides alone where a file has one.

### Verdicts out

| Row | Capability | Reason | Reverses the verdict |
|---|---|---|---|
| W5.G3 | The text of a leading comment of a parameter or a result | Go documents a parameter in the doc of its function, and godoc renders no comment inside a parameter list. The model has a trailing comment for a parameter. The frontend drops comment text that has no place in the model, as it drops a comment between declarations | A target that renders the doc of one parameter, such as a parameter tag that a Java or a TypeScript backend writes from the model |

### Failures

| Condition | Report |
|---|---|
| A carrier on a parameter of a function type in a type expression, or on a receiver | GOLANG-0003, as before |
| A directive whose rules take no parameter, on a parameter | The finding of the kernel for a misplaced directive |
| A `// +build` line that does not parse | None. The line constrains nothing |

### Migration

1. Attach the carriers of a parameter and a result.
2. Read the `// +build` lines of a file without a `//go:build` line.
3. Move the legacy line out of the stated refusals of the package documentation, and add the text
   of a leading comment of a parameter to them.

## Alternatives

### A doc for a parameter

The model could add a doc to a parameter and a result. Every backend would then render or refuse it,
and Go has no place for it except a comment in a list with one parameter per line, which godoc does
not show.

### A refusal for a field with two or more names

The frontend could refuse a carrier on a field such as `a, b int`, because the carrier cannot choose
one name. The trailing comment of such a field belongs to every name, and a carrier follows the
comment.

## Drawbacks

- A carrier on a field with two or more names stamps each parameter of the field.

## Open questions

None.

## Unresolved and future work

None.

## References

- The improvement plan rows W5.G2, W5.G3 and W5.G4, and its decision D-6.
- [RFC-0027](0027-the-subject-kinds-of-a-directive.md), the subject kinds of a directive.
- The build constraints of go/build: <https://pkg.go.dev/go/build#hdr-Build_Constraints>.
