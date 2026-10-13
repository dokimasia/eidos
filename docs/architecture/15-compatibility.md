<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# Compatibility and versioning

*Builds on: [13](13-testing-and-conformance.md). Governs every
module's releases and every schema surface.*

Splitting the system across modules works because the kernel
boundary sits around the slow-moving set, and because CI executes
every compatibility claim in both directions.

## Kernel versioning

The kernel follows semver, and the working rule is that a minor
release only adds. The generated mirrors keep adding cheap: a new
kind or field is a schema edit plus a regeneration
([02-symbol-model.md](02-symbol-model.md)).

The kernel declares one **contract version**, `plugin.ContractVersion`,
which a change that breaks a component raises. Go links one kernel into
a binary, so a compiled component cannot have another version, and
only what crosses the boundary of a binary records the version. A
declarative manifest states its version, and the loader refuses a
manifest of another version with a fault that names the plugin and both
versions. The header of the sealed state records the version, and a run
over a state of another version runs cold. `version` prints it.

The public API surface is enumerated rather than implied. It covers
the kernel packages' exported identifiers, the directive grammar and
the kernel schemas ([05-directives.md](05-directives.md)), the
manifest format, the plan-export schema
([08-workspace-and-plans.md](08-workspace-and-plans.md)), the
declarative-plugin manifest schema, the config JSON Schema, the JSON
output schemas of the command kernels, and the diagnostic codes.
Each is versioned with the kernel, and breaking any of them means a
major release.

The Go surface is diffed rather than trusted. `conformance/` ships a
check that compares a module's exported API against a committed
baseline. Additions update the baseline in review; removals and
signature changes fail outside a major. The baseline is a committed
text file, one sorted line per exported identifier with its full
signature, so a surface change arrives as a reviewable diff rather
than a tool's opinion. The canary ring catches breaks by running
real code, the surface diff names them precisely, and both run per
release.

## Satellite versioning

Satellites tag on their own cadence and declare the kernel range
they support. Their CI proves it by running the kernel conformance
suite at the declared minimum and maximum on every commit. A range
that has not been proven cannot be published, because the range in
the go.mod and the range in CI are the same data.

## The canary ring

The ring is every module of the workspace that requires the kernel:
eidos-reference, which is the designated canary and is kept
API-typical on purpose, plus eidos-plugin-shape, the language
satellites and the other consumers of the kernel.

Every CI run builds and tests each module of the workspace against the
kernel at the commit. Before the first publication of a module, this
run is the ring for that module.

The ring job of CI tests the latest published version of each module
of the ring against the kernel at the commit. It reads the modules from
`go.work` and queries the module proxy for the latest version of each.
It reads the tree of each release from its tag with `git archive`,
because a module zip leaves out the fixtures that have a go.mod and the
files outside the module. In the go.mod of the release, it drops the
replace directives and replaces the kernel and the SDK facade with the
checkout. It then runs `go test` over the packages of the release. A
module without a release is unpublished, and the job does not test it.

The release job publishes a commit only after a CI run passed on the
same content, so a ring failure blocks the tag. Real consumers test
"only adds" before each release, and no changelog has to promise it. A
human decides with the failure in hand whether the fix is kernel-side
or a coordinated satellite release.

## Deprecation

A deprecation is a marker: documentation, the `Deprecated:`
convention, and an entry in the diagnostic-code index. It gets one
minor of grace, and `doctor` flags its use in consumer workspaces.
Removal happens at the next major and nowhere else.

Directive deprecations also carry a rewrite hint in the diagnostic,
such as "`limit=` is now `bound=`; see code EID-…", because
annotations sit in consumer source, which is the surface least able
to absorb a silent change.

A directive schema and each of its parameters have a `Deprecated`
field that states the rewrite. Validation reports each use of a
deprecated directive or parameter as a Warning under
`DeprecatedDirective`, with the rewrite in the message, and `doctor`
lists these warnings.

## Schema evolution is checked, not read

Every schema-shaped surface publishes into a per-release **schema
index**: directive schemas, options schemas, metadata key
registrations, the declarative-plugin manifest schema and the
plan-export schema.

Evolution between releases is classified mechanically. Additive
changes, meaning a new optional param or a new key, pass. Breaking
changes block the tag outside a major, through the same ring that
blocks API breaks: a removed name, a type change, a newly required
param, a changed resolution kind, or a tightened closure.

`doctor` runs the consumer side of the same check. It diffs a
workspace's actual directive usage against the index of the release
the consumer is about to adopt, so a breaking annotation change
shows up before the upgrade bites, positioned at the annotations it
affects.

The index is one committed JSON document per release: every
schema-shaped surface flattened to its name, fields, types,
resolution kinds, required flags and closure, then sorted. The diff
between two releases is the classifier's input, and `doctor` carries
the same file to the consumer side.

## Output-affecting releases

A release that changes generated bytes, even cosmetically, is
**output-affecting** and says so in its notes. Consumers regenerate
on upgrade and review the diff as part of adopting the release.

Byte-determinism holds within a version. It is never promised across
versions, because freezing rendered output forever would freeze
every template and every formatter with it.

## Published per release

- The per-language support matrix, from the completeness check.
- The cross-language metadata parity matrix
  ([04-metadata.md](04-metadata.md)).
- The diagnostic-code index, with stable anchors.

A release does not publish benchmark results, and it does not wait on
a benchmark run.

## Stable diagnostic codes

Every kernel and satellite diagnostic has a stable code: `EID-####`
for the kernel, and one prefix per satellite, `GOLANG-####` for Go
and `PROTO-####` for protobuf. Codes are API. Tests and consumer tooling assert on them instead of matching
message text, the documentation anchors on them, and a code's
meaning never changes. A changed meaning is a new code.
