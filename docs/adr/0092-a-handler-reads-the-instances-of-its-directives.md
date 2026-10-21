---
adr: 0092
title: A handler reads the instances of the directives of its plugin on its subject
status: Accepted
date: 2026-10-21
supersedes: none
superseded-by: none
rfc: RFC-0085
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0092: A handler reads the instances of the directives of its plugin on its subject

## Status

Accepted

## Context

A handler sees the gating instance of its rule alone, and the directives of other plugins are
invisible to it. Under a repeatable schema the handler runs once for each instance.

- A generator of conformance suites writes one test for each type under its directive `subject`.
  The repeatable directive `excuse` on the same type excuses one check, and the test passes every
  excusal as an option of one call.
- `Out.Record` applies the data of the first call in canonical match order where two calls attach
  data to one origin of a unit. The fact store reads the value of the claim that ranks first for
  each key of a subject. So the handler of each `excuse` cannot add its option to the test.
- A typed build panicked on a wrapper around no rules, so a plugin could not declare a directive
  that no rule of its own gates.
- The dispatcher reads the validated instances of each subject for its gates already. A change of
  the validated instances of a subject makes the declaration edge of the subject dirty, and every
  invocation records the edge of its subject.

## Decision

We will let a handler read the validated instances of the directives of its plugin on the subject
of its match through `SubjectMatch.Directives`, because the dispatcher reads them for the subject
already. A change of them runs the handler again through the edge of the subject.

- `Directives(name)` takes the bare spelling of a schema of the plugin or its canonical spelling,
  and yields the instances in source order. An instance of a directive of another plugin or of the
  kernel never matches, and a negated instance matches with `Negated` set.
- The read does not record an edge. A graph match does not yield an instance. An emit match reads
  the instances on the origin, and a package match reads the instances on the package.
- `Directive(s)` without rules registers the schema and gates nothing. Its schema states its
  `Kinds` unless it overrides, and a typed build panics otherwise. The built plugin returns the
  schemas of its wrappers in the order of the declaration.
- `directive.Name.Bare` returns a spelling without its plugin prefix.

## Alternatives Considered

### A handler for each subject with all instances

A `Directive` wrapper would run its handler once for each subject with every instance of a
repeatable schema. The dispatch of a repeatable schema would then have two modes, and the canonical
match order would lose the instance of a match. The rule of the use case runs under the directive
`subject` and reads the instances of `excuse`. A mode of `excuse` alone does not serve it.

### A merge of the records of one origin

`Out.Record` would merge the data of every call that attaches data to one origin. The data is a
value of a type of the plugin, and the kernel has no merge for it. A merge would also depend on
the order of the calls, which the canonical match order fixes for the first call alone.

### A list fact that collects the instances

The annotator half would stamp one fact with the list of the excusals. Each instance runs its own
invocation, so only the first claim in canonical match order sets the value.

## Consequences

**Positive:**

- A generator writes one record of a subject from every instance of a repeatable directive on it.
- The read does not add an edge to the incrementality engine.
- A plugin declares a directive that a handler of another rule reads, and validation checks its
  instances as it checks every directive.

**Negative:**

- The rule that a handler sees only its own gating instance gains an exception for the directives
  of its plugin on the subject.
- `SubjectMatch` has 10 entries, the cap of `interfacebloat`, so the next method of the interface
  needs an optional interface.
- A schema that gates no rule and does not override needs explicit `Kinds`.

**Neutral:**

- A handler does not see the directives of other plugins. It reads their effects through facts.

## References

| What | Where |
|---|---|
| D210, the row of this decision | ../architecture/21-decisions.md |
| D52, repeatable schemas and the visibility of directives | ../architecture/21-decisions.md |
| Issue 59, the instances of a repeatable directive | <https://github.com/dokimasia/eidos/issues/59> |
