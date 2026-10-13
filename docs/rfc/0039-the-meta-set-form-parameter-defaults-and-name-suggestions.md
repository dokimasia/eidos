---
rfc: 0039
title: The meta set form, parameter defaults and name suggestions
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

# RFC-0039: The meta set form, parameter defaults and name suggestions

## Summary

The kernel `meta` directive drops a fact, and the metadata document describes a form that sets
one. A directive parameter has no default, so a plugin hard-codes the default in its handler.
A misspelled directive name reports that no schema claims it, without the name that the author
probably meant. We propose three additions to the directive package:

- `meta set=<key> value=<value>` and `meta set=<key> values=[<a>, <b>]`, which write a fact at
  directive authority.
- `ParamSpec.Default`, which validation types and applies where an instance omits the parameter.
- The nearest registered name in the finding of an unclaimed name.

## Motivation

### Setting a fact

The metadata document shows `meta java.nullability=nonnull`, an author's correction of a
frontend's stamp at directive authority. The grammar reads a parameter key as one identifier, so it
stops at the dot of `java.nullability`. The kernel `meta` schema has the parameter `drop` alone. A
parameter key remains one identifier, because a key with a dot would need a second grammar for keys.

### A parameter default

The `enum` plugin documents that its parameter `methods` defaults to true, and its handler reads
the parameter and falls back to true itself. A tool that lists the schema cannot show the default,
and two handlers of one parameter can disagree about it.

### A suggestion

`UnclaimedName` reports `+reference:buidler names no registered schema`. The author has to list
the schemas to find `+reference:builder`.

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-core/directive` | the `meta` schema has `set`, `value` and `values`. `ParamSpec` has `Default`. `UnclaimedName` names the nearest registered name |
| `eidos-core/workspace` | applies a `meta set` at directive authority, and withdraws it on a warm run |
| `eidos-reference/enum` | declares the default of `methods` in its schema |
| `docs/architecture` | the metadata and directive documents show the set form |

### Invariants

1. A fact that a directive sets has the type that its key registered, and validation reports a value
   that does not have it at the instance.
2. A parameter that an instance omits reads as its declared default.
3. A finding of an unclaimed name names the nearest registered name where one is close.

### The meta set form

```go
const (
    // MetaSet names the key that the meta directive sets.
    MetaSet ParamKey = "set"
    // MetaValue is the value that the meta directive sets, for a key of a
    // string, integer, boolean or identity type.
    MetaValue ParamKey = "value"
    // MetaValues is the list that the meta directive sets, for a key of
    // the type []string.
    MetaValues ParamKey = "values"
)
```

The `meta` schema declares `set` as a reference to a metadata key, `value` as a string and `values`
as a list of strings. Validation types a set instance by the type of its key:

| Key type | Instance | Fact |
|---|---|---|
| `bool` | `value=true` | true |
| `int64` | `value=<base-10 integer>` | the integer |
| `string` | `value=<text>` | the text |
| `[]string` | `values=[<text>, ...]` | the list |
| `symbol.Identity` | `value=<type in scope>` | the identity that the resolver binds |

A validation without a resolver, as a kit runs one, keeps the spelling of an identity value, and the
workspace, which always validates with a resolver, does not apply a set without a target.

The workspace writes the fact with `Facts.StampRaw` under a claim at directive authority, after the
drops and before the first annotator. A warm run withdraws the fact of an instance that the author
removed, as it withdraws a drop. The sets of one key on one subject rank as the claims at one
authority do, so the value of the first instance is the fact and `explain` shows the others.

`DerivedMetadataKey` refuses a set of a derived key as it refuses a drop, and its meaning reads "a
directive drops or sets a metadata key that its registrant derives".

### Parameter defaults

```go
// Default is the spelling of the value that validation applies where
// an instance omits the param under a role that admits it. Validation
// types it as it types a written value. Registration refuses a default
// on a required param, on a list or a reference param, and a default
// that does not type as the param's type. An empty Default declares no
// default.
Default string
```

A handler reads a defaulted parameter as it reads a written one. A positional parameter with a
default applies where the instance omits it and every parameter after it.

### Name suggestions

`UnclaimedName` names the registered name with the smallest edit distance from the written name,
counted as the optimal string alignment distance over bytes. The comparison takes the canonical
names where the written name has a plugin prefix, and the bare names otherwise. A name farther than
two edits, or farther than a third of the length of the longer name, is not suggested. Two names at
one distance suggest the first in name order.

### Failures

| Case | Finding |
|---|---|
| A meta instance writes neither `drop` nor `set`, or both | `MissingParam` |
| `set` names a group | `UnknownMetadataKey` |
| `set` names a derived key | `DerivedMetadataKey` |
| `set` names a key that does not admit the kind of the subject | `MisplacedDirective` |
| `value=false` for a boolean key | `BadSpelling`, which tells the author to write `drop` |
| A value that does not have the type of the key, or `value` where the key takes `values` | `BadSpelling` or `TypeMismatch` |
| `value` or `values` without `set` | `UnknownKey` |
| A registered default that does not type, or a default on a required, list or reference param | an error of `Registry.Register` |

### Migration

1. Add the set parameters, the typing of a set instance and the defaults to the directive package.
2. Apply and withdraw a set in the workspace.
3. Declare the default of `methods` in `enum`, and read the parameter without a fallback.
4. Show the set form in the metadata and directive documents.

## Alternatives

### A dotted key as the parameter key

`meta java.nullability=nonnull` reads well, and it needs a grammar of dotted keys for one schema.
Every other schema would then admit a dotted key that it never declares.

### A string value that the workspace converts

The workspace would convert after validation, so a wrong value would report as a defect without
the position of the instance.

### A default that the handler states

That is the present state, which no tool can read.

## Drawbacks

- The `meta` schema has three more parameters, and validation has one rule that reads the key of an
  instance to type its value.
- An instance that wrote `meta` alone validated before and now reports `MissingParam`.
- `ParamSpec` grows by one string.

## Open questions

None.

## Unresolved and future work

None.

## References

- The optimal string alignment distance: <https://en.wikipedia.org/wiki/Damerau%E2%80%93Levenshtein_distance>.
