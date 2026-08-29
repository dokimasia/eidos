# Benchmarking

*Builds on: [09](09-incrementality.md) (the claims under test).
Feeds: [15](15-compatibility.md) (published, budget-gated
results).*

The performance envelope ([09-incrementality.md](09-incrementality.md))
is a claim: 10k+ packages, sub-second warm regeneration, warm cost
proportional to the edit rather than the corpus. This system's rule
is that claims are executed, not asserted — the envelope gets the
same treatment as compatibility and determinism.

## The corpus generator

Benchmarks run against **seeded synthetic workspaces**, produced by
a deterministic generator in the kernel's `conformance/bench`. Real
repositories are the wrong fixture: they cannot be parameterized,
they drag licenses along, and they pin the benchmark to one shape of
code. The generator emits source trees per language, parameterized
by: package count, symbols per package, cross-package reference
density, directive density, and language mix. Same seed, same bytes.
The parameter set, pinned:

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

Three standard sizes anchor every scenario: **S** (100 packages),
**M** (1k), **L** (10k — the envelope size). Ratios between sizes
are as load-bearing as absolute numbers (below).

## The scenario matrix

Macro scenarios, each measuring wall time, per-phase attribution
(load / link / annotate / generate / render / close), peak RSS, and
allocations:

| Scenario | Asserts |
|---|---|
| cold, full run | O(corpus) accepted; amortized per-symbol budgets hold |
| warm, no change | the fingerprint gate: nothing recomputes; the envelope's sub-second bound at L |
| warm, one source file edited | cost tracks the dirty set: invalidation, early cutoff, re-render of affected files only |
| warm, one annotation-relevant edit | (symbol, key) granularity: unrelated plans do not wake |
| warm, export-chain edit | a dependent plan re-runs, independent plans do not |
| multi-plan (1 vs 3 plans, same corpus) | plan parallelism: 3 plans ≪ 3× one plan |
| sealed-graph open, cold process | the format requirement: partial load proportional to the dirty region, not the graph |

Micro benchmarks pin the per-operation costs the macros are built
from: Callable projection, TypeOf, Resolve, per-kind template
render, fingerprint pass per file, metadata read through the
tracking Reader. Every micro benchmark pins **allocations per
operation** alongside time.

## Budgets: what gates where

Timing on shared CI runners is noise; a gate that flakes teaches
people to ignore it. So budgets split by what is machine-independent:

- **Scaling-law gates — run everywhere.** Ratios, not milliseconds:
  warm-no-change must not grow with corpus size (L/M ratio bounded
  near 1); warm-one-edit must track the dirty set (same edit at M
  and L within a bounded ratio); multi-plan speedup must clear a
  floor. These assert the architecture's complexity claims and are
  immune to runner speed.
- **Allocation gates — run everywhere.** Allocs/op on the micro
  suite and total allocations per macro scenario, exact-pinned.
  Allocation counts do not vary with machine load; a regression is
  a real code change.
- **Absolute-time gates — dedicated runners only.** The envelope
  numbers (warm-no-change at L under the sub-second bound; the
  fingerprint pass; sealed-graph open) gate on a fixed runner
  profile, compared against the previous release's results on the
  same profile with significance testing over repeated runs —
  a regression is a distribution shift, not one slow run.
- **Memory gates.** Peak RSS at L bounds the resident graph
  (bytes-per-symbol ceiling): the envelope promises monorepo scale
  on a developer machine, so memory is budgeted like time.

Discipline, uniform across all of it: benchmarks run
package-isolated (a whole-suite run inflates timings; allocation
counts survive, timings don't), fixed CPU count, warm-up iterations
excluded, and the harness ships profiling flags — any scenario can
emit CPU and heap profiles for the regression it just caught.

**Where the initial budgets come from.** The envelope numbers
([09-incrementality.md](09-incrementality.md)) are ceilings, set by
requirement. Everything finer starts as a **baseline**: the first
implementation to pass the scenario matrix on the dedicated-runner
profile records its distributions, and budgets commit to the repo as
baseline × a stated headroom factor (default 1.2×) — tightened
deliberately, never loosened silently. A budget that predates its
baseline is a target and says so; the suite marks it advisory until
the baseline lands. This is the one place the spec's numbers are
allowed to be provisional, and it is labeled.

Budgets are a committed file the harness reads and the release
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

- **The kernel** benches its own layers: engine, store, fingerprint,
  sealed-graph format, template machinery — corpus-independent
  micros plus graph-shaped macros over synthetic symbol trees.
- **Language satellites** run the full matrix for their language:
  frontend parse rates, projection micros, backend render rates,
  and the macro scenarios over generated corpora
  ([11-languages.md](11-languages.md)).
- **eidos-reference** runs the multi-plan and cross-language
  scenarios — the canary is also the perf rig, so the release ring
  ([15-compatibility.md](15-compatibility.md)) catches speed
  regressions with the same machinery that catches API breaks.

Per release, results against the previous release publish beside
the support and parity matrices. A budget failure blocks the tag
the way a conformance failure does.
