---
rfc: 0038
title: The modules of an import specifier
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

# RFC-0038: The modules of an import specifier

## Summary

The TypeScript frontend resolves a module specifier through the `paths` and `baseUrl` options of the
governing tsconfig. The TypeScript rules resolve a directive's reference from the graph, which does
not record the tsconfig, so a directive and a type reference resolve one specifier to two packages. We propose a node field `Modules` on `node.Import` and `node.Export`, which contains the
packages that a specifier names where a configuration of the workspace maps it. The frontend fills
the field, and the rules try its packages in order.

## Motivation

A tsconfig maps `@fx/*` onto `fx/*`, and the module `fx/use` imports the class `Item` from
`@fx/item`. The frontend links the field type `Item` of `fx/use` to the class `Item` of the package
`fx/item`. The rules resolve the directive value `Item` in the same scope to a stand-in of the
ambient module `@fx/item`. A case in the spec of the TypeScript rules reproduces both results.

The frontend and the rules also declare the index module and the default export of TypeScript
twice, and each decides on its own whether a specifier is relative.

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-core/symbol/schema` | `Import` and `Export` have the node field `Modules` |
| TypeScript satellite, root package | `typescript.IndexModule`, `typescript.DefaultExport` and `typescript.Relative`, which the frontend and the rules share |
| TypeScript frontend | sets `Modules` of an import and of a re-export whose specifier a `paths` pattern or the `baseUrl` maps |
| TypeScript rules | try the packages of `Modules` where the field is set |

### Invariants

1. A directive and a type reference resolve one specifier to the same packages.
2. The graph records each package that a specifier names where a configuration outside the graph
   decides it.

### The field

```go
type Import struct {
    // ...
    // Modules are the packages that the specifier names, in the order in
    // which the language tries them, where a configuration of the
    // workspace maps the specifier. Modules is nil where the language
    // resolves the specifier from its text alone.
    Modules []string `eidos:"node"`
}

type Export struct {
    // ...
    // Modules are the packages that Path names, as [Import.Modules] are.
    Modules []string `eidos:"node"`
}
```

Adding the field is a schema edit and a regeneration. The emit side has no `Modules`, because a
backend does not resolve imports.

### The TypeScript frontend and rules

The frontend sets `Modules` of an import, and of an export clause or an export-star statement with
a source module, where the specifier is not relative and a `paths` pattern or the `baseUrl` maps it.
`Modules` lists the module file and the directory index of each substitution of the pattern, then
those of the `baseUrl`, then the specifier itself, which the frontend resolves in this order.

The rules resolve a specifier from these candidates:

| Specifier | Candidates |
|---|---|
| `Modules` set | the packages of `Modules`, in order |
| relative | the module file of the path, then the index of the directory |
| any other | the ambient module of the text |

A name resolves in the first candidate whose package publishes it. A stand-in for a module outside
the view has the package that the rules give it without `Modules`.

```go
// IndexModule is the module that resolution tries in a directory when
// the path of a specifier has no module file.
const IndexModule = "index"

// DefaultExport is the name of the default export of a module, and the
// name that a default import binds.
const DefaultExport = "default"

// Relative reports whether a module specifier names a path relative to
// the directory of the importing file. It does not allocate.
func Relative(specifier string) bool
```

### Failures

The change does not add a finding. A specifier without a mapping resolves as before.

### Migration

1. Add `Modules` to the schema and regenerate the model and the facade.
2. Add the three shared declarations to the TypeScript root package, and use them in the frontend
   and the rules.
3. Set `Modules` in the frontend, and read it in the rules.

## Alternatives

### Stamp the module of each import

A stamp needs a subject with an identity. The resolution step gives files and declarations an
identity and gives imports none, so a stamp on an import has no subject. An identity for each import
changes the identity scheme of every language.

### Stamp the resolution of the tsconfig on each file

A metadata value is a string, an integer, a boolean, a string list or an identity. The `paths`
option maps each pattern onto a list of substitutions, so a stamp needs an encoding of its own. The
rules would also repeat the resolution of the frontend.

### Let the rules read the tsconfig

The rules read the view alone, so a resolution reads nothing that the incremental run does not track.

## Drawbacks

- `node.Import` and `node.Export` grow by one slice header, 24 bytes on a 64-bit platform, and the
  codec writes one more field for each.
- A specifier that a mapping places has one list of candidates for each import and re-export of it.

## Open questions

None.

## Unresolved and future work

None.

## References

- The improvement plan row W2.5.
- TypeScript's module resolution: <https://www.typescriptlang.org/docs/handbook/modules/reference.html#paths>.
