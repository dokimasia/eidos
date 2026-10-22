---
adr: 0019
title: A family claims its files through a file template
status: Accepted
date: 2026-10-10
supersedes: none
superseded-by: ADR-0099
rfc: RFC-0025
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0019: A family claims its files through a file template

## Status

Accepted. [ADR-0099](0099-a-reference-with-a-target-resolves-to-a-record.md) supersedes its rule
for the name tables and the references to records.

## Context

A declarative plugin emits mirrors and implementations. The kind templates of the target render
each of them. A test file has a test function that calls each subject, and it imports a test
framework. No pair of `on` and `emit` describes such a file. The scaffolding vocabulary of a body
is also too small for a test.

The render specification already described a plugin that claims its own files through a file
template, and the render pass had no such claim. The accepted design rejected claimed file
templates, because the names in a file that a template writes are not emit declarations, so
the settle does not name, check or export them. That left a declarative plugin without a way to
generate the tests of the code that it generates.

## Decision

We will let a family declare a file template that writes each file of the family from records,
because a test file is a whole file whose shape belongs to the plugin, and no rule of a manifest
describes it.

- `plugin.Output.Template` is the path of the template in the plugin's tree for each target. A
  manifest writes it under `outputs[].template`.
- `Out.Record(data, decls...)` places declarations as records, with the data of their match.
  The check of colliding names, the name tables and the exports leave records out, and no
  reference resolves to a record.
- The render executes the template over the name and the package of the file and over its
  records, with `missingkey=error`. The file skeleton of the target frames the output with the
  imports that the template recorded through `use`.
- A file that mixes the units of a family that claims its files with other units is withheld
  under `MixedClaim`, EID-0071.

## Alternatives Considered

### No file that a template writes

A declarative plugin emits declarations alone, and the kind templates of the target write every
file, so the settle names, checks and exports every name of every file. It lost because a plugin
then cannot write a test of each subject. A test needs a body that calls the subject, and a
manifest has no syntax for statements, so the tests of generated code stay hand-written.

### Tests written as emit declarations

A rule emits a test function for each subject into the family `test`, with a body that a
template renders, and the kind templates write the file. The settle then names and checks each
test. It lost because a test file contains more than its test functions, such as a table of
cases, a helper and the imports of a test framework. The manifest would need an emission for
each such declaration, and the format would grow toward the statement vocabulary that it leaves
out.

## Consequences

**Positive:**

- A declarative plugin writes a test of each subject. The conformance fixture writes the Go test
  of each implementation this way, and `go test` runs it.
- A typed plugin claims its files the same way, through `Output.Template` and `Out.Record`.
- A file template uses the funcmap of the target to spell types and to record imports, as a kind
  template does.

**Negative:**

- The names that a file template writes are not emit declarations. A second plugin can declare
  one of them in the same package without a collision finding, a dependent plan cannot refer to
  one through an export, and `explain` cannot map an identifier of the file to its origin.
- The settle respells the name of a record, and it leaves each reference inside a record as
  written. A file template that spells such a reference gets neither its package nor the
  target's name of its referent.
- A plugin writes a file template for each target that its plans render.
- `EID-0071` means `MixedClaim`, so the kernel has no code for incomplete members.

**Neutral:**

- The render specification described this claim before the render pass had it.

## References

| What | Where |
|---|---|
| Render dispatch: whose template renders what | ../architecture/07-rendering.md |
