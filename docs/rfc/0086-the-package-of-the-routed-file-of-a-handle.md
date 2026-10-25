---
rfc: 0086
title: The package of the routed file of a handle
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-25
updated: 2026-10-25
discussion: https://github.com/dokimasia/eidos/issues/80
supersedes: none
superseded-by: none
produces-adr: ADR-0107
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0086: The package of the routed file of a handle

## Summary

We propose `Out.Package`, which returns the package of the file into which the layout of the plan
routes the declarations of a handle. A generator passes that package to `Bound.Importable` before
it records a declaration, also where a route moves the file into another directory. The layout of
the plan previews the route through the role `plugin.Placer`, which the generator context of each
call has. The preview applies the routing, the filename and the package rule of the layout. The
invocation records what the preview read, so a warm run executes the invocation again where the
package of the file can change.

## Motivation

The layout derives the package of each routed file after every generator of the plan has run. A
generator records its declarations before that, so it cannot read the package of the file that it
writes into.

- ADR-0105 gave a generator the import rule of its language through `Bound.Importable(pkg, ref)`.
  The caller passes the package of the file. ADR-0105 states as a consequence that a plugin cannot
  pass that package under a route.
- The suite generator of dokimi writes the conformance test of a struct into a family that tests
  its package from outside. Under the kernel directive `out`, the test is in the external test
  package of the `out` directory, as dokimasia/dokimi#24 and the comment on issue 80 report.
- The generator checks whether the test can call the factory of the directive `subject`. It
  compares the package of the factory with the external test package of the directory of the
  struct. So it refuses an unexported factory of the external test package of the route, which the
  test can call. It accepts an unexported factory of the external test package of the struct, which
  the test cannot import.

The tree of issue 81 shows the case:

```go
// shape/box.go
package shape

//dokimi:suite
//dokimi:out path=shapetest/
//dokimi:subject Box new=newBox
type Box struct{ items int }
```

```go
// shape/shapetest/helpers_test.go
package shapetest_test

import "example.com/repro/shape"

func newBox() *shape.Box { return &shape.Box{} }
```

The conformance test of `Box` is `shape/shapetest/box_suite_test.go`. Its package is
`example.com/repro/shape/shapetest_test`, the package of `newBox`.

A plugin cannot derive that package itself:

- The route of a declaration has three sources. The reserved key `out=` of a directive of the
  plugin takes precedence over the kernel directive `out` of the subject, which takes precedence
  over the kernel directive `out` of its package. The configuration of the plan refines the
  directory of each family and can centralise the files under an output directory.
- The Go package of a directory is the package that the source files of the directory declare. The
  load placed those files there. A directory without one takes a package that the target derives
  from the toolchain modules or from the import base of the plan. A plugin does not read the
  residents of a directory, the modules or the configuration of its plan.
- The package rule of a target reads the name of the file. A Go test file of a family that tests
  from outside declares the external test package of its directory. The internal test file of the
  same family declares the package under test.

## Detailed design

### Components

| Component | Package | Change |
|---|---|---|
| Handle | `eidos-core` | `Out.Package` returns the package of the routed file of the handle |
| Role | `eidos-core/plugin` | `Placer`, its result `Placed`, `GeneratorContext.Placer` and `Invocation.Placed` |
| Preview | `eidos-core/layout` | `NewPlacer` previews the routes of the layout of one plan |
| Journal | `eidos-core/internal/state` | the lane records the routing edges of each preview of an invocation |
| Workspace | `eidos-core/workspace` | each generator call of a plan that writes files takes a placer |
| Plugin test kit | `eidos-core/plugin/plugintest` | `Fixture.Generate` previews the routes of the layout of the fixture |
| SDK | `eidos-sdk` | the regenerated facade |

### The package of a handle

```go
package eidos

// Package returns the package of the file into which the layout of the
// plan routes the declarations of the handle, with Name set to the name in
// the package clause of the file. A generator passes it to
// [rules.Bound.Importable] before it records a declaration, so it leaves
// out a declaration that the file cannot refer to.
//
// The layout of the plan previews the route of the subject of the handle
// through [plugin.GeneratorContext.Placer]. The preview reads the routing
// directives on the subject and on its package, and the configuration of
// the plan for the family. A declaration whose origin has another routing
// directive can route into another file. After [Out.Internal], the package
// is the package of the internal test file of a family that tests its
// package from outside. The preview does not read the header of the file,
// because the header is the emit.File of its units, which the generators
// place in the same phase.
//
// The invocation records the edges that the preview read: the subject, the
// package of the subject, the residents of the directory of the file and
// the modules. For a family of one file per package, it also records the
// files of the package. A change of one of these runs the handler again.
//
// Error modes: the error of the placer where the layout derives no file for
// the declarations of the subject or the target derives no package, and an
// error for a plan that does not write files.
//
// # Allocation contract
//
// Package allocates what the placer allocates, and the list of the
// subject for an internal handle of a family that tests from outside. The
// record of the reads allocates only to grow the buffer of the lane.
func (o *Out) Package() (symbol.Identity, error)
```

- The route of the subject applies, because the layout routes a declaration whose origin has no
  routing directive by the routing of the subject that placed it. The routing of the subject
  consists of the reserved keys `out=` and `tag=` of the directives of the plugin on the subject,
  the kernel directive `out` on the subject, the path of the kernel directive `out` on its package,
  and the configuration of the plan for the family.
- The preview derives the filename of a unit without declarations, for the family and the key of
  the handle. A path override or the configuration can set the filename. Otherwise the target
  spells the name of the unit.
- The placement of the file has the residents of its directory, the modules, the import base of the
  plan and the mark of a family that tests from outside, as the layout places a routed file. An
  internal handle clears the mark.
- The message of an error is the message of the finding that the layout reports when it routes such
  a declaration. Examples are a subject without a workspace source, a tag without a family, a path
  that leaves the workspace root, and a directory for which the target derives no package.
- The layout reports an override whose path is a file for two or more families of the subject when
  it routes the store, and the plan fails. A preview does not report it, because the ambiguity
  depends on every family into which the plugin places the declarations of the subject.

The handler of the directive `subject` checks the factory against the package of the test:

```go
func (g *generator) conformanceOf(m *sdk.StructMatch, e *sdk.Emitter) error {
	factory := m.Directive().Params[NewParam].Target
	test, err := e.File(TestTag).Package()
	if err != nil {
		m.Errorf(BadFactory, "%s has no conformance test: %v", m.Struct.Name, err)
		return nil
	}
	call := &node.TypeRef{Spelling: factory.Name, Target: factory}
	if err := m.Rules().Importable(test, call); err != nil {
		m.Errorf(BadFactory, "the test of %s cannot call the factory %s: %v", m.Struct.Name, factory.Name, err)
		return nil
	}
	// The handler records the conformance test as before.
	return nil
}
```

Under `out`, `test` is `example.com/repro/shape/shapetest_test`, and `newBox` passes. A factory
`newBox` of `example.com/repro/shape_test` fails with the reason of `golang.CheckReference`: the
package is an external test package of another package, and no other package imports it.

### The role of the layout

```go
package plugin

// Placer is the role of the layout of a plan that previews a route for a
// generator of the plan: the package of the file into which the layout
// routes the declarations of a unit, before the generator records them.
// The handle of a typed generator reads it through its method Package.
type Placer interface {
	// Place returns the package of the file into which the layout routes
	// the declarations of u that take the routing of subject, and what the
	// preview read. u has the plugin, the family, the cardinality, the key
	// and the package of a unit, and no declarations. An origin of u in
	// [Unit.Inside] selects the internal test file of a family whose
	// [Output.External] is true. Place returns an error with the reason of
	// the layout where the layout derives no file for such a declaration,
	// and the error of the package rule of the target. The [Placed] of an
	// error contains the reads of the preview too.
	Place(subject symbol.Identity, u Unit) (Placed, error)
}

// Placed is the result of one preview of a route: the package of the file,
// and what the preview read to derive it.
type Placed struct {
	// Package is the package of the file, with Name set to the name in its
	// package clause. It is zero after an error.
	Package symbol.Identity
	// Origin is the declaration whose routing applied. The preview read its
	// directives and the out directive of its package.
	Origin symbol.Identity
	// Dir is the directory of the file. The package rule read its residents
	// and the modules. Dir is empty where the preview derived no directory.
	Dir string
	// Files is the package from whose workspace files the layout derives the
	// directory of a family of one file per package. It is zero for any other
	// family.
	Files symbol.Identity
}

type GeneratorContext struct {
	// The fields before Placer are unchanged.

	// Placer previews the routes of the layout of the plan for the call,
	// and is nil for a plan that does not write files.
	Placer Placer
}

type Invocation struct {
	// The fields before Placed are unchanged.

	// Placed are the previews of routes that the call made, sorted by
	// origin, directory, files and package. A journal records the edges
	// that each preview read.
	Placed []Placed
}
```

```go
package layout

// NewPlacer returns the placer of the layout of one plan over the routing
// of in. A preview applies the steps of [Route] to one declaration that
// takes the routing of a subject: the override on the subject and on its
// package, the destination of the family under the configuration, the
// filename of the unit, and the package rule of the target over the
// placement of the file. The placer reads the index, the configuration,
// the families, the directives, the speller, the packager, the residents
// and the modules of in.
func NewPlacer(in Input) plugin.Placer
```

- `Route` and the placer share each routing step, so a preview and the routed file take the same
  directory, the same name and the same package.
- The placer is safe for concurrent use, because a phase call can run its invocations on more than
  one worker.
- A placer whose input lacks the index or the speller returns an error from each preview, because
  `Route` refuses such an input. A placer without a packager gives the file the package of the
  unit, as `Route` does.

### The reads of a preview

A warm run executes a dirty group again, and with it each invocation that contributed to the group.
A group reads the routing edges of its files: the declaration edge of each origin and of the package
of each origin, the files edge of the package of each per-package unit, and the residents edge of
the directory of each file with the modules edge. An invocation that previews a route depends on
the same edges, also where it contributes no file. A generator that leaves out every declaration of
a unit after the preview leaves the unit without a file, and no group then reads the directory.

The lane of a plan records the edges of each preview of an invocation:

- the declaration edge of the origin, and of the package of the origin
- the residents edge of the directory and the modules edge
- the files edge of the package of a family of one file per package

A warm run makes these edges dirty as it does for a group. A file of the directory that appeared,
vanished or changed its package, a change of the modules, a file that joined or left the package,
and a change of the directives of the origin or of its package each run the invocation again.

- An invocation with a preview is not pure, so the lane records it also where it read, touched and
  reported nothing else.
- A record lists its edges by hash, and the row of an invocation does not contain the previews, so
  the format of the state does not change.
- Each generator call of a plan that writes files takes the placer of the plan. A call that does
  not journal an invocation of its own runs again on every change that a warm run finds, so its
  invocation under `plugin.WholeCall` does not list a preview.
- `explain` counts the new edges of an invocation as unidentified reads, as it counts the routing
  edges of a group.

### The plugin test kit

`Fixture.Generate` gives the generator a placer over the layout of the fixture where the spoke of
the fixture is a backend with both halves of a target, `plugin.FileSpeller` and `plugin.Packager`.
The layout is the input that `Fixture.Layout` returns for the plugin and the plugins of the source,
with that speller and that packager. Under its configuration, the layout writes each file beside
its source. A fixture whose spoke lacks a half does not give the generator a placer, and
`Out.Package` returns an error there.

### What does not change

- The layout routes, names and places the files of a plan as before, and it reports the same
  findings.
- `Bound.Importable` and the import rule of the layout of ADR-0105.
- The record of a group.
- A generator that does not call `Package` does not record a new edge.

### Verdicts

| Item | Verdict | Reason | Reverses the verdict |
|---|---|---|---|
| `Package` on the handle | in | the package depends on the family, the key and the internal mark of the handle | a use case that needs the package of a family without a handle |
| The routing of the subject of the handle | in | the layout routes a declaration whose origin has no routing directive by the routing of the subject that placed it | a generator that places declarations whose origins have routing directives |
| A unit without declarations | in | the package rule of every target reads the placement of the file. Each filename rule but the Java rule of a lone type reads the key, the word, the tag, the package and the internal mark of the unit, and a Java file has the package of its origin | a target whose package derives from the declarations of a file |
| No header | in | the generators place the header of a file in the same phase, so a preview cannot read it | a target with an import rule whose package derives from a header |
| An error and no finding | in | the layout reports the same reason when it routes the declaration, and the generator decides how to proceed | none known |
| The routing edges on the invocation | in | a generator can leave a unit without a file after the preview, and then no group reads the directory | none known |
| No change of the state format | in | a record lists its edges by hash | a reader of the previews of a recorded invocation |
| The path of the file | out | the import rules read the package, and no use case needs the path | a generator that writes the path of the file into the file |

## Alternatives considered

### A second generation after the layout

The workspace would route the store and run the generators a second time with the package of each
file.

**Why not:** every plan would run its generators twice. The second run can change the routes again,
because a generator that leaves out a declaration can leave a unit without a file, and a tag can
move a declaration into another family.

### The package in the plugin

Each plugin would derive the package from the directives, the configuration of the plan and the
package rule of each target.

**Why not:** a plugin does not read the residents, the modules or the configuration of its plan. It
would copy the routing of the layout and the package rule of each target. Issue 80 asks the SDK for
that derivation.

### A check in the render

The render would leave out each record whose references the file cannot import.

**Why not:** ADR-0105 rejected a render of each record apart, because the file template of a family
that claims its files loops over its records itself. The decision of the suite generator is also not
a check of one reference. It chooses between a finding and a test.

### A preview on the emitter by tag

`Emitter.PackageOf(tag)` would return the package of the file of a family for the subject.

**Why not:** the package also depends on the internal mark of the handle, which the emitter does not
have. The handle has the family, the key and the mark.

### The refusal of the layout alone

The layout refuses a declaration whose file cannot import a reference under EID-0082.

**Why not:** EID-0082 is an error, so the plan does not write a file. A plan whose generator leaves
out the declaration writes the rest of the file.

## Drawbacks

- The kernel gains a method on the handle, a role and a result type, a field of the generator
  context and of the invocation, and a constructor of the layout.
- An invocation that previews a route runs again when a file of the destination directory appears,
  vanishes or changes its package, and when the modules change. For a family of one file per package
  it also runs again when a file joins or leaves the package. The group of its file runs again on the
  same changes, so a contributing invocation ran again on them before.
- A preview runs the routing steps of the layout for one declaration, and the layout runs them again
  when it routes the store. A generator that calls `Package` for each subject routes one declaration
  more for each call.
- A target whose package derives from a header that a generator states, such as the package name of
  the protobuf target, previews the package that the rest of its rule derives.
- A target that splits a unit by its declarations derives the filename of the preview from the key,
  the word and the tag of the unit. The Java target derives the filename of a file from its type,
  and a Java file has the package of the origin of its unit, so the preview has the same package.

## Open questions

None.

## Unresolved and future work

- A preview of the file of a declaration whose origin has another routing directive is not proposed
  here.

## References

| What | Where |
|---|---|
| ADR-0105, the import rule of the target in the layout | ../adr/0105-the-layout-refuses-a-declaration-whose-file-cannot-import-a-reference.md |
| ADR-0106, the route in the scope of a directive argument | ../adr/0106-a-directive-argument-resolves-in-the-route-of-its-subject.md |
| RFC-0078, the layout of a test fixture and the internal tests of an external family | 0078-the-layout-of-a-test-fixture-and-internal-tests.md |
| The routing and the layout | ../architecture/18-routing-and-layout.md |
| Incremental runs | ../architecture/09-incrementality.md |
| Issue 80, a reference that the file cannot import discards every record of the file | <https://github.com/dokimasia/eidos/issues/80> |
| Issue 81, a directive under out cannot refer to a function of the external test package of the out directory | <https://github.com/dokimasia/eidos/issues/81> |
| dokimasia/dokimi#24, a struct under out cannot take its factory from the external test package of its kit | <https://github.com/dokimasia/dokimi/issues/24> |
| protogen v1.36.8, `File.GoImportPath` and `Plugin.NewGeneratedFile` | <https://pkg.go.dev/google.golang.org/protobuf@v1.36.8/compiler/protogen> |
| gqlgen v0.17.94, `code.NameForDir` and `code.ImportPathForDir` | <https://github.com/99designs/gqlgen/blob/v0.17.94/internal/code/imports.go> |
