---
rfc: 0020
title: Warm runs, the sealed state and re-execution by artifact
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-02
updated: 2026-10-08
discussion: none
supersedes: none
superseded-by: none
produces-adr: ADR-0011, ADR-0012, ADR-0013
---

# RFC-0020: Warm runs, the sealed state and re-execution by artifact

## Summary

Every run today is a cold run. It walks and reads the whole tree, parses
every unit, and validates, annotates, generates and renders over the whole
graph. This proposal lets a run over a workspace it ran before execute only
what the edits since that run change. The run still leaves the bytes, the
manifest, the exports and the findings that a cold run over the same tree
leaves.

A run records what it read and what it produced in a sealed state under the
brand's state directory. The state is a generation of immutable segments,
and a commit publishes a generation by replacing one pointer. The next run
stats every file against the recorded file table and hashes only the files
whose stat moved. It parses again only the units whose inputs changed, and
compares their declarations with the recorded ones by identity. The changed
declarations and the facts whose value changed form the dirty set.

Every validation, annotator invocation, generator invocation and workspace
check records the edges it read. A warm run executes again the records whose
edges meet the dirty set and the matches that appeared. It withdraws what
the matches that disappeared produced. A generated file is the unit of
re-execution. A dirty file runs every invocation that contributed to it. A
clean file executes nothing and keeps its manifest row, its export rows and
its findings. The manifest is 256 documents keyed by a hash of each path,
so a commit rewrites only the documents whose entries changed. A parse memo
keyed by a unit's inputs restores a unit that some earlier run parsed, and
two workspaces can share it. A state that does not validate sends the run
cold with one Info and never fails it.

## Motivation

### Every run is cold

The measurements below come from one machine: an AMD Ryzen 9 9950X3D, Go
1.27.1, and an XFS file system with a warm page cache.

- The end-to-end benchmark runs 1,000 packages of 200 declarations from a
  built graph through annotate, generate, settle, layout, render and a
  memory sink. A run takes 143 ms on average on one worker and 135 ms on
  four, and allocates 483,000 times. It parses nothing.
- Walking a tree of 100,001 files through `fs.FS` and statting each file
  costs 1.34 to 1.94 µs per file.
- Reading and hashing each of those files costs 37 µs per file, 3.7 s for
  the tree.
- The load reads the tail of every claimed file on every run to find the
  brand's provenance trailer, which is one open and one read per file.

The performance target is a warm run under one second at 10,000 packages.
Extrapolated from the first measurement, a cold run at that size spends
about 1.4 s in the pipeline before it parses a file.

### Read records are discarded

- The dispatcher gives each invocation a read set of its own and resets it
  for the next invocation. A stamp keeps the point reads made before it as
  its derivation, and the run keeps no other part of the set.
- A phase call of a plugin that implements a role directly reads through
  one reader, and the run discards that reader's set.
- The ledger records the manifest and nothing else.
- A claim's rank ends in the invocation's sequence number, which counts the
  matches of one phase call in order. A run that executes part of a phase
  numbers its matches differently, so a recorded claim cannot be ranked
  against a new one.
- A unit's key folds the bytes the parse read, so a run cannot compute a
  key without reading the unit's files.

### Why the kernel

The load, the dispatcher, the fact store, the settle, the layout and the run
are kernel code. A plugin cannot skip its own invocations, because the run
selects the matches a phase call runs. A frontend cannot skip its own parse
either, because the load selects the units that parse. So every mechanism in
this proposal changes the kernel, and a plugin written against the authoring
surface becomes incremental without a change of its own.

## Detailed design

### Components

| Component | Package | Responsibility |
|---|---|---|
| `Ledger` | `core/ledger` | Stores the record as named blobs: generations, segments, manifest documents and memo entries |
| The sealed state | `core/internal/state` | Encodes generations, opens their tables lazily, commits only what changed, releases unreferenced segments, keeps the memo |
| The manifest's documents | `core/manifest` | The public record as 256 documents, each containing the files whose path hashes to it |
| The fingerprint gate | `core/frontend/load` | Stats the tree against the recorded file table and finds the files and units that changed |
| `Prior`, `Memo` | `core/frontend/load` | The previous load's record and the parse memo, as the load reads them |
| The region codec | `core/node`, generated | A binary encoding of one unit's declarations with a string table of its own |
| `Source`, `Sealed` | `core/store` | A frozen graph over regions that decode on first use |
| The package edge | `core/store` | A fourth edge kind, for a package read whole |
| `Order`, `Withdraw`, `Restore` | `core/meta` | A claim rank that is stable across runs, the withdrawal of a claim, and bags restored on first use |
| `MatchKey`, `Selection`, `Journal` | `core/plugin` | The invocation interface: which matches a phase call runs, and what each one read and touched |
| `Names`, `SettleWith` | `core/plugin` | The settle of part of a plan against the names its other files declare |
| Selective dispatch | `core` | The authoring surface honours a selection and journals its invocations |
| The warm run | `core/workspace` | Computes the dirty set, executes again what it meets, keeps the rest, and commits |
| `Memo` | `core/workspace` | The memo's cap, and the ledger that stores its entries |
| `Stats` | `core/workspace` | Counts what a run executed, for probes and for reports |
| `ColdState` | `core/workspace` | The Info a run reports when it ignores an unusable state |
| `RunWarmColdSuite` and the warm checks | `core/workspace/workspacetest` | Warm runs against cold runs over a fixture |

### The invariant

A warm run and a cold run over one tree leave the same:

- bytes in every file the plans commit, and the same modification times on
  the files the commit leaves unchanged
- bytes in every manifest document
- export of each plan, as a dependent plan reads it
- findings, compared in the order output sorts them: position, code,
  message, then origin
- status for each plan

The runs may differ in `Report.Stats`, in `Report.Emits` and in the bytes of
the state directory outside the manifest. Every mechanism below exists
under this invariant, and `RunWarmColdSuite` checks it.

### The ledger

The ledger stops reading and writing the manifest itself, and stores bytes.
The kernel encodes every blob it stores.

```go
// Package ledger stores a workspace's record in the brand's state
// directory.
package ledger

// Ledger stores one workspace's record as named blobs: the generations of
// the sealed state and their segments, the manifest's documents, and the
// parse memo's entries. The kernel encodes every blob, and a Ledger stores
// bytes.
//
// A name is slash-separated and relative, and has no empty, "." or ".."
// element. A Ledger serves one run and is safe for concurrent use by that
// run's goroutines.
type Ledger interface {
    // Read returns a blob whole. A name nothing wrote returns an error
    // wrapping fs.ErrNotExist.
    Read(ctx context.Context, name string) ([]byte, error)
    // ReadAt reads len(p) bytes of a blob from offset off, under the
    // contract of io.ReaderAt.
    ReadAt(ctx context.Context, name string, p []byte, off int64) (int, error)
    // Write replaces a blob atomically and durably. A reader sees the old
    // bytes or the new ones, and once Write returns, a crash of the
    // machine does not lose the new ones.
    Write(ctx context.Context, name string, b []byte) error
    // Put replaces a blob atomically without a sync. A crash of the
    // process loses nothing, and a crash of the machine can leave the old
    // bytes, the new ones or an empty blob. The memo writes its entries,
    // which check their own bytes, through Put.
    Put(ctx context.Context, name string, b []byte) error
    // Touch sets a blob's modification time to the current time.
    Touch(ctx context.Context, name string) error
    // Remove deletes a blob. A name nothing wrote is not an error.
    Remove(ctx context.Context, name string) error
    // List returns the blobs whose names begin with dir and a slash,
    // sorted by name.
    List(ctx context.Context, dir string) ([]Blob, error)
}

// Blob is one blob that List returns.
type Blob struct {
    Name    string
    Size    int64
    ModTime time.Time
}

// OpenAt returns the ledger of the directory dir itself, which it creates
// on the first write: the location of a memo that more than one
// workspace shares.
func OpenAt(dir string) (*Dir, error)
```

`Dir` maps a name to a file through an `os.Root`. Its `Write` replaces the
file through `stagefile.Replace` and syncs the directory, as the manifest's
commit does today, and its `Put` renames without the syncs. `Mem` maps a
name to bytes and a logical clock. `Mem.Writes` counts the calls to `Write`
and `Put`, so a test tells a run that wrote nothing from a run that wrote a
generation. `BeginRun` and `CommitRun` leave the interface. `StateDir`,
`ManifestPath`, `OpenDir` and `NewMem` keep their signatures, and
`ManifestPath` returns the manifest's directory, `.<brand>/manifest`.

### The state directory

```
.<brand>/
  manifest/<xx>.json       the public record: the files whose path's SHA-256 begins with byte xx
  state/CURRENT            the name of the live generation
  state/gen/<sha256>       one generation: its header and its tables' runs
  state/seg/<xx>/<sha256>  immutable segments, named by the SHA-256 of their bytes
  memo/total               the memo's size in bytes, where the memo is in this ledger
  memo/trimmed             when the memo was last listed whole
  memo/<xx>/<key>          the parse memo, one region per entry
```

A generation is one blob, named by the SHA-256 of its bytes. It contains a
header, the SHA-256 of each manifest document, and, for each table, the
table's runs. A run is a sorted list of rows in one segment. A commit writes
at most two segments: one for the regions of the units the run parsed or
restored, and one for the runs of the tables the run changed. A segment is
immutable after its commit, so a generation shares every segment its
predecessor wrote that the new generation still references.

The header:

| Field | Meaning |
|---|---|
| Format version | The kernel's encoding of every table, run and region |
| Composition fingerprint | Everything the composition reads from outside the executable |
| Executable digest | The SHA-256 of the running executable, with the executable's path, size and modification time |
| Anchor | The instant the recording run's sweep began, less two seconds |
| Sequence | One more than the parent generation's sequence |
| Parent | The name of the generation the run opened, empty for a cold run |

A generation whose format version, composition fingerprint or executable
digest differs from the run's own is unusable, and the run goes cold. The
executable digest is how the state follows the code. A plugin whose code
changes without a new version changes the executable, so its stale records
never meet the new code. The run stats its executable and hashes it only
where the path, the size or the modification time differs from the header's
record. That costs one hash, about 12 ms for a 30 MB executable at the
measured 2.63 GB/s, on the first run after each build.

A rebuild that changes no code keeps the state. Two `go build` runs over
unchanged source produced byte-identical executables under Go 1.27.1, with
different modification times. `go run` executed its first build from a
temporary directory and its second from the build cache, with the same
digest both times, so a workspace driven through `go run` hashes its
executable once per change of code and then never again. A run that cannot
read its executable cannot tell which code wrote the state. It runs cold,
writes no generation, and reports `ColdState`.

The composition fingerprint folds everything the composition reads that the
executable does not contain:

- what it folds today: each annotator, generator, backend and check, each
  with its name, version and options in the canonical encoding, and each
  plan's name, sources and dependencies
- the brand and the workspace name
- each frontend with its name, version and options
- each plan's layout configuration
- the ignored directive spellings
- every registered key, with its kinds, its group and its contract
- each plugin's template trees, file by file, for every target it declares
  one for

A template tree read from disk at run time is the case the last item
covers. A tree embedded in the executable is in the executable digest too.
The registered keys fold because one executable can build compositions
whose keys differ, and a generation's facts and audit findings are valid
only under the keys that recorded them.

### Tables and runs

Each table maps a key to a row. A run stores rows sorted by key in blocks of
4 KiB, each block followed by its CRC-32C, and ends with a sparse index of
each block's first key. A lookup searches the sparse index and reads one
block through `ReadAt`. A deleted row is a tombstone in a newer run.

| Table | Key | Row |
|---|---|---|
| `files` | path | the stat record, the digest, the verdict of the gate, and the package the file's unit declared it in |
| `units` | unit number | the frontend, the members, the depth, the key, the round, the region's segment and offset, and the summary |
| `doors` | frontend, then the partition or a round | what the partition or the round read, its needs, and the units it returned |
| `probes` | bare identity | the units whose references named it as a candidate, and the units that followed a re-export under it |
| `modules` | language, root, module path | the number of packages whose module facts name the module |
| `validations` | subject | the validated directives, the read record and the findings |
| `claims` | subject | every claim on the subject's bag, with its envelope and value |
| `present` | key, subject | one row for each fact that reads present |
| `invocations` | phase call, match | the read record, what the invocation touched, and its findings |
| `readers` | edge | the validations, invocations, groups and checks whose read record contains the edge |
| `plans` | plan | that the plan did not commit, so its work is pending |
| `groups` | plan, group | the group's files, units and contributing invocations, its read record, and the findings its render reported |
| `artifacts` | path, and the path folded under the collision rule | the file's manifest entry, package and group, where a finding about it is positioned, its export rows and its names |
| `names` | plan, then a package and an emitted name, or an origin and an emitted name | the name entries of the plan's files, under both keys |
| `audit` | key, subject | an unmet-contract finding |
| `checks` | check | the read record and the findings |

Identities in keys and read records are 64-bit edge hashes: the first eight
bytes of the SHA-256 of the edge's canonical spelling. A row keyed by a hash
stores the spelled key beside it, and a lookup compares the spelling. Two
edges that share a hash make the run execute more than the edit requires.
They never make it execute less.

A commit adds one run to each table it changed, so its bytes follow the
rows the run changed and not the size of the table. Shake loads its whole
database at startup and saves it whole on completion. Its authors report
that the save took up to 75% of the time of a quick build. A commit merges a
table's runs into one when the table has more than eight runs. It also
merges them when the table's newer runs contain more bytes than its oldest
run. A merge drops tombstones. Under a stable table size a merge rewrites a
written row at most once on average, so merging adds a constant factor to
the bytes a commit writes.

### The manifest's documents

The manifest is 256 documents. A file belongs to the document named by the
first byte of the SHA-256 of its path, so each document contains about one
256th of the record, and a commit rewrites only the documents whose entries
changed.

```go
// Version is the format this package writes and reads: the record split
// into 256 documents by the first byte of each path's SHA-256.
const Version = 2

// Shard is one of the manifest's documents: the entries whose path's
// SHA-256 begins with the shard's byte, sorted by path.
type Shard struct {
    Version   int    `json:"version"`
    Workspace string `json:"workspace"`
    // Bucket is the shard's byte, as two lowercase hex digits.
    Bucket string  `json:"bucket"`
    Files  []Entry `json:"files"`
}

// BucketOf returns the bucket a path's entry belongs to.
func BucketOf(path string) string

// Split returns a manifest's documents, one for each bucket that contains
// a file, sorted by bucket.
func Split(m Manifest) []Shard

// Join returns the manifest the documents of one workspace record. It
// refuses two documents of one bucket, and an entry outside its bucket.
func Join(shards []Shard) (Manifest, error)

// EncodeShard returns a document's bytes: JSON indented by two spaces,
// with a final newline, under the invariants Encode checks today.
func EncodeShard(s Shard) ([]byte, error)

// DecodeShard reads what EncodeShard writes.
func DecodeShard(b []byte) (Shard, error)
```

A reader lists `.<brand>/manifest/`, decodes every document and joins
them. `Encode` and `Decode` of one whole document leave the package. The
`Manifest` and `Entry` types remain, so `Report.Manifest` and
`plugin.PlanRecord` keep their shapes.

### The commit

The state commit is the run's last step, after every plan commit, as the
record's commit is today. A run that changed nothing writes no blob.

Before any plan commits, the run prepares the rows that its commit
writes, so damage that it meets in the generation discards the run with
nothing written:

- A warm run in which every plan that recorded its phases executed in part
  reads each prior row that its commit can change, through a lookup of the
  row's key.
- A run in which a plan ran whole reads every row of the phase tables. The
  plan's new records replace all its records, and a lookup by key cannot
  find a record that the plan no longer makes.
- A plan that recorded nothing in the run, such as a plan whose upstream
  failed, keeps its records.

The commit then writes in this order:

1. Write the region segment and the run segment.
2. Write the generation.
3. Write each manifest document whose bytes differ from the digest the
   previous generation recorded, and remove each document whose bucket is
   now empty. The writes run in parallel, each through `Write`.
4. Write `state/CURRENT` with the generation's name. A reader sees the old
   name or the new one, as LevelDB's `CURRENT` file works.
5. Remove every generation and segment that the new generation does not
   reference and that is older than the run's anchor. The next commit
   retries a removal that failed.
6. Write the memo's new entries through `Put`, and trim the memo where it
   is over its cap.

| The run stops | The state | The next run |
|---|---|---|
| Before step 4 | The previous generation, segments nothing references, and manifest documents that may be newer than the generation | Compares the tree with the previous generation. A file a plan committed differs from its record, so its group executes again, writes the same bytes, and rewrites the same documents |
| During step 5 | The new generation, and segments nothing references | Removes them at its own commit |
| During step 6 | The new generation, without some memo entries | Parses those units again where it needs them |

Two runs over one workspace at once can each commit, and the generation
named last in `CURRENT` is the live one. A generation's name is its digest,
so two runs never write one blob with different bytes. Step 5 can remove a
segment that a concurrent run is about to reference. That run's generation
then fails validation in the next run, which goes cold. The lock that keeps
two runs apart is not part of this proposal.

### Plans that do not commit

A plan that does not commit keeps its previous records in the generation:
its groups, its artifacts, its invocations and its names. The shared records
advance whether the plan commits: the regions, the validations and the
claims. So the generation also lists the plan in the `plans` table. The
next warm run runs a listed plan whole, as a cold run does. A plan that
failed executes its failed work again on the next run, warm or cold, and
reports the same findings.

### Cold runs and their causes

A run runs cold when `Input.Cold` is set and when the ledger has no
`CURRENT`, and reports nothing for either. In the cases below it ignores the
generation, runs cold, and reports one `ColdState` Info that states the
cause.

| Cause | Detected at |
|---|---|
| `CURRENT` names no generation | The start of the run |
| The run cannot read its executable, so it also writes no generation | The start of the run |
| The format version, the composition fingerprint or the executable digest differs | The header |
| A block's CRC-32C does not match, a segment is missing, or a run is shorter than its record | The first read of the block or the segment |
| A region fails to decode | The first read that needs the region, which `Graph.Damaged` reports after the phase |
| A block that the commit's merge of a table's runs reads does not read whole | The commit, after the plans committed |

The run can find a damaged block or region after it has executed work
against the generation. Everything the run derived is in memory until the
commit. The run discards it, reports `ColdState`, and starts again cold over
the same input. Damage that a plan meets discards the run in the same way,
before any plan commits. A damaged block or region that no read of the run
needs is kept, because the run reads the generation lazily.

The commit merges a table's runs after the plans have committed. Damage
that the merge meets starts the run again cold too. The cold run commits the
same files, which the plans already wrote, and records a generation without
a parent, so no merge reads the damaged run again. Its report lists no
change for the files that the discarded run wrote. Damaged state costs time
and never correctness, and never fails a run that the source alone
completes.

A run over `Input.Graph` neither reads nor writes a generation, because the
fingerprint gate has no tree to compare. A dry run reads the generation and
writes nothing.

### The fingerprint gate

The gate finds the files that changed, at one stat for each unchanged file.

1. The run sets its anchor to the current time less two seconds. The margin
   covers FAT's two-second modification times and the tick of the clock
   that stamps them.
2. It walks the tree, skipping the brand's state directory, and stats every
   file. It stats every store file that a unit of the generation read.
3. A file whose recorded size and modification time match, whose change time
   and inode match where `FileInfo.Sys` reports them, and whose
   modification time precedes the generation's anchor is unchanged without
   a read.
4. A file that matches but whose modification time does not precede the
   generation's anchor is racily clean. The gate hashes it.
5. A file whose stat moved is hashed. An equal digest leaves it unchanged,
   and the gate records its new stat.
6. A path with no record is new, and a record with no file is removed.

Steps 3 and 5 are the check that Shake's `ChangeModtimeAndDigest` mode
makes: the modification time first, and the digest only where it moved. The
go command's test cache also compares a file's size and modification time
in place of its content, and caches no result that read a file younger than
two seconds.

At its commit the run writes a size of zero into every file record whose
modification time does not precede its own anchor. The next gate that meets
the file hashes it, even after a generation between copied the record
unchecked. These are git's two rules for racily clean index entries.

The commit also records each file that the run's plans created inside the
tree, with a size of zero and the digest of the bytes it wrote. The load's
record lists only the files that its walk met. Without this record, the
next gate could not find a created file removed before any walk met it, and
the file's group would remain clean. The record costs one stat for each
created file.

The gate also records each file's verdict: a frontend's claimed input, the
brand's own output, or unclaimed. The load reads a file's tail for the
provenance trailer only where the file is new or changed. The rule that
outputs are never inputs then costs one stat for each unchanged file.

A unit is unchanged when every member and every shared input is unchanged
and its partition or dependency round returned the same unit. The load runs
a frontend's partition again where the frontend's claimed paths changed. It
also runs it again where a file the last partition read changed. It runs a
frontend's dependency rounds again where a changed unit's imports changed
the needs of the first round, or where a file a round read changed.

A unit's key folds each member's and each shared input's digest in roster
order, in place of the bytes the parse read. The load computes the key from
the gate's digests before the parse. A frontend still reads through the
unit's one door, which refuses every other path, so the key covers every
input the parse can read.

The key no longer folds the composition fingerprint. A region depends on
the frontend and on the unit's inputs alone, and the generation's header
already checks the composition.

The load takes the last commit's record and the memo through two
interfaces:

```go
// Prior is the last committed load's record: what the fingerprint gate
// compares the tree against, and what a warm load keeps.
type Prior interface {
    // Anchor returns the anchor the recording run's sweep took.
    Anchor() time.Time
    // Files returns every file record, sorted by path.
    Files() iter.Seq2[FileRecord, error]
    // Units returns every unit record, in splice order.
    Units() iter.Seq2[UnitRecord, error]
    // Region decodes a recorded unit's region.
    Region(u UnitRecord) (*store.Region, error)
    // Doors returns a frontend's recorded partition, then its dependency
    // rounds in order, and nothing for a frontend the record lacks.
    Doors(frontend plugin.ID) ([]DoorRecord, error)
    // Probed returns the recorded units whose references named a bare
    // identity as a candidate.
    Probed(bare symbol.Identity) ([]int, error)
    // Followed returns the recorded units whose references followed a
    // re-export of a file of the package.
    Followed(pkg symbol.Identity) ([]int, error)
}

// FileRecord is what the gate recorded about one file.
type FileRecord struct {
    Path    string
    Size    int64
    ModTime time.Time
    // Change and Inode are the file's change time and inode where
    // FileInfo.Sys reports them, and zero elsewhere.
    Change  time.Time
    Inode   uint64
    Digest  [sha256.Size]byte
    Verdict Verdict
    // Pkg is the package the file's unit declared it in, and zero for a
    // file no unit loaded.
    Pkg symbol.Identity
}

// Verdict is what the gate found a file to be.
type Verdict uint8

const (
    // VerdictInput is a file a frontend's selection claims.
    VerdictInput Verdict = iota + 1
    // VerdictOutput is the brand's own output, proven by its trailer.
    VerdictOutput
    // VerdictUnclaimed is a file no frontend claims.
    VerdictUnclaimed
)

// UnitRecord is one recorded unit.
type UnitRecord struct {
    // Number is the unit's place in splice order.
    Number   int
    Frontend plugin.ID
    Files    []plugin.SourceRef
    Depth    plugin.Depth
    Round    int
    Key      []byte
}

// DoorRecord is what a partition or a dependency round read and returned:
// each path it read or listed with the digest of the bytes or the listing,
// a round's needs, and the units it returned.
type DoorRecord struct {
    Reads []Digested
    Needs []plugin.Need
    Units [][]plugin.SourceRef
}

// Digested is one path a door read, with the digest of what it read.
type Digested struct {
    Path   string
    Digest [sha256.Size]byte
}

// Memo is the parse memo as the load reads it.
type Memo interface {
    // Get returns the region a unit with this key parsed into, and false
    // on a miss.
    Get(key []byte) (*store.Region, bool)
    // Put records a parsed unit's region under its key. The memo writes it
    // at the run's commit.
    Put(key []byte, r *store.Region)
}
```

`load.Config` gains `Prior Prior` and `Memo Memo`, nil for a cold load and
for no memo. `load.Report` gains `Files`, the records the gate took, and
`Doors`, the records of each frontend's partition and rounds. Each
`UnitReport` gains `From`, which states whether the unit parsed, came from
the memo or came from the generation. The load returns a graph from
`store.Sealed` over the regions it took from the generation and the fresh
ones.

A warm load also reports what changed against the record:

- `Moved` lists the files whose record does not prove them unchanged,
  `Vanished` the recorded files that the walk did not find, and `Was` the
  records of both.
- `Changes` lists the declarations that appeared, disappeared or changed,
  the packages that changed, the subjects whose directives or stamps
  changed, and the packages whose declared name changed.
- `Changes` lists each directive spelling that a subject gained or lost,
  with the packages of those subjects, so a reader's scope can tell whether
  the change concerns it.

### Regions and the lazy graph

A region is one unit's share of the graph:

- the unit's packages, files and declarations, after identity assignment
  and link
- the raw directives and classification stamps attached to them
- the link record: each reference's candidate tiers as the frontend's
  `Resolve` returned them, and each re-export query the link followed
- the findings the unit's parse and its link reported

The model generator emits a binary codec for the node model into `core/node`.
Each region has a string table of its own, so a region decodes without any
other blob, in a generation or in the memo.

```go
// AppendBinary appends the binary encoding of s to dst, adding every
// string s spells to t.
func AppendBinary(dst []byte, s symbol.Symbol, t *StringTable) ([]byte, error)

// DecodeBinary decodes one symbol from b against t, and returns it with
// the number of bytes it read.
func DecodeBinary(b []byte, t *StringTable) (symbol.Symbol, int, error)
```

The store gains a graph over regions:

```go
// Source is a sealed graph's regions: one per frontend unit, in splice
// order. A warm run builds one over its last generation's regions and the
// units it parsed or restored.
type Source interface {
    // Regions returns every region's summary, in splice order.
    Regions() []RegionInfo
    // Region decodes the i-th region.
    Region(i int) (*Region, error)
}

// RegionInfo is what a run knows of a region without decoding it.
type RegionInfo struct {
    // Packages are the packages the region contributes files to,
    // sorted.
    Packages []symbol.Identity
    // Files are the region's files, each with its package, sorted by
    // path.
    Files []RegionFile
    // Kinds are the declaration kinds the region declares, sorted.
    Kinds []symbol.Kind
    // Directives are the spellings of the raw directives attached in
    // the region, sorted.
    Directives []directive.Name
}

// RegionFile is one file of a region and the package it declares.
type RegionFile struct {
    Path string
    Pkg  symbol.Identity
}

// Region is one unit's share of the graph.
type Region struct {
    Packages   []*node.Package
    Directives map[symbol.Identity][]directive.Raw
    Stamps     map[symbol.Identity][]meta.RawStamp
}

// Sealed returns a frozen graph over the regions of src. Lookup and
// PackageOf decode the regions of the package an identity names, and
// ByKind and ByDirective decode the regions whose summary lists the kind
// or the spelling, each on first use. The enumeration order is the order
// of a graph that loaded the same regions. A region that fails to decode
// reads as absent, and Damaged then reports the failure.
func Sealed(src Source) *Graph

// Damaged returns the first decoding failure of a sealed graph's regions,
// and nil where every region it decoded read whole.
func (g *Graph) Damaged() error
```

A warm run compares a changed unit's declarations with its previous region
by identity:

- An identity only the new region declares appeared.
- An identity only the old region declares disappeared.
- An identity both declare changed where its encodings differ. The encoding
  covers the declaration's subtree, its position, its link targets and the
  directives and stamps attached to it.
- A package changed when any declaration in it appeared, disappeared or
  changed, or when its own fields changed.

An identity contains its package's path, so two declarations of one
identity can meet only inside one package. Where a changed unit adds or
removes an identity that another unit of the same package declares, the run
parses that unit again too. Its declaration moves between kept and dropped.

### Linking again

A reference's target depends on which of its candidates the graph declares,
and on what the candidates' packages re-export. Two rules select references
again from the recorded tiers:

- Where an identity appears or disappears, every unit whose references named
  its bare identity as a candidate selects those references again. The
  `probes` table finds those units.
- Where a changed unit's file re-exports a name, every unit whose references
  followed a re-export of that file's package selects them again. The
  `probes` table records those follows too.

A reference whose target changes changes its declaration. Selection from the
record needs no frontend call except in two cases, and each parses at most
the units of one package:

- **A re-export the link never followed.** A candidate that named a
  declaration stops naming one, and the link asks the candidate's package
  what it re-exports under the name. The unit that removed the declaration
  parsed in this run, so the run asks that parse. Only a re-export in another
  unit of the same package needs that unit parsed again. No frontend that
  ships today implements the re-export role. The kit and the conformance
  fixtures do.
- **An import of a new file.** A reference's target moves to another file in
  a language whose import names a file, and the link asks the referencing
  file how it imports the new one. The referencing unit parses again.
  protobuf is the one such language. A target can move there without an edit
  to the referencing file only where that file imports both files.

`Stats.Reparsed` counts the units a run parsed again to link, so a
workspace where these cases recur shows them.

### Edges

A read record lists the edges one execution read. The store's `ReadSet`
gains the fourth kind:

| Edge | Recorded by | Dirty when |
|---|---|---|
| Declaration | `Lookup`, whose edge covers the declaration's members, and the subject of every invocation | The declaration appeared, disappeared or changed |
| Fact | `Fact`, `FactOf`, and a projection's fact read | The fact's value or its presence changed |
| Membership | `ByKind` by kind, and `ByDirective` by spelling | A declaration of the kind, or a subject of the spelling, appeared or disappeared in the reader's scope |
| Package | `PackageOf`, and `Lookup` of a package's identity | The package changed |

```go
// Packages returns every package edge, in identity order: each package a
// reader took whole through PackageOf, or through Lookup of the package's
// own identity.
func (s *ReadSet) Packages() iter.Seq[symbol.Identity]
```

A reader that took a package whole can walk any of its declarations without
another tracked read, so the package edge is dirty when any member changes.
An invocation receives its subject as the match's value and makes no read
for it. Its record lists the subject's declaration edge on every run. The
subject's encoding covers its directives, so a changed gating instance
dirties every invocation of the subject.

A membership edge has no scope of its own. A generator invocation records
it under its plan's sources, and an annotator invocation under the whole
graph. The load lists the packages in which each kind and each directive
spelling changed. A declaration that appears in package P dirties a plan's
membership readers only where the plan's sources admit P.

A change to P's `gen.module` fact, or to P's files, binds the plan's scope
for P again. Where the new binding and the generation's admission of P
differ, the plan runs whole. A reader records only the
reads inside its scope, so no edge lists the readers that P's move
concerns. The run compares the generation's admission of each package whose
files or module fact changed with the new binding, so a plan runs whole
only for an actual move.

### The dirty set and its order

The dirty set is a set of edges. It grows in the order of the run's phases,
and each phase reads only what the phases before it settled.

1. **Load.** The diff of every changed unit, and every reference whose
   target changed, add declaration, package and membership edges.
2. **Validation.** The run validates the subjects of changed declarations
   and the subjects whose validation record meets the dirty set. A subject
   whose validated directives changed adds its declaration edge.
3. **Stamps and drops.** The run withdraws the classification stamps of
   every changed unit and the drops of every changed validation, then
   applies the new ones. Each fact whose winner changed adds its fact edge.
4. **Annotate, bucket by bucket.** Each annotator phase call runs the
   matches whose records meet the dirty set and the matches that appeared.
   Before the call, the run withdraws the claims of every match it runs
   again and of every match that disappeared. After the bucket, each fact
   whose winner changed adds its fact edge, before the next bucket reads.
5. **Plans, in dependency order.** Each plan executes its dirty groups,
   settles, routes, renders and exports, as the next sections describe. A
   plan that the generation lists as pending runs whole, and so does a plan
   whose scope moved a package.
6. **Close.** Collisions, the audit and the checks read records.

The capability order puts a provider's bucket before its consumer's, so an
annotator reads only facts that earlier buckets stamped. Processing the
buckets in order is a topological order of the facts.

### The invocation interface

A phase call receives two new context fields from the run. The authoring
surface honours both. A plugin that implements a role directly ignores
them, and the run records its call as one invocation.

```go
// WholeCall names a phase call that journals no invocation of its own.
const WholeCall RuleID = -1

// MatchKey identifies one invocation across runs. Its order is the
// canonical match order: plugin, rule, subject identity, gating instance,
// then host.
type MatchKey struct {
    Plugin ID
    Rule   RuleID
    // Subject is the match's subject: an emit-phase match's origin, and
    // zero for a graph-wide rule and for a whole call.
    Subject symbol.Identity
    // Instance is the gating directive instance, zero without one.
    Instance int
    // Host is the emit value an emit-phase rule matched, zero otherwise.
    Host EmitRef
}

// UnitRef names one accumulated unit of a plan: its plugin, its family,
// its package and its key.
type UnitRef struct {
    Plugin ID
    Tag    string
    Pkg    symbol.Identity
    Key    string
}

// EmitRef names one emit value by its unit and its place in the
// depth-first walk of the unit's declarations.
type EmitRef struct {
    Unit  UnitRef
    Index int
}

// Selection restricts a phase call to what a warm run executes again.
type Selection struct {
    // Matches are recorded matches to run again where their gate still
    // admits them, sorted in canonical match order.
    Matches []MatchKey
    // Candidates are subjects whose matches may have changed. The call
    // evaluates every rule's gate over each, runs each match that
    // Matches does not list, and reports what it found through the
    // journal.
    Candidates []symbol.Identity
}

// Journal receives a phase call's records in canonical match order, after
// the call's effects apply.
type Journal interface {
    // Invoked records one invocation.
    Invoked(inv Invocation)
    // Evaluated records the matches a candidate subject has now, none
    // where it has none.
    Evaluated(subject symbol.Identity, matches []MatchKey)
}

// Invocation is what one handler call read and touched.
type Invocation struct {
    Match MatchKey
    // Reads are the edges the call read, valid until Invoked returns,
    // so the call reuses the set for its next invocation.
    Reads *store.ReadSet
    // Exports are the plans whose export the call read, sorted.
    Exports []string
    // Units are the units the call placed declarations into, sorted.
    Units []UnitRef
    // Hosts are the emit values whose slots the call appended into.
    Hosts []EmitRef
    // Claimed are the facts the call stamped, sorted.
    Claimed []meta.FactRef
    // Findings are the findings the call reported.
    Findings []diag.Diag
}
```

`AnnotatorContext` and `GeneratorContext` each gain two fields:

```go
    // Select restricts the call to what a warm run executes again, and
    // nil runs every match.
    Select *Selection
    // Journal receives a record for each invocation the call runs, and
    // nil keeps none.
    Journal Journal
```

The authoring surface's phase call behaves this way:

- With `Select` nil it enumerates every match, as it does today.
- With `Select` set it runs the listed matches that its gates still admit,
  then evaluates every rule over the candidates. It runs each match it finds
  that the selection does not list.
- Emit-phase rules ignore `Select` and run over the emit values the plan's
  store contains.
- Effects apply in canonical match order whatever the selection, and the
  journal receives its records in the same order.

A rule enumerates its subjects in the index's order, and the run's records
are in identity order. The orders agree wherever an outcome depends on
order:

- A fact's subject is the invocation's subject or a declaration the subject
  declares, so only invocations of one subject can claim the same fact.
- A unit orders its declarations by origin identity, then instance, then
  sequence, so only invocations of one subject share an origin.
- A slot receives appends from the invocations that matched the emit value
  it belongs to, and from the invocation that created that value. They
  share the value's origin.

Within one subject the sequence orders by rule, then instance, which is the
order of `MatchKey`.

A phase call that journals nothing is one invocation, keyed by `WholeCall`.
Its read record is its reader's set, and its claimed facts are the ones
`Facts.ClaimedBy` lists for its plugin. The run executes it again whenever
the run found any change, because it can read through the untracked index.
That is the implicit subscription to everything in scope that `Subscribed`
already documents. Before the call, the run withdraws every claim the
plugin made in that bucket on the facts the call claimed last time. A
generator call that journals nothing joins its plan's dirty invocations,
and the plan still executes in part.

### Facts across runs

`Claim.Seq` becomes a rank key that every run computes the same way:

```go
// Order is a claim's place among the claims of one plugin in one bucket:
// the rule that made it, the subject of the invocation that made it, and
// the gating instance. A frontend's stamp has rule zero and the stamp's
// index among its subject's stamps as its instance. A drop has rule zero
// and the directive's instance.
type Order struct {
    Rule     int
    Subject  symbol.Identity
    Instance int
}

type Claim struct {
    Subject   symbol.Identity
    Authority Authority
    Bucket    int
    Plugin    diag.Origin
    // Order replaces Seq: the last step of the rank, in canonical match
    // order.
    Order   Order
    Pos     position.Pos
    Derived []Read
}
```

The fact store can withdraw a claim and restore a bag on first use:

```go
// Withdraw removes the claim on (c.Subject, k) that has c's rank source,
// and ranks the remaining claims again. Withdrawing a claim the store
// does not contain is not an error.
func (f *Facts) Withdraw(k KeyID, c Claim) error

// WithdrawGroup removes a group drop with c's rank source.
func (f *Facts) WithdrawGroup(g GroupName, c Claim) error

// FactRef names one fact: a subject and a key.
type FactRef struct {
    Subject symbol.Identity
    Key     KeyName
}

// ClaimedBy returns the facts a plugin claimed through this store in the
// current run, sorted by subject, then key.
func (f *Facts) ClaimedBy(p diag.Origin) []FactRef

// BagSource restores a subject's recorded claims the first time a read, a
// write or a withdrawal needs the subject's bag.
type BagSource interface {
    // Claims returns every recorded claim on the subject.
    Claims(subject symbol.Identity) ([]StoredClaim, error)
    // Present returns the subjects on which a key read present when the
    // source was recorded, in identity order.
    Present(k KeyName) ([]symbol.Identity, error)
}

// StoredClaim is one recorded claim: its key or its group, its envelope,
// and its value, nil for a drop.
type StoredClaim struct {
    Key   KeyName
    Group GroupName
    Claim Claim
    Value any
    Drop  bool
}

// Restore returns a fact store over r that restores bags from src on first
// use. ByKey merges the recorded presence with the transitions of the run.
func Restore(r *Registry, src BagSource) *Facts
```

The `claims` table stores each bag's whole claim record: every claim with its
envelope and value, the drops included, in rank order. A warm run compares
each touched fact's new winner with its recorded winner. An equal value and
presence is early cutoff: the fact adds no edge, and nothing downstream of
it runs. A claim's derivation remains the point reads made before the claim,
which `explain` shows. Invalidation follows the invocation's whole read
record, and the derivation is a prefix of it.

### Artifacts and groups

A generated file is an artifact. Its row in the `artifacts` table records:

- the manifest entry: the path, the plan, the digest, the plugins and the
  sources
- the package the file declares
- the group the file belongs to
- the position and the description of the file's first declaration, which a
  finding about the file reports at
- the export rows of the file's declarations, for a plan that a dependent
  plan or a check reads
- the name entries the file declares

A group is a set of files that execute together:

- the files the backend splits one unit into
- the files that two units share
- the files that contain an emit value an emit-phase invocation matched,
  and the files that invocation places into

Each group's row lists its files, its units, the invocations that
contributed to it, its read record and the findings that the render
reported for its files. The read record lists the edge of each of the
group's units, so a warm run finds the groups of a unit among the readers
of the unit's edge, and no row indexes the units. It also lists what the
group's settle, routing and render read:

- each name entry that its references read, whether a file declared it or
  not
- each collision scope that one of its names settles in
- each origin's name override in the target, which the settle reads as a
  fact
- each directory that a target placed one of its files in, whose residents
  the target derives the file's package from
- the files of the package of each per-package unit, because the layout
  writes the unit's file beside the package's first directory
- the toolchain modules, where a target placed a file against them

A group is dirty in each of these cases:

- An invocation that contributed to it is dirty: its read record meets the
  dirty set, or it read an export that changed.
- A match that contributed to it disappeared.
- A dirty invocation or a match that appeared placed a declaration into one
  of its units.
- One of its files changed on disk since the commit that wrote it.
- An edge of its read record is dirty: a name entry changed, a name entered
  or left one of its scopes, a name override changed, or the residents of a
  directory, the files of a package or the modules changed.

A plan executes its dirty groups this way:

1. The plan's generators run in bucket order. Each phase call's selection
   lists the matches that contribute to a dirty group, and its candidates
   are the subjects whose matches may have changed.
2. A dirty invocation or a new match can place into a clean group's unit,
   which makes that group dirty. The plan repeats step 1 for the new dirty
   groups until no group joins.
3. The plan discards every unit of a clean group. Such a unit received only
   the contributions of clean invocations that ran for a dirty group, and
   those contributions are unchanged.
4. Emit-phase rules run over the units of dirty groups. A group that contains
   the emit value an emit-phase invocation matched also contains the files
   that invocation places into, so every matched value is present.
5. The settle runs over the dirty groups' units against the name entries of
   the clean groups.

A dirty file executes every invocation that contributed to it, because an
accumulated file is one output. A clean file executes nothing.

### Names across files

The settle and the layout read across files. A reference resolves through
the package table and the origin table of the whole plan. A name collides
with the names of its scope in every file of the package. The layout
qualifies a reference into a declaration that another file's package
declares. The `names` table keeps the plan's name entries, and the settle
and the layout read the clean groups' entries through one interface:

```go
// Names is the name table of the files a warm run did not generate again:
// what a reference into those files resolves against, and what a new
// name collides with.
type Names interface {
    // InPackage returns the entry a file-level declaration of the package
    // declares under an emitted name, and false where none does.
    InPackage(pkg, emitted string) (NameEntry, bool)
    // OfOrigin returns the entry a declaration derived from the origin
    // declares under an emitted name, and false where none does.
    OfOrigin(origin symbol.Identity, emitted string) (NameEntry, bool)
    // InScope returns the entries of one collision scope, sorted by
    // settled name.
    InScope(scope string) []NameEntry
}

// NameEntry is one file-level name of a plan's file.
type NameEntry struct {
    Scope     string
    Package   string
    Origin    symbol.Identity
    Emitted   string
    Settled   string
    Ambiguous bool
    // File is the path the layout routed the declaration to, and FilePkg
    // the package that file declares.
    File    string
    FilePkg symbol.Identity
}

// NameKey names one entry a reference read.
type NameKey struct {
    Package string
    Origin  symbol.Identity
    Emitted string
}

// Settled is what one SettleWith call declared and read.
type Settled struct {
    // Declared are the store's file-level names as the settle left them.
    Declared []NameEntry
    // Read are the entries of others that the store's references and
    // collision checks read.
    Read []NameKey
    // Facts are the facts the settle read: each origin's name override in
    // the target.
    Facts []meta.FactRef
}

// SettleWith settles a store that contains part of a plan's units against
// the names of the plan's other files. Settle is SettleWith with no
// others.
func SettleWith(
    e *Emit, b Backend, facts *meta.Facts, sink *diag.Sink, others Names,
) (Settled, error)
```

The settle reads a declaration's name override through a point read of its
origin's fact, so a settle over part of a plan reads the overrides of that
part alone. `layout.Input` gains `Others plugin.Names` and `Kept []string`,
the paths of the plan's clean files, and `Residents` becomes a function of a
directory.

Before the settle mutates anything, the plan compares the emitted names of
its dirty groups with their recorded entries. A name that appeared,
disappeared or moved between scopes dirties every clean group with an entry
in that scope, and every clean group that read an entry under that name.
After the settle, a settled spelling that differs from its recorded entry
dirties every clean group that read the entry. Either case adds groups and
returns the plan to its first step with a fresh store. The set of dirty
groups only grows, so the plan stops within as many rounds as it has groups.
A plan whose names do not change settles once.

### Render, output and the sweep of stale files

The plan routes, renders and stamps the files of its dirty groups, and
stages them as it does today. A file whose bytes equal its recorded digest
is unchanged, and the sink leaves it and its modification time alone.

The gate stats every file the record lists. A clean file whose stat moved is
read:

- A missing file makes its group dirty. The run then writes the file again.
- A file whose body hashes to its trailer and whose digest equals the record
  is unchanged. The gate records its new stat.
- Any other file makes its group dirty, and the render stages it. The
  preparation then reports what it found exactly as a cold run's does. An
  intact stale output is updated, and a drifted or foreign file is an
  Error.

A file that a dirty group no longer produces is stale. Its plan stages its
removal. The sweep keeps a stale file that drifted, and stages its removal
again on every run, warm or cold. Each run then reports its `KeptOutput`
warning.

### Exports

A plan's export is the recorded export rows of its clean files and the new
rows of its dirty files, sorted by `ExportKey.Compare` and then by file. It
is in the sealed state alone, and no document publishes it. A warm run reads
the recorded rows of a plan's clean files only when a dependent plan that
executes, or a check that runs, reads the export. The export changed when a
dirty file's rows differ
from its recorded rows, or when a file with rows appeared or disappeared, so
the run compares the dirty files' rows and nothing else. A dependent
invocation that read the export through its match's `Export` is dirty when
the export changed. A dependent call that journals nothing reads every
export its context hands over, so any changed export dirties it. An edit
that changes a producer's output and none of its export rows dirties no
dependent.

### Close

- **Collisions** look each routed path up in the folded keys of the
  `artifacts` table. After a clash with a file that another plan keeps, the
  run compares every plan's routed and kept files. It then reports the same
  finding as a cold run.
- **The sweep of removed plans** belongs to cold runs. Removing a plan
  changes the composition fingerprint, so the run after it is cold.
- **The audit** evaluates the contracts over the declarations that appeared
  or changed, and over the subjects whose fact under a contract's key
  changed. It drops the finding of a declaration that disappeared. The
  `audit` table keeps the other findings.
- **A check** runs again after any change in the graph or the facts,
  because `CheckContext.Index` and `CheckContext.Facts` record no reads. It
  also runs again when a plan it reads rendered, added or removed a file or
  changed its export, and when the generation has no record of the check.
  It then reads the plans' whole records, kept and new, as a cold run hands
  them over. Otherwise the run reports its recorded findings.

### Findings across runs

Every record keeps the findings its execution reported:

- a region, the findings of its parse and its link
- a validation, an invocation and a check, the findings each reported
- a group, the findings that the render reported for its files
- an audit row, its finding

The settle and the layout report only Errors. An Error leaves its plan
pending, so the next run executes the plan whole and reports the finding
again.

A warm run reports the findings of every record it keeps and of every
execution it runs. Output sorts findings by position, code, message and
origin, so a warm run's findings equal a cold run's. The run derives a
finding about a whole plan, `FailedDependency`, again on every run.

### The parse memo

The memo is a second store of regions, keyed by the unit's key and the
executable digest. Its entries are self-checking blobs in a ledger of their
own.

```go
// Memo configures the parse memo.
type Memo struct {
    // Limit is the memo's cap in bytes. Zero keeps no memo.
    Limit int64
    // Open returns a fresh ledger for the memo's entries on each run, and
    // nil keeps them under memo/ in the composition's own ledger.
    Open func() (ledger.Ledger, error)
}

// Memo configures the parse memo. The zero Memo, the default, keeps none.
func (b *Builder) Memo(m Memo) *Builder
```

- **Lookup.** The load consults the memo for a changed unit, between the
  generation and a parse. A hit decodes the region and selects its
  references against the current graph, without a parse.
- **Write.** The run's commit writes every unit the run parsed, under its
  key, through `Put`. An entry's trailing CRC-32C covers its bytes, so an
  entry a machine crash tore decodes as a miss, and the commit removes it.
  A dry run writes nothing.
- **Recency.** A hit touches the entry.
- **Eviction.** The commit adds the bytes it wrote to `memo/total`. Where the
  total exceeds the cap, or where `memo/trimmed` is more than a day old, it
  lists the memo, computes the true total, and removes the entries with the
  oldest modification times until the total is under 90% of the cap. ccache
  marks a hit and evicts by modification time the same way, and the go
  command lists its build cache at most once a day.
- **Cold runs.** A cold run reads neither the generation nor the memo, and
  writes both as any run does.

Two workspaces share a memo when their compositions open one ledger, such as
`ledger.OpenAt` over a directory under the user's cache directory. Sharing
is safe by construction:

- An entry's key covers the unit's inputs, the frontend and the executable,
  so an entry that two workspaces both read is the parse either would make.
- Two runs that write one entry write the same bytes, and each write
  replaces the blob atomically.
- A removal that races a read leaves the reader a miss.

Concurrent commits can each add bytes to `memo/total` and lose one update.
The daily listing corrects the total, so the memo exceeds its cap by at most
one day of writes.

The executable digest in the key means a memo entry serves the build that
wrote it. A branch switch with one build of the tool restores every unit
that build parsed before. A new build parses again and fills the memo.

### Scaling and its gates

A warm run's cost has two parts. The stat sweep is linear in the number of
files, because a stat for each file is the cheapest change detection there
is without a file-system watcher, and the kernel takes none. Everything else
follows the edit. The warm gates measure the two parts separately:

| Gate | What it asserts | Where it runs |
|---|---|---|
| Zero work | A warm run with no change hashes no file, parses no unit, decodes no region, runs no invocation, calls no check and writes no blob. The probe is exact | Every runner |
| Warm, no change | Time per walked file at 10,000 packages within 1.15 of the time at 1,000 | Every runner |
| Warm, one edit | The cost of the edit, meaning the edited run's time less the time of a run with no change over the same tree, at 10,000 packages within 1.5 of the cost at 1,000, over per-source and per-package outputs | Every runner |
| Sub-second | A warm run with no change at 10,000 packages under the absolute bound | Dedicated runners |

The edit's cost is proportional to the edit because the run keeps
everything that grows with the corpus out of it:

- **The manifest.** The run rewrites the documents of the buckets whose
  entries changed. Encoding measured 5.2 ms for a 10,000-entry manifest,
  about 520 ns per entry. One document of 100,000 files would cost about
  52 ms of encoding on every edited run. A bucket of that record contains about
  390 entries, about 0.2 ms.
- **Residents.** A directory's residents come from the load's file records,
  which a run reads one directory at a time, in place of a map over every
  file of the graph. Each resident takes its package's name from the graph.
- **Modules.** The `modules` table counts each module's packages, and a
  change to a package's module facts updates its row.
- **The routing index.** An index admits a package, and looks up a
  subject's skip ruling, the first time a match or a route asks.
- **The settle.** It reads overrides with point reads.
- **Exports.** The run compares the dirty files' export rows.
- **Validation, the audit and the checks.** Each executes for what changed.

### Run statistics and the report

```go
// Stats counts what one run executed: what a probe asserts and what a
// report summarizes.
type Stats struct {
    // Cold reports that the run ignored the sealed state.
    Cold bool
    // Statted and Hashed count the files the gate statted and hashed.
    Statted, Hashed int
    // Parsed, Restored and Kept count the units the run parsed, restored
    // from the memo, and took from the generation. Reparsed counts the
    // units it parsed again to link.
    Parsed, Restored, Kept, Reparsed int
    // Decoded counts the regions the run decoded.
    Decoded int
    // Validated counts the subjects the run validated.
    Validated int
    // Invoked counts the invocations each phase call ran.
    Invoked []Invoked
    // Rendered counts the files the plans rendered.
    Rendered int
    // Checked counts the workspace checks the run called.
    Checked int
    // Generation reports that the commit wrote a generation, Written the
    // bytes it wrote to the state, and Size the live generation's bytes.
    Generation    bool
    Written, Size int64
}

// Invoked counts the invocations of one phase call.
type Invoked struct {
    // Plan is the call's plan, empty for an annotator.
    Plan   string
    Plugin plugin.ID
    Phase  plugin.Phase
    Count  int
}
```

`Report` gains `Stats Stats`, and `Input` gains `Cold bool`. `Report.Emits`
contains the units each plan generated in the run: every unit on a cold run,
and the units of the dirty groups on a warm one. A composition that declares
no output keeps no ledger and runs cold every time, so its emit stores remain
the run's whole product.

### The warm checks

`workspacetest.Fixture` gains one field:

```go
    // Edit changes one declaration of the tree at root that the first
    // plan generates from. The first plan's output changes, and its export
    // does not.
    Edit func(root string) error
```

```go
// RunWarmColdSuite checks the warm path against the cold one over the
// fixture, each check in a parallel subtest over directories of its own.
func RunWarmColdSuite(t *testing.T, f Fixture)

// AssertWarmUnchanged runs the fixture in root cold and then warm, with
// every file's modification time an hour in the past before each run, and
// then once more with nothing changed. It checks that the last run read
// the sealed state, hashed no file, parsed no unit, decoded no region, ran
// no invocation and called no check, and that it left every file under
// root as it was, the manifest's documents and the state included.
func AssertWarmUnchanged(tb assert.TB, f Fixture, root string)

// AssertTouched runs the fixture in root as AssertWarmUnchanged does, sets
// one source file's modification time to the present without changing its
// bytes, runs again, and checks that the run hashed that file once and
// parsed nothing.
func AssertTouched(tb assert.TB, f Fixture, root string)

// AssertWarmEdited runs the fixture cold in warm and applies the edit
// there, then runs warm. It applies the edit to a fresh copy in cold and
// runs cold. It checks that the two directories contain the same files
// outside the state directory, that the two records list the same
// entries, that a probe plan depending on every plan read the same
// exports, and that the two runs reported the same findings.
func AssertWarmEdited(tb assert.TB, f Fixture, warm, cold string)

// AssertDamaged runs the fixture cold in root, cuts every segment of the
// state to its first byte, runs again, and checks that the run reported
// one ColdState Info, ran cold, and left the files that the first run
// left.
func AssertDamaged(tb assert.TB, f Fixture, root string)

// AssertRestored composes the fixture with a memo whose cap removes no
// entry, runs it cold in root, applies the edit, runs, writes the
// fixture's tree back over the edit, and runs again. It checks that the
// last run parsed no unit and restored from the memo each unit that the
// run after the edit parsed.
func AssertRestored(tb assert.TB, f Fixture, root string)
```

`AssertWarmUnchanged` and `AssertTouched` run the fixture twice before the
run that they check. A cold run records each file that it created with a
size of zero, so only the second run's records prove the files unchanged.
With every modification time an hour in the past, no record is racily
clean.

`AssertWarmEdited` compares the records' entries and not the documents'
bytes, because a disk ledger names the workspace after its directory.

`RunWorkspaceSuite` gains two warm checks:

```go
// AssertExportCutoff runs the fixture cold in root beside a probe plan and
// a dependent plan whose one invocation reads the first plan's export,
// applies the edit, and runs warm. It checks that the first plan wrote a
// file again, that its export is unchanged, and that the dependent ran no
// invocation.
func AssertExportCutoff(tb assert.TB, f Fixture, root string)

// AssertWarmChecked runs the fixture with a recording check that reads the
// first plan: cold in warm, then the edit and a warm run there, and the
// edited tree cold in cold. It checks that the warm call read the records
// the cold call read.
func AssertWarmChecked(tb assert.TB, f Fixture, warm, cold string)
```

The kernel's own tests run fixture frontends, annotators and backends
through the probes the suites cannot vary:

- An annotator that runs again and stamps identical values runs no
  generator invocation that read the fact, and the run renders no file.
- A declaration of an enumerated kind that appears outside a plan's sources
  runs none of the plan's enumerations, and one inside them runs the
  enumeration again.
- An edit to one member of a package that a reader took through
  `PackageOf` runs the reader again.
- A removed file or a changed module fact that moves a package into a
  plan's sources or out of them leaves the files of a cold run.
- A warm run decodes the recorded region of each edited unit, which the
  load compares, and the kept regions that its reads touch.
- The bytes a commit writes after one edit are below a tenth of the state's
  size, and the commit rewrites one manifest document.
- A plan that failed in one run executes its failed work again in the next.
- A removed declaration that a re-export of another unit publishes, and a
  target that moves to another imported file, each parse one unit again.
- A memo over its cap removes its least recently used entries at the
  commit, a cold run keeps the memo's entries, and two workspaces over one
  memo ledger restore each other's units.
- A read that fails at any point of eight warm scenarios, and damage at the
  commit's merge, each start the run again cold.

The Go conformance fixture, two plans over one tree, runs the probe of the
(symbol, key) grain: an edit that only the stubs plan reads runs no
invocation of the registry plan. The second run of
`pipelinetest.AssertIdempotent` is cold, because a warm run over an
unchanged tree executes nothing and would not check that the phases produce
the same bytes again.

`plugintest.RunPluginSuite` gains `AssertSelective`: a selection that lists
every match the full call ran reproduces the full call's emit, facts and
findings, and the journal lists every match once.

### Failure semantics

| Code | Name | Severity | Position | Meaning |
|---|---|---|---|---|
| EID-0062 | `ColdState` | Info | The state directory's `CURRENT` | The run ignored the sealed state and ran cold, and the message states the cause |

A cause from the table of cold runs is a `ColdState` Info and never an
error. A ledger that fails to read or write is an error, as it is today,
because nothing in the source causes it. A commit whose generation write
fails leaves the first crash window, and the run returns the error. A memo
entry that does not decode is a miss, and the commit removes it.

### Cost

Measured on the machine the motivation names:

| Step | Cost |
|---|---|
| Walk and stat through `fs.FS` | 1.34 to 1.94 µs per file, about 134 to 194 ms for 100,000 files on one goroutine |
| Read and hash a file | 37 µs per file |
| SHA-256 | 2.63 GB/s |
| The pipeline at 1,000 packages, cold, without a parse | 143 ms on average on one worker |
| Encode a 10,000-entry manifest | 5.2 ms |
| Replace a 2 KiB blob, synced | 745 µs on one goroutine, 143 µs each on 16 goroutines |
| Replace a 2 KiB blob, unsynced | 15.6 µs |
| A warm run after one edit, 1,000 packages of 200 declarations, from a tree | 400 ms as the mean of 12 runs, and 1.33 million allocations |
| The state of those 200,000 declarations | 495 bytes per declaration |

Estimated, at 10,000 packages, 100,000 files and two million declarations:

- **A warm run with no change** stats every file, reads the `files` and
  `units` tables once, decodes no region, executes nothing and writes
  nothing. The manifest's documents fill `Report.Manifest`. The cost is
  about the stat sweep, 134 to 194 ms, plus the decoding of one row for each
  file and unit and of the manifest's documents. It grows linearly with the
  number of files.
- **A warm run after one edit** adds the parse of one unit, the lookups of
  the dirty edges, the executions the dirty set meets, the render of the
  dirty files, a commit of the changed rows, and one synced write for each
  manifest bucket the edit touched.
- **A cold run** adds the encoding of every region and every record to its
  commit, and at most 256 synced manifest writes, about 37 ms on 16
  goroutines. It writes every unit to the memo where one is configured,
  through `Put`. Its dispatch hands one record for each invocation to the
  journal, which appends the record's edge hashes to a buffer of the phase
  call's own.
- **Memory.** A warm run keeps in memory the regions it decoded, the bags it
  restored and the records it read. A run that touches every region uses the
  memory of a cold run.
- **Disk.** The state contains one row for each file, unit, validated
  subject, fact, invocation, edge, artifact and name. At 495 bytes per
  declaration, two million declarations take about 1 GB.

### Migration

| Caller | Change |
|---|---|
| `ledger.Ledger`, `ledger.Dir`, `ledger.Mem`, and the crash-window test's ledger | Implement the blob methods. `Mem.Writes` counts writes |
| `workspace.begin` and `commitRecord` | Read and write the record through the state |
| `manifest.Encode` and `manifest.Decode`, with their tests and benchmark | Become `EncodeShard` and `DecodeShard`, and `Version` becomes 2 |
| `rundir.Record`, which the pipeline and workspace suites read the record through | Lists and joins the manifest's documents |
| `load.Config.PluginSet`, set in `workspace/run.go`, `rulestest/fixture.go` and its test, `frontendtest/suite.go` and `checks.go`, and 3 load test files | The field is removed. The load takes `Prior` and `Memo` |
| `frontendtest.AssertFingerprinted` | Drops the case that the composition's fingerprint re-keys every unit |
| `meta.Claim.Seq`, set in `stamper.go`, twice in `workspace/run.go`, in `frontendtest/checks.go` and in 2 meta test files, and read in `stamper_test.go` | Set and read `Order` |
| `layout.Residents` and `layout.Input.Residents` | A function of a directory |
| `plugin.NewIndex` | Admits packages and looks up skip rulings on first use |
| `plugin.Settle` | Unchanged. `SettleWith` takes the clean files' names |
| `workspacetest.Fixture` literals in `eidos-conformance` | Set `Edit` |
| `Workspace.Fingerprint` | Folds the composition inputs listed above |
| `eidos-sdk` | The facade regenerates for the new exported names |

## Alternatives considered

### Persist the emit store

The run records each plan's units after Generate, and a warm run restores
the clean groups' units instead of keeping their name entries, export rows
and findings. The settle and the layout then run over a whole store, with no
name table and no repeat of a plan's first step.

**Why not:** the settle and the layout would run over every unit of the plan
on every warm run. That cost is linear in the plan and not in the edit. The
recorded units would double the state, and an emit codec would become part
of the format. The specification also keeps emit out of the sealed state, so
that no warm path can read a sibling plan's emit.

### Re-execute whole plans

A plan executes again whenever anything its scope admits changed, and keeps
its previous output only when nothing did. No group, contributor set or name
table is needed.

**Why not:** one edit in a monorepo touches most plans' scopes, so most plans
would execute whole. The warm cost after one edit would grow with the plan,
and the edit gate bounds that growth for per-source and per-package files.

### Verify from the outputs down

Every artifact checks its recorded inputs, recursively, on every run, the
way rustc's `try_mark_green` and Salsa's `maybe_changed_after` verify a
query. The run works without a reverse index.

**Why not:** a compiler demands a small part of its graph per session, and a
generator demands every artifact on every run. Verifying from the outputs
down visits every artifact's record on every run, so a warm run with no
change grows with the corpus. Bazel's documentation states the trade-off:
bottom-up invalidation is optimal when the same top-level node is built
again, and Bazel does only bottom-up invalidation.

### An embedded database

A key-value store such as SQLite or bbolt keeps the tables.

**Why not:** the kernel takes no third-party dependency. A B-tree's update
paths also buy generality that immutable runs and a pointer swap do not use.

### A global intern table

One table assigns every identity, key and path a dense identifier for all
generations, and every region and record stores identifiers.

**Why not:** a memo entry would then depend on the intern table of the
generation that wrote it. A table that restarts empty after a cold run would
make every older memo entry unreadable, and a table that only grows needs a
compaction that rewrites every reference. With a string table per region and
a 64-bit hash per edge, every region decodes on its own. A hash collision
costs an extra execution and never a missed one.

### Keep the composition fingerprint in the unit key

Every unit key folds the composition's fingerprint, as it does today.

**Why not:** a region depends on the frontend and on the unit's inputs
alone. A frontend's classification stamps are raw data, and the run
interprets them against the registry after the load. Folding the
fingerprint into the key makes every memo entry miss after any change to the
composition, and the generation's header already checks the composition.

### A codec for the frontend's import bindings

Each frontend encodes its import bindings, and a region records them, so a
warm run re-links a clean unit by running the frontend's `Resolve` again.

**Why not:** every frontend would implement and test a codec for a record
the kernel treats as opaque. The recorded candidate tiers serve every
re-selection but two, and each of those parses at most the units of one
package.

### Re-export and import tables as data

The re-export and file-import roles return each file's tables at parse
time, and the kernel resolves every follow from the region alone. No
re-selection then parses a unit again.

**Why not:** it changes two frontend roles to save a parse that happens only
in the two bounded cases, and no frontend that ships today implements the
re-export role. The query form also leaves each language's rules for
wildcard re-exports in that language's frontend, where a table would move
them into the kernel.

### Parse again to re-link

A unit whose references need a new selection parses again, and the run
records no candidates.

**Why not:** adding a declaration that the references of a thousand units
name would parse those thousand units. Selecting again from recorded tiers
decodes each of them once instead.

### Hash every file

The gate reads and hashes every file on every run. A touch then never makes
a file look changed, and the gate needs no racy-clean rule.

**Why not:** measured at 37 µs per file against 1.34 to 1.94 µs for a stat,
hashing every file costs 3.7 s at 100,000 files, nearly four times the
whole budget.

### Gate on plugin versions alone

The header checks the composition fingerprint, which folds each plugin's
declared version, and records no executable digest.

**Why not:** a plugin whose code changes without a new version leaves records
that a warm run trusts and a cold run would not reproduce. The digest makes
that a cold run instead of a stale output. rustc discards its incremental
state whenever the compiler's commit differs, on the stated ground that
ignoring the state is always safe and compilers change rarely.

### One manifest document

The manifest remains one document, rewritten whenever an entry changes.

**Why not:** encoding costs about 520 ns per entry, so every edited run at
100,000 files spends about 52 ms on the manifest alone. The edit's cost
would then grow with the corpus.

### One manifest document per directory

Each directory of generated files has a document of its own, so a reader
finds a directory's entries in one place.

**Why not:** a cold run at 10,000 directories writes 10,000 synced
documents, about 1.1 to 1.4 s on this machine even on 16 to 64 goroutines.
Buckets by hash bound the documents at 256.

### Unsynced manifest documents

The commit writes the documents through `Put`, as the memo writes its
entries.

**Why not:** a machine crash can then leave a torn document behind a durable
`CURRENT`, and no subsequent run rewrites a bucket whose entries do not
change.

### Publish each plan's export as a document

The run writes each plan's export beside the manifest, as the record does.

**Why not:** nothing reads it. A dependent plan reads the producer's export
rows from the sealed state, and a document would add one more write that
grows with the plan.

### A memo in the state directory only

The memo is always under the workspace's own state directory.

**Why not:** two worktrees of one repository then parse every unit the other
already parsed. Content-addressed entries make sharing safe, so the
location is the composition's choice.

### Gate warm runs on raw time ratios

The scaling gates compare a warm run's whole time at 10,000 packages with
its time at 1,000.

**Why not:** the stat sweep grows tenfold between the two sizes by
construction, so the ratio fails whatever the rest of the run does, and a
gate that cannot pass teaches people to ignore it.

### Replay no findings

A warm run reports only the findings of what it executed.

**Why not:** a warning from a kept file would disappear from the second run
and return on the next cold one. Two runs over one tree would then report
different findings. rustc's on-disk cache stores the side effects a query
emitted, its diagnostics among them, beside the query's cached result.

### Validate, audit and check everything on every run

Validation, the audit and the workspace checks run whole on every run, and
only the phases after them are incremental.

**Why not:** a warm run with no change would then grow with the corpus,
because validation, the audit and a check that enumerates the graph each
read every subject they cover.

## Drawbacks

- The kernel gains the package `core/internal/state`, sixteen tables, a run
  format with compaction, a generated region codec, and one code.
- `ledger.Ledger` changes every method, so every implementation outside the
  kernel breaks.
- The manifest's format becomes 256 documents, so a reader of
  `manifest.json` moves to listing and joining them.
- `meta.Claim.Seq` becomes `Claim.Order`, and every caller that builds a
  claim changes.
- `core/plugin` gains `MatchKey`, `UnitRef`, `EmitRef`, `Selection`,
  `Journal`, `Invocation`, `Names`, `NameEntry`, `NameKey`, `Settled`,
  `SettleWith`, `WholeCall` and `ExportKey.Compare`, and both phase contexts
  gain two fields.
- `core/store` gains `Source`, `RegionInfo`, `RegionFile`, `Region`,
  `Sealed`, `Graph.Damaged` and the package edge. `core/meta` gains `Order`,
  `FactRef`, `Withdraw`, `WithdrawGroup`, `ClaimedBy`, `BagSource`,
  `StoredClaim` and `Restore`. `core/frontend/load` gains `Prior`,
  `FileRecord`, `Verdict`, `UnitRecord`, `DoorRecord`, `Digested`, `Memo`,
  `Changes` and `Spelling`. `core/ledger` gains `Put`, `Touch`, `List`,
  `Blob` and `OpenAt`.
- A plugin that implements a role directly executes whole whenever the run
  found any change.
- A plan executes whole when a package enters or leaves its sources.
- Damage that the commit's merge meets starts the run again cold after the
  plans committed, and the cold run's report lists no change for the files
  that the discarded run wrote.
- A warm run with no change still stats every file, so it grows linearly
  with the file count.
- Every build that changes the consumer's executable makes its first run
  cold, and the memo's entries from the earlier build stop serving.
- A dirty group executes every invocation that contributed to it, so a
  per-plan file costs its whole contributor set on every edit to it.
- A change of names across files repeats a plan's generation, once for each
  round that adds groups.
- The state contains one row for each file, unit, validated subject, fact,
  invocation, edge, artifact and name.
- A modification time from a clock more than two seconds ahead of the run's
  clock, such as a network file system's, defeats the racy-clean rule.
- Without a lock, a run that removes unreferenced segments can send a
  concurrent run's next run cold.
- A shared memo can exceed its cap by one day of writes.

## Unresolved and future work

- The lock on the state directory, which keeps two runs apart, is not
  proposed here.
- A run narrowed to part of the workspace is not proposed here.
- The `watch` command, which polls this gate, is not proposed here.

## References

| What | Where |
|---|---|
| Incrementality and performance: the gate, the sealed state, red-green and the memo | [09-incrementality.md](../architecture/09-incrementality.md) |
| Workspaces and plans: the runtime, the commit and exports | [08-workspace-and-plans.md](../architecture/08-workspace-and-plans.md) |
| Testing and conformance: warm≡cold | [13-testing-and-conformance.md](../architecture/13-testing-and-conformance.md) |
| Output and determinism: drift, the manifest and exports in the state directory | [17-output-and-determinism.md](../architecture/17-output-and-determinism.md) |
| Benchmarking: the warm scenarios and the scaling gates | [19-benchmarking.md](../architecture/19-benchmarking.md) |
| The command kernels: `run --cold` and the state directory | [20-cli.md](../architecture/20-cli.md) |
| D9, D10, D17 and D56: no daemon, red-green at (symbol, key), the target, the constants | [21-decisions.md](../architecture/21-decisions.md) |
| D60, D61, D72, D77 and D78: the runtime, the artifact granule, the sealed state, the memo, the read grain | [21-decisions.md](../architecture/21-decisions.md) |
| D103 to D111: the decisions this proposal records | [21-decisions.md](../architecture/21-decisions.md) |
| git, racily clean index entries | https://github.com/git/git/blob/c46c1e37724f0478939de636ab8ea5a89086d532/Documentation/technical/racy-git.adoc |
| Go, the test cache's stat rule | https://github.com/golang/go/blob/67c1d421161d3d1ae9f5fd005e84c29fd0d9f896/src/cmd/go/internal/test/test.go |
| Go, the build cache's daily trim | https://github.com/golang/go/blob/67c1d421161d3d1ae9f5fd005e84c29fd0d9f896/src/cmd/go/internal/cache/cache.go |
| rustc, incremental compilation in detail | https://rustc-dev-guide.rust-lang.org/queries/incremental-compilation-in-detail.html |
| rustc, the side effects the on-disk cache stores | https://github.com/rust-lang/rust/blob/c1658342b0ea7eab61db1960082668c77f553522/compiler/rustc_middle/src/query/on_disk_cache.rs |
| rustc, the version check on incremental state | https://github.com/rust-lang/rust/blob/c1658342b0ea7eab61db1960082668c77f553522/compiler/rustc_incremental/src/persist/file_format.rs |
| Salsa, verifying a memo in a new revision | https://github.com/salsa-rs/salsa/blob/30b614d826d697c47bc0f21591fab218f2004032/src/function/maybe_changed_after.rs |
| Shake, the `shakeChange` modes | https://github.com/ndmitchell/shake/blob/02594b855981856ebc97d4b3813976a6f9b760d3/src/Development/Shake/Internal/Options.hs |
| Mitchell, "Shake Before Building", ICFP 2012: the database in section 4.2, its save time in section 8 | https://ndmitchell.com/downloads/paper-shake_before_building-10_sep_2012.pdf |
| Bazel, Skyframe's incrementality | https://github.com/bazelbuild/bazel/blob/764f29e87ae6f540d4be8f0817f90acb3be0cb53/docs/reference/skyframe.mdx |
| LevelDB, `SetCurrentFile` | https://github.com/google/leveldb/blob/7ee830d02b623e8ffe0b95d59a74db1e58da04c5/db/filename.cc |
| ccache, cache size management | https://ccache.dev/manual/latest.html#_cache_size_management |
