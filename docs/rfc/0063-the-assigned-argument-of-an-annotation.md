---
rfc: 0063
title: The assigned argument of an annotation
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

# RFC-0063: The assigned argument of an annotation

## Summary

A Rust attribute passes its arguments in parentheses, as in `#[derive(Debug)]`, or assigns one
argument to its name, as in `#[deprecated = "use g"]`. The model records both forms as a name and a
list of arguments, and the Rust backend writes the parenthesised form back. rustc refuses
`#[deprecated("use g")]`. We propose a field of `symbol.Annotation` that marks the assigned form.
The Rust frontend sets it, the Rust backend writes the form, and the backends of Java, TypeScript
and Go refuse an annotation that has it.

## Motivation

- RFC-0015 states that every attribute that a declaration states is an entry of its annotations,
  and that the Rust backend writes it back.
- The Rust frontend records `#[deprecated = "use g"]` as the name `deprecated` with the one argument
  `"use g"`, which is also the record of `#[deprecated("use g")]`.
- The Rust backend writes `#[deprecated("use g")]`, which rustc refuses as a malformed attribute. The
  same holds for `must_use`, `path`, `export_name`, `link_name`, `link_section` and every tool
  attribute that takes the assigned form.
- A Rust plan that mirrors the attributes of a declaration therefore writes a file that does not
  compile, and no finding reports it.

## Detailed design

### Components

| Component | Change |
|---|---|
| `core/symbol` | `Annotation` gains `Assigned` |
| `core/node` | The binary encoding of an annotation writes `Assigned` after the arguments |
| `core/internal/gen/model` | The generated codec tests give each field of annotations an assigned annotation too |
| `core/internal/state` | `Format` increases from 3 to 4 |
| `lang/textfmt` | `Marked` and `MarkedInline` write an assigned marker with its argument after an equals sign |
| `lang-rust/frontend` | An attribute that assigns its argument is an assigned annotation |
| `lang-rust/backend` | Writes an assigned annotation in the assigned form |
| `lang-java/backend`, `lang-typescript/backend`, `lang-go/backend` | Refuse an assigned annotation |

### Invariants

1. An assigned annotation has exactly one argument. A frontend sets `Assigned` for the assignment
   form of its language alone.
2. A backend whose language has the assignment form writes it. Every other backend refuses an
   assigned annotation, and the render reports the refusal at the declaration.
3. The sealed state of a run before the change is not read. A run that meets it runs cold.

### The model

```go
package symbol

type Annotation struct {
    Name string   `json:"name"`
    Args []string `json:"args,omitzero"`
    // Assigned marks an annotation whose one argument the source assigns
    // to its name, as Rust writes #[deprecated = "use g"], and does not
    // pass in parentheses.
    Assigned bool `json:"assigned,omitzero"`
}
```

The JSON form leaves the field out where it is false, so the emit codec reads every earlier record.
The binary encoding of a node writes the field as one byte after the arguments of each annotation.
The encoding of every generation changes, so `state.Format` becomes 4.

### Writing an assigned annotation

`textfmt.Marked` and `textfmt.MarkedInline` write an assigned marker as its opener, its name, ` = `,
its argument and its closer. The Rust backend passes `#[` and `]`, so it writes
`#[deprecated = "use g"]` on the line before the declaration, and before a parameter on its line.

Java, TypeScript and Go pass the arguments of an annotation in parentheses or after a space alone.
The template helpers of their annotations return an error for an assigned annotation, and the helper
of a parameter list does so for a parameter. The error states the name of the annotation and that
the language does not assign the argument of an annotation.

### Verdicts

| Item | Verdict | Reason | Reverses the verdict |
|---|---|---|---|
| The model records the assigned form of a Rust attribute | in | The backend writes back what the frontend read, and the two forms are different attributes to rustc | none |
| A backend of another language refuses an assigned annotation | in | Java, TypeScript and Go have no assignment form. A silent change of the form would write an annotation that the source did not state | A language with the form |
| A rule that rewrites an assigned annotation of one language into a form of another | out | An annotation has a meaning in its own language alone, and RFC-0015 lifts no annotation into a fact | A mapping of annotations between languages that a design states |

### Failures

| Condition | Report |
|---|---|
| A Java, TypeScript or Go plan writes an assigned annotation | The render refuses the declaration with the error of the helper |
| A run meets a generation of format 3 | The run runs cold, as for every change of the format |

### Migration

1. Add the field, its encoding, the generated codec tests and the format.
2. Write the assigned form in `textfmt`, and set the field in the Rust frontend.
3. Refuse the field in the helpers of Java, TypeScript and Go.

## Alternatives

### The assignment in the name

The Rust frontend could record `#[deprecated = "use g"]` as the name `deprecated = "use g"` without
arguments, and `Marked` would write it back. A rule that reads the annotation `deprecated` would not
find it, and the name would contain the argument.

### The assignment in the argument

The frontend could record the argument as `= "use g"`, and the Rust backend could write an argument
that opens with an equals sign without parentheses. The argument would then not be verbatim, and a
rule would read the equals sign as part of the value.

### The form from the name of the attribute

The Rust backend could write the assigned form for the attributes that rustc defines with it, such
as `deprecated` and `must_use`. A tool attribute or the helper attribute of a macro can take either
form, so the backend would guess for each of them.

## Drawbacks

- An annotation has one more field, and three backends have one more refusal.
- The first run after the change runs cold.

## Open questions

None.

## Unresolved and future work

None.

## References

- [RFC-0015](0015-tree-sitter-frontends-and-dependency-stores.md), the annotations of a declaration.
- [RFC-0011](0011-neutral-emit-and-the-target-lowering-seams.md), the refusal of a construct that a
  target does not spell.
- The Rust Reference, the meta item attribute syntax: `MetaNameValueStr` and `MetaListPaths`.
