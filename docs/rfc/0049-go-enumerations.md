---
rfc: 0049
title: Go enumerations
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

# RFC-0049: Go enumerations

## Summary

The Go frontend promotes a defined type to an enumeration only where its typed constants are in the
file of the type and its target is an ordered basic type. The Go rules project every enumeration in
the identifier form, with the whole name of each variant as its text. They find a zero only at the
value 0, and an out-of-range value only among integers. We propose a promotion per package over
every defined type with typed constants. The projection reads the underlying type of an
enumeration to choose its form and its zero, and to derive its out-of-range value. The constants
of a lowered enumeration get docs that open with their names, and the reference enum plugin reads
the projection. The verdicts refuse one constant group for a lowered enumeration and a print verb
in the projection. They also refuse the constants that another package declares of an
enumeration.

## Motivation

- A constant of an enumeration in another file of its package stays a constant, so the methods
  that a generator writes leave it out. Where every constant of a type is in another file, the type
  stays a defined type, and the kernel refuses `+enum` on it as misplaced.
- A defined type over `bool` or over another defined type does not promote.
- The Go rules return the identifier form for `type Region string`. A String method that a
  generator writes from the projection then returns `RegionUS` where the declaration states
  `"us-east"`. The reference enum plugin reads the exact values itself to write the declared text.
- The text of the variant `StatusActive` is `StatusActive`. The reference enum plugin strips the
  name of the type itself.
- The rules compare an exact value with 0 to find the zero, so a string enumeration has no zero.
- The rules derive an out-of-range value from integer values alone. A string enumeration has none,
  and neither has a float enumeration with fractional values.
- The frontend stamps the exact value 1.5 as `3/2`. `DeriveSamples` writes the stamp as raw Go, and
  Go divides `3/2` as integers, so the sample of a float enumeration is 1 where the variant is 1.5.
- The Go backend copies the doc of a variant to its constant. The constant `phaseOpen` of the
  canonical fixture has the doc `open admits writes.`

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-lang-go/frontend` | The promotion runs per package over every defined type with typed constants |
| `eidos-lang-go/rules` | `EnumOf`, `ZeroValue`, `DeriveSamples` and `LiteralFor` read the underlying builtin and the exact values of an enumeration |
| `eidos-lang-go/backend` | The doc of a constant of a lowered enumeration opens with the name of the constant |
| `eidos-reference/enum` | The plugin takes the form and the texts from `EnumOf`, and reports a variant without a text |

### Invariants

1. A constant whose declared type is a defined type of its package is a variant of the enumeration
   of that type wherever the go command builds the file of the constant with the file of the type.
2. The text of each variant of a value-formed enumeration is its declared value.
3. `EnumInfo.OutOfRange` is a value of the underlying type that no variant has.
4. `ZeroValue` and `DeriveSamples` return Go values of the enumeration.

### Promotion

The frontend promotes after it has lowered every file of a package. A defined type promotes where
it has no type parameters, its target is a name without type arguments, and a constant of the
package has it as its declared type. Go gives a constant a boolean, a numeric or a string type, so
the underlying type of such a type is one of these, whatever name its target has.

- The constants become the variants in the order of the files of the unit, and within a file in
  the order of their declarations.
- A constant in a test file becomes a variant only of a type that a test file declares. The go
  command builds a file outside the tests without the test files.
- A constant of the type in another package stays a constant of that package.
- The methods of the type fold in from every file of the package.

The constant evaluation and the `golang.underlyingKind` stamps run after the promotion.

### The underlying builtin

The rules resolve the underlying type of an enumeration to a builtin through the view, and read
its `rules.TypeShape`. A reference to a defined type resolves to its target, and a reference to an
enumeration resolves to its underlying type. The walk stops at the depth bound of the derivation,
and a type that does not resolve to a builtin has the zero `rules.TypeShape`. `LiteralFor` types a
literal through the same walk, so a literal for an enumeration has the type of its underlying
builtin.

Where the underlying type does not resolve to a builtin, `EnumOf` reads its kind from the evaluated
values. Go gives a constant the kind of its type, so a quoted value makes a string, `true` or
`false` makes a bool, and any other value makes a number of an unknown width. An enumeration over a
number of an unknown width has no `OutOfRange`.

The rules read an exact value as a rational number, which reads every form that the frontend
stamps: an integer, a fraction such as `3/2`, a quoted string and a truth value.

### The projection

The form of an enumeration over a string is `EnumValue`, and every other enumeration has the form
`EnumIdentifier`.

- The text of a variant of a value-formed enumeration is its unquoted exact value. A variant whose
  value the frontend did not evaluate has the zero `emit.Value` as its text.
- The text of a variant of an identifier-formed enumeration is its name without the name of the
  type. Where the name starts with the name of the type and the rest starts with an upper-case
  letter, a digit or an underscore, the text is the rest without a leading underscore. Where that
  text is empty, or the name does not start that way, the text is the whole name. `StatusActive`
  has the text `Active`, `Colorless` of `Color` keeps its name, and `Phase_UNSPECIFIED` has the
  text `UNSPECIFIED`.

| Underlying | `Zero` is the first variant with | `OutOfRange` |
|---|---|---|
| string | `""` | `""` where no variant has it, and otherwise a run of `?` one byte longer than the longest text |
| bool | `false` | the truth value that no variant has |
| integer | 0 | one past the largest value where the type holds it, and otherwise one below the smallest value |
| float | 0 | the largest value plus one at the precision of the type, or the next value of that precision where the sum rounds back to the largest. Where that value is infinite, the same rule applies below the smallest value |

- An integer or float set that reaches both ends of its type has no `OutOfRange`.
- The rules derive `OutOfRange` only where the frontend evaluated every variant. A variant without
  an evaluated value can have any value of the type.
- `Duplicate` keeps its meaning: the first variant whose exact value a later variant repeats.
- `Foreign` keeps its meaning, so it is empty for Go. A variant is a constant of the package of its
  type.
- `ZeroValue` of an enumeration converts the zero of its underlying type.
- `DeriveSamples` of an enumeration converts the exact values of its first two distinct variants,
  each typed as a literal of the underlying type. Where the frontend evaluated fewer than two
  distinct values, it converts the pair of the underlying type.

A generator converts an undeclared value through `Enum.Underlying` and lifts that type with
`rules.EmitRef`, which adds its import. The projection therefore does not add a field for the
conversion.

### The lowering

The Go backend lowers an enumeration into a defined type and one constant per variant. Where the
first word of the doc of a variant is the name of the variant, the doc of its constant opens with
the name of the constant instead. The variant `open` of the enumeration `phase`
has the constant `phaseOpen` with the doc `phaseOpen admits writes.`

### The reference enum plugin

The plugin asserts `rules.EnumRules` on the rules of the language of its match, and takes the form
and the texts from `EnumOf`. It reports these findings and writes nothing for the enumeration:

| Code | Condition |
|---|---|
| `NoProjection` | The rules of the language of the enumeration do not implement `rules.EnumRules` |
| `UnknownText` | A variant has no text, at the variant |

The parser check of the plugin takes a run of `?` one byte longer than the longest text as its
unknown text. The plugin derives that text from the texts of the projection.

### Verdicts out

| Row | Capability | Reason | Reverses the verdict |
|---|---|---|---|
| W5.G34 | One constant group for a lowered enumeration | The settle reads each constant of a lowered enumeration as a declaration of its file. It checks the name of each constant for a collision and respells the references to it. A group needs an emit construct whose members the settle reads, and no target besides Go declares one | A second target that declares a group of declarations in one construct |
| W5.G8 | The print verb of an enumeration | A verb is a spelling of the fmt package of Go, which a template chooses from the underlying type. The reference enum prints an undeclared value through a local type with the underlying type and `%v`, which formats every basic type | A generator that formats an undeclared value in two languages from one projection |
| go-projections#57 | The constants that another package declares of an enumeration | A search for them reads every constant of the graph for each enumeration, and the store indexes no declaration by the target of its type | A store index of declarations by the target of their type |

### Failures

| Condition | Report |
|---|---|
| A variant of a value-formed enumeration without an evaluated value | `Text` is zero, and the reference enum reports `UnknownText` as an Error |
| A variant without an evaluated value | `OutOfRange` is zero |
| An enumeration under `enum` whose language has no `EnumRules` | The reference enum reports `NoProjection` as an Error |

### Migration

1. The frontend promotes per package.
2. The rules read the underlying builtin and the exact values.
3. The backend opens the doc of each constant with its name.
4. The reference enum plugin reads the projection.

The golden files of the Go backend change where the doc of a constant opened with the name of its
variant.

## Alternatives

### A promotion in the annotate sibling

The annotate sibling could promote over the sealed graph, where it sees every file of a package. An
annotator stamps facts and does not change declarations, and a promotion replaces a defined type
and its constants with an enumeration.

### The form from the values

The TypeScript rules derive the form from the values of the members. The Go rules derive it from
the underlying type, because a string type makes every value a string, the values that the frontend
did not evaluate included.

### A fixed marker as the unknown text

A marker such as `__unknown__` can collide with a declared text. A run of `?` one byte longer than
the longest text differs from every text.

## Drawbacks

- An enumeration can have variants in more than one file, so an edit of any file of its package
  changes the enumeration.
- A set of variants with an unevaluated value has no out-of-range value, so a generator writes no
  check of an undeclared value for it.

## Open questions

None.

## Unresolved and future work

None.

## References

- The improvement plan rows W5.G1, W5.G5 to W5.G9, W5.G34 and W9.34.
- [RFC-0036](0036-the-facts-that-the-model-drops.md), which adds the underlying type of an
  enumeration.
- [RFC-0027](0027-the-subject-kinds-of-a-directive.md), which refuses a directive on a declaration
  of another kind.
