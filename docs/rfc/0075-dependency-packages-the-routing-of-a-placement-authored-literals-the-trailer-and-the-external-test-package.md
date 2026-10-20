---
rfc: 0075
title: Dependency packages, the routing of a placement, authored literals, the trailer and the external test package
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Draft
created: 2026-10-20
updated: 2026-10-20
discussion: dokimasia/eidos#35, dokimasia/eidos#36, dokimasia/eidos#37, dokimasia/eidos#38, dokimasia/eidos#39 and dokimasia/eidos#40
supersedes: none
superseded-by: none
produces-adr: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0075: Dependency packages, the routing of a placement, authored literals, the trailer and the external test package

## Summary

The kernel, the layout, the output contract and the Go package rule lack these contracts:

- A plan that patterns narrow does not read the dependency packages of its language (#35).
- A rule of a plan runs on a declaration of a vendored package (#36).
- A declaration that a handler places for a subject of another origin does not take the routing
  of the subject (#37).
- An authored text of a text, boolean or scalar type lifts without the rules of its language
  (#38).
- gofmt changes every stamped Go file, and the next run reads the file as edited (#39).
- A family of test files cannot declare the external test package of its directory (#40).

We propose these changes:

- The load marks each package of a dependency round. A rule of the generate phase skips its
  declarations, and the scope of a plan admits the dependency packages of its language.
- A unit records the subject of each declaration that a match places for another origin. The
  router takes the routing of that subject where the origin of the declaration has none.
- An authored text lifts through the `LiteralFor` of its language first, for every shape.
- The stamp writes one blank line before the trailer, and the reader verifies both layouts.
- `plugin.Output.External`, which the Go package rule reads for the test files of a family.

The table of verdicts lists each departure from the proposal of an issue.

## Motivation

| Issue | The problem |
|---|---|
| #35 | `Reader.Lookup` returns no declaration of a package that the scope does not admit, and no pattern admits a package of a store. The member walk of a plan that `./internal/...` narrows records a gap at an embed of `io.Closer`, and a generator of doubles refuses the interface |
| #36 | The selection does not claim a vendor tree, so a vendored package loads as a unit of a dependency round. Its files are in a workspace directory, so `./...` admits it, and a rule runs on its declarations. The run writes `vendor/example.com/dep/dep_probe.go`, which `go mod vendor` removes again |
| #37 | The router reads the routing of each declaration from its origin. The double of a handle without a directive is written beside the source, and the double of its subject is written into the directory of the `out` of the subject. A test package that imports the double of the subject has no double of the handle |
| #38 | `View.Authored` lifts the text of a text, boolean or scalar type without `LiteralFor`. `value=NewLevel()` on `type Level int` lifts to the number `NewLevel()`, which a double in another package writes as an unqualified call |
| #39 | `Contract.Stamp` writes the trailer on the line after the body. gofmt puts a blank line between the last declaration of a file and a comment after it, so `go fmt ./...` changes every stamped Go file. The next run reports each file as edited and refuses to overwrite it |
| #40 | `spell.Package` gives a test file the external test package of its directory only where the origin of the file is that package. A generator of black-box checks cannot write them into `store_test` beside `store` |

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-core/store` | `RegionInfo.Dependency`, which the load sets for the region of a unit of a dependency round, and `Graph.Dependency` |
| `eidos-core/frontend/load` | The region of each unit of a dependency round has the mark, on a cold load and on a warm load. `Changes.Dependency`, and a comparison that covers the mark |
| `eidos-core/plugin` | `Index.Dependency`, `Unit.Subjects`, `Output.External` and `Placement.External` |
| `eidos-core`, root | The dispatch skips a subject of a dependency package for a rule of the generate phase. The emitter records `Unit.Subjects` |
| `eidos-core/workspace` | The scope of a plan admits each dependency package of its language |
| `eidos-core/layout` | The router takes the routing of the subject where the origin of a declaration has none, and passes `External` of the family of a file to the package rule |
| `eidos-core/rules` | `View.Authored` lifts every shape through `LiteralFor` first, `RefusedAuthored`, and a `Complete` that keeps a refused stated half |
| `eidos-core/output` | `Contract.Stamp` writes one blank line before the trailer, and the reader verifies both layouts |
| `eidos-sdk` | The facade of each addition, which the facade generator writes |
| `eidos-lang-go` | `spell.Package` gives a test file of an external family the external test package of its directory |
| `eidos-conformance`, `eidos-reference`, the backends | The golden files of every stamped file |

### Invariants

1. A rule of the generate phase runs on the declarations of the source packages of its plan, and
   never on a declaration of a dependency package.
2. A reader of a plan returns the declarations that a reader of an unscoped plan of its language
   returns, except those of the source packages that the patterns and the module of the plan
   leave out.
3. A declaration takes the routing of its origin, and the routing of the subject that placed it
   where its origin has none.
4. An authored text lifts to a value that its language reads, to a string for a text type, or to
   a refusal.
5. gofmt leaves a stamped Go file unchanged, and a file of every earlier layout verifies.
6. A test file of an external family declares the package from which an importer tests its
   package, where the target has one.

### Dependency packages

A dependency round loads the packages of a store and the vendored packages that the selection
does not claim, at the depth of signatures. The summary of the region of such a unit has the
mark `RegionInfo.Dependency`, and `Graph.Dependency` reports a package that a dependency round
loaded whole:

```go
// Dependency reports whether a dependency round loaded the package of an
// identity, untracked. A package is a dependency package where
// [RegionInfo.Dependency] marks every region of it. Dependency reports
// false for a package that the regions do not list, and for every package
// of a graph [New] returned. It does not decode a region or allocate.
func (g *Graph) Dependency(id symbol.Identity) bool
```

The record of a warm load contains the round of each unit, so the load sets the mark of a kept
unit as it sets the mark of a parsed one, and the state keeps its format. The comparison of a
warm load covers the mark. A declaration whose unit moves between the partition and a dependency
round is in `Changes.Changed`, so the warm run dispatches it again, and its package is in
`Changes.Dependency`:

```go
// Dependency lists the packages that the load and the record both
// declare, and that a dependency round loads in only one of them.
Dependency []symbol.Identity
```

Each kind of rule treats a dependency package by its phase:

- A rule of the generate phase skips a subject of a dependency package, in every plan, scoped or
  not. A handler still reads such a declaration through its reader, as the member walk reads the
  method set of an embedded interface.
- An annotator runs on every declaration, because every plan can read a fact of a dependency
  declaration, such as the authored sample of a vendored type.
- A rule of the emit phase runs on every emit value of its plan, also where the origin of the
  value is a dependency declaration. The value is the plan's own output, such as the method of a
  double that an embedded interface of the standard library declares.

The scope of a plan admits each dependency package of its language, whatever its patterns and its
module. A plan without a language admits the dependency packages of every language. A reader of a
plan that `./internal/...` narrows then returns `io.Closer`, by identity and in an enumeration,
as the reader of a plan that its language alone scopes returns it.

A dependency package is in the scope by its language alone, so a change of its files does not
rebind the scope of a plan. A change of a dependency declaration that a reader read runs the
reader again, through the edge that the read recorded. A package in `Changes.Dependency` rebinds
the scope of each plan whose verdict on it the move changes, and the plan runs whole. A package
that a dependency round loaded in the generation keeps its admission without a file, because the
run does not track the files of a store.

### The routing of a placement

The emitter records the subject of each declaration whose origin is not the subject of the match
that placed it:

```go
// Subjects maps the origin of each declaration that a match placed for
// another subject to that subject. It is nil where every declaration of the
// unit originates from the subject of its match.
Subjects map[symbol.Identity]symbol.Identity
```

The router reads the routing of the origin of a declaration first. Where the origin has no
routing, the router reads the routing of the subject in `Unit.Subjects`. The subject is then the
origin of the routing:

- An `out` path resolves against the source directory of the subject.
- A move to a family of another cardinality derives its key from the subject.
- A finding about the routing is positioned at the subject.

A lowering keeps the origin of every declaration, so the map applies after the settle as it
applies before it. Where two matches place declarations of one origin in one unit, the map records
the subject of the first placement in the order of the unit.

The double of the handle of #37 is then written with the double of its subject:

| Declaration | Origin | Subject | File |
|---|---|---|---|
| `StoreProbe` | `Store` | `Store` | `store/probetest/store_probe.go` |
| `CursorProbe` | `Cursor` | `Store` | `store/probetest/store_probe.go` |

### Authored literals

`View.Authored` lifts a stated text through the `LiteralFor` of the language of the declaration
that states it, in the scope of that declaration, for every shape. The rules of the view lift a
text of their own language, and the resolver of the view returns the rules of another one. Where
`LiteralFor` does not read the text, the lift depends on the shape of the type:

- A text type lifts to a string with the text as its content.
- A boolean and a scalar refuse with `RefusedAuthored`.
- Every other form lifts to raw text in the language of the declaration.

```go
// RefusedAuthored means an authored text that the rules of its language do
// not read as a value of a boolean or a scalar type.
RefusedAuthored
```

`Complete` keeps a stated half, a refused one included, so the caller receives the refusal of an
authored text instead of a derived value.

The samples of #38 then lift to values of the declarations that the author named:

| Parameter | Authored text | Value |
|---|---|---|
| `tag Tag` | `NewTag()` | a call of `kinds.NewTag` |
| `level Level` | `NewLevel()` | a call of `kinds.NewLevel` |
| `rank Rank` | `Top` | the name of `kinds.Top` |

### The trailer

`Contract.Stamp` writes one blank line between the body and the trailer. The blank line is part of
the frame, as the blank line after the derivation lines is, and the trailer attests the body
without it. gofmt separates a comment after the last declaration of a file by one blank line, so
it leaves a stamped Go file unchanged.

Where the bytes between the frame and the trailer end in a blank line, the reader checks the
digest of the bytes before that line first, and then the digest of the bytes with it. So a file
that an earlier release stamped verifies, also after gofmt added the blank line. The next run
writes the new layout into each intact file once.

We measured go/format of Go 1.27.2 over seven stamped probe files. With the blank line, it left
each of them unchanged. Without it, it changed six of them: a struct, a function, a constant, a
variable block, an import and a package clause alone. A body whose last line ends with a comment
is unchanged in both layouts.

### The external test package

A family states that its files test their package from outside:

```go
// External reports that the files of the family test their package
// through its exported API, as an importer does. The Go target writes each
// test file of the family into the external test package of its
// directory. The other targets write the files of the family as they
// write the files of any family.
External bool
```

The layout passes `External` of the family of a file to the package rule of the target as
`Placement.External`. The Go rule gives a test file of an external family the external test
package of its directory. That is the package with the suffix `_test` that a resident of the
directory declares, or else the package of the directory with `_test` after its path and its
name. A file of such a family that is not a test file keeps the package of its directory, because
Go declares an external test package in test files alone.

A reference from the file into the package under test then crosses into another package. The
layout qualifies it and the file imports the package, as for a reference into any other package.

### Allocations

| Call | Allocations |
|---|---|
| `Graph.Dependency` and `Index.Dependency` | 0 |
| The admission of a subject by the dispatch of a rule of the generate phase | 0 more |
| The comparison of a warm load | the list of the moved packages, and none where no package moves |
| The flush of a unit | the map of subjects for a unit with a declaration whose origin is not its subject, and none for any other unit |
| `layout.Route` of one unit | 51, unchanged |
| `View.Authored` of a text that `LiteralFor` reads | 1, the binding, beside what `LiteralFor` and the read set allocate |
| `View.Authored` of a text of a builtin type that the rules do not read | 3: the binding, the first group of its memo and the shape |
| `Contract.Stamp` | 1, the bytes of the file, unchanged |
| `Contract.Verify` and `output.Read` of either layout | 4, the record, unchanged |
| `spell.Package` of a test file of an external family | 0 where a resident declares the external test package, and 2 for a derived one: its path and its name |

### Verdicts

| Item | Verdict | Reason | Reverses the verdict |
|---|---|---|---|
| A dependency package is a package of a unit of a dependency round (#35, #36) | in | The load reads a vendored package and a package of a store through the rounds alike. A pattern cannot tell a vendored directory from a source directory | A language whose dependency packages its partition returns |
| A rule of the generate phase skips a subject of a dependency package (#36) | in | A generator writes the output of the sources of its plan, and the tooling of a dependency removes an output beside it | none |
| An annotator runs on a dependency declaration (#36) | in | Every plan can read a fact of a dependency declaration | An annotator whose facts of a dependency declaration no plan reads |
| A rule of the emit phase runs on an emit value whose origin is a dependency declaration (#36) | in | The value is the plan's own output | none |
| The scope of a plan admits the dependency packages of its language, whatever its patterns and its module (#35) | in, a departure from D100, which states that no pattern admits a package that a store provides | A reader of a narrowed plan reads what a reader of a plan that its language alone scopes reads | none |
| A declaration whose unit moves between the partition and a dependency round is changed (#36) | in | A rule of the generate phase runs on the declaration in only one of the two runs, so a warm run that kept the invocation would differ from a cold run | none |
| An enumeration of a reader returns the declarations of the dependency packages of its language (#35) | in | A reader of a plan has one scope, and the dispatch skips the subjects that #36 leaves out | A generator that must enumerate the sources of its plan alone |
| The routing of the subject applies where the origin has none (#37) | in | The handler placed the declaration for the subject, so the declaration is part of the output of the subject | none |
| A map from origin to subject, and not a list beside the declarations (#37) | in | A lowering replaces and drops declarations, and it keeps the origin of each | none |
| An origin that two subjects place in one unit takes the subject of the first placement (#37) | in | The order of a unit is defined, so the choice is deterministic | A unit whose subjects route one origin to two directories |
| Every shape lifts through `LiteralFor` first (#38) | in | A name and a call are values of every type, and `LiteralFor` reads them in the scope of the declaration | none |
| The rules of the language of the declaration that states the text lift it (#38) | in | The scope of the text is the declaration that states it | none |
| A boolean and a scalar refuse a text that `LiteralFor` does not read (#38) | in | Such a text is no value of the type, and a backend writes the text of a number verbatim | none |
| `Complete` keeps a refused stated half (#38) | in | A derived value would replace the authored text without a report | none |
| One blank line before the trailer in every target (#39) | in, a departure: the issue states a Go file | The frame has one layout in every target, and the formatter of Go needs the blank line | A formatter of a target that removes a blank line before a comment |
| A file of the earlier layout verifies (#39) | in | Every generated file of a release would read as edited otherwise | none |
| `External` is a field of the family (#40) | in, a departure: the issue also names a marker in the header | A plugin states the package once for a family, and the Go backend refuses the annotations of a header | A generator that states the package of one file of a family |
| The external test package derives where no resident declares it (#40) | in | The first black-box test of a directory has no resident of that package | none |
| A file of an external family that is not a test file keeps the package of its directory (#40) | in | Go declares an external test package in test files alone | none |

### Failures

| Condition | Report |
|---|---|
| A subject of a dependency package for a rule of the generate phase | none: the dispatch skips it |
| An authored text of a boolean or a scalar that `LiteralFor` does not read | the half refuses with `RefusedAuthored` |
| A file that an earlier release stamped | none: it verifies, and the next run writes the new layout |
| A test file of an external family in a directory that no module and no import base contains | the error of the Go package rule, as for any file |

### Migration

1. Mark the regions of the dependency rounds in the load, add `Graph.Dependency` and
   `Index.Dependency`, and list the moves between the partition and a dependency round in
   `Changes.Dependency`.
2. Skip the dependency subjects of a rule of the generate phase in the dispatch, and admit the
   dependency packages of the language of a plan in its scope.
3. Record `Unit.Subjects` in the emitter, and read the routing of the subject in the router.
4. Lift every authored shape through `LiteralFor` first, add `RefusedAuthored`, and keep a
   refused stated half in `Complete`.
5. Write the blank line before the trailer, verify both layouts, and update the golden files.
6. Add `Output.External` and `Placement.External`, and read them in the Go package rule.
7. Regenerate the facade.

## Alternatives

### The dependency packages in the workspace alone

The workspace could compute the dependency packages from the report of the load and pass them to
the scope of each plan. The dispatch of an annotator and the index of a plan would then need the
set as well. The graph contains every package already, so the mark is one bit of a region.

### Rules that skip every dependency declaration

An annotator and a rule of the emit phase could skip a dependency declaration too. No plan could
then read the authored sample of a vendored type, and a weaver would skip the method of a double
that an embedded interface of a dependency declares.

### A list of subjects beside the declarations

A unit could list the subject of each declaration at the index of the declaration. A lowering
replaces one declaration with two or more and drops a refused one, so the list would follow every
rewrite of the settle. The map follows the origin, which a lowering keeps.

### A direct lift that checks the text

The kernel could keep the direct lift of a text, a boolean and a scalar and check that the text is
a literal. A name and a call are values of these types too. The kernel cannot read them in the
scope of the declaration, because that takes the rules of the language.

### A blank line for Go alone

The contract could write the blank line for the Go target alone. The frame would then have two
layouts, and the reader would need the target of a file to verify it.

### A marker in the header of a file

The Go satellite could read a marker such as `golang:package` from the header of a file, as the
protobuf satellite reads `protobuf:package`. A plugin would state it on every file of the family,
and the Go backend refuses the annotations of a header.

## Drawbacks

- Every stamped file of every target changes once by the blank line before its trailer, and so
  does every golden file.
- The dispatch of a bare rule of a narrowed plan crosses the declarations of the dependency
  packages of its language and skips them, as the dispatch of an unscoped plan does.
- A package that moves between the partition and a dependency round runs each plan whole whose
  scope the move changes.
- The rules read an authored text that is the name of a declaration of its scope as that
  declaration. A string sample `value=release` lifts to the constant `release` where the scope
  declares one.
- `plugin.Unit`, `plugin.Output` and `plugin.Placement` each gain a field, and `rules.Refusal`
  gains a value.

## Open questions

None.

## Unresolved and future work

None.

## References

- The issues: [#35](https://github.com/dokimasia/eidos/issues/35),
  [#36](https://github.com/dokimasia/eidos/issues/36),
  [#37](https://github.com/dokimasia/eidos/issues/37),
  [#38](https://github.com/dokimasia/eidos/issues/38),
  [#39](https://github.com/dokimasia/eidos/issues/39) and
  [#40](https://github.com/dokimasia/eidos/issues/40).
- [RFC-0010](0010-output-contract.md), the frame and the trailer of a stamped file.
- [RFC-0019](0019-source-scopes-exports-and-checks.md), the source scope of a plan.
- [RFC-0014](0014-the-projection-vocabulary.md), the lift of an authored value.
- [RFC-0016](0016-layout-and-routing.md), the routing of a declaration by its origin.
- [RFC-0051](0051-the-names-of-declarations-as-values.md), a name and a call as values.
- [21-decisions.md](../architecture/21-decisions.md), D100: the source scope of a plan, and D142:
  an authored sample of a type without a literal of its own lifts through `LiteralFor`.
- [17-output-and-determinism.md](../architecture/17-output-and-determinism.md), the provenance
  trailer.
- cmd/go, test packages: <https://pkg.go.dev/cmd/go#hdr-Test_packages>
