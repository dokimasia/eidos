---
rfc: 0058
title: The build constraint of a generated file
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-13
updated: 2026-10-13
discussion: none
supersedes: none
superseded-by: none
produces-adr: none
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0058: The build constraint of a generated file

## Summary

A generator writes a file beside a Go source file, and the generated file refers to the declarations
of its source. Where a build constraint restricts the source to some platforms, the generated file
has no constraint and compiles everywhere, so it refers to declarations that do not exist on the
other platforms. We propose that the Go frontend records the build constraint of each file as an
annotation of the file. The layout gives a routed file the annotations of the one source file that
it derives from, and the Go backend writes the constraint of that source above the package clause.
A directive that a plugin places on a generated file is out.

## Motivation

- Over a copy of the reference tree, `catalog/host.go` under `//go:build !windows` with a builder
  directive gave `catalog/host_builder.go` without a constraint, and `GOOS=windows go vet ./catalog/`
  reported `undefined: Host`.
- The Go frontend reads the `//go:build` line of a file when the load selects the files of a
  package, and drops the line from the annotations of the file.
- A family that writes one file for each source file names it after its source with its word after
  the name, so `x_linux.go` gives `x_linux_builder.go`, whose name constrains nothing.
- The emit model gives a generated file annotations for file-level directives, and nothing writes
  them.

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-lang-go/frontend` | It records the build constraint of each file as the annotation `go:build` of the file, and stamps the same expression on a file that the constraint leaves out |
| `eidos-core/plugin` | `File` has the identity and the annotations of the one source file that it derives from |
| `eidos-core/layout` | `Route` sets them on a file whose units derive from one source file |
| `eidos-core/workspace` | A group reads the declaration edge of the source file of each of its files |
| `eidos-core/backend/render` | The file skeleton receives the annotations of the source file |
| `eidos-lang-go/backend` | The file skeleton writes the `go:build` annotation of the source above the package clause, through the helper `constraint` |

### Invariants

1. A generated Go file that derives from one source file compiles under the build constraint of that
   source and under no other.
2. A warm run writes a generated file again after an edit of the constraint of its source.

### The constraint of a file

The Go frontend computes the build constraint of each file as one expression of `go/build/constraint`,
the conjunction of two parts:

- The `//go:build` line, or the conjunction of the `// +build` lines where the file has no
  `//go:build` line, as the load already reads them.
- The GOOS and GOARCH tags that the name of the file implies, as the go command reads them.

A line that does not parse does not add to the expression. A file with a constraint gets the
annotation `go:build`,
whose one argument is the expression as `constraint.Expr.String` writes it, such as
`linux && amd64`. The annotation comes before the other annotations of the file. A file that the
constraint leaves out keeps its stamp `golang.constraint`, whose value is the same expression. A
`//go:build` line inside the comment of a declaration is not a constraint, and the frontend leaves
it out of the annotations of the declaration as before.

### The source of a routed file

`plugin.File` gets two fields:

- `Source` is the identity of the source file that every unit of the file derives from, and zero
  for a file of a package, of a plan, or of two or more source files.
- `Annotations` are the annotations of that source file.

`layout.Route` sets both where every unit of a file is a per-source unit with one key. The key is
the path of the source file, so the identity of the file has the language and the package of the
units, the path as its name, and the kind `File`. The layout looks the file up in its index.

A group reads the declaration edge of the source file of each of its files. An edit of the source
file changes its fingerprint, so the edge goes dirty, and the warm run routes and renders the file
again. A group whose units derive from the edited file goes dirty through its invocations already,
so the read adds work only after an edit of a declaration that the generated file does not derive
from.

### The skeleton

The render passes the annotations of the source file to the file skeleton beside the name and the
package of the file. The Go skeleton writes the annotation `go:build` as `//go:build <expression>`,
followed by a blank line, above the package clause. It does not write another annotation of the
source, because a `//go:generate` line in a generated file would run its generator a second time. The
output contract writes its header above the constraint, and the go command reads a `//go:build` line
after other line comments. gofmt leaves the order as it is.

The TypeScript, Java and Rust skeletons write nothing from the annotations.

### Verdicts

| Capability | Verdict | Reason | Reverses the verdict |
|---|---|---|---|
| File-level directives that a plugin places on a generated file through `emit.File` | out | No generator of the ensemble writes a file-level directive, and the Go backend skips an `emit.File` in a unit under `UnspeltKind`. The constraint of a source reaches its generated file through the layout | A generator that writes a file-level directive of its own, such as a constraint that no source states |

### Failures

| Condition | Report |
|---|---|
| A `//go:build` line that does not parse | None. The line adds nothing to the constraint, and the go command reports it |
| A generated file of a package or of two or more sources | None. The file gets no constraint, as the limit below states |

### Limits

A file of a per-package family, such as the registry of a package, derives from every source file of
the package. It does not get a constraint, so it compiles where a constrained source does not when
it refers to a declaration of that source.

### Migration

1. Record the constraint of each file in the Go frontend.
2. Give `plugin.File` its source, set it in `layout.Route`, and read its edge in the groups.
3. Pass the annotations of the source to the skeleton, and write the constraint in the Go skeleton.
4. Reproduce the defect of `catalog/host.go` in the reference tree, and check the generated file
   under `GOOS=windows`.

## Alternatives

### The suffix of the source in the name of the generated file

The Go file speller could keep the GOOS and GOARCH suffixes of the source at the end of the name of
the generated file, so `x_linux.go` gives `x_builder_linux.go`. The name covers no `//go:build`
line, and it changes the names of generated files.

### A constraint of each plugin

Each generator could copy the constraint of its source through `emit.File`. Every generator of the
ensemble would repeat the rule, and one that leaves it out would write a file that breaks.

### The conjunction of the constraints of every source

The layout could give a file of a package the conjunction of the constraints of the sources of its
origins. A file of a package also refers to declarations that are no origin of it, such as the
types of fields, so the conjunction would be incomplete, and it would leave out every platform that
one constrained source excludes.

## Drawbacks

- An edit of a source file renders the files that derive from it again, also where the edit leaves
  them unchanged.
- A file of a package keeps the limit above.

## Open questions

None.

## Unresolved and future work

None.

## References

- D87 in [21](../architecture/21-decisions.md), the annotations of a file.
- [RFC-0052](0052-go-parameters-as-subjects-and-the-legacy-build-line.md), the `// +build` lines.
- [Build constraints](https://pkg.go.dev/cmd/go#hdr-Build_constraints) of the go command.
