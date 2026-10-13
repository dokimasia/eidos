---
rfc: 0035
title: One name for each concept
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-12
updated: 2026-10-12
discussion: none
supersedes: none
superseded-by: none
produces-adr: 0027
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0035: One name for each concept

## Summary

Some words of the kernel, the Go satellite and the command line have two or three meanings, and
some concepts have two names. A reader who meets one of these words has to find out from the
context which concept the code means. The languages also evaluate their constant values in
different layers, and the language document states a placement that differs from both.

We propose these changes:

- The Go key constants whose names differ from their keys take the names of their keys.
- `golang.Scope` becomes `golang.Bindings`, so the kernel's `rules.Scope` is the only `Scope` of
  the resolution step.
- The position of a plugin in the schedule is a turn. The word bucket keeps one meaning, the byte of
  the hash that places a manifest entry in its document.
- The kinds of execution that `explain` lists are execution kinds, because the glossary gives the
  word record to the declarations of a family.
- The JSON field of the command line that contains the member of a list is `member`, because the
  config key `workspace` and the manifest's `workspace` field contain the name of a workspace.
- Each language evaluates its constant and enum values in the layer that has the facts that the
  evaluation needs. The language document states that rule.

The manifest and the sealed state store the same bytes as before. The JSON output of `plan`,
`explain` and every command changes four field names and one event name, and no release contains
that output yet.

## Motivation

### The key constants of the Go satellite

Every key constant of the repository is the Pascal case of the last part of its key, followed by
`Key`. The Go satellite declares fourteen keys that follow the rule and two that do not.
`ReceiverPointerKey` is the key `golang.receiverIsPointer`, and `UnderlyingKey` is the key
`golang.underlyingKind`. A reader who searches the code for the key that `explain` prints finds no
constant of that name. The frontend writes both keys, and no plugin, rule or document reads them.

### The Scope types

The kernel's `rules.Scope` is where a spelling is read, the subject of the spelling and the file
whose imports qualify it. The Go satellite's `golang.Scope` is what the imports of one Go file bind.
Both types meet in one expression of the Go rules, which resolve a name through
`scopeOf(scope.File).Candidates(scope.Subject.Package, name)`. The kernel calls the slot that keeps
a satellite's import bindings `Bindings`, and the other four satellites keep theirs unexported as
`scope` or `bindings`.

### Bucket

The word bucket has these meanings in the kernel:

- The position of an annotator or a generator in the schedule. Build numbers the plugins from one,
  in the order of priority, capability and name, so no two plugins share a number. A claim ranks by
  it, and a generator sees the units of the plugins before it. The documents also call it a group of
  plugins of one priority, which the code does not build.
- The byte of the SHA-256 of a path, which places the entry of the path in one document of the
  manifest. This is the meaning that a reader expects of the word.
- The declarations of one destination in the layout router, and a few local variables.

The first meaning has five exported fields: `meta.Claim.Bucket`, `plugin.AnnotatorContext.Bucket`,
`plugin.GeneratorContext.Bucket`, `workspace.Component.Bucket` and `plugintest.Fixture.Bucket`. The
`plan` and `explain` commands print it as `bucket`. The second meaning has `manifest.Shard.Bucket`,
whose JSON key `bucket` is part of the stored manifest, and `manifest.BucketOf`.

### Record

The workspace package uses the word record for the record of a run that the ledger keeps, read as
the previous run and written as the next generation. It also uses the word for the record of one
execution in a generation: `RecordKind`, `ExplainedRecord` and `Explanation.Records`. The glossary
defines a record as a declaration that a handler places into a family that claims its files, which
neither package means. The docblock of `RecordKind` already states that it is the kind of an
execution.

### Workspace in the command line

The config key `workspace` sets the name of the workspace, which the manifest records in its field
`workspace`. The JSON events of every command have a field `workspace` that contains the root of a
member of a list, such as `tools/gen`. These values differ for every member whose config names
its workspace, and two members of a list can have one workspace name. The renderer already calls
the value a member in its own fields.

### The evaluation of constant values

The Go frontend evaluates each constant with the type checker of the standard library, with the
imports stubbed. It stamps the exact value under `golang.constValue`, and the Go rules read the
stamp. The TypeScript rules evaluate the members of an enum each time a projection asks, from the
spellings that the frontend keeps. The protobuf rules parse the declared number of each value. The
language document states that every satellite evaluates values with a small evaluator in `rules/`,
which describes TypeScript and contradicts Go.

The Go rules cannot evaluate the values. The graph has no ordinal of an iota row, it keeps an empty
value for an implicit row, and it does not keep the basic type of a typed constant. A blank row and
an unexported row that the depth leaves out also take part in the arithmetic of a group.

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-lang-go` | `ReceiverIsPointerKey` and `UnderlyingKindKey` replace the two key constants. `Bindings` and `NewBindings` replace `Scope` and `NewScope` |
| `eidos-core/meta` | `Claim.Turn` replaces `Claim.Bucket` |
| `eidos-core/plugin` | `AnnotatorContext.Turn` and `GeneratorContext.Turn` replace the two `Bucket` fields |
| `eidos-core/plugin/plugintest` | `Fixture.Turn` replaces `Fixture.Bucket` |
| `eidos-core/workspace` | `Component.Turn` replaces `Component.Bucket`. `ExecutionKind`, its six constants, `ExplainedExecution` and `Explanation.Executions` replace the record names. The unexported fields of the schedule follow |
| `eidos-core/layout` | the unexported type `bucket` of the router becomes `destination` |
| `eidos-sdk` | the facade is generated again |
| `eidos-cli` | the JSON fields `turn` and `member`, the list `members`, the event `explain.execution` and `Renderer.Member` replace the old names |
| `docs/architecture` | the sentences that use bucket for a turn, and the row of the language document on values |

### Invariants

1. A word of the code has one meaning inside a module, and a concept has one name across the
   modules of the repository.
2. The stored bytes stay as they are. The sealed state stores a turn as a number and a key by its
   string, and the manifest keeps its key `bucket`.
3. A JSON field of the command line changes its name only before the first release. The version of
   the event schema stays 1.0, because no release contains the schema.
4. The values of a language are evaluated once, in the layer that has every fact that the evaluation
   reads.

### The Go key constants

The constant takes the name of its key, and the key does not change. The docblocks use the word of
the key.

```go
// ReceiverIsPointerKey marks a method declared on a pointer
// receiver.
ReceiverIsPointerKey meta.KeyName = "golang.receiverIsPointer"

// UnderlyingKindKey records the kind of the underlying type of a
// defined type: basic, named, pointer, slice, array, map, chan or
// func.
UnderlyingKindKey meta.KeyName = "golang.underlyingKind"
```

### The bindings of a Go file

The type keeps its fields, its methods and its contracts under the new name.

```go
// Bindings is what the imports of one Go file bind: the import path
// that each qualifier binds, the dot-imported paths, and every
// unaliased import in source order. The zero Bindings binds nothing.
type Bindings struct {
    named map[string]string
    dots  []string
    all   []string
}

func NewBindings(imports []*node.Import) Bindings
func (b Bindings) Import(qualifier string) (string, bool)
func (b Bindings) Candidates(pkg, spelling string) []symbol.Identity
```

The frontend records the value in the kernel's `Bindings` slot, and the Go rules read it back as
`scope.Bindings.(golang.Bindings)`.

### The turn of a plugin

A turn is the position of an annotator or a generator in the schedule of its role, counted from one.
Turn zero is outside the schedule, and a stamp of a frontend and a `meta drop` directive have it. A
claim ranks by its authority, then by the earlier turn, then by the name of the plugin, and then by
its place in canonical match order.

```go
type Claim struct {
    Subject   symbol.Identity
    Authority Authority
    Turn      int
    Plugin    diag.Origin
    Order     Order
    // Pos and Derived are unchanged.
}
```

`AnnotatorContext.Turn` and `GeneratorContext.Turn` are the turn of the phase call, and the store
of a generator contains the units of the earlier turns. `Component.Turn` is the turn that every
claim and every unit of the plugin records. The command line prints the field as `turn`, and the
text of `explain` reads `turn 3`. The error of a failed call reads
`workspace: annotator shape in turn 3`, followed by its cause.

The manifest keeps its meaning of the word. `manifest.Shard.Bucket` and `manifest.BucketOf` are
the byte of the hash of a path, which is a hash bucket.

### The kinds of execution

```go
// ExecutionKind is the kind of execution that an [ExplainedExecution]
// describes. The zero value is not a valid kind.
type ExecutionKind uint8

const (
    ExecutionValidation ExecutionKind = 1
    ExecutionInvocation ExecutionKind = 2
    ExecutionCheck      ExecutionKind = 3
    ExecutionGroup      ExecutionKind = 4
    ExecutionAudit      ExecutionKind = 5
    ExecutionRegion     ExecutionKind = 6
)

// ExplainedExecution is one execution in a generation.
type ExplainedExecution struct {
    Kind ExecutionKind
    // The other fields are unchanged.
}
```

`Explanation.Executions` lists the executions. `String` returns the same six words, and
`ExecutionKind(n)` for an undeclared kind. The event of the command line is `explain.execution`,
and its text line starts with `execution`. The internal kinds of the sealed state keep their names,
because a record of the state is the row that stores an execution.

### The member of a list

```go
// diagEvent, errorEvent, fileEvent, outcomeEvent and header
Member string `json:"member,omitempty"`

// tally
Member string `json:"member,omitempty"`

// summaryEvent
Members []tally `json:"members,omitempty"`

// Member starts the output of the member m of a list. The events
// that follow have the name of m in their field member.
func (r *Renderer) Member(m Member)
```

A run of a member `tools/gen` of a list writes events such as this one.

```json
{"event":"outcome","plan":"go-stubs","status":"committed","member":"tools/gen"}
```

### The evaluation of values

Each language evaluates a value in the layer that has the facts that the evaluation reads:

- The Go frontend evaluates every constant whose value the package itself determines, and stamps
  the exact value. The evaluation covers iota arithmetic and typed constants. A value that an
  import feeds has no stamp.
- The TypeScript rules evaluate the members of an enum from its declaration, because the value of a
  member depends only on the members before it, which the model keeps in order with their
  spellings.
- The protobuf rules parse the declared number of each enum value.

The row of the language document reads as follows.

```markdown
| enum and constant values, including iota-style arithmetic | the layer that has the facts: the Go frontend evaluates through the type checker and stamps the exact value, and the TypeScript and protobuf rules evaluate from the declaration |
```

### What keeps its name

| Name | Reason |
|---|---|
| `rules.Scope` | the kernel's word for where a spelling is read, with 73 references in five modules. Once the Go type is `Bindings`, the Go rules use one type of that name |
| `manifest.Shard.Bucket` | a hash bucket, whose key `bucket` is stored in every document of the manifest |
| the internal `state.RecordKind` | a record of the sealed state is the stored row of an execution, and the package is internal |
| the config key `workspace` | the name of the workspace, which the manifest records under the same name |
| the key `plugins` of a plan | the plugins of the plan by directory, because a declarative plugin has no name until it loads, while `options` keys a plugin by the name that it declares |
| the three layout fields of `workspace.PlanConfig` | a refinement, in which a zero field leaves the field of the plan unchanged. `layout.Config` is a whole configuration, and its `Plugins` and `Families` have no refinement rule |

### Failures

The change does not add a failure. A program that reads the old JSON fields finds them absent, and
a consumer that compiles against the old Go names fails to compile.

### Migration

1. Rename each Go identifier with the refactoring tool of the language, module by module, so every
   reference and doc link follows.
2. Rename the JSON tags, the event name, the texts of `explain` and the two error texts by hand, and
   update the cases that pin them.
3. Generate the facade again.
4. Rewrite the sentences of the architecture documents that use bucket for a turn, and the row on
   values.

The sealed state does not need a migration, because its stored bytes do not change.

## Alternatives

### Rename the keys instead of the constants

The keys `golang.receiverPointer` and `golang.underlying` would match the constants. The key is
what `explain` prints and what the sealed state stores, so the change would run every state cold
once and change the input and output of `explain`. Renaming the constants changes twelve lines in one
module and nothing that a user types or reads.

### Rename rules.Scope

The kernel type has 73 references in five modules, the facade and its test. Its name states its
meaning in the resolution contract. The Go type has 31 references in one module, and the kernel
already calls its kind of value bindings.

### Keep bucket for the turn, and rename the hash bucket

A hash bucket is the meaning that a reader brings to the word. Its name is stored as the JSON key of
every manifest document, so a rename would need a third version of the manifest or a reader of both
spellings. The turn is stored as a number alone.

### Name the turn by an existing word

Each of the words rank, tier, stage, order, schedule, priority, position and shard has a meaning in
the kernel already. Claims sort by rank, and the turn is the second step of that rank. The
projection has tiers, and the candidates of a frontend come in tiers too. Order is the place of a
claim in match order. The code has no identifier named turn.

### Rename the record of a run instead of the execution kinds

The record of a run is one concept. A run reads the record of the previous run and writes its own
record as the next generation. The ledger and the state use the word in that sense. The execution
kinds are the other concept, and their docblock already calls them executions.

### Make PlanConfig embed layout.Config

An embedded or named `layout.Config` exposes `Plugins` and `Families`. No refinement rule covers
them. It also promotes `Check` and `AppendBinary` onto a type that is not a configuration. A shared
type of the three fields adds a type that two packages use beside their own contracts. The command
line keeps its file type apart in any case, because the workspace package defines no file format.

### Move the evaluation of Go values into the rules

The rules would need the ordinal of every iota row, the expression of every implicit row and the
basic type of every typed constant. The model keeps none of them, so the move needs schema fields
and still loses the rows that the depth leaves out. The evaluation would also run on every
projection instead of once per parse.

### Evaluate TypeScript and protobuf values in their frontends

One placement for every language is simpler to state. The TypeScript evaluator reads only the
members of one declaration, and the model keeps them. No reader outside the rules needs the values,
so a stamp would add a key to every enum of every load for no reader.

## Drawbacks

- The turn touches 79 exported references, about 20 unexported ones, 2 error texts and about 40
  sentences of the architecture documents.
- The execution kinds touch about 80 references in the workspace package, the command line and their
  tests.
- The member touches six fields, one method and the cases of the renderer.
- A consumer of the JSON output that the working tree writes today changes four field names and one
  event name. No release contains that output.
- Accepted design records keep the old words, because an accepted record does not change. A reader
  of an older record meets bucket for a turn and record for an execution.

## Open questions

None.

## Unresolved and future work

- The facts that the model drops are not proposed here: the nullish members of a TypeScript
  reference and the three forms of `readonly`.
- The other meanings of record in the kernel, such as `plugin.PlanRecord` and the records of a load,
  are not proposed here.

## References

- The glossary: [`docs/architecture/GLOSSARY.md`](../architecture/GLOSSARY.md).
- The language document: [`docs/architecture/11-languages.md`](../architecture/11-languages.md).
- The compatibility policy: [`docs/architecture/15-compatibility.md`](../architecture/15-compatibility.md).
- The shared helpers of the satellites, whose unresolved work lists these renames:
  [RFC-0031](0031-the-shared-helpers-of-the-satellites.md).
- The projection vocabulary, which reads the Go values from the stamp:
  [RFC-0014](0014-the-projection-vocabulary.md).
