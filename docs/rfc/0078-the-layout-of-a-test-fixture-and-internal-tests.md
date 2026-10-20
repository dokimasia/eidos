---
rfc: 0078
title: The layout of a test fixture and the internal tests of an external family
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-20
updated: 2026-10-20
discussion: https://github.com/dokimasia/eidos/issues/46
supersedes: none
superseded-by: none
produces-adr: ADR-0078, ADR-0079
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0078: The layout of a test fixture and the internal tests of an external family

## Summary

We propose two changes to the packages of generated test files and of the files that a spec
renders. `backendtest` routes the store of a fixture through the layout of a plan, with the inputs
that `plugintest` takes from its fixture, so a spec renders the files that a workspace writes. A
handler marks the declarations of an external family that test their package from inside, such as
the check of an unexported declaration. The Go target writes those declarations into an internal
test file beside the external one, which declares the package under test.

## Motivation

- `backendtest.Rendered` renders other files than a workspace writes for the same units. It places
  each file beside its source and gives the package rule no family. A test file of an external
  family keeps the package of its directory, an `out` directive moves no file, and a bare reference
  into another package stays bare. A spec that pins the rendered tree of a plugin cannot pin the
  package of an external family or a qualified reference.
- A family that states `Output.External` writes every Go test file into the external test package
  of its directory. The go command compiles such a package apart, with access to the exported
  identifiers of the package alone. A generator that checks an unexported declaration writes a
  check that refers to it, and `go vet` refuses the file:

  ```text
  vet: store/store_probe_test.go:21:17: name entryProbe not exported by package store
  ```

  The family has no way to put that check into the package under test.

## Detailed design

### Components

| Component | Package | Change |
|---|---|---|
| Kernel | `eidos-core/backend/backendtest` | `Fixture.Layout`, and `Fixture.Context` routes through it and returns the error of the layout |
| Kernel | `eidos-core/plugin/plugintest` | `Fixture.Layout`, the routing input over the fixture |
| Kernel | `eidos-core/plugin` | `Unit.Inside`, and `Placement.External` reads it |
| Kernel | `eidos-core` | `Out.Internal`, and the flush records `Unit.Inside` |
| Kernel | `eidos-core/layout` | a routed unit keeps the `Inside` origins of its declarations, and a file with one is not external |
| SDK | `eidos-sdk` | the regenerated facade |
| Go | `eidos-lang-go` | `spell.Split` and the internal test file in `spell.Filename` |

### Invariants

1. A fixture with a layout renders the files that a workspace writes for the same plan and units.
2. A declaration that a handler marks internal is in a file that declares the package under test.
3. A declaration without the mark keeps the package of its family.
4. A target without external test packages writes the same files with the mark as without it.

### The layout of a fixture

```go
package backendtest

type Fixture struct {
    // The other fields are unchanged.

    // Layout is the input of the layout of the plan that routes the
    // store, and nil routes it as Files does. The context of a render
    // sets its store, its speller, its packager, its binder, its target
    // and its sink from the fixture and the backend under test.
    Layout *layout.Input
}

// Context lowers the fixture to the context of one render call by b.
// Where Layout is set, the layout of the plan routes the settled store,
// and its findings arrive in sink. Context returns the error of
// layout.Route for a defect of the input.
func (f *Fixture) Context(b backend.Backend, sink *diag.Sink) (*plugin.RenderContext, error)
```

```go
package plugintest

// Layout returns the input of the layout of a plan over the fixture,
// as a workspace routes the store of a plan whose generators are
// plugins: the index of the fixture, the residents and the modules of
// its graph, the families that each plugin declares, and a sealed
// registry of the schemas of the kernel and of each plugin. Its
// configuration is the zero layout.Config, which writes each file beside
// its source. Layout seals the graph, as the first phase call does.
func (f *Fixture) Layout(tb assert.TB, plugins ...plugin.Plugin) *layout.Input
```

A spec of the issue's plugin runs as a workspace runs it:

```go
f := plugintest.Loaded(t, source)
res := f.Generate(t, probe)
files := backendtest.RenderSettled(t, func(tb assert.TB) (backend.Backend, *backendtest.Fixture) {
    return gobackend.New(), &backendtest.Fixture{Emit: res.Emit, Trees: trees, Layout: f.Layout(tb, probe)}
})
```

### The internal tests of an external family

```go
package eidos

// Internal marks the declarations that the handle places as tests of
// their package from inside, such as the check of an unexported
// declaration in a family that states External. The Go target writes
// them into the internal test file of the family, which declares the
// package under test. A family that is not External tests from inside
// already, and the mark does not change its files. Internal returns o.
func (o *Out) Internal() *Out
```

```go
package plugin

type Unit struct {
    // The other fields are unchanged.

    // Inside are the origins whose declarations test the package of the
    // unit from inside, sorted and distinct, in a unit of a family that
    // states External. It is nil where every declaration tests from
    // outside.
    Inside []symbol.Identity
}
```

- The flush of an accumulator of an external family records the origin of each marked declaration
  in `Unit.Inside`.
- A routed unit keeps the `Inside` origins of its own declarations. The layout passes
  `Placement.External` as true where the family of the first unit of a file is external and no unit
  of the file has an `Inside` origin.
- `spell.Split` of the Go target splits a unit with `Inside` origins into its external part and its
  internal part. `spell.Filename` names a test file of an internal part with `internal` before the
  test suffix.

The plugin of the issue marks the check of a struct that is not exported:

```go
out := e.File(sdk.TagTest)
if s.Visibility != symbol.VisibilityPublic {
    out = out.Internal()
}
out.Record(model, check)
```

| File | Package | Contents |
|---|---|---|
| `store/store_probe.go` | `store` | `StoreProbe`, `entryProbe` and their constructors |
| `store/store_probe_test.go` | `store_test` | the check of `Store`, through `store.StoreProbe` |
| `store/store_probe_internal_test.go` | `store` | the check of `entry`, through `entryProbe` |

The other targets do not write an external test package, so they do not split a unit, and a file
keeps every declaration of its unit in its order.

### Failures

| Failure | Effect |
|---|---|
| A defect of the layout input, such as a nil index | `Fixture.Context` returns the error of `layout.Route`, and `Rendered` fails the test |
| A routing finding, such as a family that the plugins do not declare | the finding arrives in the sink of the render, and `RenderSettled` fails on it |
| A schema of a plugin that does not register | `plugintest.Fixture.Layout` fails the test |
| `Internal` on a handle of a family that is not External | nothing changes |

### Verdicts

| Item | Verdict | Reason | Reverses the verdict |
|---|---|---|---|
| The routing input from the plugin test kit | in | the plugin test kit has the graph, the facts, the directives and the scope that the layout reads | a fixture whose layout reads a load report |
| The mark per placement of a handle | in | a generator decides per subject, and a template can write a reference that no layout reads | a target that derives the package of a test from the references of a file |
| An internal test file of its own in Go | in | the external checks keep the external test package, which breaks an import cycle through a package that imports the package under test | none known |
| A mark in a declarative plugin | out | a declarative plugin declares no external family | a declarative family with External |

### Migration

`backendtest.Fixture.Context` returns an error, so each of its five callers in the kit checks it. A
spec that renders without a layout keeps `Files`, and renders the same files as before. A plugin
without the mark writes the same files as before.

## Alternatives

### The visibility of the referents in the layout

The layout could keep a file in its package where the file refers to an unexported declaration of
the package under test. The layout sees a bare reference to an emitted declaration. It does not see a
reference that a template writes as text, or a reference to a source declaration, which the speller
of the target qualifies at render.

### A mark of the whole unit

With a mark of the whole unit, one unexported subject of a source file moves every check of the file
into the package under test. An external check that imports a package which imports the package
under test then forms an import cycle. The external test package breaks such a cycle.

### A second family with a tag that ends in `_test`

A plugin could declare a second family without External, whose Go filename ends in
`_internal_test.go`. A test file of the Go target follows from the tag `test`, and another target
spells such a tag as a file that is no test, as `store.probe.internal_test.ts`.

### A second unit per accumulator

The flush could write the marked declarations into a unit of their own. The emit store and the run
state key one unit by its plugin, its tag, its package and its key, so a second unit would change
the key of a unit and the records of the run state.

### Families in `backendtest.Files`

`Files` could read the families of the plugins and apply `External`. It would then repeat the
routing of `out` directives and the qualification of references, which the layout owns.

## Drawbacks

- `backendtest.Fixture.Context` changes its signature, and its five callers in the kit change.
- `plugintest` and `backendtest` import `layout`.
- `plugin.Unit` gains one field, the emitter one method and the Go target one split.
- A source file with internal checks gives two Go test files where it gave one.

## Open questions

None.

## Unresolved and future work

- An external family of a declarative plugin.

## References

| What | Where |
|---|---|
| Issue 46, the layout of a fixture | <https://github.com/dokimasia/eidos/issues/46> |
| Issue 45, a test of an unexported declaration | <https://github.com/dokimasia/eidos/issues/45> |
| `go help packages`, the external test package | <https://pkg.go.dev/cmd/go#hdr-Package_lists_and_patterns> |
