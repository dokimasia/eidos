---
rfc: 0053
title: Helpers without a consumer
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

# RFC-0053: Helpers without a consumer

## Summary

The comparison with the previous version lists nine helpers of the Go and the TypeScript satellites
that eidos does not have. No plugin, template or rules package of eidos reads any of them. We propose
the verdict out for each, with the consumer that would reverse it.

## Motivation

- The improvement plan lists each helper as a capability without a counterpart.
- A helper without a consumer has no test that a consumer states, and a later consumer can need
  another form of it.

## Detailed design

### Verdicts out

| Row | Capability | Reason | Reverses the verdict |
|---|---|---|---|
| W5.G15 | The rules of the tag of `encoding/json`: the serialized name, the exclusion and the options of a field | No generator reads the JSON form of a field. `TagRules.Tag` returns the text under one key, from which a generator reads the parts that it needs | A generator that writes or checks the JSON form of a struct |
| W5.G20 | The direction of a channel as a structured value | The frontend keeps the spelling of a reference, and the Go backend reads the direction from it. No generator reads the direction | A generator that writes a channel of another direction than its source |
| W5.G22 | An enumeration of every entry of a struct tag | The one consumer of tags, the reference plugin defaults, reads one key | A generator that copies the tags of a field |
| W5.G23 | Doc helpers: the summary, the deprecation, the opening of a godoc comment and wrapping at 72 columns | A backend writes the doc lines that a plugin states, and no plugin reads the summary or the deprecation of a source doc | A generator that copies the summary or the deprecation of a declaration |
| W5.G30 | A raw string literal | The Go backend quotes a string through `strconv.Quote`, which Go reads back to the same text. A raw string differs in its spelling alone | A generated literal whose quoted spelling a reader cannot read, such as a regular expression |
| W5.G38 | A rune literal kind | Go reads a rune literal as its integer value, and a rune type takes an integer literal | A target whose character type takes no integer literal |
| W5.G39 | The bounds and the bit size of a number type in `eidos-lang/numeric` | `numeric.Int` and `numeric.Float` bound a value, and the one other range, the value that no variant of a Go enumeration has, is int64 arithmetic in the enumeration rules | A second rules package that needs the range of a type outside `Int` and `Float` |
| W5.T9 | A helper for a union of string literal types, and `never` for an empty one | No TypeScript template writes such a union | A generator that writes a union of literal types |
| W5.T13 | Vocabulary helpers that dispatch a nested node, quote text, rewrite an identifier and indent a block | No TypeScript template needs one | A template that needs one |

## Alternatives

### The helpers of the previous version

eidos could carry each helper of the previous version. Each would be public API without a consumer,
and the release would fix its form before a consumer states what it needs.

## Drawbacks

- A consumer that needs a helper waits for its record and its code.

## Open questions

None.

## Unresolved and future work

None.

## References

- The improvement plan rows W5.G15, W5.G20, W5.G22, W5.G23, W5.G30, W5.G38, W5.G39, W5.T9 and
  W5.T13.
