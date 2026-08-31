# Benchmarking

*Builds on: [09](09-incrementality.md) (the claims under test).
Feeds: [15](15-compatibility.md) (published, budget-gated results).*

The performance target in [09-incrementality.md](09-incrementality.md)
is a claim: ten thousand packages or more, warm regeneration under a
second, and warm cost that tracks the edit rather than the corpus.
This project executes its claims rather than asserting them, so the
target gets the same treatment as compatibility and determinism.

## The corpus generator

Benchmarks run against seeded synthetic workspaces, built by a
deterministic generator in the kernel's `conformance/bench`. Real
repositories are the wrong fixture: you cannot parameterize them,
they drag licences along, and they pin the benchmark to one shape of
code.

The generator emits source trees per language, parameterized by
package count, symbols per package, cross-package reference density,
directive density and language mix. The same seed produces the same
bytes. The parameter set, pinned:

```go
type Corpus struct {
    Seed             int64
    Packages         int                      // S 100 · M 1k · L 10k
    SymbolsPerPkg    int
    RefDensity       float64                  // cross-package refs per symbol
    DirectiveDensity float64                  // directives per 100 symbols
    Mix              map[rules.Source]float64 // sums to 1
}
```

Three sizes anchor every scenario: **S** at 100 packages, **M** at
1,000, and **L** at 10,000, which is the target size. The ratios
between sizes carry as much weight as the absolute numbers, for the
reason the budgets section gives.

## The scenario matrix

Each macro scenario measures wall time, per-phase attribution across
load, link, annotate, generate, render and close, peak RSS, and
allocations:

| Scenario | What it asserts |
|---|---|
| cold, full run | O(corpus) is accepted; the amortized per-symbol budgets hold |
| warm, no change | the fingerprint gate does its job: nothing recomputes, and L stays under the sub-second bound |
| warm, one source file edited | cost tracks the dirty set: invalidation, early cutoff, and re-rendering only the affected files |
| warm, one annotation-relevant edit | (symbol, key) granularity: unrelated plans do not run again |
| warm, export-chain edit | a dependent plan runs again and independent plans do not |
| multi-plan, 1 against 3 plans on one corpus | plan parallelism: three plans cost far less than three times one plan |
| sealed-graph open, cold process | the format requirement: a partial load proportional to the dirty region rather than to the graph |

Micro benchmarks pin the per-operation costs the macro scenarios are
built from: Callable projection, TypeOf, Resolve, per-kind template
render, the fingerprint pass per file, and a metadata read through
the tracking Reader. Every micro benchmark pins allocations per
operation alongside time.

## Budgets: what gates where

Timing on a shared CI runner is noise, and a gate that flakes
teaches people to ignore it. So the budgets split by what is
independent of the machine.

**Scaling-rule gates run everywhere.** They compare ratios rather
than milliseconds: warm-no-change must not grow with corpus size,
with the L/M ratio bounded near 1; warm-one-edit must track the
dirty set, with the same edit at M and L staying within a bounded
ratio; and the multi-plan speedup must clear a floor. These assert
the architecture's complexity claims and do not care how fast the
runner is.

**Allocation gates run everywhere.** Allocations per operation on
the micro suite, and total allocations per macro scenario, both
pinned exactly. Allocation counts do not move with machine load, so
a regression means a real code change.

**Absolute-time gates run on dedicated runners only.** The target
numbers, meaning warm-no-change at L under the sub-second bound, the
fingerprint pass, and the sealed-graph open, gate against a fixed
runner profile. Each compares with the previous release's results on
that same profile, with significance testing over repeated runs, so
a regression is a shift in the distribution rather than one slow
run.

**Memory gates.** Peak RSS at L bounds the resident graph through a
bytes-per-symbol ceiling. The target promises monorepo scale on a
developer's machine, so memory gets budgeted like time.

The discipline is uniform across all of it. Benchmarks run one
package at a time, because a whole-suite run inflates timings even
though allocation counts survive it. The CPU count is fixed, warm-up
iterations are excluded, and the harness ships profiling flags, so
any scenario can emit CPU and heap profiles for the regression it
just caught.

**Where the initial budgets come from.** The target numbers in
[09-incrementality.md](09-incrementality.md) are ceilings, set by
requirement. Everything finer starts as a baseline: the first
implementation to pass the scenario matrix on the dedicated-runner
profile records its distributions, and the budgets commit to the
repository as that baseline times a stated headroom factor, 1.2 by
default. Tighten a budget deliberately; never loosen one quietly. A
budget that predates its baseline is a target and says so, and the
suite marks it advisory until the baseline arrives. This is the one
place where the specification's numbers are allowed to be
provisional, and it is labelled.

The budgets are a committed file the harness reads and the release
pipeline enforces:

```yaml
# conformance/bench/budgets.yaml
scaling:
  warm-no-change: {ratio: L/M, max: 1.15}
  warm-one-edit:  {ratio: L/M, max: 1.5}
allocs:
  callable-projection: {per-op: 3}
absolute:                    # dedicated runners only
  warm-no-change-L: {max-ms: 900, baseline: pending}  # advisory
```

## Who runs what

- **The kernel** benchmarks its own layers: the engine, the store,
  fingerprinting, the sealed-graph format and the template
  machinery. That means corpus-independent micro benchmarks plus
  graph-shaped macro scenarios over synthetic symbol trees.
- **Language satellites** run the full matrix for their language:
  frontend parse rates, projection micro benchmarks, backend render
  rates, and the macro scenarios over generated corpora
  ([11-languages.md](11-languages.md)).
- **eidos-reference** runs the multi-plan and cross-language
  scenarios. The canary is also the performance rig, so the release
  ring ([15-compatibility.md](15-compatibility.md)) catches speed
  regressions with the same machinery that catches API breaks.

Results against the previous release publish beside the support and
parity matrices. A budget failure blocks the tag the way a
conformance failure does.
