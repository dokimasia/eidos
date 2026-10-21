---
rfc: 0083
title: References to declarations that the plan emits
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-21
updated: 2026-10-21
discussion: https://github.com/dokimasia/eidos/issues/53
supersedes: none
superseded-by: none
produces-adr: ADR-0088
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0083: References to declarations that the plan emits

## Summary

We propose one field of the emit `TypeRef`, `Emitted`, which marks a reference to a declaration
that the plan emits. A generator writes such a reference with the origin of its referent as the
target and the emitted name of the referent as the spelling. The layout qualifies it in every
language by the package of the file of its referent. The layout also marks each bare reference that
it qualifies into another package. The TypeScript speller imports a marked reference from the module
path of its referent relative to the file.

## Motivation

- A generator cannot refer to a declaration that another generator emits from an origin of another
  package. The layout qualifies a bare reference only by the package of its own unit. It qualifies
  a reference by origin only where the target is in another language than the plan. The
  reproduction of issue 53 writes `ThingFixture` into `modeltest`, and the user of `store` spells
  it `ThingFixture` and `model.ThingFixture`. The Go compiler refuses both fields.
- The settle already follows a resolved reference to its origin where the spelling is the emitted
  name of a declaration of that origin. A spoke then qualifies the reference by the package of its
  target, because `spellref.PackageOf` reads the target for a target of the plan's language. The
  reference takes the package of the origin, not the package of the file of the referent.
- The TypeScript speller imports a reference without a target from its package as written. The
  layout sets that package to the module path of the file of the referent, such as
  `model/thing.maker`. tsc and Node read that path as the name of a package, so the import of
  issue 54 does not resolve. The file needs `./thing.maker`.
- A reference by origin without a mark is ambiguous in the plan's language. A generator of data
  transfer objects can emit `dto.Thing` from `model.Thing`. A reference to the source `model.Thing`
  spells `Thing` too, so its target and its spelling match the emitted declaration as well.

## Detailed design

### Components

| Component | Package | Change |
|---|---|---|
| Schema | `eidos-core/symbol/schema` | `TypeRef.Emitted`, in the emit model alone |
| Emit model | `eidos-core/emit` | the regenerated `TypeRef` |
| Settle | `eidos-core/plugin` | an emitted reference marks the store, as a translated reference does |
| Layout | `eidos-core/layout` | qualifies an emitted reference by the file of its referent, marks each bare reference that it qualifies, and refuses an emitted reference without a referent |
| Spoke helpers | `eidos-lang/spellref` | `PackageOf` returns the package of an emitted reference |
| TypeScript speller | `eidos-lang-typescript/backend` | imports an emitted reference relative to the file |
| Protobuf writer | `eidos-lang-protobuf/backend` | qualifies an emitted reference to a protobuf declaration with the package clause of the file of its referent |
| SDK | `eidos-sdk` | the regenerated facade |

### The field

```go
package schema

// Emitted marks a reference to a declaration that the plan emits. A
// generator sets it with the origin of the referent in Target and the
// emitted name of the referent in Spelling. The layout sets it on each
// bare reference that it qualifies into another package. The layout
// qualifies an emitted reference in every language by the package of the
// file of its referent. Only the emit model has it, because a generator
// and the layout set it.
type TypeRef struct {
	// The fields before Bound are unchanged.
	Bound   bool `eidos:"emit"` // the referent is code that the binding of the plan's target generates
	Emitted bool `eidos:"emit"` // the referent is a declaration that the plan emits
	// The fields after Bound are unchanged.
}
```

A generator of the reproduction of issue 53 writes the field `ByOrigin` this way:

```go
&emit.Field{Name: "ByOrigin", Type: &emit.TypeRef{Spelling: "ThingFixture", Target: thingID, Emitted: true}}
```

### The settle

The settle follows an emitted reference by its origin, as it follows every resolved reference. It
marks the store for an emitted reference, as it marks it for a translated reference, so the layout
runs its pass of references. `plugin.Emit.Translates` keeps its name, and its contract widens:

```go
package plugin

// Translates reports whether the settle found a type reference whose
// target is in another language than the plan, or an emitted reference.
// The layout qualifies such a reference by the file of its referent.
func (e *Emit) Translates() bool
```

### The layout

- An emitted reference resolves as a translated reference resolves. Its referent is the declaration
  of the plan whose origin is the target of the reference and whose settled name is its spelling.
- The layout sets the package of an emitted reference to the package of the file of its referent.
  It clears the package where the referent is in the package of the file of the reference.
- The layout sets `Emitted` on each bare reference that it qualifies into another package, because
  the referent of such a reference is a declaration of the plan too.
- An emitted reference without a referent refuses the referencing declaration under a new code:

```go
package layout

// UnemittedReference reports an emitted reference to a declaration that
// the plan does not emit, at the origin of the referencing declaration.
// The referencing declaration is refused.
var UnemittedReference = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number:  81,
	Meaning: "an emitted reference refers to a declaration that the plan does not emit",
})
```

### The spokes

`spellref.PackageOf` returns the package of an emitted reference whatever language its target has.
The spokes of Go, Java, Rust, Python and TypeScript read the package of a reference through it.

```go
package spellref

// PackageOf returns the package that a spoke of the language lang
// qualifies the reference t with. A type parameter has no package. An
// emitted reference has the package that the layout set. A reference with
// a target of lang has the package of its target, and any other reference
// has its own package.
func PackageOf(t *emit.TypeRef, lang symbol.Lang) string
```

The TypeScript speller imports an emitted reference from the module path of its referent relative
to the path of the file, as it imports a translated reference. A reference without a target and
without the mark keeps the specifier of its source, such as `react`. The spokes of Go, Java, Rust
and Python import the package that the layout sets, as they import a translated reference.

The protobuf writer imports the file that the package of a reference names. It writes a reference
to a protobuf declaration as its source writes it. It writes an emitted reference to a protobuf
declaration as it writes a translated reference, under the package clause of the file of its
referent where that clause differs from the clause of the file.

### The reproductions

- In issue 53, both fields `ByOrigin` read `modeltest.ThingFixture`, and each file imports
  `example.com/shop/model/modeltest`.
- In issue 54, `model/thing.user.ts` imports `ThingFixture` from `./thing.maker`.
- The field `StoreUser.Bare` stays `ThingFixture`. A bare reference refers to a declaration of the
  package of its unit, and `store` declares no `ThingFixture`.

### Verdicts

| Item | Verdict | Reason | Reverses the verdict |
|---|---|---|---|
| An emitted reference in every language | in | the file of the referent decides the package, whatever language the plan writes | none known |
| One mark for the reference of a generator and for a bare reference that the layout qualifies | in | both refer to a declaration of the plan, and a speller imports both by one rule | a speller that imports the two in different ways |
| A reference by origin without the mark, in the plan's language | as before: the package of its target | a reference to a source declaration can spell the emitted name of a declaration of the same origin | none known |
| A code of its own for an emitted reference without a referent | in | the cause differs from a translated reference without a referent, which a missing translation causes | none known |
| A bare reference to a declaration of another package | out | a bare reference refers to a declaration of the package of its unit, and the reference by origin covers another package | a generator that cannot know the origin of its referent |
| A helper of the emitter that builds an emitted reference | out | a generator sets one field of a literal, as the reproductions do | a second generator that builds the same reference from a declaration of the plan |

## Alternatives

### Matching by origin and name without a mark

The layout would qualify every reference of the plan's language whose target and spelling match a
declaration of the plan. In the example of the data transfer objects, a reference to `model.Thing`
would then read `dto.Thing`.

### A field for the origin beside the target

`TypeRef` would get an `Origin`, and `Target` would keep the source declaration. The settle and the
layout key a reference by origin on `Target` already, so a second identity would split one rule
across two fields.

### The layout sets the target to zero

The layout would set the target of a qualified reference to zero, so `PackageOf` would return its
package. The TypeScript speller picks a module path or a specifier by the target. A bare reference
that the layout qualifies has no target, so the speller would still read the same fields for a
module path of the plan and for a specifier of the source.

### A relative specifier from the layout

The package of the reference would hold `./thing.maker`, as the layout writes it. A specifier is a
rule of the language, and the layout serves every language. The Go spoke would then receive a path
that is no import path.

## Drawbacks

- The emit `TypeRef` gets one more field, which every generator can set.
- `plugin.Emit.Translates` keeps its name while its contract widens to emitted references.
- A reference that a generator marks without a referent is refused. Without the mark it rendered
  with the package of its target.
- The layout writes a field of a reference that a generator can also write. The kernel and the
  spokes read it as one fact either way.

## Open questions

None.

## Unresolved and future work

None.

## References

| What | Where |
|---|---|
| Issue 53, a reference by origin in the language of the plan | <https://github.com/dokimasia/eidos/issues/53> |
| Issue 54, the import of a reference that the layout qualifies | <https://github.com/dokimasia/eidos/issues/54> |
