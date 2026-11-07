---
rfc: 0090
title: The declared name of the package of a reference
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-11-07
updated: 2026-11-07
discussion: https://github.com/dokimasia/eidos/issues/121
supersedes: none
superseded-by: none
produces-adr: ADR-0122
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0090: The declared name of the package of a reference

## Summary

A Go import without an alias binds the name that the package clause of the imported package
declares. The resolution step of the load binds such an import to the name that its path assumes.
A reference through a package whose clause declares another name then resolves to no declaration,
or to a declaration of another package. We propose a new optional frontend role,
`plugin.Qualifier`. The resolution step keeps a declaration that a candidate names only where the
qualifier admits the name that the package of the declaration declares. The Go frontend takes the
role, and the Go rules apply the same decision to the names of a directive.

## Motivation

- `github.com/aws/aws-sdk-go-v2/aws/signer/v4` declares `package v4`. `golang.AssumedName` reads
  the last element `v4` as a major version and binds the import to `signer`. No import binds the
  qualifier of the reference `v4.SignerOptions` in `service/s3`. `Bindings.Candidates` then probes
  every import without an alias. `aws/signer/v4` and `internal/v4a` both declare `SignerOptions`,
  and the load reports EID-0038.
- A run under `strict` fails every plan on that Warning. A run without `strict` takes the first
  import in source order as the target. Where that import is the other package, the Go backend
  writes a type of the wrong package.
- The frontend parses each unit apart from the others, so the bindings of its parse cannot read the
  clause of an imported package. The resolution step has every loaded package, and
  `node.Package.Name` is the name that its clause declares.
- The render gets the declared name of each loaded package through `RenderContext.Names`. The
  resolution step has no such input.

## Detailed design

### Components

| Component | Package | Change |
|---|---|---|
| Kernel | `eidos-core/plugin` | the optional role `Qualifier` |
| Kernel | `eidos-core/frontend` | `Builder.Qualifies`, which declares the function of the role of a built frontend |
| Kernel | `eidos-core/frontend/load` | the selection keeps the declarations that the qualifier admits, and a warm load selects the references of a renamed package again |
| Kernel | `eidos-core/frontend/frontendtest` | the statement `name` of the scripted language, and `ScriptedQualifier` |
| SDK | `eidos-sdk` | the regenerated facade |
| Go | `eidos-lang-go` | `Bindings.Tiers`, `Qualifies`, `Bindings.ImportIn`, `ImportNameIn` and `PackageBindings.Names`, the frontend in the qualifier role, and the rules |

### Invariants

1. A reference through an import without an alias resolves to a declaration of the package whose
   clause declares the qualifier, wherever the load has read that package.
2. A warm load and a load that restores a unit from the parse memo give each reference the target
   that a cold load gives it.

### The role

```go
// Qualifier is the optional frontend role of a language whose import binds
// the name that the imported package declares, as a Go import without an
// alias binds the name in the package clause of the imported package. The
// resolution step passes each declaration that a candidate of a reference
// names to Qualifies, with the name that the package of the declaration
// declares. It keeps the declaration among the hits of its tier only where
// Qualifies reports true, so two hits that Qualifies admits report as an
// ambiguity, and a hit that it refuses does not.
type Qualifier interface {
	// Qualifies reports whether a declaration of a package that declares
	// name is a target of spelling, where the candidate that names the
	// declaration is in the tier of that index among the tiers that Resolve
	// returned. The resolution step calls it after every unit parsed, and the
	// selection of a kept or restored unit calls it for the recorded tiers of
	// the unit, so it reads no bindings. name is empty for a package whose
	// units declare no name.
	Qualifies(spelling string, tier int, name string) bool
}
```

The name of a package is the name that its parsed units declare. Where the parsed units declare no
name, the first name that its kept and restored units declare in splice order is the name. The load
parses every unit of a package whose units declare two names. A parsed and a kept unit of one
package then declare one name.

A tier keeps its admitted declarations alone. Two admitted declarations of one tier report under
`AmbiguousReference`. The frontend returns its tiers from the bindings of its parse, which no other
unit changes. A warm load and the parse memo reuse the recorded tiers of a unit, and the selection of
such a unit applies the qualifier to them again.

`Builder.Qualifies` declares the function of the role for a frontend that the kit builds. Every
frontend that the kit builds implements the role, as it implements `plugin.KeyProvider`, and a
declaration without `Qualifies` admits every declaration, as a frontend outside the role does. The
kit composes each other optional role into a struct type of its own, so a role that a declaration
can omit would double those types.

### The warm load

A kept unit selects its references again where one of their candidates names a declaration of a
package that a parsed or restored unit renamed. The load adds the declarations of such a package to
the declarations that appeared or disappeared, both those before the edit and those after it. A
package that the load reads now and did not read before contributes its declarations as appeared
declarations already. A package that the load no longer reads contributes them as disappeared
declarations.

### The tiers of Go

`Bindings.Tiers` returns the candidates of a spelling in two tiers:

| Tier | Candidates | Admitted by `golang.Qualifies` |
|---|---|---|
| `AliasTier` | The name in the package of the import whose alias is the qualifier. For a bare name, the name in the package of the file and in each dot import | Always |
| `ImportTier` | The name in each import without an alias, in source order | Where the package declares the qualifier, or declares no name |

A qualified spelling whose qualifier an alias binds has no `ImportTier`. A qualified spelling
without such an alias has an empty `AliasTier`. The frontend returns the tiers from Resolve and
declares `golang.Qualifies` as its qualifier. Two imports without an alias whose packages declare
the qualifier report under `AmbiguousReference`, because Go refuses such a file.

The uses of a unit are the candidates of `Bindings.Candidates`, which keeps the probe that it
states. A dependency round loads the package of each use before the load reads any clause.
`golang.FrontendVersion` rises, because the record of each qualified reference gains a tier.

### The rules of Go

The rules resolve the name of a directive through `Bindings.Tiers`. They admit each declaration with
`golang.Qualifies` and the name of its package in the view. `PackageBindings` becomes a struct with
the files of the package and the field `Names`, which returns the name that the package at an import
path declares in the view. An import without an alias of another file binds the qualifier that its
package declares. A package outside the view binds the name that its path assumes, as before.
`ImportNameIn` and `Bindings.ImportIn` take the same function.

### Failures

| Condition | Effect |
|---|---|
| The path of one import assumes the qualifier, its package declares another name, and the package of a second import declares the qualifier | The uses of the reference name the first package alone, so the second package loads only where another use names it. Otherwise the reference keeps its spelling |
| A reference through a package that the load did not read | The reference keeps its spelling, as before |
| Two imports without an alias whose packages declare the qualifier | `AmbiguousReference`, because Go refuses the file |

### Verdicts

| Item | Verdict | Reason | Reverses the verdict |
|---|---|---|---|
| The qualifier at the selection | in | The selection of a kept unit reads its recorded tiers without a parse, so the decision reads the spelling, the tier and the name alone | none known |
| The declared names in `ImportScope` | out | Tiers that depend on another unit make the record of a unit wrong for a warm load and for the parse memo | none known |
| Every import in the uses of a qualified reference | out | A use loads a package, so a dependency round would load each import of every signature | none known |

## Alternatives

### The declared names in the scope of Resolve

The resolution step could pass a function from an import path to the declared name in
`ImportScope`, as the render gets `RenderContext.Names`. The tiers of Resolve would then depend on
the packages of other units. A kept unit selects from its recorded tiers without a parse, and the
parse memo restores a unit into another load. Both would keep tiers that the names of another load
decided.

### The declared name in the bindings of the parse

The Go frontend could read the package clause of each imported package when it parses a file. A
parse reads the members of its unit and its shared inputs alone, and the key of a unit does not fold
the files of another unit. An edit of the clause would not invalidate the importing unit.

### The probe by the last element of the path

The probe could try first the imports whose last path element is the qualifier, which covers
`aws/signer/v4`. Go binds the name of the package clause, so the rule would still guess wrong for a
package that declares a name unlike any element of its path.

## Drawbacks

- The plugin contract gains one optional role, and every frontend that the kit builds implements
  it.
- The resolution step calls the qualifier once for each declaration that a candidate names, for
  each frontend that the kit builds, and it looks up the name of the package of each such
  declaration.
- A warm load after an edit of a package clause selects again each kept reference with a candidate
  in that package.
- The record of a Go unit has an empty first tier for each qualified reference without an alias,
  and every Go unit parses once after the rise of the frontend version.

## Open questions

None.

## Unresolved and future work

- Whether a strict run fails on a Warning in a unit that no plan reads is not proposed here.

## References

| What | Where |
|---|---|
| Issue 121, an import whose package clause differs from its assumed name | <https://github.com/dokimasia/eidos/issues/121> |
| dokimi issue 61, the strict runs of a tree with the S3 client of the AWS SDK | <https://github.com/dokimasia/dokimi/issues/61> |
| RFC-0061, the declared name of an imported Go package in the render | 0061-the-declared-name-of-an-imported-go-package.md |
| The Go specification, import declarations | <https://go.dev/ref/spec#Import_declarations> |
| go/types, the name that an import binds | <https://github.com/golang/go/blob/go1.27.3/src/go/types/resolver.go> |
