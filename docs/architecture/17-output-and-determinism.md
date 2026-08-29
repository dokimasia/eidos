# Output and determinism

*Builds on: [08](08-workspace-and-plans.md) (plans, manifest),
[16](16-diagnostics.md). Feeds: [09](09-incrementality.md) (what
warm≡cold means), [18](18-routing-and-layout.md) (ownership).*

Generated output is committed source under review. Everything in
this document exists so that the same workspace over the same input
produces the same bytes on every machine, every time — and so that
what reaches disk is always a complete, consistent tree.

## What determinism covers

Byte-identity of every rendered file and of the manifest, across
machines, operating systems, runs, and warm/cold
([09-incrementality.md](09-incrementality.md)). The laws that make
it hold:

- **Every ordering is defined.** Plugins order by capability
  topology with an alphabetical tie-break; slot contents likewise;
  files within a manifest sort by path; imports sort by the target
  language's formatter. Nothing observable ever depends on map
  iteration, registration order, or scheduling.
- **No clocks, no randomness, no environment.** Output never
  contains timestamps, hostnames, usernames, absolute paths, or
  toolchain paths. The header carries attribution
  (brand, source), none of it invocation-shaped.
- **One text policy.** UTF-8, LF line endings, on every platform —
  a formatter that emits CRLF is wrapped, not obeyed. Manifest and
  diagnostic paths are slash-separated and workspace-relative.
- **Deterministic identifiers only.** Anything generated that needs
  uniqueness derives it from content or declared names, never from
  counters that depend on visit order.

The warm≡cold rung enforces the whole property; this document is
what it enforces.

## The sink contract

A sink receives `(target, bytes)` and owes four behaviors:

- **Write-if-changed.** Identical bytes leave the file untouched —
  mtime included. Consumers' build systems key on mtimes; a
  generator that rewrites unchanged files poisons every downstream
  cache.
- **Atomic per file.** Temp-and-rename in the target directory; a
  reader never observes a half-written file, and cancellation
  mid-run never leaves one behind.
- **Staged per plan.** A plan's writes stage together and commit
  with its manifest slice on plan success
  ([08-workspace-and-plans.md](08-workspace-and-plans.md)); a failed
  plan leaves the previous generation of its files intact.
- **Root-jailed.** A sink refuses any path that escapes its root —
  layout bugs surface as diagnostics, not as files outside the
  workspace.
- **Whole files only.** eidos writes complete files it manifests and
  nothing else — it never merges into, appends to, or rewrites
  regions of a file it does not wholly own. Partial ownership has no
  determinism story (the unowned half is uncontrolled input), no
  sweep story, and no drift story. A plugin that must contribute
  into a hand-written file does it through a generated sibling the
  hand-written file imports.

Disk, memory (for tests), and fan-out sinks ship with the kernel;
the contract is public for consumers with exotic destinations.
The contract, pinned — a sink is staged by construction, which is
what makes dry-run and plan isolation free:

```go
type Sink interface {
    Write(path string, body []byte) error // stage: workspace-relative,
                                          // root-jailed, not yet visible
    Commit() ([]Written, error)           // write-if-changed + atomic
                                          // rename, per file
    Discard() error                       // dry-run, failed plan
}
type Written struct {
    Path   string
    Action Action // Created | Updated | Unchanged
    Hash   string // sha256 of the bytes — the manifest's value
}
```

Commit's renames are atomic per file, not jointly: a crash
mid-commit leaves a mixed tree — new files owned by their
trailers, old files owned by the previous manifest — and the next
run heals it by construction: re-derive, write-if-changed,
manifest rewritten. No repair pass exists because none is needed.

## Drift

The trailer makes hand edits detectable instead of silently lost;
the manifest's recorded hash is the warm-path accelerator of the
same check. Before writing or sweeping an owned path, the engine
compares the on-disk body hash to the attested one:

- **Match** — the normal case; write-if-changed proceeds.
- **Missing** — recreate.
- **Differs** — the file **drifted**: someone edited generated
  output. The run refuses to overwrite it, reports an Error naming
  the file and the last run that produced it, and `prune` never
  sweeps it. `run --overwrite-drift` accepts the loss explicitly;
  the honest fix is moving the edit into source, config, or a
  directive, where it survives regeneration.

Dry run reports drifted paths as their own category (create /
update / unchanged / stale / drifted).

**Adoption.** A path with no manifest record but an existing file —
the first run ever, or a migration from another generator — is not
drift, and it should not be a wall. Output whose body hash matches
adopts the file silently into the manifest; a differing body is
the tree-collision Error
([18-routing-and-layout.md](18-routing-and-layout.md)), and
`--adopt` accepts replacement explicitly. Adopting a tool must not
fail on file one.

## Concurrency between runs

One writer per workspace: a run takes an exclusive lock on the
workspace's state directory for its duration. A second concurrent
invocation fails fast with a diagnostic naming the holder rather
than interleaving staged commits — two runs racing one manifest is
the orphan shape with extra steps. The state directory is the
brand's (`.<brand>/`, [20-cli.md](20-cli.md)), so two *different*
eidos-built binaries sharing one repository never contend on the
lock — their mutual safety is the ownership law above, not
serialization.

## Cancellation

Context cancellation aborts between units of work — never mid-file
(atomicity above), never mid-manifest. A canceled run reports what
committed and what didn't; because plans stage, the answer is always
"these whole plans, and nothing partial."

## The generated-file header

Every rendered file opens with the target language's comment form
of:

1. A machine-recognizable generated-code marker (the target
   ecosystem's conventional spelling, so linters, review tools, and
   coverage exclusions detect it without configuration).
2. Attribution: brand and the source it derives from
   (workspace-relative). The invocation command is deliberately
   absent: `run ./svc/...` and `run ./...` produce identical
   bytes, and a header that varies with phrasing forces
   write-if-changed to rewrite files whose content never moved.
   Which command produced a file is the manifest's and
   `explain`'s answer, not the file's.
3. Nothing else — no versions that churn diffs, no dates.

## The provenance trailer

Every rendered file *ends* with the target language's comment form
of `<brand>:provenance sha256:<hash>` — the hash of the body, the
bytes between header and trailer. Where the header says
"generated, don't edit", the trailer proves *whose* and *intact*:
it is the file's ownership record, carried in the file because
the manifest is not committed and a clone arrives without state.

- **The ownership law.** eidos overwrites or deletes only what it
  can prove it owns — a trailer with this brand, or a manifest
  entry. A hand-deleted trailer turns the file into a
  hand-written one: colliding with it is an Error
  ([18-routing-and-layout.md](18-routing-and-layout.md)), never
  an overwrite; the sweep honours the same proof.
- **Drift is decidable per file**: recompute the body hash,
  compare with the trailer. The trailer must be the file's final
  bytes — content after it is drift, which closes the
  append-past-the-marker tamper case.
- **First contact works without state.** Excluding own outputs
  from Load
  ([08-workspace-and-plans.md](08-workspace-and-plans.md)),
  adoption, drift refusal, and CI verification all function in a
  fresh clone, because the proof travels in the bytes. A CI gate
  is regenerate-and-compare-body-hashes; it reads no state
  directory.

The hash is content-derived; the trailer is as deterministic as
the body it attests.

## Dry run

`--dry-run` executes every phase — including Layout, Close checks,
and audit mode — and writes nothing: the manifest is computed and
reported, with a diff summary against the tree (create / update /
unchanged / stale / drifted). Dry run is the review surface for
"what would this change," and its output is the same versioned JSON
as everything else.

## The manifest

The workspace manifest is versioned public API: for every file — the
producing plan, the content hash, the emitting plugins, and the
source declarations it derives from (provenance pointers, so `prune`
and `explain` read the same record). It lives in the brand's
state directory (`.<brand>/manifest.json`, [20-cli.md](20-cli.md))
and is **not committed** to version control: committed, it is a
merge-conflict generator; uncommitted, nothing is lost, because
every per-file answer it accelerates — ownership, drift,
adoption — also rides the provenance trailer. The document,
concretely — versioned at the root and naming its workspace
([08-workspace-and-plans.md](08-workspace-and-plans.md)):

```json
{"version":1,"workspace":"platform",
 "files":[
  {"path":"svc/store_stub.go","plan":"go-services",
   "hash":"sha256:9f2c…","plugins":["stubgen","acme-audit"],
   "sources":["golang:svc/store.Store#Get(ctx,string)"]}
 ]}
```

The `sources` entries are canonical symbol identities
([02-symbol-model.md](02-symbol-model.md)). Format changes follow
the compatibility policy
([15-compatibility.md](15-compatibility.md)).

Plan exports publish into the same state directory, one document
per (plan, export) beside the manifest, under the export schema
([08-workspace-and-plans.md](08-workspace-and-plans.md)). They
persist because they are cross-run inputs: an export's hash folds
into its dependents' artifact fingerprints
([09-incrementality.md](09-incrementality.md)), so a warm run must
read the previous export without re-running its producer.
