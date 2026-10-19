---
rfc: 0070
title: Names collide in the routed package
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Draft
created: 2026-10-19
updated: 2026-10-19
discussion: none
supersedes: none
superseded-by: none
produces-adr: tbd
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0070: Names collide in the routed package

## Summary

The settle checks whether two file-level names collide in the package that
a unit derives from. The layout routes the units after the settle, and a
target reads every name in the package of its routed file. Two source
packages can write one name twice into one routed package without a
finding. The settle can also refuse two names of one source package that
collide in no file. This RFC moves the check of file-level names into the
layout. The layout knows the routed package of every file. The settle
keeps the check of members, because a member collides in its host wherever
the host is routed.

## Motivation

`resolveTop` in `eidos-core/plugin/settle.go` keys the scope of a
file-level name by `Unit.Pkg.Package`. That is the source package for a
unit of a source file or a package, and the empty string for a unit of
the plan. The layout runs after the settle and sets `File.Pkg` through the
`Packager` of the target. The `Packager` returns the package that the
frontend of the target names when it reads the routed file:

| Target | The package of a routed file |
|---|---|
| Go, Java | the directory of the file |
| TypeScript | the file, because every file is a module |
| Rust | the module of the file |
| protobuf | the package that the file declares |
| GraphQL | one namespace for every file of the plan |

The scopes differ, with these effects:

- **A collision without a finding.** The Go packages `svc/a` and `svc/b`
  each declare `type User struct`. A plan toward GraphQL writes both
  types into the one namespace of the plan, so the schema declares
  `type User` twice. The settle compares `svc/a` with `svc/b` and reports
  nothing. The rule `UniqueTypeNamesRule` of graphql-js 17.0.2 refuses the
  schema. A layout refinement that routes two Go packages into one
  directory has the same effect toward Go, and two files whose headers
  state one proto package have it toward protobuf.
- **A refusal without a collision.** One Go package declares
  `user_record` in one file and `userRecord` in another. A plan toward
  TypeScript with a unit for each source file writes one module for each
  file. The respell spells both names `UserRecord`, and each module
  declares one of them. The settle reports `CollidingNames` in the Go
  package and reverts both names to their emitted spellings.

The first effect writes a file that the toolchain of the target refuses.
The second refuses a plan that writes valid files.

The check belongs in the layout, because the layout is the first step that
knows the routed package of every declaration. It does not belong in a
backend. The rule is the same in every target: each file-level name occurs
once in its package.

## Detailed design

### Components

| Component | Responsibility after this RFC |
|---|---|
| `plugin` settle | Respells every name, applies the overrides, checks the members of each host, and builds the reference tables. It no longer checks file-level names |
| `layout` | Checks the file-level names of every routed package after it names the packages, and returns the names with the files |
| `plugin.Names` | Returns the kept names of one routed scope |
| `internal/state` | Records each kept name under its routed scope as well, and hashes the edge of each routed scope |
| `workspace` | Records the names that the layout returns. A group reads the routed edge of each name of its files, and a changed name makes that edge dirty |

### Invariants

1. Each settled file-level name occurs once in its routed scope. The scope
   is the routed package, and for a method or a received field also its
   receiver.
2. A collision is reported once, under `CollidingNames`, as an Error of the
   layout. The plan that reports it commits nothing, because a plan with
   an Error does not commit.
3. A warm run reports the same collisions as a cold run over the same tree.
4. The check reads the settled names. It does not change a name, so a
   reference does not have to follow one.

### The scope of a file-level name

The scope of a file-level declaration is the package of its routed file,
`File.Pkg`. A method with a receiver and a field that another type
receives have the scope of that package and the emitted spelling of the
receiver, as `NameEntry.Receiver` records it. A record of a family that
claims its files does not declare a name. It does not join a scope.

The check orders the names of a scope by the path of their file. The
names of one file follow the order of their declarations, and the kept
names of the scope take part in the same order. The first declaration of
a settled name in a scope opens a group. Each later declaration of the
same name joins it. The finding of a group is positioned at the origin of
its second declaration, with the origin of the first as related:

```text
svc/a.User in schema/a.graphql and svc/b.User in schema/b.graphql settle to "User" in one package
```

The layout reports the finding and keeps every declaration of the group.
The Error fails the plan, so no file of the group is written.

### The layout

`Route` returns the names that it checked. The workspace records them, so
it does not walk the files a second time:

```go
// Routes is the result of routing one plan: the files and the
// file-level names that they declare.
type Routes struct {
	// Files are the routed files, sorted by path.
	Files []plugin.File
	// Names are the file-level names of Files, as [plugin.NamesOf]
	// returns them.
	Names []plugin.NameEntry
}

// Route resolves the declarations of one settled plan to the files they
// are written in, and returns the files with their file-level names. It
// reports [plugin.CollidingNames] for two file-level
// declarations of one routed scope with one settled name. Where
// Input.Others is set, it compares the names of the routed files with
// the names of the kept files of each scope too.
func Route(in Input) (Routes, error)
```

### The kept names

A warm run routes the files of its dirty groups alone. The check also
reads the names of the kept files of each scope that a routed name is in:

```go
type Names interface {
	InPackage(pkg, emitted string) (NameEntry, bool)
	OfOrigin(origin symbol.Identity, emitted string) (NameEntry, bool)
	InScope(pkg, receiver string) []NameEntry

	// InFileScope returns the entries of one routed scope, sorted by
	// settled name: the names that the kept files of the package pkg
	// declare under receiver, and every other file-level name of the
	// package where receiver is empty.
	InFileScope(pkg symbol.Identity, receiver string) []NameEntry
}
```

`InScope` keeps its source scope. A bare reference and the spelling of a
receiver resolve in the source package. The record lists each name a
second time under the plan, the routed package, the receiver and the
emitted name. `state.FileScopeEdge(plan, pkg, receiver)` hashes the
routed scope.

### Warm runs

A group reads the routed edge of each name of its files, beside the edge
of its source scope. `warmPlan.nameChanged` makes the routed edge of a
changed name dirty. A name compares with its routed file and package
already, so a name that moves into another routed package counts as
changed. The rounds of the warm plan run until no group joins, so the
layout of the last round checks every routed scope that changed against
the kept names of the scope. Invariant 3 follows.

### Migration

- `resolveTop` keeps building `table` and `byOrigin`, and drops the
  `first` and `groups` maps with their finding. Its doc comment and the doc
  comment of `SettleWith` state the new split.
- The cases of `plugin/settle_test.go` that assert a file-level
  `CollidingNames` move to `layout/route_test.go`, with a routed package in
  each fixture. The member cases remain.
- The workspace uses `Routes.Names` in `groups.go` and in
  `warmPlan.respelled`, in place of its own call of `plugin.NamesOf`.
- The SDK facade regenerates for `plugin.Names` and the new method.

## Alternatives considered

### The routed package before the settle

The settle could key each name by a routed package that the layout derives
first.

**Why not:** the path of a routed file can depend on a settled name. A
backend can split a unit into one file for each type and name each file
after its type. The `Packager` reads the routed path, so the routed
package of a unit is not known before its names are.

### Both checks

The settle could keep its check by source package, and the layout could
add the check by routed package.

**Why not:** the settle still refuses the names of the second effect, and
a collision whose two scopes agree is reported twice.

### A check in each backend

Each backend could check the names of its files in the render.

**Why not:** the rule is the same in every target, and six backends would
each contain a copy of it.

## Drawbacks

- The settle no longer reverts a respelled collision. Where two names
  settle to one name in one routed package, the render writes the settled
  names, and the settle wrote the emitted names before. The plan commits
  nothing in both designs, so no written file differs.
- The record lists each file-level name twice. A corpus of 200,000
  file-level declarations records 200,000 more rows and the edge of each
  routed scope.
- `layout.Route` changes its result type. The workspace and four calls in
  `layout/route_test.go` change with it.
- The allocation ceilings of `Route` grow by what `plugin.NamesOf`
  allocates, which the workspace no longer allocates. A run allocates the
  same, and the ceilings of `BenchmarkRoute` and `TestRouteAllocs` change.
- `plugin.Names` gains a method, so a type outside the kernel that
  implements it adds `InFileScope`.

## References

| What | Where |
|---|---|
| graphql-js 17.0.2, the rule that refuses two types of one name | <https://github.com/graphql/graphql-js/blob/v17.0.2/src/validation/rules/UniqueTypeNamesRule.ts> |
