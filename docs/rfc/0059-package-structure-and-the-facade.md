---
rfc: 0059
title: Package structure and the facade
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-13
updated: 2026-10-13
discussion: none
supersedes: none
superseded-by: none
produces-adr: ADR-0038
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0059: Package structure and the facade

## Summary

The improvement plan lists 24 items about the packages and the APIs that an author of a plugin or
of a command learns: splits of packages and types, changes of the facade for plugin authors, and
entry points for consumer commands. We propose a verdict for each. A split of a package that every
design reads is an optional refactor, so the structural items are out, and the proposed decision D-9
is out with them. The root package of each satellite is the importable vocabulary of its language,
so the proposed decision D-8 is out too. The builder of the facade completes the plugin of a
directive schema, the doubles of the reference ensemble take their targets from the satellites, and
`cli.Run` runs a command line and returns its status. ADR-0038 records the size of the facade
against the estimate of RFC-0006.

## Motivation

- The plan proposes D-9, which moves the parts of `workspace` and `plugin` into internal packages
  behind their stable API. Every design of the improvement plan reads both packages, and no defect
  needs a boundary inside them.
- The plan proposes D-8, which adds a helper package to each satellite and a lint that refuses its
  import in a neutral file. The architecture describes both as Tier 3 of the projection layer, and
  neither exists.
- The facade has 979 exported package-level identifiers in 23 packages. An annotator or a
  generator imports eleven of them, with 745 identifiers. RFC-0006 estimated roughly fifteen
  hand-written symbols and ten generated Match types for the root package. The root package has
  63 identifiers, and 12 of its Match types are generated.
- Thirteen directive schemas of the reference plugins and the shape catalog state
  `Plugin: string(ID)`, and the workspace refuses any other value.
- The tests of `eidos-cli` run each case of the dispatch as a process of the test binary, one
  process at a time, because `Main` exits and the dispatch is unexported.
- The doubles of the reference ensemble restate the targets of the Go and the TypeScript satellites
  as `"golang"` and `"typescript"`.

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-core` (the facade's builder) | A schema of `Directive` without a plugin takes the name of the plugin that the builder lowers |
| `eidos-reference`, `eidos-plugin-shape` | The schemas of the plugins leave the plugin out |
| `eidos-reference/internal/double` | `GoTarget` and `TypeScriptTarget` are the targets that the root packages of the two satellites export |
| `eidos-cli` | `Run` runs a command line and returns the status of its command, and `Main` calls it |
| `docs/architecture/03-projection.md` | Tier 3 is the root package of each satellite |
| `docs/architecture/13-testing-and-conformance.md` | The paragraph on a Tier-3 import lint states the depguard rules that keep a module off the kernel or off the satellites |
| ADR-0038 | The size of the facade |

### Verdicts

| Row | Item | Verdict | Reason | Reverses the verdict |
|---|---|---|---|---|
| W7.1 | Internal packages behind the stable API of `workspace` (D-9) | out | Every design of the waves reads the package, and a split moves its 30 files behind new import paths without a defect to fix. The callers keep the API of the composition, so an internal package changes no caller | A defect whose fix needs a boundary inside the package |
| W7.2 | The frontend kit, the backend kit and the run journal of `plugin` as packages of their own (D-9) | out | Every satellite, plugin and suite imports `plugin`, and the facade re-exports it whole. A split moves its halves behind new import paths in every module of the repository and of every consumer, and fixes no defect | A consumer that needs one half and must not load the other |
| W7.3 | Smaller types in place of the lowering of the TypeScript frontend | out | The type walks one parse tree, and its seven files already part it by concern: the statements, the declarations, the members, the signatures, the types, the markers and the comments. Types of their own would pass the state of the walk between them | A defect that two concerns cause through the shared state of the walk |
| W7.4 | Resolution and the receiver rule out of the root package of the Go satellite | out | The root package is the Tier-3 vocabulary of Go, which the frontend, the rules, the backend and the reference plugins import. A move of the import scope or the receiver rule changes their imports and no behaviour | A Go helper that a consumer needs without the rest of the vocabulary |
| W7.5 | Comment writing, import writing and normalization of `eidos-lang/textfmt` apart | out | The backends call the package as one set of text forms, and a split changes their imports and no output | A satellite that needs one of the forms and must not load the others |
| W7.6 | One binding model of `render.ImportSet`, or one type per model | out | `Bind`, `BindItem` and `Claim` reserve their names in one table, whose collisions every target reads, and RFC-0037 binds the import of a template through the same table. One type per model would copy the table | A target whose binding model conflicts with the shared table |
| W7.7 | A narrower `load.Prior`, or the record type in `load` | out | The load declares the interface that it reads, and the state implements it, so `load` imports nothing of the state. The seven methods are the reads of a warm load | A second implementation that implements fewer reads |
| W7.8 | The decode of a config file apart from the load of its plugin directories | out | The directories of the declarative plugins are part of the document, and `Apply` applies a document to a builder in one step. A move of the load into `open` changes no command | A consumer that decodes a config file without eidos-declarative |
| W7.9 | A text and a JSON renderer behind one interface, and a separate tally | out | One renderer writes each event in the format of the run, and the totals count the events that it writes. Two renderers would copy the totals | A third output format |
| W7.10 | The help methods of `Command` as an optional interface | out | The help of a binary lists each command with its synopsis and prints its usage, so a command without them would be missing from the help | A command that a binary leaves out of its help |
| W7.11 | A curated list of the identifiers of the facade | out | The facade re-exports the plugin-facing packages whole, as RFC-0012 states, and its generator keeps it equal to the kernel. A list beside it would hide nothing, because the import paths of the kernel stay public, and the import lint keeps plugin code on the paths of the facade. ADR-0038 records the surface | A consumer that needs a published surface narrower than the packages |
| W7.12 | An import lint for the modules of plugins | closed | The rule `plugin-modules` of `.ergon/local/.golangci.yml` denies `go.dokimi.dev/eidos/core` to the satellites, the shape catalog and the reference plugins. It leaves out the composition, the binary and the test kit of the reference ensemble and the tools of the shape catalog, which are host side, as RFC-0012 states. eidos-declarative is a library of the command line: the config step of `eidos-cli` loads the plugin directories of a config file through it, and its tests compose a workspace. The command line imports the kernel, as a host does | A plugin module of the repository that the rule does not cover |
| W7.13 | Provider interfaces with one implementation | closed | `CapabilityProvider`, `TemplateProvider`, `KeyBinder` and `Subscribed` are part of the SPI, which the language guide keeps public for the plugins that the builder of the facade does not build. The plugin of the builder implements them | A release that removes the SPI for plugins outside the builder |
| W7.14 | A helper package of each satellite, and a lint of binding files (D-8) | out | The root package of each satellite is its Tier-3 vocabulary: the identity, the target, the fact keys and the naming rules. The reference plugins import it where they register their Go templates and read `golang.SatisfiesErrorKey`. A fact that two neutral consumers read is a Tier-2 capability, such as `EnumRules` and `ErrorTypeRules`. A lint of binding files needs a declaration of those files, and sentinel registers its Go templates in the file of its rules. The rule `shape-catalog` of the lint configuration denies the satellites to the shape catalog, which is neutral as a whole | A neutral plugin that reads a fact of one language outside a capability |
| W7.15 | An exported helper that cuts the template tree of a target | out | `fs.Sub` of the standard library cuts the tree, and the helper of the ensemble adds a panic on a path that the embedding of the plugin fixed when it compiled | A plugin whose template directory a consumer chooses at run time |
| W7.16 | Target constants in the facade | out for the facade | A target is the identity of a satellite, and the root package of each satellite exports it, as `golang.Target`. The facade names no satellite. The doubles take the two targets from the root packages of the satellites, as middlewaregen and sentinel take `golang.Target`, and the two root packages import the facade and the standard library alone | A target that no satellite of the repository declares |
| W7.17 | One call that registers a key and returns its handle | out | A plugin claims its namespace once and registers each key with its spec, and each instance of a plugin keeps its own handle, because an instance belongs to one workspace. Each of the five plugins registers its keys in one short function | A plugin that registers the same key in two or more instances of one workspace |
| W7.18 | A schema of a directive without the name of its plugin | in | The builder of the facade knows the name of the plugin, and the workspace refuses any other value. The builder completes the schema before it derives the canonical spelling that the dispatch matches, so `Directives()` of the built plugin returns the complete schema | none |
| W7.19 | One tag for the keys of the options and of the config document | out | The tag `opt` names a key of the options of a plugin, which the options step and the published schema read. The tag `yaml` names a field of the config document, which yaml.v3 decodes. One tag would make either decoder read the tags of the other | An option that the config document decodes through yaml.v3 |
| W7.20 | The workspace suites and the load in the facade | out | The tests of the reference ensemble compose a workspace, which is host side, as RFC-0012 states for the composition. The facade re-exports the kits that test a plugin: `plugintest`, `backendtest`, `frontendtest`, `rulestest`, `nodetest` and `toolchain` | A plugin module outside this repository that tests its plugins through the workspace suite |
| W7.21 | The partial result of `open` as an exported function | out | `doctor` is a command of the kernel, and no consumer command continues with the members that built. `Open` returns the error of the first member that fails | A consumer command that continues with the members that built |
| W7.22 | The rendering of a run report for a consumer command | out | No consumer command runs a workspace and reports its files. A consumer runs the kernel command `run` | A consumer command that runs a workspace and reports its files |
| W7.23 | The flag grammar and the help layout for a consumer command | out | A consumer command parses its own flags with the package `flag`, and registers the shared flags that `Main` passes it through `Flags.Register`, which is exported | A consumer command whose flags need the grammar of the kernel commands beyond the shared flags |
| W7.24 | An entry point that returns the status | in | The tests of the command line run the dispatch in the test process through it, and run as processes only what `Main` adds | none |

### The plugin of a schema

`Directive(schema, rules...)` takes a schema whose `Plugin` is empty or the name of the plugin. The
builder lowers each wrapper once. Where the schema of a wrapper has no plugin, the builder gives a
copy of it the name of the plugin, and the rules under the wrapper share that copy. The dispatch
matches the rules under the wrapper by the canonical spelling of the copy, and `Directives()`
returns the copy, which the workspace registers. The workspace refuses a schema that names another
plugin, as before.

### The entry point of the command line

`cli.Run(ctx, stdio, compose, args, extra...)` runs the command line `args`, whose first element is
the path of the binary as `os.Args` has it, and returns the status of the command. It checks the
commands and panics for each defect of the binary that `Main` panics for, and for `args` without an
element. `Main` builds the process IO and the context of the signals, calls `Run` with `os.Args`,
and exits with the status. A failure of the process IO exits with `StatusFailed`, as before.

The spec of `main.go` runs each case of the dispatch through `Run`. It runs as processes of the
test binary the cases that need a process: the exit status of `Main` and the failure of its process
IO.

### The size of the facade

ADR-0038 records the counts of the facade, measured with the parser of Go over the files of
`eidos-sdk` that are not tests:

| Package | Identifiers |
|---|---:|
| `plugin` | 150 |
| `symbol` | 126 |
| `rules` | 113 |
| `emit` | 89 |
| `directive` | 76 |
| the root package | 63 |
| `meta` | 43 |
| `node` | 41 |
| `diag` | 26 |
| `store` | 17 |
| `position` | 1 |
| The eleven packages of an author | 745 |
| The 23 packages of the facade | 979 |

### Tier 3

The projection document states that the root package of each satellite is Tier 3: the identity of
the language, its target, its fact keys and its naming rules. A plugin imports it where it binds
the language, such as the registration of its templates of the target. A fact that two neutral
consumers read is a Tier-2 capability. The testing document states that the depguard rules of the
repository keep the kernel out of the modules of plugins and the satellites out of the shape
catalog, which is neutral as a whole.

### Failures

| Condition | Report |
|---|---|
| `Run` with an empty command line | A panic, because a binary passes `os.Args`, which has the path of the binary |
| A schema that names another plugin | The fault of the workspace's Build, as before |

### Migration

1. Complete the plugin of a schema in the builder, and leave it out of the schemas of the
   reference plugins and the shape catalog.
2. Take the targets of the doubles from the root packages of the two satellites.
3. Add `cli.Run`, let `Main` call it, and move the dispatch cases of its spec into the test process.
4. Change the Tier-3 sections of the projection and the testing documents.
5. Write ADR-0038.

## Alternatives

### Internal packages behind the stable API (D-9)

`workspace` could validate the composition, conduct the run, execute the plans, commit, run the warm
path and explain the state in internal packages behind its API. Each internal package would read
the state of the run that the others write, so the run would pass that state across the package
boundaries. The split fixes no defect, and every design that reads the package would change its
references.

### Helper packages of each satellite (D-8)

Each satellite could have an `sdk` package with the Go-only and the TypeScript-only helpers, and a
lint that refuses it in a neutral file. The root package of each satellite already exports the
vocabulary of its language, and a lint of files needs a declaration of the binding files of each
plugin.

### A plugin of a schema that the workspace completes

The workspace could fill an empty plugin when it registers the schemas of a composition. The
builder derives the spelling that the dispatch matches before any workspace exists, so the dispatch
would match the rules by the bare spelling.

## Drawbacks

- `workspace` has 30 production files and `plugin` 150 exported identifiers, and a reader learns
  them as they are.
- The root package of the facade has 63 identifiers, about four times the estimate of RFC-0006.

## Open questions

None.

## Unresolved and future work

None.

## References

- The improvement plan rows W7.1 to W7.24, W10.8 and W10.33, and the proposed decisions D-8 and
  D-9.
- [RFC-0006](0006-authoring-surface-and-dispatch.md), the authoring API and its estimate.
- [RFC-0012](0012-the-sdk-contract-module.md), the facade and the import lint.
- [RFC-0037](0037-five-contracts-that-the-reviewed-defects-need.md), the binding of a template
  import.
- [03](../architecture/03-projection.md), the tiers of the projection layer.
- [11](../architecture/11-languages.md), the public SPI.
- [13](../architecture/13-testing-and-conformance.md), the import lint.
