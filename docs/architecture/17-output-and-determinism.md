# Output and determinism

*Builds on: [08](08-workspace-and-plans.md) (plans, manifest),
[16](16-diagnostics.md). Feeds: [09](09-incrementality.md) (what
warm≡cold means), [18](18-routing-and-layout.md) (ownership).*

Generated output is committed source that people review. Everything
in this document exists so that the same workspace over the same
input produces the same bytes on every machine, every time, and so
that what reaches disk is always a complete, consistent tree.

## What determinism covers

Every rendered file and the manifest are byte-identical across
machines, operating systems, runs, and warm against cold
([09-incrementality.md](09-incrementality.md)). Four rules make that
hold.

**Every ordering is defined.** Plugins order by capability topology
with an alphabetical tie-break, and slot contents order the same
way. Files within a manifest sort by path. Imports sort the way the
target language's formatter sorts them. Nothing anyone can observe
depends on map iteration, registration order or scheduling.

**No clocks, no randomness, no environment.** Output never contains
a timestamp, a hostname, a username, an absolute path or a toolchain
path. The header carries the brand and the source it derives from,
and nothing that varies with the invocation.

**One text policy.** UTF-8 and LF line endings, on every platform. A
formatter that emits CRLF gets wrapped rather than obeyed. Manifest
and diagnostic paths are slash-separated and workspace-relative.

**Deterministic identifiers only.** Anything generated that needs to
be unique derives that from content or from declared names, never
from a counter that depends on visit order.

The warm≡cold check enforces the whole property, and this document is
what it enforces.

## The sink contract

A sink receives a target and bytes, and owes five behaviours.

**Write if changed.** Identical bytes leave the file untouched,
including its mtime. Consumers' build systems key on mtimes, so a
generator that rewrites unchanged files breaks every downstream
cache.

**Atomic per file.** Write to a temp file and rename within the
target directory. A reader never sees a half-written file, and
cancelling mid-run never leaves one behind.

**Staged per plan.** A plan's writes stage together and commit with
its manifest slice when that plan succeeds
([08-workspace-and-plans.md](08-workspace-and-plans.md)). A plan
that fails leaves the previous generation of its files in place.

**Root-jailed.** A sink refuses any path that escapes its root, so a
layout bug produces a diagnostic instead of files outside the
workspace.

**Whole files only.** eidos writes complete files that it manifests
and nothing else. It never merges into, appends to, or rewrites part
of a file it does not wholly own. Owning half a file gives you no
determinism story, since the other half is uncontrolled input, no
sweep story and no drift story. A plugin that must contribute into a
hand-written file does it through a generated sibling that the
hand-written file imports.

Disk, memory and fan-out sinks ship with the kernel, and the
contract is public for consumers with somewhere exotic to write. A
sink stages by construction, which is what makes dry-run and plan
isolation free:

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
    Hash   string // sha256 of the bytes, which is the manifest's value
}
```

Commit renames each file atomically, but not all of them jointly. A
crash mid-commit leaves a mixed tree, where new files are owned by
their trailers and old files by the previous manifest, and the next
run heals it by construction: derive again, write if changed, and
rewrite the manifest. No repair pass exists because none is needed.

## Drift

The trailer makes a hand edit detectable instead of quietly lost,
and the manifest's recorded hash is the warm-path shortcut to the
same check. Before writing or deleting a path it owns, the engine
compares the body hash on disk with the attested one.

**Match** is the normal case, and write-if-changed proceeds.
**Missing** means recreate the file. **Different** means the file
drifted: somebody edited generated output. The run refuses to
overwrite it, reports an Error naming the file and the run that last
produced it, and `prune` never deletes it. `run --overwrite-drift`
accepts the loss explicitly. The honest fix is moving the edit into
source, config or a directive, where regeneration will keep it.

Dry run reports drifted paths as their own category, alongside
create, update, unchanged and stale.

**Adoption.** A path with no manifest record but an existing file is
not drift. That happens on the first run ever, or when migrating
from another generator, and it should not be a wall. If the output's
body hash matches the file, eidos adopts it into the manifest
silently. If the body differs, that is the tree-collision Error
([18-routing-and-layout.md](18-routing-and-layout.md)), and
`--adopt` accepts replacement explicitly. Adopting a tool must not
fail on the first file.

## Concurrency between runs

One writer per workspace. A run takes an exclusive lock on the
workspace's state directory for its duration. A second concurrent
invocation fails immediately with a diagnostic naming the holder,
rather than interleaving staged commits, because two runs racing one
manifest leaves generated files nothing tracks.

The state directory belongs to the brand, `.<brand>/`
([20-cli.md](20-cli.md)), so two different eidos-built binaries
sharing one repository never contend on the lock. What keeps them
safe from each other is the ownership rule below, not serialization.

## Cancellation

Cancelling the context stops the run between units of work, never
mid-file, since writes are atomic, and never mid-manifest. A
cancelled run reports what committed and what did not, and because
plans stage, the answer is always whole plans and nothing partial.

## The generated-file header

Every rendered file opens with the target language's comment form
of three things.

First, a marker that machines recognize as generated code, spelled
the way the target ecosystem spells it, so linters, review tools and
coverage exclusions detect it without configuration.

Second, attribution: the brand, and the source the file derives
from, workspace-relative. The invocation command is deliberately
absent, because `run ./svc/...` and `run ./...` have to produce
identical bytes, and a header that varies with the phrasing forces
write-if-changed to rewrite files whose content never moved. Which
command produced a file is what the manifest and `explain` answer.

Third, nothing else. No version that churns the diff, no date.

## The provenance trailer

Every rendered file ends with the target language's comment form of
`<brand>:provenance sha256:<hash>`, hashing the body, meaning the
bytes between the header and the trailer.

The header says "generated, do not edit". The trailer proves whose
it is and that it is intact. It is the file's ownership record,
carried in the file itself, because the manifest is not committed
and a fresh clone arrives with no state.

**The ownership rule.** eidos overwrites or deletes only what it can
prove it owns, through a trailer carrying this brand or a manifest
entry. Delete the trailer by hand and the file becomes a
hand-written one: colliding with it is an Error
([18-routing-and-layout.md](18-routing-and-layout.md)) rather than
an overwrite, and the sweep honours the same proof.

**Drift is decidable per file.** Recompute the body hash and compare
it with the trailer. The trailer must be the file's final bytes, so
anything after it counts as drift, which closes the
append-past-the-marker case.

**First contact works with no state.** Excluding a workspace's own
outputs from Load
([08-workspace-and-plans.md](08-workspace-and-plans.md)), adoption,
refusing to overwrite drift, and CI verification all work in a fresh
clone, because the proof travels in the bytes. A CI gate regenerates
and compares body hashes, and reads no state directory at all.

The hash comes from the content, so the trailer is as deterministic
as the body it attests.

## Dry run

`--dry-run` executes every phase, including Layout, the Close checks
and audit mode, and writes nothing. It computes the manifest and
reports it, with a diff summary against the tree covering create,
update, unchanged, stale and drifted. Dry run is the surface for
reviewing what a run would change, and its output is the same
versioned JSON as everything else.

## The manifest

The workspace manifest is versioned public API. For every file it
records the producing plan, the content hash, the emitting plugins,
and the source declarations the file derives from, so `prune` and
`explain` read the same record.

It lives in the brand's state directory, at
`.<brand>/manifest.json` ([20-cli.md](20-cli.md)), and it is **not
committed**. Committed, it generates merge conflicts. Uncommitted,
nothing is lost, because every per-file answer it speeds up, meaning
ownership, drift and adoption, also rides in the provenance trailer.

The document is versioned at the root and names its workspace
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

Plan exports publish into the same state directory, one document per
plan and export, beside the manifest, under the export schema
([08-workspace-and-plans.md](08-workspace-and-plans.md)). They
persist because they are inputs to later runs: an export's hash
folds into its dependents' artifact fingerprints
([09-incrementality.md](09-incrementality.md)), so a warm run has to
read the previous export without re-running its producer.
