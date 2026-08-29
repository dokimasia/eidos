# Incrementality and performance

*Builds on: [02](02-symbol-model.md) (identity),
[04](04-metadata.md) (read sets),
[08](08-workspace-and-plans.md). Feeds:
[17](17-output-and-determinism.md) (warm≡cold),
[19](19-benchmarking.md) (the proof obligations).*

The engine that makes the monorepo envelope (10k+ packages,
sub-second warm regeneration) hold. The design premise: loading and
type-checking source dominates end-to-end time, so a cache that sits
*below* the loader memoises the cheap part and pays serialization on
top — a net loss. Every layer here sits above the expensive work it
skips. Every number in this document is a target until the
dedicated-runner baseline exists
([19-benchmarking.md](19-benchmarking.md)); none is a measurement.

## The dependency graph

```
input files ─► frontend units ─► symbols ─► metadata facts ─► emit decls ─► rendered files
                                            (symbol × key)
```

Every derived artifact records its **read set**: reads flow through
tracking Readers, so the captured edges are the plugin's actual
inputs. Edges key on canonical symbol identity
([02-symbol-model.md](02-symbol-model.md)) — a reparse that leaves a
declaration unchanged leaves its identity, and therefore every edge
through it, intact. The same edges serve `explain` (provenance
answers "derived from what", [04-metadata.md](04-metadata.md)) and
invalidation, so neither can drift from the other.

## The layers, cheapest first

1. **Fingerprint gate above the loader.** A stat-first pass decides
   whether to load at all — size and mtime answer for unchanged
   files, content hashes run only on suspicion (a file whose stat
   moved) — orders of magnitude cheaper than the load it skips
   (target: on the order of 100 ms for 100k files, warm; hashing
   everything every run would forfeit the target by itself). Unit
   fingerprints fold in the plugin-set fingerprint: an upgraded
   plugin invalidates graphs it must reinterpret, because the
   cached graph carries the metadata downstream plugins read. The
   fingerprint's completeness is structural, not disciplinary: the
   frontend kit's `u.Read` is the only door bytes enter through
   ([11-languages.md](11-languages.md)), and every read through it
   feeds the unit fingerprint — a frontend cannot read what the
   cache doesn't know about, so the stale-but-current failure mode
   is unexpressible. The gate also enforces the
   outputs-are-never-inputs law
   ([08-workspace-and-plans.md](08-workspace-and-plans.md)): paths
   the workspace owns — manifest entry or provenance trailer —
   are excluded here, at stat cost, before any parse. Declared
   output families make a cheap pre-filter; ownership proof is
   the truth, because `out=` redirects and the orphaned outputs
   of a removed plugin match no current declaration.
2. **Scoped and signature-only loading.** Patterns define scope;
   in-scope symbols load fully, dependency symbols load as
   signatures only — declarations without bodies or private members
   ([11-languages.md](11-languages.md) covers binary dependency
   formats). The graph you mutate is small; the graph you resolve
   against is shallow. This is what keeps memory-resident honest at
   10k packages.
3. **Red-green re-derivation** along recorded edges, with early
   cutoff (below), per plan.
4. **Parallelism**: frontends per unit, within-bucket plugin
   parallelism (opt-in), and plans in parallel — plans are
   read-isolated by construction; only declared export edges
   sequence them.

## The artifact granule

Emit is never persisted; files are — so the re-execution granule is
the **artifact** (one generated file). The ledger's artifact table,
part of the sealed state, holds per artifact:

```
ArtifactRow{ plan, relpath,
  inputs:  hash( contributing (rule, subject, directive-instance)…,
                 each subject's read-set closure values,
                 plugin version, template tree hash, options hash,
                 policy + target lowering version, dependency
                 exports' hashes ),
  contributors: []subjectID,   output: contentHash }
```

Warm run, per plan: the dirty symbol frontier (Link's diff plus
fact re-stamps surviving early cutoff) intersects contributor sets
and read-set closures → the dirty artifact set. A dirty artifact
re-fires its contributing rules **for its whole contributor set** —
an accumulator file is one output and rebuilds whole. A clean
artifact executes *nothing*: no dispatch, no render, no write; its
manifest row carries forward and its bytes stay on disk (drift
still checked). O(dirty) is real without persisting emit because
"whose inputs moved" is a table lookup, not a rerun. Two earlier
laws are what make partial re-execution byte-stable: contributions
order by subject identity, and gate-free handlers make contributor
sets computable from tables rather than from running code.

An artifact whose identity changes (layout inputs moved its path)
is not "carried": the old row's producer is gone — swept — and the
new path is a new artifact. The sweep and the table agree by
construction because both read the same records.

## Red-green with early cutoff

- Change detection: input fingerprints diff → dirty seeds →
  propagate along recorded edges.
- **Early cutoff**: a dependent recomputes only if a read *value*
  changed. An annotator that re-runs and stamps the identical fact
  leaves everything downstream green.
- **Rule gates are subscription records.** The authoring surface's
  declarative gates ([06b-authoring.md](06b-authoring.md)) — `(kind,
  directive)`, `(kind, originFact)` — are dirty-routing keys: the
  engine wakes exactly the rules whose gate a change can affect,
  which is why gates hidden inside handlers are banned; the engine
  cannot route around selectivity it cannot see.
- **Structural reads have two grains**
  ([02-symbol-model.md](02-symbol-model.md) pins the rule per
  query): targeted reads record per-identity edges; enumerations
  record set-membership edges — an added or removed symbol wakes
  enumerators, a mutated one wakes only its targeted readers.
- **Granularity is (symbol, key)** for metadata reads. Rationale,
  measured against the language landscape: bags are fat (annotation
  lifting alone puts dozens of `java.*` keys on one class) and
  readers are many (one Go struct is read by the go plan for its
  own facts, the mocks plan for shape facts, the Python-client plan
  for canonical-type facts). Per-symbol granularity would wake
  every generator in every plan on any stamp change; per-key,
  unrelated plans don't even wake. The bookkeeping is O(actual
  reads) — reads already flow through `meta.Key.Get` on the
  tracking Reader, and the per-key value diff is the comparison
  early cutoff needs anyway.

### The warm pass, step by step

The rules above, applied in order — the warm body of
`Workspace.Run` ([08-workspace-and-plans.md](08-workspace-and-plans.md)),
written out so O(dirty) names an algorithm rather than a hope:

1. **BeginRun.** Read `CURRENT` and validate the state header
   (the sealed state, below): format version, contract version,
   plugin-set fingerprint. Any mismatch discards the generation
   and the run goes cold. Stat-first sweep over the scope:
   changed, added, and removed files become the dirty unit set;
   owned outputs are excluded (the gate, above).
2. **Load.** Reparse dirty units at their recorded `Depth` —
   consulting the parse memo first (below): a memo hit restores
   the region without parsing. Carried regions stay sealed until
   a read touches them.
3. **Diff by identity.** Unchanged declarations keep their
   canonical identities
   ([02-symbol-model.md](02-symbol-model.md)); the changed
   remainder seeds the dirty symbol frontier S.
4. **Link the frontier.** Resolutions that changed — a spelling
   now landing on a different identity — extend S.
5. **Annotate.** Wake exactly the rules whose gate tuple —
   (kind), (kind, directive), (kind, factKey) — intersects S.
   Every re-stamp compares against the persisted fact value:
   equal is a green stop (early cutoff); changed joins S as
   (symbol, key) dirt.
6. **Generate, per plan** (export topo order, else parallel).
   Dirty artifacts: rows whose contributor set or read-set
   closure intersects S, plus rows whose non-graph inputs moved
   (plugin version, options, template tree, policy, dependency
   exports). Each dirty artifact re-fires its contributing rules
   over its whole contributor set; a clean artifact executes
   nothing and carries its manifest row.
7. **Layout → Render → Stage**, re-fired artifacts only.
8. **Close, commit.** Close reads records and is cold-identical
   by construction; plan commits run, then the state writer's
   `Commit` — the ledger's `CommitRun`, strictly last
   ([08-workspace-and-plans.md](08-workspace-and-plans.md)).

## The interning law

D44's boundary-strings rule, extended to performance: every
registered name that the engine handles in volume — canonical
identities, metadata keys, file paths — carries a **dense
run-local ID**, interned once; the string spelling exists only at
boundaries (manifests, exports, explain, JSON). Read-set records
are packed interned-ID pairs, deduplicated per derived artifact;
edge comparisons are integer compares. And the L-scale memory
story is the bags: a symbol's `meta.Bag` is **small-slice-backed
over interned key IDs**, upgrading to a map only past a threshold —
a two-fact bag must not pay a map's overhead a few million times.

## The sealed state, designed

D9 requires the persisted state to be lazily loadable; this is the
design that discharges the requirement. The stored set is
[08](08-workspace-and-plans.md)'s trio — sealed graph, fact store,
artifact table — plus the header that gates all three.

**On disk.** One generation is one directory in the brand's state
dir ([20-cli.md](20-cli.md)):

```
.<brand>/state/
  CURRENT              names the live generation ("gen-41");
                       rewritten atomically at commit
  gen-41/
    header             format + contract versions, plugin-set
                       fingerprint, section checksums
    graph              intern table · region index · regions
    facts              per-symbol bags, packed interned pairs
    artifacts          the ArtifactRow table
```

Commit writes `gen-42/` completely, fsyncs, atomically rewrites
`CURRENT`, then best-effort deletes `gen-40/`. A crash at any
point leaves `CURRENT` naming a complete generation — old or new,
never partial — the conservative half of the two-phase commit
([08-workspace-and-plans.md](08-workspace-and-plans.md)).

**The graph file** is an intern table, a region index, and
regions. A region is one frontend unit — so the parse granule,
the graph-write lock granule, and the invalidation granule are
one grain, with no translation layer between them. The index maps
unit identity to (offset, length, fingerprint, checksum); opening
the file costs the header, the index, and the intern table.
Regions decode on first touch: dirty regions eagerly at Load,
clean regions only when a tracked read lands in one. The
lazy-region behavior is the contract — open cost proportional to
what the run touches — and memory-mapping is a permitted
mechanism behind it, never a mandated one: the kernel does not
buy a performance property with platform-specific machinery when
the observable contract can be stated without it.

**Encoding** is length-prefixed and stdlib-only (the kernel's
zero-dependency law). Canonical identities, key names, and paths
live once in the intern table; regions and tables reference dense
IDs per the interning law. IDs are generation-local, assigned at
write; the canonical identity is the join, as everywhere
([02-symbol-model.md](02-symbol-model.md)).

**The fact store** persists each bag as packed (keyID, value)
pairs — the same small-slice layout the in-memory bags use, so a
warm open restores bags without re-annotation, and the persisted
values are exactly what early cutoff diffs a re-stamp against.
Facts persist because they must: bags that die with the run force
full re-annotation every warm run, O(subjects) against the
envelope.

**The failure posture is cold, never wrong.** A version mismatch,
a checksum failure, a truncated section: the generation is
discarded and the run proceeds cold, reporting one Info. Stale or
damaged state must never be able to fail a run that source alone
could complete — the state is an accelerator, and an accelerator
with veto power is a dependency.

The contracts, pinned — the byte format behind them is versioned
and private; these are what the engine holds:

```go
type SealedState interface {
    Header() StateHeader                   // versions, fingerprints
    OpenRegion(u UnitRef) (Region, error)  // decode on first touch
    Facts() FactSnapshot                   // what early cutoff diffs against
    Artifacts() []ArtifactRow
}
type StateWriter interface {
    PutRegion(u UnitRef, r Region) error
    PutFacts(FactSnapshot) error
    PutArtifacts([]ArtifactRow) error
    Commit() error                         // fsync, swap CURRENT — CommitRun
}
```

Considered and refused: **an embedded database** — a third-party
dependency in the zero-dep kernel, and a B-tree's write paths buy
generality the write-whole-generation swap never uses; **one file
per unit** — 10k+ opens per warm run at L, a syscall floor that
erodes the sub-second budget by itself; **deserialize-everything
encodings** (gob, JSON) — the daemon pressure D9 exists to avoid,
reintroduced as a format; **committed state** — D63's manifest
argument, worse, because the conflicts are binary.

## The parse memo

The sealed state answers the edit loop; the memo answers
**history**. It is the second, optional persistence layer: a
content-addressed store of parsed regions keyed by (unit
fingerprint, frontend version) — both values that already exist —
holding the same serialized region encoding the graph file uses.

Why a generational snapshot is not enough: switch branches and the
fingerprint diff marks thousands of units dirty against generation
N — but most of those parses happened in *some* earlier run. The
memo is where they still live. A unit dirty against the current
generation but seen in any past one is a memo hit: the region
restores without parsing.

The rules that keep it honest:

- **One client.** Only the Load path consults it, between "sealed
  region" and "reparse". Plugins never see it; the `u.Read` door
  and the fingerprint discipline are untouched, because a memo hit
  means `Parse` never runs at all.
- **Correct by key.** Same bytes plus same frontend version is the
  same parse — the hit-equals-fresh-parse property is a tautology
  of the key, not a promise, and the frontend-version fold is why
  a frontend upgrade misses cleanly. Warm≡cold licenses the memo
  the way it licenses everything.
- **Eviction is the contract.** The memo is size-capped (the
  `Cache` config group, [08-workspace-and-plans.md](08-workspace-and-plans.md)),
  evicted least-recently-used, enforced at `CommitRun`. A cache
  that grows monotonically is a disk leak with a cache's
  reputation — this cap is the eviction policy v1 deferred to a
  layer that never came.
- **Writes are boring.** Every fresh parse writes through;
  entries are immutable, written atomically, keyed by content —
  the sink's discipline, reused.

**CI restores are safe by design.** Both layers — state and memo —
can be cached and restored by CI machinery: header validation,
checksums, and the cold fallback mean a stale, truncated, or
foreign restore costs time, never correctness. There is
deliberately **no remote or shared cache protocol** — the
no-daemon argument again: a protocol becomes public API with
version skew and an availability dependency, while a directory
restored by the CI's own cache step captures the win without
either. Revisit trigger, recorded: an organisation demonstrably
saturating CI-restore bandwidth.

`run --cold` ([20-cli.md](20-cli.md)) ignores both layers without
deleting them — the debugging handle, and the fresh-state leg of
the warm≡cold rung.

## The non-negotiable

**Warm ≡ cold, byte for byte.** A conformance rung runs the same
workspace cold and warm and diffs the manifests
([13-testing-and-conformance.md](13-testing-and-conformance.md)).
Every optimization above — memoization, cutoff, laziness, parallel
plans — is licensed by that rung and only that rung. A cache that
cannot prove equivalence is a determinism bug wearing a speedup.

The envelope itself is executed, not asserted:
[19-benchmarking.md](19-benchmarking.md) defines the corpus
generator, the scenario matrix that pins these layers one by one,
and the budget gates that block a release on a speed regression.

## Batch-only delivery: no daemon

**Decision: there is no daemon and no client-server protocol.**

A Go process starts in milliseconds; the fingerprint pass is
µs-per-file stats; the only real warm cost is opening the sealed
graph — which is a *format* problem, not a process-lifecycle
problem. A daemon's costs are the ugly kind: a versioned protocol
that becomes public API, state drift, snapshot double-buffering, and
the restart-it support tax. For a generator — invoked on save,
pre-commit, CI — batch latency targeted at the low hundreds of ms
meets the envelope without any of it.

What replaces it:

- **The sealed-state design above**: region-lazy, open cost
  proportional to the dirty region. The property is load-bearing;
  a deserialize-everything format silently reintroduces the
  daemon pressure.
- **`--watch`**: an optional command kernel that polls the
  fingerprint gate — the stat/hash pass *is* the change detector,
  and at the targeted sweep cost a one-second poll is cheap. One
  process, the ordinary engine, no OS watcher dependency (the
  kernel stays zero-dep), no protocol, no state another process can
  drift from.
- Revisit trigger, recorded: an IDE surface (explain-on-hover,
  directive completion) being built is the one client that
  genuinely wants residency.
