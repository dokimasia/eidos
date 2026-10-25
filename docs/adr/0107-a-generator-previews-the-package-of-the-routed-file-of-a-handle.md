---
adr: 0107
title: A generator previews the package of the routed file of a handle
status: Accepted
date: 2026-10-25
supersedes: none
superseded-by: none
rfc: RFC-0086
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0107: A generator previews the package of the routed file of a handle

## Status

Accepted

## Context

The layout derives the package of each routed file after every generator of the plan has run, and a
generator records its declarations before that.

- `Bound.Importable(pkg, ref)` takes the package of the file that the generator writes into. Under a
  route, that package is the package of a directory that the generator does not know.
- The suite generator of dokimi writes the conformance test of a struct under `out` into the external
  test package of the `out` directory. It compares the package of the factory of the test with the
  external test package of the directory of the struct, so it refuses a factory that the test can
  call and accepts one that the test cannot import.
- The route of a declaration reads the directives of the plugin and of the kernel on the subject, the
  kernel directive `out` on its package and the configuration of the plan. The package rule of the
  target reads the residents of the directory, the modules, the import base of the plan and the name
  of the file. A plugin does not read any of these.
- A warm run executes a dirty group again with the invocations that contributed to it, and a group
  records the routing edges of its files. A unit that a generator leaves without declarations has
  no file, so no group reads its directory.

## Decision

We will let a generator read the package of the routed file of a handle through `Out.Package`,
because the handle has the family, the key and the internal mark that the package depends on. The
layout of the plan previews the route through the role `plugin.Placer` of the generator context, and
the invocation records the routing edges of each preview.

- `layout.NewPlacer` returns the placer of one plan. A preview applies the steps of `Route` to one
  declaration that takes the routing of the subject of the handle: the override on the subject and
  on its package, the destination under the configuration, the filename and the package rule of the
  target. It derives the filename of a unit without declarations and does not read the header of
  the file.
- `Out.Package` returns the package with the name of its package clause. After `Out.Internal` it
  returns the package of the internal test file of the family. An error has the message of the
  finding of the layout. In a plan that does not write files, `Out.Package` returns an error.
- `Invocation.Placed` lists the previews of a call. The lane records, for each preview, the
  declaration edges of its origin and of the package of the origin, the residents edge of its
  directory, the modules edge, and the files edge of the package of a family of one file per
  package. The state format does not change.
- Each generator call of a plan that writes files takes the placer of the plan. A call that does not
  journal an invocation of its own runs again on every change that a warm run finds, so its
  invocation under `plugin.WholeCall` does not list a preview.
- `plugintest.Fixture.Generate` previews the routes of the layout of the fixture where its spoke is a
  backend with a filename speller and a package rule.

## Alternatives Considered

### A second generation after the layout

The workspace would route the store and run the generators again with the packages of their files.
Every plan would run its generators twice, and the second run can change the routes again.

### The package in the plugin

Each plugin would derive the package itself. A plugin does not read the residents, the modules or
the configuration of its plan, and it would copy the routing of the layout and the package rule of
each target.

### A preview on the emitter by tag

The package also depends on the internal mark of the handle, which the emitter does not have.

## Consequences

**Positive:**

- A generator passes the package of its routed file to `Bound.Importable`, under a route too. The
  suite generator of dokimi can check the factory of a struct under `out` against the external test
  package of the `out` directory.
- A warm run executes an invocation again where the package of its preview can change, also where the
  invocation left its unit without a file.

**Negative:**

- A preview routes one declaration that the layout routes again with the store.
- A preview does not read the header of the file. The protobuf target previews the package that its
  rule derives without a header, also where a generator states a package in the header.
- The layout reports an override whose path is a file for two or more families of the subject only
  when it routes the store. A preview does not report it.
- An invocation that previews a route runs again when a file of its destination directory appears,
  vanishes or changes its package, and when the modules change.

**Neutral:**

- The layout routes, names and places the files of a plan as before.

## References

| What | Where |
|---|---|
| D225, the row of this decision | ../architecture/21-decisions.md |
| RFC-0086, the package of the routed file of a handle | ../rfc/0086-the-package-of-the-routed-file-of-a-handle.md |
| ADR-0105, the import rule of the target in the layout | 0105-the-layout-refuses-a-declaration-whose-file-cannot-import-a-reference.md |
| The routing and the layout | ../architecture/18-routing-and-layout.md |
| Issue 80, a reference that the file cannot import discards every record of the file | <https://github.com/dokimasia/eidos/issues/80> |
| dokimasia/dokimi#24, a struct under out cannot take its factory from the external test package of its kit | <https://github.com/dokimasia/dokimi/issues/24> |
