---
rfc: 0070
title: Names collide in the routed package
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-19
updated: 2026-10-20
discussion: none
supersedes: none
superseded-by: none
produces-adr: ADR-0061
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
layout. The layout knows the routed package of every file, and the target
names the namespace of the names of each file. The settle keeps the check
of members, because a member collides in its host wherever the host is
routed.

## Motivation

`resolveTop` in `eidos-core/plugin/settle.go` keys the scope of a
file-level name by `Unit.Pkg.Package`. That is the source package for a
unit of a source file or a package, and the empty string for a unit of
the plan. The layout runs after the settle and sets `File.Pkg` through the
`Packager` of the target. The `Packager` returns the package that the
frontend of the target names when it reads the routed file:

| Target | The package of a routed file | The namespace of its names |
|---|---|---|
| Go, Java | the directory of the file | the package |
| TypeScript | the file, because every file is a module | the package |
| Rust | the module of the file | the package |
| protobuf | the file, because a file imports a file | the protobuf package that the file declares |
| GraphQL | one package for every file of the plan | the package |

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
once in its namespace. The namespace is the package of the file, except in
protobuf, where the files of one protobuf package share their names.

## Detailed design

### Components

| Component | Responsibility after this RFC |
|---|---|
| `plugin` settle | Respells every name, applies the overrides, checks the members of each host, and builds the reference tables. It no longer checks file-level names |
| `plugin.NameScoper` | Is the optional role of a target that names a namespace apart from the package of a file |
| `layout` | Names the namespace of each routed file beside its package, checks the file-level names of every namespace, and returns the names with the files |
| `plugin.Names` | Returns the kept names of one routed scope |
| `backend` kit | Declares the namespace rule of a target through `Builder.NameScopes` |
| `eidos-lang-protobuf` | Names the protobuf package of a file as its namespace |
| `internal/state` | Records the namespace of each file, and each kept name under its routed scope as well |
| `workspace` | Records the names that the layout returns |

### Invariants

1. Each settled file-level name occurs once in its routed scope. The scope
   is the namespace of the routed file, and for a method or a received
   field also its receiver. The overloads of one function in a target
   whose language overloads functions share one name.
2. A collision is reported once, under `CollidingNames`, as an Error of the
   layout. The plan that reports it commits nothing, because a plan with
   an Error does not commit.
3. A warm run reports the same collisions as a cold run over the same tree.
4. The check reads the settled names. It does not change a name, so a
   reference does not have to follow one.

### The scope of a file-level name

The scope of a file-level declaration is the namespace of its routed file,
`File.Scope`. A method with a receiver and a field that another type
receives have the scope of that namespace and the emitted spelling of the
receiver, as `NameEntry.Receiver` records it. A record of a family that
claims its files does not declare a name. It does not join a scope. A
file without a package, for which the target derives none, has no
namespace, so its names join no scope.

### The namespace of a file

The layout derives the namespace of each file after its package:

```go
// NameScoper is the role of a target whose file-level names share one
// namespace across the files of different packages. The layout checks
// the file-level names of a routed file in the namespace that NameScope
// returns for the package of the file, and in the package of the file
// where the target does not implement the role.
type NameScoper interface {
	// NameScope returns the namespace of the file-level names of a file
	// that declares the package pkg.
	NameScope(pkg symbol.Identity) symbol.Identity
}
```

`plugin.File.Scope` records the namespace beside `File.Pkg`. A backend of
the kit declares its rule through `Builder.NameScopes`, and the built
backend serves it as `NameScoper`. A kit backend without a rule returns
the package. The protobuf backend returns the protobuf package that a file
declares, the `Name` of its package, so the files of one protobuf package
check their names in one namespace. Every other backend of the hub uses
the package.

The check orders the names of a scope by the path of their file. The
names of one file follow the order of their declarations, and the kept
names of the scope take part in the same order. The first declaration of
a settled name in a scope opens a group. Each later declaration of the
same name joins it. The finding of a group is positioned at the origin of
its second declaration, with the origins of the others as related:

```text
svc/a.User in schema/a.graphql and svc/b.User in schema/b.graphql settle to "User" in one package
```

The layout reports the finding and keeps every declaration of the group.
The Error fails the plan, so no file of the group is written.

A group of functions of one emitted name is no collision where the target
implements the role `plugin.Overloader` and its `Overloads` reports true,
because its declarations are the overloads of one function. Python
declares the overloads of a function this way. Two emitted names that
settle to one name still collide, and so does a function beside a
declaration of another kind. The backend of such a target checks the
overloads of a name itself.

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
	// settled name: the names that the kept files of the namespace scope
	// declare under receiver, and every other file-level name of those
	// files where receiver is empty.
	InFileScope(scope symbol.Identity, receiver string) []NameEntry
}
```

`InScope` keeps its source scope. A bare reference and the spelling of a
receiver resolve in the source package. The artifact of a file records its
namespace. The record lists each name a second time under the plan, the
namespace of its file, the receiver and the settled name, so `InFileScope`
reads the rows of one routed scope in the order of their settled names.

### Warm runs

A warm run routes the files of its dirty groups alone. The layout of each
round compares their names with the kept names of each routed scope, and
the kept names leave out every file of a dirty group. Kept files do not
collide with each other, because the run that wrote them committed. So
the layout of the last round finds every collision that a cold run finds.
Invariant 3 follows.

The check does not change a name, and a render reads the names of its own
file alone. So a kept file is not rendered again when a name of its routed
scope changes. A group does not read an edge of its routed scope.

### Migration

- `resolveTop` keeps building `table` and `byOrigin`, and drops the
  `first` and `groups` maps with their finding. Its doc comment and the doc
  comment of `SettleWith` state the new split.
- The cases of `plugin/settle_test.go` that assert a file-level
  `CollidingNames` move to `layout/collision_test.go`, with a routed package
  in each fixture. The member cases remain.
- The workspace uses `Routes.Names` in `groups.go` and in
  `warmPlan.respelled`, in place of its own call of `plugin.NamesOf`.
- The protobuf backend declares `NameScopes(spell.NameScope)`.
- The artifact of the sealed state records `Scope` after `Pkg`.
- The SDK facade regenerates for `plugin.Names`, `plugin.NameScoper`,
  `plugin.File.Scope` and `Builder.NameScopes`.

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

### The namespace on each name entry

Each `NameEntry` could record the namespace of its file, so the check
would read it from the entry.

**Why not:** the namespace is a fact of the file, which `File.Scope` and
the artifact record once. A field of each entry costs 88 bytes for each
of the 200,000 names of the canonical corpus. The check finds the file of
each name in one walk of the files and the names, which are in one order.

### An edge of each routed scope

A group could read an edge of the routed scope of each name of its files,
and a changed name could make the edge dirty, as a changed name makes the
edge of its source scope dirty.

**Why not:** the layout reads the kept names of a routed scope without a
render of their files, and no render reads the names of the other files
of its package. The edge would render every file of a routed scope again
for each new name. Every file of a GraphQL plan is in one namespace, so a
new type would render the whole plan again.

## Drawbacks

- The settle no longer reverts a respelled collision. Where two names
  settle to one name in one routed package, the render writes the settled
  names, and the settle wrote the emitted names before. The plan commits
  nothing in both designs, so no written file differs.
- The record lists each file-level name twice. A corpus of 200,000
  file-level declarations records 200,000 more rows.
- `layout.Route` changes its result type. The workspace and four calls in
  `layout/route_test.go` change with it.
- The allocation ceilings of `Route` grow by three allocations of a pass:
  the entries and the sorted declarations of `plugin.NamesOf`, which the
  workspace no longer allocates, and the order of the check.
  `BenchmarkRoute` measures 30,180 allocations over 200,000 declarations
  and 54 over one package of 200.
- `plugin.Names` gains a method, so a type outside the kernel that
  implements it adds `InFileScope`.
- The kernel gains one optional role, one field of a routed file and one
  builder method of the kit. The artifact of each file records its
  namespace, so the format of the sealed state changes.

## References

| What | Where |
|---|---|
| graphql-js 17.0.2, the rule that refuses two types of one name | <https://github.com/graphql/graphql-js/blob/v17.0.2/src/validation/rules/UniqueTypeNamesRule.ts> |
