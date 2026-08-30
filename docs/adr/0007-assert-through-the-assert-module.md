---
adr: 0007
title: Test code asserts through the assert module
status: Accepted
date: 2026-08-30
supersedes: none
superseded-by: none
---

# ADR-0007: Test code asserts through the assert module

## Status

Accepted

## Context

Every test in the kernel hand-rolls its comparisons and its failure
text on the standard library: 964 `t.Fatal` sites across 44 test
files, 11 of them re-implementing error-text matching and 10
re-implementing slice or struct comparison. Each site decides its
own failure wording, so what a failure looks like depends on which
file it is in.

That practice leaves two of the suite's own needs unmet. The shared
test fixtures fail through `Fatalf`, so exercising a failure branch
fails the test that exercises it, and those branches stay uncovered
for want of a recorder. The repository's tests also state their
contracts in subtest names while the failures state only what was
observed, so nothing ties a failure back to the contract it broke.

`go.dokimi.dev/assert` is the organisation's own assertion library:
four surfaces over one comparison core.

| Surface | What it does |
|---|---|
| `assert` | stops the test at the first failure |
| `expect` | records and continues; generated from the same core, so the two cannot drift |
| `golden` | compares output against a file that records what it should be |
| `bench` | fails a benchmark that exceeds a declared ceiling |

A conformance gate holds `assert` and `expect` to one
language-neutral standard. A `Recorder` seat lets a test read what
an assertion reported, which is how an assertion gets tested. Every
assertion takes a contract message last and prints it as the
failure's first line, followed by a structural diff that reaches
unexported fields. Its only dependency is `go-cmp`. The module is
unpublished and lives beside this repository.

## Decision

Write test checks through `go.dokimi.dev/assert`, because one
library states the contract on every failure where 964 hand-rolled
sites each spell their own. Each module that tests with it requires
the module and replaces it with the sibling checkout at
`../../assert-go`.

## Alternatives Considered

### Keep hand-rolling on the standard library

The practice until now, and the zero-dependency purity is real:
`eidos-core/go.mod` requires nothing today.

It lost on the counts above. A hand-rolled `Fatalf` helper cannot
have its failure path tested, because the only way it reports is by
ending the test, which is why the fixtures' failure branches are
uncovered. Every new package also re-decides its failure wording,
and 44 files already disagree.

### Publish the module first, adopt it after

Adoption would then pin a version instead of a directory.

It lost because publishing freezes the API, and nothing has used
this library yet, so whatever is wrong with it would freeze too.
This repository is its first consumer: reviewing it here can still
change the API, and not waiting costs one sibling checkout and a
replace directive per module.

## Consequences

**Positive:**

- A failure's first line states the contract that broke; the diff
  that follows is structural, reaches unexported fields, and is
  rendered once by one library instead of per call site.
- The fixtures' failure branches become testable: `Rejects` drives
  a check against a wrong implementation on a `Recorder` seat and
  answers the failure message, so a helper's rejections are
  assertions like any other.
- One comparison semantics repository-wide: nil-versus-empty,
  NaN, and float exactness are decided once, with per-call options
  where a test needs them relaxed.
- Benchmarks state ceilings in code: an allocation bound is
  machine-independent and holds the zero-alloc paths at zero,
  while a latency bound is per-machine and belongs beside the
  pinned-baseline gate rather than in its place. Today's
  benchmarks only record numbers.
- A file-shaped expectation is one call: rendered output and
  serialised documents compare against a golden file with scrubbing
  and an explicit update flag, instead of a hand-rolled read,
  compare and rewrite in every such test.

**Negative:**

- A module carrying the replace cannot be published: replace
  directives apply only in the main module, so a consumer would
  see a requirement on an unpublished module and fail to resolve
  it. Publishing the assert module is a precondition for
  publishing any module that asserts through it.
- Every contributor and CI checkout needs `assert-go` beside this
  repository, and the replace tracks whatever that directory
  holds: there is no version pin, so two machines can test against
  two states of the library.
- `go.mod` and `go.sum` gain the assert module and `go-cmp`.
  Production code stays standard-library-only by convention and
  review; nothing mechanical refuses an assert import outside a
  test file.
- An aborting assertion stops the test through `Fatalf`, which
  only the test's own goroutine may call. Checks made inside
  spawned goroutines keep using `t.Errorf` or the `expect`
  surface, which records and continues.
- 964 sites across 44 files are rewritten, and until that
  finishes the suite holds two assertion styles side by side.

**Neutral:**

- The contract message sometimes restates the subtest name. That
  is the convention working: the name addresses the reader, the
  message addresses the failure.
- The generator's mirror guard keeps its own byte-comparison. It
  holds a committed tree equal to a regeneration, which is a build
  invariant over the repository rather than a testdata expectation,
  so that one comparison stays as it is.

## References

- `go.dokimi.dev/assert`, the library: package documentation and
  the conformance standard live in its repository, checked out
  beside this one
- [13-testing-and-conformance.md](../architecture/13-testing-and-conformance.md),
  the testing discipline the suite is held to
