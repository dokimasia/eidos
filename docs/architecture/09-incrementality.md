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
memoise the cheap part and add serialization on top, which loses.
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
returns what a fact was derived from
([04-metadata.md](04-metadata.md)), and invalidation. Neither can
drift from the other, because they are the same edges.

## The layers, cheapest first

**1. The fingerprint gate, above the loader.** A stat-first pass
decides whether to load anything at all. Size and mtime stand in for
an unchanged file, together with the change time and the inode where
the platform's file information reports them, and a content hash
runs only on suspicion, meaning a file whose stat moved. That is
orders of magnitude cheaper than the load it skips, and the target is
on the order of 100ms for 100,000 files, warm. Hashing everything
every run would miss the target on its own. The walk skips the
brand's state directory.

Size and mtime miss one case. A file written twice within one
timestamp tick keeps both, so a second write after the run read the
file leaves a record that looks current and is not. The gate handles
such racily clean files with git's two rules for its index, against
an anchor that the generation's header records: the time the run's
stat sweep began, less a margin for the coarsest mtime resolution in
common use, FAT's two seconds, and for the tick of the clock that
stamps an mtime.

- At the gate, a file whose size and mtime match its record, and
  whose mtime is not older than the recording generation's anchor,
  is hashed instead of trusted.
- A commit writes a zero size into every such record, so the next
  gate that meets the file hashes it, even when a generation in
  between copied the record forward unchecked.

The anchor precedes the sweep because a run commits seconds after it
reads. An anchor at the commit would trust a file rewritten within
its own tick after the read.

A unit's key folds the digest of each member and each shared input
in roster order, the reads of the partition or dependency round that
returned the unit, the depth, the frontend's name, language, version
and options, the brand and the node model's fingerprint. The load
computes the key from the gate's digests before the parse, so the
parse memo can restore a region without parsing. The key does not
fold the composition, because a region depends on its frontend and
its inputs alone.

The generation's header checks the composition and the code once per
run. It records two digests:

- The composition fingerprint folds everything the composition reads
  from outside the executable: its plugins with their versions and
  options, its plans with their sources, dependencies and layouts, the
  brand, the workspace name, the ignored directive spellings and the
  template trees read from disk.
- The executable digest is the SHA-256 of the running executable. The
  run hashes the executable only where its path, size or modification
  time differs from the header's record.

A difference in either sends the run cold. The executable digest makes
an upgraded plugin safe without trusting its declared version: code
that changed is a different executable, and a rebuild of unchanged
source produces the same bytes.

The key is complete by construction. The frontend kit's `u.Read` is
the only door bytes enter through
([11-languages.md](11-languages.md)), and the door refuses every path
whose digest the key does not fold. A frontend cannot read what the
key does not cover, so a stale graph that looks current is not
expressible.

The gate also enforces the rule that outputs are never inputs
([08-workspace-and-plans.md](08-workspace-and-plans.md)). Paths the
workspace owns, proven by a manifest entry or a provenance trailer,
are excluded here before anything parses. The gate records each
file's verdict, so it reads a file's tail for the trailer only where
the file is new or its content changed. Declared output families make
a cheap pre-filter, but the ownership proof is what decides, because
`out=` redirects and the orphaned outputs of a removed plugin match
no current declaration.

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
the **artifact**, meaning one generated file. Artifacts that must
execute together form a **group**:

- the files a backend splits one unit into
- the files two units share
- the files that contain an emit value an emit-phase invocation
  matched, and the files that invocation places into

The sealed state's `artifacts` table records per artifact:

- the manifest entry: path, plan, digest, plugins and sources
- its group, whose row lists the group's units and the invocations
  that contributed to them
- the position and the description of its first declaration, which a
  finding about the file is reported at
- the export rows of its declarations
- the name entries it declares and the name entries its references
  read, across files of the plan
- the facts the settle read for it, each origin's name override
- the digest of its placement: its directory's residents and the
  modules containing it
- the findings the settle, the layout and the render reported for it

On a warm run, per plan, a group is dirty in each of these cases:

- A contributing invocation's read record meets the dirty set, or the
  invocation read an export that changed.
- A contributing match disappeared.
- A dirty invocation or a new match placed a declaration into one of
  the group's units.
- One of its files changed on disk since the commit that wrote it.
- A fact the settle read for it changed, or the placement of one of
  its files changed.
- A name entry one of its files read changed, or an entry entered or
  left a collision scope that one of its names settles in.
- The plan did not commit in the last run, and the group was dirty
  then.

A dirty group re-runs every invocation that contributed to it,
because an accumulated file is one output and rebuilds whole. A clean
group executes nothing: no dispatch, no render, no write. Its
manifest rows, export rows, name entries and findings remain, and its
bytes remain on disk, though drift is still checked. A change to
names across files, a new collision or a respelled referent, makes
the groups that read the changed names dirty, and the plan generates
again until no group joins.

The warm cost of an edit follows the output cardinality of each
family it dirties ([18-routing-and-layout.md](18-routing-and-layout.md)):

| Cardinality | A dirty artifact re-runs over | Warm cost |
|---|---|---|
| `PerSource` | the edited source's contributors | the source's own outputs, whatever the corpus size |
| `PerPackage` | the edited package's contributors | the package's outputs |
| `PerPlan` | every contributor in the plan's scope | the whole file, which grows with the corpus |

A `PerPlan` file costs its full size on every edit to one of its
contributors, in any design, because rendering, formatting and
hashing one file reads every byte of it. The warm-one-edit gate
([19-benchmarking.md](19-benchmarking.md)) applies to per-source and
per-package outputs, and a plan with a per-plan family adds that
file's full cost to each such edit.

O(dirty) is real without persisting emit, because "whose inputs
moved" is a table lookup rather than a rerun. Two earlier rules are
what make partial re-execution byte-stable: contributions order by
subject identity, and gate-free handlers make contributor sets
computable from tables rather than by running code.

An artifact whose identity changes, because layout inputs moved its
path, is not kept. The old row's producer is gone, so it is swept,
and the new path is a new artifact. The sweep and the table agree by
construction, because both read the same records.

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

**Structural reads have three grains**, and
[02-symbol-model.md](02-symbol-model.md) pins the rule per query.

- A targeted read, `Lookup`, records a declaration edge, which a
  change anywhere in the declaration's subtree dirties. Every
  invocation's subject is such an edge too, because the match hands
  the subject over without a read.
- An enumeration by kind or by directive records a membership edge
  under the reader's scope, so adding or removing a symbol in scope
  runs the enumerators again, while changing one runs only its
  targeted readers.
- A package taken whole, through `PackageOf` or a package's own
  `Lookup`, records a package edge, which any member's change
  dirties, because the reader walks the members without another
  tracked read.

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

1. **Open.** Read `CURRENT` and validate the generation's header,
   meaning the format version, the composition fingerprint and the
   executable digest. Any mismatch discards the generation and the
   run goes cold. Then run the stat-first sweep over the scope:
   changed, added and removed files become the dirty unit set, and
   owned outputs are excluded at the gate.
2. **Load.** Reparse the dirty units at their recorded `Depth`,
   consulting the parse memo first. A memo hit restores the region
   without parsing. Clean regions remain sealed until a read touches
   them.
3. **Diff by identity.** Unchanged declarations keep their canonical
   identities ([02-symbol-model.md](02-symbol-model.md)), and the
   changed remainder seeds the dirty set S.
4. **Link again.** A reference whose candidate appeared or
   disappeared, or whose followed re-export changed, selects its
   target again from the candidate tiers its region recorded. A
   re-export the link never followed, or an import of a new file,
   parses one unit of the package again. A resolution that changed
   extends S.
5. **Validate.** The subjects of changed declarations, and the
   subjects whose validation read a member of S, validate again. A
   changed validation extends S with its subject.
6. **Stamps and drops.** The claims of changed units and changed
   validations are withdrawn and applied again. Each fact whose
   winner changed joins S.
7. **Annotate**, bucket by bucket. Run exactly the matches whose read
   record meets S and the matches that appeared, and withdraw the
   claims of matches that disappeared. Every re-stamp compares
   against the persisted fact value. Equal stops there, which is
   early cutoff. Changed joins S as (symbol, key) dirt before the
   next bucket reads.
8. **Generate, per plan**, in export topological order, otherwise in
   parallel. The plan's dirty groups re-run their contributing
   invocations through a selection, settle against the clean groups'
   name entries, and route, render and stage their files. A plan's
   export changes only where a dirty file's export rows changed, and
   an unchanged export re-runs no dependent.
9. **Close.** Collisions, the audit and the checks read records, and
   each re-runs only for what changed. Every kept record reports the
   findings its execution reported, so the run's findings equal a
   cold run's.
10. **Commit.** The plan commits run, then the state commit, strictly
    last ([08-workspace-and-plans.md](08-workspace-and-plans.md)).

## The interning rule

The registered-name rule, extended to performance. Every registered
name the engine handles in volume, meaning canonical identities,
metadata keys and file paths, carries a **dense run-local ID**,
interned once. The string spelling exists only at the boundaries:
manifests, exports, explain and JSON.

Read-set records are packed pairs of interned IDs, deduplicated per
derived artifact, so comparing edges compares integers.

The sealed state stores no run-local ID. Each region keeps a string
table of its own, and every edge on disk is the first eight bytes of
the SHA-256 of its spelling, so a region decodes without the
generation that wrote it. Two spellings that share a hash make a run
execute more than an edit requires, and never less.

At L scale, memory comes down to the bags. A symbol's `meta.Bag` is
backed by a small slice over interned key IDs, and upgrades to a map
only past a threshold, because a two-fact bag must not carry a map's
overhead a few million times over.

## The sealed state, designed

The persisted state has to be loadable lazily and committable
incrementally, and this is the design that delivers both.

**On disk**, the state is in the brand's state directory
([20-cli.md](20-cli.md)):

```
.<brand>/state/
  CURRENT              names the live generation, replaced atomically
                       at commit
  gen/<sha256>         one generation: the header, the digest of each
                       manifest document, and each table's runs
  seg/<xx>/<sha256>    immutable, content-addressed segments of runs
                       and regions, shared between generations and
                       released when no live generation references one
```

**Tables are sorted runs.** Every table, from the file records to the
fact store, the invocation records, the reverse index of edges and
the artifact rows, maps a key to a row. A run stores rows sorted by
key in blocks of 4 KiB, each with its CRC-32C, and a sparse index of
each block's first key. A lookup reads one block. A commit adds one
run to each table it changed, and merges a table's runs once there
are more than eight or its newer runs contain more bytes than its
oldest, so a merge rewrites a written row at most once on average.

**Incremental commit is the write side's contract, mirroring lazy
regions on the read side.** Opening costs what the run touches, and
committing costs what the run changed: one segment for the regions of
the units it parsed or restored and one for the runs of the tables it
changed, plus the generation, never the corpus. A full-generation
rewrite would scale
the commit with the corpus and fail the warm-one-edit scaling gate
([19-benchmarking.md](19-benchmarking.md)) on the last step alone. A
run that changed nothing writes nothing at all.

The commit writes its segments and the generation, then the manifest
documents whose bytes changed
([17-output-and-determinism.md](17-output-and-determinism.md)), then
replaces `CURRENT`, then releases whatever no live generation
references, on a best-effort basis, and last writes the memo's new
entries. Everything before `CURRENT` is synced. A crash at any point
leaves `CURRENT` naming a complete generation, old or new but never
partial, which is the conservative
half of the two-phase commit
([08-workspace-and-plans.md](08-workspace-and-plans.md)). A crash
before `CURRENT` leaves outputs newer than the live generation's
records, so the next run finds their groups dirty and writes the same
bytes.

**Regions.** A region is one frontend unit, so the parse granule, the
graph-write lock granule and the invalidation granule are one grain,
with no translation layer between them. A region contains the unit's
declarations after assignment and link, the raw directives and stamps
attached to them, the link record and the findings of its parse and
link. The link record lists each reference's candidate tiers and the
re-exports the link followed, so a warm run links again from the
record. The `units` table maps each unit to its segment, offset and
summary, so opening a generation costs the header and the tables a
run reads. Regions decode on first touch: dirty regions
eagerly at Load, and clean regions only when a tracked read arrives in
one.

Lazy regions are the contract, meaning open cost proportional to
what the run touches. Memory-mapping is a permitted mechanism behind
that contract rather than a mandated one, because the kernel does
not buy a performance property with platform-specific machinery when
it can state the observable contract without it.

**Encoding** is length-prefixed and uses only the standard library,
per the kernel's zero-dependency rule. The node model's binary codec
is generated from the schema, as its JSON codec is, and the
generation header's format version covers it.

**The fact store** persists the claim record, not winning values
alone: per (symbol, key), the winning claim with its envelope, value
and derived reads, every drop tombstone, and the losing claims, in
rank order and keyed by edge hash. The envelopes are load-bearing for
warm≡cold, because a warm re-stamp arbitrates against the claims
already held: a store keeping only winning values would let a plugin
re-stamp beat the directive drop it lost to on the cold run. Losers
persist so `explain` walks one record warm or cold
([04-metadata.md](04-metadata.md)). The winning values keep the
small-slice layout the in-memory bags use, so a warm open restores
the bags without re-annotating, and they are exactly what early
cutoff diffs a re-stamp against. Facts persist because they have to:
bags that die with the run force full re-annotation on every warm
run, which is O(subjects) against the target.

**When the state is unusable, the run goes cold rather than wrong.**
A version or fingerprint mismatch, a different executable, an
executable the run cannot read, a checksum failure, a missing segment
or a truncated run discards the generation, and the run proceeds
cold, reporting one `ColdState` Info. A failure found mid-run
discards everything the run derived, which is in memory until the
commit, and starts again cold. Stale or damaged state must never fail
a run that source alone could complete, because the state is there to
make things
faster, and something that can veto a run is a dependency instead.

The contract, pinned. The byte format is versioned and private to the
kernel, and the ledger stores bytes:

```go
type Ledger interface {
    Read(ctx context.Context, name string) ([]byte, error)
    ReadAt(ctx context.Context, name string, p []byte, off int64) (int, error)
    Write(ctx context.Context, name string, b []byte) error // atomic and synced
    Put(ctx context.Context, name string, b []byte) error   // atomic, unsynced: the memo
    Touch(ctx context.Context, name string) error
    Remove(ctx context.Context, name string) error
    List(ctx context.Context, dir string) ([]Blob, error)
}

type Blob struct {
    Name    string
    Size    int64
    ModTime time.Time
}
```

Considered and refused. **An embedded database**: a third-party
dependency in a kernel that has none, and a B-tree's write paths buy
generality that writing immutable segments and swapping a pointer
never uses.
**One file per unit**: ten thousand or more opens per warm run at L,
which is a syscall floor that eats the sub-second budget by itself.
**Encodings that deserialize everything**, such as gob or JSON:
those reintroduce as a format exactly the pressure that ruled out a
daemon. **Committing the state**: the same argument that keeps the
manifest uncommitted, made worse by binary conflicts. **A global
intern table**: a memo region encoded against one generation's
identifiers would read only with that generation's table, and a table
that only grows needs a compaction that rewrites every reference.

## The parse memo

The sealed state returns the edit loop. The memo returns
**history**.

It is a second, optional persistence layer: a content-addressed
store of parsed regions keyed by the unit's key and the executable's
digest. An entry contains the same region encoding the generation
uses.

A generational snapshot alone is not enough. Switch branches and the
fingerprint diff marks thousands of units dirty against generation
N, but most of those parses already happened in *some* earlier run.
The memo is where they still live. A unit that is dirty against the
current generation but was seen in any past one is a memo hit, and
the region restores without parsing.

Five rules keep it honest.

**One client.** Only the Load path consults it, between "sealed
region" and "reparse". Plugins never see it, and the `u.Read` door
and the fingerprint discipline are untouched, because a memo hit
means `Parse` never runs at all.

**Correct by key.** The same inputs and the same executable produce
the same parse, so the key alone guarantees that a hit equals a fresh
parse. Keying on the executable makes a frontend change miss even
when nobody bumped its version. Warm≡cold licenses the memo the way it
licenses everything else.

**Eviction is part of the contract.** The memo is size-capped through
the `Cache` config group
([08-workspace-and-plans.md](08-workspace-and-plans.md)) and evicted
least-recently-used at the commit. A hit touches its entry. When the
memo's size total passes the cap, or when its last full listing is a
day old, the commit removes the entries with the oldest modification
times until the total is under 90% of the cap. A cache that only
grows is a disk leak, and this cap is the eviction policy that
version 1 deferred to a layer nobody ever built.

**Writes are boring.** Every fresh parse writes through. Entries are
immutable, written atomically without a sync, keyed by content, and
end in a CRC-32C, so an entry a machine crash tore reads as a miss.

**Sharing is safe.** The memo's entries are in a ledger the
composition chooses, by default under the brand's state directory.
Two worktrees that open one memo ledger restore each other's parses,
because an entry addressed by its inputs and its executable is the
parse either would make.

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
of the warm≡cold check.

## The non-negotiable

**Warm and cold produce identical bytes and identical findings.** A
conformance check runs the same workspace cold and warm and diffs the
files, the manifests and the findings
([13-testing-and-conformance.md](13-testing-and-conformance.md)).

Every optimization above is licensed by that check and by nothing
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
