---
rfc: 0085
title: The instances of a directive and the subjects of a key that a handler reads
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-21
updated: 2026-10-21
discussion: https://github.com/dokimasia/eidos/issues/59
supersedes: none
superseded-by: none
produces-adr: ADR-0092, ADR-0093
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0085: The instances of a directive and the subjects of a key that a handler reads

## Summary

We propose two reads for a handler. `Directives` on a match returns the validated instances of one
directive of the plugin on the subject of the match, in source order. `Subjects` returns the
subjects under the scope of the match on which a key reads present, in identity order, and it
records a membership edge on the key. A `Directive` wrapper around no rules registers its schema
and gates nothing, so a plugin can declare a directive that a handler of another rule reads.

## Motivation

A generator of conformance suites needs the instances of a directive on a subject and the subjects
of a key. The dispatcher indexes both for its gates, and a handler cannot read either index.

A generator writes one conformance test for each type under its directive `subject` (issue 59).
The repeatable directive `excuse` on the same type excuses one check of the suite, and the test
passes each excusal as an option of one call:

```go
// Store is the Log on PostgreSQL.
//
//acme:subject logs.Log new=NewTestStore
//acme:excuse Log.Replay/cancel reason="the test server finishes a cancelled query"
//acme:excuse Log.Append/expires reason="the server reads the wall clock"
type Store struct{ … }
```

The kernel prevents the test:

- `m.Directive()` returns the gating instance alone, and no other method of a match returns an
  instance. Under a repeatable schema the handler runs once for each instance.
- `Out.Record` applies the data of the first call in canonical match order where two calls attach
  data to one origin of a unit. The handler of each `excuse` cannot add its option to the record of
  the test.
- The fact store reads one value for each key of a subject, the value of the claim that ranks
  first, so the annotator half cannot collect the instances into one fact.
- A typed build panics on a wrapper around no rules, so a plugin cannot declare `excuse` without a
  rule that the directive gates.

A fact on one declaration adds to the output of another (issue 58). The generator binds the checks
that a consumer declares on functions of the package of a suite:

```go
package logtest

// KeepsTheBody checks that Replay returns the body that Append stored.
//
//acme:check Log.Replay
func KeepsTheBody(h *LogHandle) { … }
```

The annotator half stamps a fact on `KeepsTheBody` with the identity of the method `Replay` of
`Log`. The generator half runs on the interface `Log` in another package, and it lists each
function whose fact names a method of `Log`. The fact store indexes the subjects of each key at
stamp time, and the dispatcher reads the index for `HasKey`. A handler can list the subjects only
through a walk of every package of its reader with a call of `FactOf` for each function. The walk
records a declaration edge on every declaration of the scope, so an edit of any declaration runs
the handler of every suite again.

## Detailed design

### Components

| Component | Package | Change |
|---|---|---|
| Match | `eidos-core` | `Directives` returns the instances of a directive of the plugin on the subject |
| Directive grammar | `eidos-core/directive` | `Name.Bare` returns a spelling without its plugin prefix |
| Tracked fact reads | `eidos-core` | `Subjects` lists the subjects of a key and records the membership edge on the key |
| Builder | `eidos-core` | a `Directive` wrapper around no rules registers its schema and gates nothing |
| Read set | `eidos-core/store` | a sixth grain, the membership edge on a key |
| State | `eidos-core/internal/state` | `KeyEdge`, the hash of the new edge |
| Warm run | `eidos-core/workspace` | a fact whose presence changed makes the edge of its key dirty in the package of its subject |
| Explain | `eidos-core/workspace` | the grain `ReadKey` |
| SDK | `eidos-sdk` | the regenerated facade |

### The instances of a directive

```go
package eidos

// Directives returns the validated instances of the directive name of the
// plugin on the subject of the match, in source order. name is the bare
// spelling that a schema of the plugin states, or the canonical spelling of
// that schema. An instance matches where its canonical spelling is the name
// of the plugin, a colon and the bare name, so an instance of a directive of
// another plugin or of the kernel never matches.
// Directives yields a negated instance with Negated set. A graph match has
// no subject, so Directives does not yield an instance there. The instances
// are the storage of the validated table, so a handler does not mutate
// them.
//
// The read does not record an edge. The subject of the invocation is an
// edge of every invocation, and a change of the validated instances of the
// subject makes that edge dirty, so the handler runs again.
//
// # Allocation contract
//
// A range over Directives allocates nothing.
func (m *match) Directives(name directive.Name) iter.Seq[*directive.Directive]

type SubjectMatch interface {
	// The entries before Directives are unchanged.
	Directive() *directive.Directive
	Directives(name directive.Name) iter.Seq[*directive.Directive]
	// The entries after Directives are unchanged.
	Kernel() meta.KernelKeys
}
```

- The read covers the plugin's own directives on the subject of the match. A handler of a rule
  under a repeatable `Directive` wrapper reads every instance of the schema, the instance of its
  match among them.
- `m.Directive()` returns the gating instance, and nil for bare and fact-gated matches.
- A handler does not see the directives of another plugin. It reads the effects of such a directive
  through the facts that their plugin stamps.
- An emit match reads the instances on the origin, and a package match reads the instances on the
  package.

The handler of `subject` writes one record with every excusal:

```go
func conformance(m *eidos.StructMatch, e *eidos.Emitter) error {
	var excused []excusal
	for d := range m.Directives(excuseName) {
		check, _ := d.Param(excuseCheck)
		reason, _ := d.Param(excuseReason)
		excused = append(excused, excusal{check: check.Str, reason: reason.Str})
	}
	e.File(eidos.TagTest).Record(suiteData{excused: excused}, testOf(m))
	return nil
}
```

### A schema that gates no rule

`Directive(s)` without rules registers the schema `s` and gates nothing. A typed build no longer
panics on such a wrapper. It panics where the schema states no `Kinds` and does not override,
because no rule gives the kinds of the declarations that an instance may be written on. An override
schema takes the kinds of the bare and fact-gated rules of the plugin. A `Where` or a `Gated`
wrapper around no rules still panics, because it declares nothing. The built plugin returns the
schemas of its wrappers in the order of the declaration.

```go
Handle(
	Directive(subject, OnStruct(conformance)),
	Directive(excuse),
)
```

`excuse` states `Kinds: []symbol.Kind{symbol.KindStruct}` and
`Requires: []directive.Name{"subject"}`. Validation then refuses an excusal on another kind and an
excusal without a subject.

### The subjects of a key

```go
package eidos

// Subjects returns the subjects under the scope of the match on which k
// reads present, in identity order. It records the membership edge on k
// into the read set of the invocation when the range starts. A subject in
// the scope that gains or loses k then runs the handler again, and a change
// of the value of k on a subject does not. A handler reads the value on a
// subject through [FactOf], which records the fact edge. A named handle
// resolves by its name in the registry of the run, because the presence of
// a key does not depend on its value type. A name that the registry does
// not contain does not yield a subject, and the edge records the name. The
// zero Key does not yield a subject or record an edge. A graph match
// enumerates as a match with a subject does.
//
// The enumeration lists the subjects that read present when the range
// starts. In the annotate phase these are the subjects of the stamps of the
// load, of the directives and of the annotators of earlier turns. A handler
// does not read a key that another match of its own phase call stamps,
// because handlers are order-independent within a plugin.
//
// # Allocation contract
//
// A range over Subjects allocates what [meta.Facts.ByKey] allocates, and
// what the read set allocates to record the edge.
func Subjects[T meta.FactValue](m Matcher, k meta.Key[T]) iter.Seq[symbol.Identity]
```

The handler of the suite lists the checks of the methods of `Log`:

```go
func suite(m *eidos.InterfaceMatch, e *eidos.Emitter) error {
	var bound []symbol.Identity
	for id := range eidos.Subjects(m, checkKey) {
		method, _ := eidos.FactOf(m, id, checkKey)
		if method.Owner == m.Interface.Name && method.Package == m.Interface.Identity().Package {
			bound = append(bound, id)
		}
	}
	e.File(eidos.TagTest).Record(suiteData{checks: bound}, suiteOf(m))
	return nil
}
```

### The membership edge on a key

The read set gains a sixth grain:

```go
package store

// RecordKey records the membership edge on key: what an enumeration of the
// subjects of a key records. The edge is dirty where a subject gains or
// loses the key, and a change of the value on a subject leaves it clean.
func (s *ReadSet) RecordKey(key meta.KeyName)

// Keys returns every membership edge on a key, in name order.
func (s *ReadSet) Keys() iter.Seq[meta.KeyName]
```

```go
package state

// KeyEdge returns the hash of the membership edge on a key: what an
// enumeration of the subjects of the key records. It allocates nothing for
// a name that fits in 250 bytes.
func KeyEdge(key meta.KeyName) EdgeHash
```

- The spelling of the edge opens with a tag of its own, so it differs from the spelling of every
  edge of another grain.
- The settle step of a warm run compares the presence of each fact that a step touched with its
  presence in the generation. A fact whose presence changed makes the edge of its key dirty in the
  package of its subject, as an enumeration by kind or by directive routes its edge. Each plan then
  runs its readers of the edge where its scope admits the package.
- A fact whose value changed and whose presence did not leaves the edge clean. Its fact edge at the
  subject and the key is dirty for the changed value.
- `explain` lists the edge as a read of the grain `ReadKey`, with the name of the key.
- A record lists its edges by hash, so the format of the state does not change.

### What does not change

- The gate of a rule, the canonical match order and the dispatch of a repeatable schema.
- `Fact`, `FactOf` and the fact edge.
- The visibility of another plugin's directives.
- `Out.Record`, which applies the data of the first call in canonical match order.

### Verdicts

| Item | Verdict | Reason | Reverses the verdict |
|---|---|---|---|
| `Directives` on the match | in | the dispatcher reads the validated table of the subject already, and the read is on the subject alone | a use case that reads the instances on another declaration |
| The plugin's own directives alone | in | a handler reads the effects of another plugin's directive through its facts | none known |
| No edge for `Directives` | in | a change of the validated instances of the subject makes the edge of the subject dirty | a read of the instances on another declaration |
| A `Directive` wrapper around no rules | in | a plugin declares a directive that a handler of another rule reads | none known |
| Kinds for a schema that gates no rule | in | no rule gives the kinds of the declarations that an instance may be written on | none known |
| `Subjects` with a membership edge | in | the fact store indexes the subjects of each key at stamp time, and an edit of another declaration leaves the handler clean | none known |
| Any key handle | in | facts are the channel between plugins, and `FactOf` reads any key | a key whose subjects a plugin must not see |
| A named handle resolved by its name | in | the presence of a key does not depend on its value type, and a resolution by type keeps `Subjects` from inlining, so each range allocates its loop | a key whose subjects differ by value type |
| A check against a read of a key that the phase call stamps | out | `Fact` and `FactOf` state the same rule without a check | a check of every tracked fact read of the annotate phase |
| A merge of `Out.Record` across instances | out | the data of a record has a type of the plugin, and the kernel has no merge for it | none known |

## Alternatives considered

### A handler for each subject with all instances

A `Directive` wrapper would run its handler once for each subject with every instance of a
repeatable schema. `m.Directive()` would return a list of the instances.

**Why not:** the dispatch of a repeatable schema would have two modes. The canonical match order
would also lose the instance of a match. The rule of the use case runs under the directive
`subject` and reads the instances of `excuse`. A mode of `excuse` alone does not serve it.

### A merge of the records of one origin

`Out.Record` would merge the data of every call that attaches data to one origin.

**Why not:** the data is a value of a type of the plugin. The kernel has no merge for it, and a
merge would depend on the order of the calls, which the canonical match order fixes for the first
call alone.

### A list fact that collects the instances

The annotator half would stamp one fact with the list of the excusals.

**Why not:** the fact store reads the value of the claim that ranks first for each key of a
subject. Each instance runs its own invocation, so only the first claim in canonical match order
sets the value.

### A walk of the scope with `FactOf`

The generator half would walk every package of its reader and read the fact of each function.

**Why not:** the walk records a declaration edge on every declaration of the scope. An edit of any
declaration then runs the handler of every suite again, and the walk costs the size of the scope on
every run.

### A gate on the value of a fact

`KeyEquals` would admit the functions whose fact names a method of `Log`, and a rule on those
functions would add each binding to the suite of `Log`.

**Why not:** the rule runs on each check function and not on `Log`. The suite of `Log` is one record
of the invocation on `Log`, and a record takes the data of the first call in canonical match order,
so the invocations of the checks cannot add to it.

## Drawbacks

- The kernel gains one method on the match, one generic function, one grain of the read set with
  its map, one edge spelling and one grain of `explain`. `SubjectMatch` has 10 entries, the cap of
  `interfacebloat`, so the next method of the interface needs an optional interface.
- A handler that ranges over `Subjects` runs again whenever a subject in its scope gains or loses
  the key. A key on thousands of declarations runs its readers on most warm runs that stamp it.
- A handler that ranges over `Subjects` and reads the value on each subject records one fact edge
  for each subject, as a walk with `FactOf` records it.
- The rule that a handler sees only its own gating instance gains an exception for the plugin's own
  directives on the subject. The documentation of the authoring surface and the decision log state
  the new rule.
- A schema that gates no rule and does not override needs explicit `Kinds`.
- A named handle of another value type lists the subjects of the key, and `FactOf` reads each value
  absent through it.

## Open questions

None.

## Unresolved and future work

- A read of the instances of the plugin's directives on another declaration is not proposed here.
  A handler reads the effects of such a directive through the facts that the plugin stamps.

## References

| What | Where |
|---|---|
| Issue 58, the subjects of a key | <https://github.com/dokimasia/eidos/issues/58> |
| Issue 59, the instances of a repeatable directive | <https://github.com/dokimasia/eidos/issues/59> |
| D52, repeatable schemas and the visibility of directives | ../architecture/21-decisions.md |
| D55, the indexed dispatch | ../architecture/21-decisions.md |
| go/analysis, `Pass.AllObjectFacts` | <https://pkg.go.dev/golang.org/x/tools/go/analysis#Pass> |
| The Rust reference, derive macro helper attributes | <https://doc.rust-lang.org/reference/procedural-macros.html#derive-macro-helper-attributes> |
| Java SE 21, `AnnotatedElement.getAnnotationsByType` | <https://docs.oracle.com/en/java/javase/21/docs/api/java.base/java/lang/reflect/AnnotatedElement.html> |
