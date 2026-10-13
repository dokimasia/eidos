---
adr: 0046
title: The shape catalog keeps the departures of its implementation
status: Accepted
date: 2026-10-13
supersedes: none
superseded-by: none
rfc: RFC-0022
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0046: The shape catalog keeps the departures of its implementation

## Status

Accepted

## Context

RFC-0022 designs the shape catalog, its specs, its generated registries and the predicates of its
consumers. The module `eidos-plugin-shape` and its tools module implement it. Later accepted
designs decide seven places where the code departs from the design. This record keeps seven other
departures and fixes six defects of the code and its documents:

- The example of the package comment calls `Build`, which the builder of a plugin does not have.
- A spec can list an exclusion on one of its two params alone. The check of the pair then does not
  report it where the listing param sorts after the other.
- `Is`, `Has` and `In` take a name of any form, so `Is(Shape(Atomic))` tests the family key of a
  mixin.
- The refusals under `UnknownKey` of `role`, of a key outside its role and of `value` beside `drop`
  end without the fix.
- The Go fixture does not check the authority of a declared shape or the removal of each fact of a
  dropped group.
- The architecture document lists the name types as `ShapeName`, `MixinName` and `ContractName`.
  The package does not declare them.

The decisions are recorded here, because an accepted design is not amended.

## Decision

We keep the code as built where it departs from RFC-0022, and fix the six defects.

### The fixes

- The example of the package comment builds its plugin with `Generator`.
- The spec frontend reports a fault for a param that excludes another param, where the other param
  does not exclude it. Each exclusion is a pair that both params state, so the check reports once
  for the pair.
- `Is`, `Has`, `In` and `Plays` take the family key of a spec of their own form alone. For a name of
  another form the predicate has no key, and the Build of the plugin panics.
- The refusal of a key outside the role of an instance ends with the keys that the role admits. The
  refusal of `role` on a schema or a variant without roles ends with the keys that the instance
  accepts. The refusal of `value` beside `drop` ends with the drop alone.
- The Go fixture checks that the declared shape has directive authority, and that the drop removes
  the fact of each key of the group.
- The architecture document lists the name types as `shape.Shape`, `shape.Mixin` and
  `shape.Contract`.

### Departures that later designs decide

| Design | Code | Record |
|---|---|---|
| The Go rules resolve a member reference and follow aliases | The kernel's `rules.Resolve` walks the reference and its search order, and `rules.Unaliased` follows the aliases. The Go rules supply what Go code can use | RFC-0031 |
| Only the plugin `shape` writes keys of the namespace `shape` | `meta set` stamps any registered key that is not derived, at directive authority. An author can set a family key, a param or `shape.detected` | RFC-0039 |
| `Build()` builds a plugin, and `BindKeys` binds its subscriptions again in place | A plugin builds through `Generator`, `Annotator` or `Dual`. `BindKeys` returns a bound copy and leaves its receiver unchanged | RFC-0030 |
| A gate runs in a later bucket | The position in the schedule is a turn | RFC-0035 |
| `jsonschema.Of(t)` keys a property by the yaml key of its field | `jsonschema.Of(t, tag)` takes the tag that names the keys | RFC-0045 |
| A drop of a summary key hides the shape from `shape.Of` | The four summary keys are derived, so a drop or a set of one reports `DerivedMetadataKey` | RFC-0037 |
| An annotator with a catalog predicate requires `shape.Capability` | Build refuses an annotator that runs before the registrant of the keys. `After` orders an annotator after it | RFC-0030 |

### Departures without an earlier record

| Design | Code | Reason |
|---|---|---|
| An editor reads the modeline on the first line of each spec | Each spec has its licence header on lines 1 and 2, and its modeline on line 4 | yaml-language-server reads a modeline among the comments of the header of a document, so the licence header can precede it |
| A binding at a negative index reports `SpecInvalid` at the section | The fault is at the value of the binding | The value is the text at fault, so an editor marks the index |
| The tools composition passes `specfront.Keys` to `Builder.Keys` | The frontend registers its keys through its role, and neither composition calls `Builder.Keys` | Every composition that loads the frontend registers its keys, with no second call to forget |
| An annotator with a catalog predicate requires `shape.Capability` | An annotator at a later priority also passes | The gate needs the keys stamped before its predicate reads them, which a later turn gives |
| `excludes` lists the params that an instance does not write beside this one | The spec frontend requires both params of an exclusion to list each other | A pair that one param states alone gives the check no single key to report at |
| The catalog's two modules have a coverage floor of 100 | The gate runs lint, test, race and audit, and no module has a coverage floor | The gate of the repository measures the coverage of no module |
| `registry.gen.go` has 6,034 lines | It has 6,035 lines | The count leaves out the line of the provenance digest |

## Alternatives Considered

### Close the namespace of the catalog to `meta set`

The catalog could register its family keys and its params as derived, so no author sets them. A
derived key cannot be dropped either, and the drop of a classification is the override that the
design gives an author. The meta set form applies to every key that the registrant does not
derive.

### Report a one-sided exclusion at the check of the instance

The check could report a pair from either side. The spec would still state a rule on one of its
params alone, and a reader of the other param would not see it.

## Consequences

- An author who sets a family key of the catalog through `meta set` can give a callable two shape
  flags. `shape.Is` then admits the callable for both shapes.
- A spec states an exclusion on both of its params.
- RFC-0022 has its departures in this record.

## References

- [RFC-0022: The shape catalog, its specs and its generated registries](../rfc/0022-the-shape-catalog.md)
- [RFC-0030: The contracts of the SPI and the authoring API](../rfc/0030-the-contracts-of-the-spi-and-the-authoring-api.md)
- [RFC-0031: The shared helpers of the satellites](../rfc/0031-the-shared-helpers-of-the-satellites.md)
- [RFC-0035: One name for each concept](../rfc/0035-one-name-for-each-concept.md)
- [RFC-0037: Five contracts that the reviewed defects need](../rfc/0037-five-contracts-that-the-reviewed-defects-need.md)
- [RFC-0039: The meta set form, parameter defaults and name suggestions](../rfc/0039-the-meta-set-form-parameter-defaults-and-name-suggestions.md)
- [RFC-0045: The config file of a binary](../rfc/0045-the-config-file-of-a-binary.md)
- [The shape catalog](../architecture/12-shape-catalog.md)
