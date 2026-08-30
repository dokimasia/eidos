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

Two of the suite's own needs go unmet by that practice. The shared
test fixtures fail through `Fatalf`, so their failure branches
cannot be exercised without failing the test that exercises them —
they sit uncovered for lack of a recorder. And the repository's
tests state their contracts in subtest names while the failures
state only observations; nothing ties a failure back to the
contract it broke.

`go.dokimi.dev/assert` is the organisation's own assertion library,
four surfaces over one comparison core. `assert` stops the test at
the first failure; `expect` records and continues, generated from
the same core so the two cannot drift, with a conformance gate
holding both to a language-neutral standard. `golden` compares
output against a file recording what it should be, which is the
shape a generator's rendered output wants. `bench` fails a
benchmark that exceeds a declared ceiling. A `Recorder` seat lets
an assertion be tested by reading what it reported. Every
assertion takes a contract message last and prints it as the
failure's first line, followed by a structural diff that reaches
unexported fields. Its only dependency is `go-cmp`. The module is
unpublished and lives beside this repository.

## Decision

Write test checks through `go.dokimi.dev/assert` and its
surfaces — `assert` to stop, `expect` to continue, `golden` for
file-shaped expectations, `bench` for benchmark ceilings —
required by each module that tests with it and replaced to the
sibling checkout at `../../assert-go`, because one library states
the contract per failure where 964 hand-rolled sites each spell
their own.

## Alternatives Considered

### Keep hand-rolling on the standard library

The practice until now, and the zero-dependency purity is real:
`eidos-core/go.mod` requires nothing today.

It lost on the counts above and on what they cannot do. A
hand-rolled `Fatalf` helper cannot have its failure path tested,
because failing is dying; the fixtures' failure branches are
uncovered for exactly that reason. Every new package re-decides
failure wording, and the drift is already visible across 44 files.

### Publish the module first, adopt it after

Adoption would then pin a version instead of a directory.

It lost because the surface freezes at publication, and a library
published before its first real consumer freezes its mistakes. This
repository is that consumer: the feedback flows while the API can
still move, and the cost of not waiting is one sibling checkout and
a replace directive per module.

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
- The generator's mirror guard keeps its own byte-comparison: it
  holds a committed tree equal to a regeneration, which is a build
  invariant over the repository, not a testdata expectation. That
  is one comparison staying put, not a surface going unused.

## References

- `go.dokimi.dev/assert`, the library: package documentation and
  the conformance standard live in its repository, checked out
  beside this one
- [13-testing-and-conformance.md](../architecture/13-testing-and-conformance.md),
  the testing discipline the suite is held to
