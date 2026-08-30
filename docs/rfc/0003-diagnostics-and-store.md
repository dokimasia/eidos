---
rfc: 0003
title: Diagnostics and the node store
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-08-30
updated: 2026-08-30
discussion: none
supersedes: none
superseded-by: none
produces-adr: tbd
---

# RFC-0003: Diagnostics and the node store

## Summary

This RFC pins the two packages every later one reports and reads
through: `diag`, which carries a positioned finding under a stable
code, and `store`, which holds the node graph, seals it at freeze,
and hands out the tracked reader that is the only path a read takes.

## Motivation

Nothing else in the kernel can be built first. A plugin's every
finding is a diagnostic, a plugin's every read goes through the
reader, and the phase discipline that separates loading from
annotating is the store's freeze rather than a convention.

Two properties have to be built in from the start rather than
retrofitted, because retrofitting either one means revisiting every
call site.

The reader records what it read. Read tracking is what the
incrementality engine invalidates along, and a read that skips the
reader is invisible to it: the result is output that is stale and
looks current. Nothing consumes the edges yet, so this RFC settles
the shape and the grain and leaves the consumer to the engine.

A diagnostic carries a code rather than a message. Consumers script
against codes, tests assert on them, and documentation anchors to
them, so a code is API from the first one registered.

## Detailed design

### diag

```go
// Package diag reports positioned findings under stable codes.
package diag

// Severity says what a finding means for the run.
type Severity uint8

const (
    // SeverityError means the run is wrong: a refusal, a violated
    // contract, a collision. Any Error fails the run.
    SeverityError Severity = iota
    // SeverityWarning means the output stands but a human should
    // look. Warnings never fail a run.
    SeverityWarning
    // SeverityInfo covers provenance and progress.
    SeverityInfo
)

// Prefix owns a range of codes: the kernel's, a satellite's, and a
// consumer's own registered at Build.
type Prefix string

// KernelPrefix owns every code the kernel reports.
const KernelPrefix Prefix = "EID"

// PluginID names whoever reported a finding: a plugin's declared
// name, or one of the kernel phases below.
type PluginID string

// The kernel phases that report findings. A phase is an origin like
// any plugin, so a consumer filtering by origin needs no second rule
// for the kernel.
const (
    PhaseBuild    PluginID = "build"
    PhaseLoad     PluginID = "load"
    PhaseLink     PluginID = "link"
    PhaseFreeze   PluginID = "freeze"
    PhaseAnnotate PluginID = "annotate"
    PhaseGenerate PluginID = "generate"
    PhaseRender   PluginID = "render"
    PhaseClose    PluginID = "close"
)

// Code identifies a finding across releases. A changed meaning is a
// new code, never an edit, because consumers assert on it.
type Code struct {
    Prefix Prefix
    Number int
}

func (c Code) String() string // "EID-0007"

// Diag is one finding.
//
// Pos is never zero: a diagnostic without a position is a bug in
// whatever emitted it, which the conformance suite checks.
type Diag struct {
    Code     Code
    Severity Severity
    Pos      position.Pos
    Msg      string          // one sentence, present tense
    Origin   PluginID        // the plugin or kernel phase
    Related  []position.Pos  // the colliding twin, the export
}

// Registry holds every registered code, and refuses a number twice
// within one prefix.
type Registry struct{ ... }

func NewRegistry() *Registry
func (r *Registry) Register(p Prefix, s CodeSpec) (Code, error)

type CodeSpec struct {
    Number  int    // unique within the prefix
    Meaning string // anchored in the code index
}
```

Registration answers an error rather than panicking, because Build
collects every fault in one pass and a panic would report the first.

### Codes are declared where they are registered

Every registered name becomes a typed value rather than a string a
caller spells, and for a code that value is the registration itself:

```go
// codes.go, in the package that reports them
var (
    // FrozenWrite refuses a structural write after the seal.
    FrozenWrite = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
        Number:  1,
        Meaning: "a declaration was added or removed after Freeze",
    })
)
```

The declaration and the registration are one statement, so nothing
can drift from the registry and no generator is needed. That is the
difference from the registries whose names arrive as data: a shape
catalog reads its names from spec files and a metadata key can be
declared by a manifest, so those generate constants because the Go
side has nothing to declare from. A code is written in Go, so it
declares itself.

`MustRegister` panics, and only it does: a duplicate code is a
programming error at package initialization, before a run exists to
report into. `Register` answers an error for the registries Build
populates from config.

### The sink

A finding has to reach somewhere, and every role's contract is the
same: attach the finding and carry on, or answer an error and stop
the phase.

```go
// Sink collects the findings of one run.
//
// It is safe for concurrent use: frontends, and annotators under
// the parallelism opt-in, report while running alongside each
// other. Findings answer in report order per origin, and the run
// sorts them for output, so a parallel run reports what a serial
// one does.
type Sink struct{ ... }

func NewSink() *Sink

// Report attaches one finding.
func (s *Sink) Report(d Diag)

// Errorf, Warnf and Infof report at one severity without the
// caller assembling a Diag.
func (s *Sink) Errorf(c Code, at position.Pos, by PluginID, format string, args ...any)

// Failed reports whether any Error was attached, which is what
// decides the run's outcome.
func (s *Sink) Failed() bool

// All answers every finding, in a stable order.
func (s *Sink) All() iter.Seq[Diag]
```

Suppression, which reads a directive at a declaration, belongs to
the directive chunk: the sink filters on what it is told, and
nothing tells it yet.

### store

The graph holds node declarations, indexes them, and seals.

```go
// Package store holds the node graph and the tracked reads over it.
package store

// Graph is the run's node declarations.
//
// AddPackage is safe to call concurrently: frontends shard per unit
// and load in parallel, so the graph serializes writes itself rather
// than asking every frontend to. Serialization is per package, so
// two frontends adding two packages do not contend.
//
// Reads are safe to make concurrently with each other once the
// graph is frozen, which is the only state annotators and
// generators see it in.
type Graph struct{ ... }

func New() *Graph

// AddPackage adds a parsed package. It is refused after Freeze.
func (g *Graph) AddPackage(p *node.Package) error

// Freeze seals the graph and builds the kind index. It is
// idempotent.
func (g *Graph) Freeze()

// Frozen reports whether the graph is sealed.
func (g *Graph) Frozen() bool

// Reader hands out a tracked read handle recording into rs and
// seeing only what sc admits. It is refused before Freeze:
// identities are not assigned and the graph is still moving, so an
// edge recorded then would name a declaration that may not survive
// the phase.
func (g *Graph) Reader(rs *ReadSet, sc Scope) (*Reader, error)

// ByKind and Lookup answer untracked. They are the kernel's own
// path, for the dispatcher deciding which rules a phase runs:
// dispatch is not a plugin's read and must not record one, and it
// answers for every plan rather than one. Everything a plugin
// reaches goes through a Reader.
func (g *Graph) ByKind(k symbol.Kind) iter.Seq[symbol.Symbol]
func (g *Graph) Lookup(id symbol.Identity) (symbol.Symbol, bool)
```

The untracked path is deliberately on the graph rather than the
reader, so the two are told apart by which value a caller holds. A
plugin is handed a reader and never the graph, which is what keeps
the single-door rule structural rather than a review comment.

The reader is the only path a read takes.

```go
// Reader is a tracked, scope-filtered read handle.
type Reader struct{ ... }

// ByKind enumerates the declarations of one kind. It records a
// set-membership edge: the reader runs again when a declaration of
// that kind is added or removed, and never when one merely changes.
func (r *Reader) ByKind(k symbol.Kind) iter.Seq[symbol.Symbol]

// Lookup answers one declaration by identity. It records a
// per-identity edge, so a change to that declaration alone re-runs
// the reader.
func (r *Reader) Lookup(id symbol.Identity) (symbol.Symbol, bool)

// PackageOf answers the package holding a declaration, recording a
// per-identity edge on the package.
func (r *Reader) PackageOf(id symbol.Identity) (*node.Package, bool)
```

### Scope

A reader sees one plan's source packages and nothing else.

```go
// Scope decides which packages a reader may see.
//
// A nil Scope admits everything, which is what a composition with
// one plan wants and what a fixture uses.
type Scope func(pkg symbol.Identity) bool
```

A declaration outside scope is neither answered nor recorded. Both
halves matter: answering it would let one plan observe another's
sources, and recording it would let a change the plan could never
have seen re-run it. Scope filtering and edge recording therefore
compose, rather than the second undoing the first.

The predicate is a function rather than the plan's own source
vocabulary, because that vocabulary matches on facts a frontend
stamps and no frontend exists yet. A plan builds a Scope from its
predicate when plans arrive; until then a caller passes nil or a
package-set closure.

### The two grains

The grain rule is the reader's whole contract, and it is what keeps
a read from costing more than it observed.

| Read | Edge | Re-runs when |
|---|---|---|
| `Lookup`, `PackageOf`, walking a declaration's members | per identity | that declaration changes |
| `ByKind`, a package's declaration list | set membership | a declaration enters or leaves the set |

An enumeration prices "I asked for the whole set" honestly as
sensitivity to membership. Sensitivity to a change inside the set
comes from the per-identity edges the enumerator records for the
declarations it actually touched while iterating, which is why
`ByKind` answers an iterator rather than a slice: it records what
the caller reached, not what it might have.

```go
// ReadSet is what one derived artifact read.
//
// Edges deduplicate, so a loop reading one declaration a thousand
// times records one edge. Nothing consumes a ReadSet yet; the
// engine invalidates along these edges.
type ReadSet struct{ ... }

func (s *ReadSet) Identities() iter.Seq[symbol.Identity]
func (s *ReadSet) Kinds() iter.Seq[symbol.Kind]
func (s *ReadSet) Len() int
```

### Concurrency

Frontends run concurrently across units, so the writes arrive
concurrently. The graph owns that rather than the caller: a
correctness rule stated in prose is a rule a frontend author can
miss, and the failure it produces is a torn index rather than an
error.

| Phase | What runs in parallel | What the graph does |
|---|---|---|
| Load | frontends, per unit | serializes writes per package |
| Annotate | one bucket at a time; parallel inside a bucket is a workspace opt-in | refuses structural writes; reads are concurrent |
| Generate | plans against each other | reads only, through per-plan readers |

A `ReadSet` belongs to one derived artifact and is not shared, so a
reader is not safe for concurrent use even though the graph beneath
it is. That is the honest split: the graph is shared and the
bookkeeping is not.

### Freeze

Loading and annotating are different phases, and the difference is
enforced rather than documented: after Freeze the graph refuses any
write that adds or removes a declaration, answering a registered
kernel code. Metadata writes are not structural and stay legal,
which is the entire point of the seal.

The kind index builds at Freeze, where it is free: nothing may add a
declaration afterwards, so the index cannot go stale. The directive
index builds in the same pass once directives exist, for the same
reason and at the same moment.

## Alternatives considered

### A reader that answers slices rather than iterators

`ByKind` returning `[]symbol.Symbol` is simpler to consume. It lost
because the enumeration would have to record a per-identity edge for
every declaration of that kind, whether the caller looked at it or
not, which prices the read at the size of the set rather than at
what it observed. An iterator records as the caller advances.

### Recording reads outside the reader

A caller could record its own edges, which would let a hot path skip
the bookkeeping. It lost because an unrecorded read is invisible to
invalidation, and the failure it produces is output that is stale
and looks current. One door, always tracked, is what makes the
completeness structural.

### Panicking on a duplicate code

Registration could panic, since a duplicate is a programming error.
It lost because Build collects every fault in one pass so a consumer
fixing a composition gets the whole bill; a panic reports one.

## Drawbacks

- The reader records edges nothing reads yet, so the bookkeeping is
  cost without benefit until the engine lands. It is a map insert
  per distinct read, and the alternative is changing every call site
  later.
- The graph carries the lock every write pays for, including the
  single-threaded fixture case that never contends. The alternative
  puts the rule in every frontend, where one of them will forget it.
- Codes are API from the first registration, so a code whose meaning
  turns out wrong cannot be edited, only superseded.

## Open questions

None. The three this RFC opened are settled above and below: the
reader carries its scope, the directive index waits for directives,
and the read set gains its metadata grain with the metadata that
produces those reads.

## Unresolved and future work

- Interning identities and packing read-set edges as ID pairs is a
  performance commitment the engine makes; the shape here is the
  boundary form.
- The ledger owns `Track`, which is what hands a reader to a phase.
  Until it exists, a caller makes a reader from the graph directly.
- `ByDirective` and the directive index arrive with the directive
  chunk, which owns the registry that types a directive name. They
  build at Freeze in the same pass as the kind index.
- `ReadSet` gains the per-key metadata grain with the metadata
  chunk. A metadata read records at (declaration, key), which is
  finer than either grain here, and nothing produces one yet.

## References

- [02-symbol-model.md](../architecture/02-symbol-model.md), the
  reader's surface and the grain rule
- [09-incrementality.md](../architecture/09-incrementality.md), what
  the recorded edges are for
- [16-diagnostics.md](../architecture/16-diagnostics.md), the
  diagnostic record and the severity contract
- [08-workspace-and-plans.md](../architecture/08-workspace-and-plans.md),
  freeze and the phase order
- [RFC-0001](0001-symbol-model-contract.md), the declarations the
  graph holds
