---
rfc: 0061
title: The declared name of an imported Go package
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-13
updated: 2026-10-13
discussion: none
supersedes: none
superseded-by: none
produces-adr: ADR-0041, ADR-0042
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0061: The declared name of an imported Go package

## Summary

The Go backend imports a package under the name that its path assumes. A package can declare
another name, and the import then binds that name while the file qualifies with the assumed one, so
the file does not compile. We propose that the render pass gives the backend the declared name of
each package that the run loaded, and that the Go backend binds an import under that name. This
decides the proposed decision D-14. We also decide the other open decisions and records of the
improvement plan: a migration of the spellings of the previous version is out (D-5), the
projections of Java and Rust are a milestone of the roadmap (D-12), the Go finaliser removes no
import (W2.10), and two ADRs record the departures of the options of a plugin and of the parse check
of the toolchain suite.

## Motivation

- The import set binds a Go package under `golang.Qualifier` of its path, and the import block
  states a name only where it differs from the last element of the path. A package
  `example.com/x/store` that declares `package storage` is imported without a name, so it binds
  `storage`, and a reference to its type `Row` is written as `store.Row`, which the compiler
  refuses.
- The graph has the declared name of each package that the run loaded, as `node.Package.Name`.
- The improvement plan proposes D-5, a command that rewrites the spellings of the previous version,
  and D-12, rules packages of Java and Rust for the capabilities that nothing implements.
- W2.10 waits on D-14, and two records differ from the code: RFC-0006 states that an untagged option
  takes its lowered name, and D92 states that parsing needs no toolchain.

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-core/plugin` | `RenderContext.Names` returns the declared name of a package by its path, and `RenderedFile.Packages` lists the paths whose names a file read |
| `eidos-core/backend/render` | `ImportSet.Declared` returns the declared name of a path and records the path, and the pass lists the recorded paths of each file |
| `eidos-core/workspace` | The render of a plan gets the names of the packages of its index, a group reads the declaration edge of each package whose name one of its files read, and a warm run makes the declaration edge of a renamed package dirty |
| `eidos-lang-go/backend` | `bind` binds a package under its declared name where the set has one |
| ADR-0041 | The options of a plugin keep their two refusals |
| ADR-0042 | The toolchain suite requires the toolchain before its parse |

### Invariants

1. A generated Go file that refers to a declaration of a package that the run loaded compiles,
   whatever name the package declares.
2. A warm run after an edit of a package clause writes each file that imports the package again,
   as a cold run does.

### The declared names

`RenderContext.Names` is a function from the path of a package to the name that its package
clause declares, and it reports false for a path that the run did not load. The workers of the
render call it concurrently, and the sealed graph that it reads does not change during a render. The workspace looks the
package up in the index of the plan, under the language of the target of the plan, so it builds no
table. A package of another language has no name for a Go path.

The render pass gives the set of each file the function, and `ImportSet.Declared(path)` returns the
declared name and records the path. The Go backend binds a path under the declared name where the
set has one, and under `golang.Qualifier` of the path otherwise. A declared name is a Go identifier
and no keyword, so the backend does not escape it. The import block states the name where it
differs from the last element of the path, as before, so a package that declares another name than
its last element gets a name in the block and binds it.

A template that imports a package through `use` writes the qualified names in its own text, so the
template states the name that the package declares, and its import keeps the name that the template
states or the path assumes.

The rendered file lists the paths that its set recorded, as `RenderedFile.Packages`. A group reads
the declaration edge of the package of each such path. The load lists each package whose clause an
edit renamed, and a warm run makes the declaration edge of each such package dirty, so it renders
each file that imports the package again. An edit of another declaration of the package leaves the
edge clean.

### Verdicts

| Row | Item | Verdict | Reason | Reverses the verdict |
|---|---|---|---|---|
| D-14 | The declared name of a Go package | in | The graph has the name, and an import under the assumed name does not compile where the two differ | none |
| W2.10 | The removal of an import that a file does not reference | out | A template records an import through `use` where its text uses the package, and the toolchain check of the generated tests refuses an unused import. A finaliser that removes imports needs the name that each import binds, and a package outside the load has no declared name | A template that records an import before it knows whether it uses the package |
| D-5 | A command that rewrites the spellings of the previous version | out | The names of the catalog, the keys and the plugins follow their accepted designs, and every carrier has the brand of its binary. A consumer rewrites its carriers once with a substitution for each name, and eidos keeps no code that only such a rewrite runs | A consumer whose sources a substitution of names cannot rewrite |
| D-12 | Rules packages of Java and Rust for `ThrowsRules` and `OwnershipRules` | out of the improvement plan | The projections of Java and Rust are a milestone, and the roadmap does not schedule it. The two capabilities stay in the projection vocabulary for the languages that the architecture names | A milestone for the projections of Java and Rust |
| W10.17 | An untagged option field that takes its lowered name | out | ADR-0041 records that the options of a plugin keep both refusals of `ValidateOptions` | A plugin that needs an implicit key |
| W10.42 | The sentence of D92 that parsing needs no toolchain | changed by ADR-0042 | TypeScript parses through tsc, Java through java and Rust through rustfmt, so `RunToolchainSuite` asks `Require` before its parse check as before its other checks | A satellite whose parse check runs without its toolchain |

### Failures

| Condition | Report |
|---|---|
| A package that the run did not load and that declares another name than its path assumes | None. The import binds the assumed name, and the toolchain check of the output refuses the reference |

### Migration

1. Add `RenderContext.Names`, `RenderedFile.Packages` and `ImportSet.Declared`, and fill them in the
   workspace and the pass.
2. Read the declaration edge of each recorded package in the groups, and make the edge of a renamed
   package dirty in a warm run.
3. Bind the declared name in the Go backend.
4. Write ADR-0041 and ADR-0042.

## Alternatives

### A name in every import

The Go backend could state a name in every import. A file would then bind the name that it
qualifies with, whatever the package declares. Most imports would repeat the last element of their
path, which gofmt leaves and goimports removes.

### The declared name on each reference

A frontend could record the declared name of the package of each reference. The model would carry
the name on every reference, while the graph has it once on the package.

## Drawbacks

- A package outside the load keeps the assumed name.
- The context of a render gets one more field.

## Open questions

None.

## Unresolved and future work

None.

## References

- The improvement plan rows W1.5, W2.10, W10.17, W10.27 and W10.42, and the proposed decisions
  D-5, D-12 and D-14.
- [RFC-0006](0006-authoring-surface-and-dispatch.md), the options of a plugin.
- D92 in [21](../architecture/21-decisions.md), the toolchain harness.
- [03](../architecture/03-projection.md), the capabilities of Java and Rust.
