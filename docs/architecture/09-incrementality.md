# Incrementality and performance

*Builds on: [02](02-symbol-model.md) (identity),
[04](04-metadata.md) (read sets),
[08](08-workspace-and-plans.md). Feeds:
[17](17-output-and-determinism.md) (warm≡cold),
[19](19-benchmarking.md) (the proof obligations).*

This engine is what makes the target hold: ten thousand packages or
more, with warm regeneration under a second.

The premise it is built on: loading and type-checking source
dominates end-to-end time. A cache sitting *below* the loader would
memoise the cheap part and pay serialization on top, which loses.
Every layer here sits above the expensive work it skips.

Every number in this document is a target until the
dedicated-runner baseline exists
([19-benchmarking.md](19-benchmarking.md)). None of them is a
measurement.

## The dependency graph

```
input files ─► frontend units ─► symbols ─► metadata facts ─► emit decls ─► rendered files
                                            (symbol × key)
```

Every derived artifact records its **read set**. Reads flow through
tracking Readers, so the captured edges are the plugin's actual
inputs.

Edges key on canonical symbol identity
([02-symbol-model.md](02-symbol-model.md)), so a reparse that leaves
a declaration unchanged leaves its identity intact, and every edge
through it with it. The same edges serve `explain`, where provenance
answers what a fact was derived from
([04-metadata.md](04-metadata.md)), and invalidation. Neither can
drift from the other, because they are the same edges.

## The layers, cheapest first

**1. The fingerprint gate, above the loader.** A stat-first pass
decides whether to load anything at all. Size and mtime answer for
an unchanged file, and a content hash runs only on suspicion,
meaning a file whose stat moved. That is orders of magnitude cheaper
than the load it skips, and the target is on the order of 100ms for
100,000 files, warm. Hashing everything every run would miss the
target on its own.

Unit fingerprints fold in the plugin-set fingerprint, so an upgraded
plugin invalidates the graphs it now has to reinterpret, because the
cached graph carries the metadata that downstream plugins read.

The fingerprint is complete for a structural reason rather than a
disciplinary one. The frontend kit's `u.Read` is the only door bytes
enter through ([11-languages.md](11-languages.md)), and every read
through it feeds the unit fingerprint. A frontend cannot read what
the cache does not know about, so a stale graph that looks current
is not expressible.

The gate also enforces the rule that outputs are never inputs
([08-workspace-and-plans.md](08-workspace-and-plans.md)). Paths the
workspace owns, proven by a manifest entry or a provenance trailer,
are excluded here, at the cost of a stat, before anything parses.
Declared output families make a cheap pre-filter, but the ownership
proof is what decides, because `out=` redirects and the orphaned
outputs of a removed plugin match no current declaration.

**2. Scoped and signature-only loading.** Patterns define the scope.
In-scope symbols load fully, and dependency symbols load as
signatures only, meaning declarations without bodies or private
members ([11-languages.md](11-languages.md) covers binary dependency
formats). The graph you mutate stays small, and the graph you
resolve against stays shallow. This is what keeps a memory-resident
graph honest at ten thousand packages.

**3. Red-green re-derivation** along the recorded edges, with early
cutoff, per plan.

**4. Parallelism**: frontends per unit, plugins inside a bucket as
an opt-in, and plans against each other, since plans are
read-isolated by construction and only declared export edges
sequence them.

## The artifact granule

Emit is never persisted. Files are. So the re-execution granule is
the **artifact**, meaning one generated file. The ledger's artifact
table, part of the sealed state, holds per artifact:

```
ArtifactRow{ plan, relpath,
  inputs:  hash( contributing (rule, subject, directive-instance)…,
                 each subject's read-set closure values,
                 plugin version, template tree hash, options hash,
                 policy + target lowering version, dependency
                 exports' hashes ),
  contributors: []subjectID,   output: contentHash }
```

On a warm run, per plan: the dirty symbol frontier, meaning Link's
diff plus the fact re-stamps that survived early cutoff, intersects
the contributor sets and read-set closures, which gives the dirty
artifact set.

A dirty artifact re-runs its contributing rules **over its whole
contributor set**, because an accumulator file is one output and
rebuilds whole. A clean artifact executes nothing: no dispatch, no
render, no write. Its manifest row carries forward and its bytes
stay on disk, though drift is still checked.

O(dirty) is real without persisting emit, because "whose inputs
moved" is a table lookup rather than a rerun. Two earlier rules are
what make partial re-execution byte-stable: contributions order by
subject identity, and gate-free handlers make contributor sets
computable from tables rather than by running code.

An artifact whose identity changes, because layout inputs moved its
path, is not carried. The old row's producer is gone, so it is
swept, and the new path is a new artifact. The sweep and the table
agree by construction, because both read the same records.

## Red-green with early cutoff

Change detection works by diffing input fingerprints into dirty
seeds, then propagating along the recorded edges.

**Early cutoff**: a dependent recomputes only when a read *value*
changed. An annotator that runs again and stamps the identical fact
leaves everything downstream green.

**Rule gates are subscription records.** The authoring surface's
declarative gates ([06b-authoring.md](06b-authoring.md)), meaning
`(kind, directive)` and `(kind, originFact)`, are the keys the
engine routes dirtiness through. It runs exactly the rules whose
gate a change can affect, which is why a gate hidden inside a
handler is banned: the engine cannot route around selectivity it
cannot see.

**Structural reads have two grains**, and
[02-symbol-model.md](02-symbol-model.md) pins the rule per query. A
targeted read records a per-identity edge. An enumeration records a
set-membership edge, so adding or removing a symbol runs the
enumerators again, while changing one runs only its targeted
readers.

**Metadata reads have (symbol, key) granularity.** The reason,
measured against the language landscape: bags are fat, since
annotation lifting alone puts dozens of `java.*` keys on one class,
and readers are many, since one Go struct is read by the Go plan for
its own facts, by the mocks plan for shape facts, and by the
Python-client plan for canonical-type facts.

Per-symbol granularity would run every generator in every plan again
on any stamp change. Per-key, unrelated plans do not run at all. The
bookkeeping is O(actual reads), because reads already flow through
`meta.Key.Get` on the tracking Reader, and the per-key value diff is
the comparison early cutoff needs anyway.

### The warm pass, step by step

The rules above applied in order, which is the warm body of
`Workspace.Run`
([08-workspace-and-plans.md](08-workspace-and-plans.md)), written
out so that O(dirty) names an algorithm rather than a hope.

1. **BeginRun.** Read `CURRENT` and validate the state header,
   meaning the format version, the contract version and the
   plugin-set fingerprint. Any mismatch discards the generation and
   the run goes cold. Then run the stat-first sweep over the scope:
   changed, added and removed files become the dirty unit set, and
   owned outputs are excluded at the gate.
2. **Load.** Reparse the dirty units at their recorded `Depth`,
   consulting the parse memo first. A memo hit restores the region
   without parsing. Carried regions stay sealed until a read touches
   them.
3. **Diff by identity.** Unchanged declarations keep their canonical
   identities ([02-symbol-model.md](02-symbol-model.md)), and the
   changed remainder seeds the dirty symbol frontier S.
4. **Link the frontier.** A resolution that changed, meaning a
   spelling that now lands on a different identity, extends S.
5. **Annotate.** Run exactly the rules whose gate tuple, one of
   (kind), (kind, directive) or (kind, factKey), intersects S. Every
   re-stamp compares against the persisted fact value. Equal stops
   there, which is early cutoff. Changed joins S as (symbol, key)
   dirt.
6. **Generate, per plan**, in export topological order, otherwise in
   parallel. The dirty artifacts are the rows whose contributor set
   or read-set closure intersects S, plus the rows whose non-graph
   inputs moved, meaning plugin version, options, template tree,
   policy or dependency exports. Each dirty artifact re-runs its
   contributing rules over its whole contributor set, and a clean
   artifact executes nothing and carries its manifest row.
7. **Layout, Render, Stage**, for the re-run artifacts only.
8. **Close and commit.** Close reads records and is identical to the
   cold path by construction. The plan commits run, then the state
   writer's `Commit`, which is the ledger's `CommitRun` and is
   strictly last
   ([08-workspace-and-plans.md](08-workspace-and-plans.md)).

## The interning rule

The registered-name rule, extended to performance. Every registered
name the engine handles in volume, meaning canonical identities,
metadata keys and file paths, carries a **dense run-local ID**,
interned once. The string spelling exists only at the boundaries:
manifests, exports, explain and JSON.

Read-set records are packed pairs of interned IDs, deduplicated per
derived artifact, so comparing edges compares integers.

At L scale, memory comes down to the bags. A symbol's `meta.Bag` is
backed by a small slice over interned key IDs, and upgrades to a map
only past a threshold, because a two-fact bag must not pay a map's
overhead a few million times over.

## The sealed state, designed

The persisted state has to be loadable lazily, and this is the
design that delivers that. The stored set is
[08](08-workspace-and-plans.md)'s trio, meaning the sealed graph,
the fact store and the artifact table, plus the header that gates
all three.

**On disk**, one generation is one directory in the brand's state
directory ([20-cli.md](20-cli.md)):

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

Commit writes `gen-42/` completely, fsyncs it, atomically rewrites
`CURRENT`, then deletes `gen-40/` on a best-effort basis. A crash at
any point leaves `CURRENT` naming a complete generation, old or new
but never partial, which is the conservative half of the two-phase
commit ([08-workspace-and-plans.md](08-workspace-and-plans.md)).

**The graph file** holds an intern table, a region index and the
regions. A region is one frontend unit, so the parse granule, the
graph-write lock granule and the invalidation granule are one grain,
with no translation layer between them.

The index maps unit identity to an offset, a length, a fingerprint
and a checksum, so opening the file costs the header, the index and
the intern table. Regions decode on first touch: dirty regions
eagerly at Load, and clean regions only when a tracked read lands in
one.

Lazy regions are the contract, meaning open cost proportional to
what the run touches. Memory-mapping is a permitted mechanism behind
that contract rather than a mandated one, because the kernel does
not buy a performance property with platform-specific machinery when
it can state the observable contract without it.

**Encoding** is length-prefixed and uses only the standard library,
per the kernel's zero-dependency rule. Canonical identities, key
names and paths live once in the intern table, and regions and
tables reference dense IDs. The IDs belong to a generation and are
assigned at write time, and the canonical identity is the join, as
it is everywhere ([02-symbol-model.md](02-symbol-model.md)).

**The fact store** persists each bag as packed (keyID, value) pairs,
which is the same small-slice layout the in-memory bags use. So a
warm open restores the bags without re-annotating, and the persisted
values are exactly what early cutoff diffs a re-stamp against. Facts
persist because they have to: bags that die with the run force full
re-annotation on every warm run, which is O(subjects) against the
target.

**When the state is unusable, the run goes cold rather than wrong.**
A version mismatch, a checksum failure or a truncated section
discards the generation, and the run proceeds cold, reporting one
Info. Stale or damaged state must never fail a run that source alone
could complete, because the state is there to make things faster,
and something that can veto a run is a dependency instead.

The contracts, pinned. The byte format behind them is versioned and
private, and these are what the engine holds:

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

Considered and refused. **An embedded database**: a third-party
dependency in a kernel that has none, and a B-tree's write paths buy
generality that writing a whole generation and swapping never uses.
**One file per unit**: ten thousand or more opens per warm run at L,
which is a syscall floor that eats the sub-second budget by itself.
**Encodings that deserialize everything**, such as gob or JSON:
those reintroduce as a format exactly the pressure that ruled out a
daemon. **Committing the state**: the same argument that keeps the
manifest uncommitted, made worse by binary conflicts.

## The parse memo

The sealed state answers the edit loop. The memo answers
**history**.

It is a second, optional persistence layer: a content-addressed
store of parsed regions keyed by unit fingerprint and frontend
version, both of which already exist, holding the same serialized
region encoding the graph file uses.

A generational snapshot alone is not enough. Switch branches and the
fingerprint diff marks thousands of units dirty against generation
N, but most of those parses already happened in *some* earlier run.
The memo is where they still live. A unit that is dirty against the
current generation but was seen in any past one is a memo hit, and
the region restores without parsing.

Four rules keep it honest.

**One client.** Only the Load path consults it, between "sealed
region" and "reparse". Plugins never see it, and the `u.Read` door
and the fingerprint discipline are untouched, because a memo hit
means `Parse` never runs at all.

**Correct by key.** The same bytes plus the same frontend version is
the same parse, so a hit equalling a fresh parse follows from the
key rather than from a promise, and folding in the frontend version
is why a frontend upgrade misses cleanly. Warm≡cold licenses the
memo the way it licenses everything else.

**Eviction is part of the contract.** The memo is size-capped
through the `Cache` config group
([08-workspace-and-plans.md](08-workspace-and-plans.md)), evicted
least-recently-used, and enforced at `CommitRun`. A cache that only
grows is a disk leak, and this cap is the eviction policy that
version 1 deferred to a layer nobody ever built.

**Writes are boring.** Every fresh parse writes through. Entries are
immutable, written atomically and keyed by content, reusing the
sink's discipline.

**CI restores are safe by design.** CI machinery can cache and
restore both layers, state and memo, because header validation,
checksums and the cold fallback mean a stale, truncated or foreign
restore costs time and never correctness.

There is deliberately **no remote or shared cache protocol**, for
the same reason there is no daemon: a protocol becomes public API
with version skew and something that has to be available, while a
directory restored by CI's own cache step gets the win without
either. Revisit trigger, recorded: an organisation demonstrably
running out of CI-restore bandwidth.

`run --cold` ([20-cli.md](20-cli.md)) ignores both layers without
deleting them, which is the debugging handle and the fresh-state leg
of the warm≡cold rung.

## The non-negotiable

**Warm and cold produce identical bytes.** A conformance rung runs
the same workspace cold and warm and diffs the manifests
([13-testing-and-conformance.md](13-testing-and-conformance.md)).

Every optimization above is licensed by that rung and by nothing
else: memoization, cutoff, laziness and parallel plans. A cache that
cannot prove it produces the same answer is a determinism bug that
happens to be fast.

The target itself is executed rather than asserted.
[19-benchmarking.md](19-benchmarking.md) defines the corpus
generator, the scenario matrix that pins these layers one at a time,
and the budget gates that block a release on a speed regression.

## Batch only: there is no daemon

**Decision: no daemon, and no client-server protocol.**

A Go process starts in milliseconds. The fingerprint pass costs
microseconds per file in stats. The only real warm cost is opening
the sealed graph, and that is a format problem rather than a process
lifecycle problem.

A daemon's costs are the ugly kind: a versioned protocol that
becomes public API, state drift, snapshot double-buffering, and the
support burden of telling people to restart it. For a generator
invoked on save, at pre-commit and in CI, batch latency in the low
hundreds of milliseconds meets the target without any of that.

What replaces it. **The sealed-state design above**, which is
region-lazy, with open cost proportional to the dirty region. That
property carries weight: a format that deserializes everything
quietly brings the daemon pressure back.

And **`--watch`**, an optional command kernel that polls the
fingerprint gate, since the stat and hash pass already is the change
detector, and at the target sweep cost a one-second poll is cheap.
One process, the ordinary engine, no OS watcher dependency, since
the kernel takes no dependencies, no protocol, and no state another
process can drift from.

Revisit trigger, recorded: somebody building an IDE surface, meaning
explain-on-hover or directive completion, which is the one client
that genuinely wants a resident process.
