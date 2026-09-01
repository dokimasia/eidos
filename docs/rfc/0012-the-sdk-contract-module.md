---
rfc: 0012
title: The SDK facade module
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Draft
created: 2026-09-01
updated: 2026-09-01
discussion: none
supersedes: none
superseded-by: none
produces-adr: tbd
---

# RFC-0012: The SDK facade module

## Summary

One module, `go.dokimi.dev/eidos/sdk`, re-exports the kernel's
plugin-facing surface and defines nothing of its own: one alias,
constant or wrapper per exported symbol, generated from a curated
package list. Plugin code imports SDK paths alone; the kernel
stays the single home of every definition and keeps its own
layout freedom behind the facade. The rule is enforced on import
paths, not on the module graph: the kernel arrives as a
transitive dependency, and that is accepted.

## Motivation

A backend satellite today imports six kernel packages, and
nothing marks which kernel exports are the supported plugin
surface and which are internals a plugin happens to reach. The
supported surface is implicit — whatever the kernel exports — and
an implicit surface drifts the way implicit fact coverage drifted
before RFC-0011 made it data: one reach-around import at a time,
each hardening an internal into a contract nobody declared.

The facade makes the surface a curated artifact. A symbol is in
the SDK because the list admits it and the generator emitted it;
a new kernel export is invisible to plugin authors until that
happens, so additions are deliberate. Plugin code, examples and
documentation speak one stable namespace, `sdk/...`, and the
kernel can reorganize what the facade covers without touching a
plugin's imports.

The dokimi project is about to author generators and annotators
against this kernel. Every week before the namespace exists adds
imports a later boundary has to migrate; after it exists, a new
plugin cannot take an unsupported dependency without the lint
naming it.

## Detailed design

### The module

The repository gains the module `go.dokimi.dev/eidos/sdk` in the
directory `eidos-sdk`, requiring `go.dokimi.dev/eidos/core`. It
holds generated files and package documentation, nothing else.

### One facade package per surface

The curated list, one SDK package per kernel package, same
package names:

| SDK package | Re-exports |
|---|---|
| `sdk` (root) | the authoring facade and the backend kit |
| `sdk/plugin` | identities, units, contexts, roles, hooks |
| `sdk/emit`, `sdk/node` | the generated models |
| `sdk/symbol`, `sdk/position`, `sdk/diag` | the shared vocabulary and findings |
| `sdk/meta`, `sdk/directive`, `sdk/store` | facts, directives, the graph's read surface |
| `sdk/render` | the language declaration, coverage, the builtin names |
| `sdk/output` | the stamp contract and sinks |
| `sdk/backendtest`, `sdk/plugintest` | the conformance suites |

Per symbol, the generated form: a type alias (`type Backend =
plugin.Backend`), a re-declared constant, or a wrapper function
carrying the original's documentation. Aliases are the underlying
types, so values flow between facade and kernel spellings without
conversion; wrapper functions inline, so the facade costs
nothing at run time.

### Generation

A generator under the kernel's `internal/gen` reads the curated
list, parses each package from source, and emits the facade with
the source documentation carried over — kernel import paths
respelt to facade ones — the way the model generator carries the
schema's. Emission works on syntax alone: the conformance kits
spell assert-module types in exported signatures, and a
type-checking load would need that module resolvable wherever
generation runs, while reprinting parsed declarations does not.
What syntax cannot re-export faithfully — a dot import, an
unexported type in an exported signature, a kernel package
outside the curated list — refuses at its position rather than
narrowing silently.

The mirror guard holds the committed facade byte-equal to a
fresh run, so the facade cannot rot against the kernel: a kernel
surface change fails the guard until the facade regenerates, and
the diff shows exactly what the supported surface gained or
lost.

### What the facade does and does not promise

It promises one curated, stable import namespace: plugin source
speaks `sdk/...` paths only, and what those paths export is a
reviewed list rather than an accident of kernel layout.

It does not promise type isolation. An alias is the kernel's
type; a breaking kernel change to a re-exported symbol is a
breaking SDK change in the same moment. Compatibility work
happens in the kernel, once, for both spellings.

### Enforcement

The repository's lint enforces the rule: a depguard rule in the
shared golangci configuration denies `go.dokimi.dev/eidos/core`
imports across every plugin module's tree, tests included, and
`ergon lint` runs golangci per module, so the refusal is
reported in the module that drifted. A plugin repository outside
this one states the same rule in its own lint configuration. The
reference module stays outside the deny list, because a
reference composition is host side and composes the workspace,
which the facade does not re-export. The module graph is not the
boundary: `go.mod` carries the kernel transitively, and that is
deliberate — the rule lives where the drift happens, in import
statements.

## Alternatives considered

- **The dependency inversion** — a contract module the kernel
  imports, with plugins requiring it alone. Attempted and
  reverted on 2026-09-01: every package a plugin or its
  conformance tests touch proved contract-side, including the
  render pass the suites drive and the kits that compose it, so
  the kernel hollowed to orchestration and codegen and the
  boundary bought a rename at a milestone's cost. Recorded here
  so it is not re-proposed.
- **A hand-written facade** — rots against the kernel; the
  mirror-guarded generator is the difference between a curated
  surface and a stale one.
- **`internal/` alone** — enforces reachability but curates
  nothing and names no namespace; the public-package set stays an
  implicit surface.

## Drawbacks

- Every re-exported symbol has two spellings; the lint and the
  documentation keep plugin code on the SDK's.
- The facade's godoc duplicates the kernel's.
- A kernel surface change touches two modules: the change and the
  regenerated facade. The mirror guard makes forgetting
  impossible, not cheap.

## Unresolved and future work

- The curated list starts as the table above; dokimi's own
  surface layers on top in its repository.
- Whether the generator admits per-symbol exclusion inside an
  admitted package, or stays package-whole, is decided when the
  first package needs it.

## References

- [RFC-0002: The model generator](0002-model-generator.md)
- [RFC-0009: The render pass and the backend kit](0009-render-pass-and-backend-kit.md)
- [RFC-0011: Neutral emit and the target lowering seams](0011-neutral-emit-and-the-target-lowering-seams.md)
- [Architecture: repositories and the kernel](../architecture/01-repos-and-kernel.md)
- [Architecture: plugins](../architecture/06-plugins.md)
- [Architecture: compatibility](../architecture/15-compatibility.md)
