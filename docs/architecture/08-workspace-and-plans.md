# Workspaces and plans

*Builds on: [06](06-plugins.md), [07](07-rendering.md). Feeds:
[09](09-incrementality.md) (the engine runs this frame),
[10](10-cross-language.md) (exports),
[17](17-output-and-determinism.md) (manifest and sweep),
[18](18-routing-and-layout.md) (layout per plan).*

Rendering one language needs exactly one answer for import
resolution, formatting and layout, so one backend per rendering pass
is worth keeping as an invariant.

But a project is rarely one rendering pass. A schema becomes a Go
server and a TypeScript client. A monorepo generates Go and Python
side by side. Run an independent tool per language and you parse the
same source N times, you have nowhere to check that the languages
agree, and each tool tracks its output without knowing about the
others. Delete a configuration under that arrangement and its
generated files stay on disk forever, with nothing tracking them.

The frame that resolves this: **one workspace, N plans, one backend
per plan.**

## The model

```
Load ─► Link ─► Freeze ─► Annotate ─► Plan "go-server"  ─► Generate ► Layout ► Render ► Sink ─┐
(N frontends,  (spellings  (buckets, ├► Plan "ts-client" ─► ...                               ├─► Close
 parallel,      resolve to  tracked  └► Plan "docs"      ─► ...      (plans parallel,         │
 fingerprinted) identities) reads)                                    topo on exports)        │
                                        merged manifest · collision check · sweep · checks ───┘
```

Link is the resolution step
([02-symbol-model.md](02-symbol-model.md)). After every frontend
finishes, type spellings resolve once into canonical identities,
which is what makes cross-file and cross-package lookup a plain join
for every later phase.

A **workspace** holds N frontends, the annotator set, N named plans,
the output, the cache, the diagnostic sink and the config. The
consumer's binary builds it, through the fluent builder or through
YAML onto the same typed config structs. The output is a factory
that opens a fresh sink. A sink serves one staging, so a run opens
one for each plan it commits, and no plan has a sink of its own.

A **plan** is one write side: a generator set, a layout policy,
exactly one backend, a source scope, the plans it depends on and a
name. Plans are values, meaning declarable data that can be exported
as presets, rather than plugins ([06-plugins.md](06-plugins.md)).

**Freeze is enforced.** After Annotate the store seals, and a
structural write is refused with a stable code.

**Plans are isolated.** One plan failing does not abort its
siblings. The workspace joins the errors and reports status per
plan, and a plan's manifest slice commits only when that plan
succeeds. A plan that depends on a failed plan commits nothing
either.

**Outputs are never inputs.** A workspace does not read its own
generated files as source. Load excludes every file whose provenance
trailer names the workspace's brand
([17-output-and-determinism.md](17-output-and-determinism.md)), at
the fingerprint gate, before anything parses.

Without that rule, the second run parses the first run's output,
bare rules fire on generated symbols, and warm≡cold collapses into
run N differing from run N+1. No consumer could write the exclusion
themselves either, because the scope vocabulary below has no
negation. Files another tool generated stay ordinary input: parsed
and classified, never excluded
([11-languages.md](11-languages.md)). Only the loop back to itself
is closed.

## The runtime: Workspace and Run

The frame above is the semantics. This is the machine. One split
carries the weight: **the composition is immutable, and the run owns
everything mutable.**

```
Workspace (built once, survives forever)
├─ composition   plugins (lowered), plans (values), config
├─ registries    schemas, meta keys, diag codes, capabilities, languages
├─ interner A    composition-static names → dense IDs
└─ dispatch plan per phase, per bucket: gate→handler tables — COMPILED

Run (per invocation; holds the workspace lock)
├─ state         the sealed state over the ledger's blobs: the gate,
│                read records, the dirty set, open and commit
├─ graph         node symbols · freeze · kind + directive indexes
├─ facts         bags · fact-key index · stamp arbitration
├─ interner B    run-local symbol IDs
├─ dispatcher    stateless executor of the compiled plan
├─ plans[i]      emit store · layout · render · staged sink ·
│                manifest slice · export     (isolated, parallel)
└─ diag          the one collector
```

Four rules become structure rather than discipline.

Plugins are stateless values, because there is no run state outside
`Run` to hide anything in. `--watch` is many Runs over one
Workspace.

**The dispatch plan compiles at Build**, the way a query plan does,
because compile-time plugins make every subscription known at Build.
The run-time dispatcher is then a stateless executor over static
tables.

Those same tables are the sealed state's invalidation metadata, so
dispatch and invalidation cannot disagree with each other.

The conductor is a function rather than a state machine: `Run()`
reads as straight-line code.

The seams, as contracts:

```go
type State interface {                          // the sealed state, over the ledger's blobs
    Open(ctx context.Context) (*RunPlan, error) // header check + stat-first sweep
    Groups(p PlanID) (dirty []GroupID, kept []manifest.Entry)
    Commit(ctx context.Context) error           // AFTER sinks commit
}
type RunPlan struct {
    ParseUnits []UnitWork // (unit, Depth) to parse, or to restore from the memo
    KeptUnits  []UnitRef  // regions the generation keeps, decoded on first read
}
type Graph interface {
    AddUnit(u ParsedUnit) error                    // Load only
    Link(frontier []UnitRef) (dirty SymbolSet, err error)
    Freeze()                                       // builds directive index
    Reader(reads *ReadSet, sc Scope) (Reader, error) // the only read path
    ByKind(symbol.Kind) SubjectSet
    ByDirective(name string) SubjectSet
    Lookup(symbol.ID) (Symbol, bool)
}
type Facts interface {
    Stamp(sym symbol.ID, k KeyID, v Value, at Authority, by PluginID)
    Withdraw(k KeyID, c Claim) error               // a claim whose match runs again or disappeared
    ByKey(KeyID) SubjectSet
    Of(symbol.ID) TrackedBag
}
type EmitStore interface {                 // one per plan
    Add(v emit.Node) error                 // Generate only; frozen at Layout
    ByTarget() iter.Seq2[Target, []emit.Node]
    AppendSlot(host emit.Node, s SlotName, v emit.Node, p Provenance) error
}
type Dispatcher struct{ tables PhaseTables }       // Build's output
// One call serves cold (sel nil: every match) and warm (sel: the
// recorded matches of dirty groups, plus the candidate subjects whose
// matches may have changed). It yields matches in canonical order,
// (bucket, plugin, rule, subject identity, instance, host), and
// journals what each invocation read and touched.
func (d *Dispatcher) MatchesOver(ph Phase, bucket int, sel *Selection, j Journal) iter.Seq[MatchWork]
type PlanExec interface {
    Generate(dirty []GroupID, d *Dispatcher, g Graph, f Facts) error
    LayoutAndRender(ctx context.Context) ([]StagedFile, error)
    Stage([]StagedFile) error   // temp files, not yet visible
    Export() (ExportDoc, error)
    Commit() ([]manifest.Entry, error) // atomic renames + slice
}
```

**The sealed state** is the persisted record: the graph's regions,
the fact store, the read records, the artifact and group tables and
the plans' name entries, written one generation at a time as
immutable segments, with a swap of `CURRENT` on success
([09-incrementality.md](09-incrementality.md)). Facts persist because
they have to. Bags discarded with the run would force full
re-annotation on every warm run, which is O(subjects) against the
performance target. Symbol IDs are local to a run. Canonical
identities are the stored form, spelled in each region's string table
and hashed in every edge.

**The commit protocol is two-phase, and its behaviour under a crash
is stated.** Every plan stages. A plan's `Commit`, meaning its
renames plus its manifest slice, runs only when that plan succeeds.
The state's `Commit` runs last, after every sink. So the engine's
memory never records an output that disk does not hold, and a crash
between the two fails conservatively: derive again, write identical
bytes, report `Unchanged`.

**The score**, meaning who runs what, in order:

```
Workspace.Run (the only conductor; cli `run` calls it, main calls cli)
├─ state.Open          header check, stat-first sweep → RunPlan
├─ Load                kit units, parallel, the memo first, and kept
│                      regions decode on first read
├─ Link                changed units, and the references whose
│                      candidates moved, from the recorded tiers
├─ Freeze              directive index built here, free
├─ Annotate            dispatcher over the matches the dirty set meets,
│                      early cutoff against persisted fact values
├─ per plan (topo on exports, else parallel):
│    Generate          dirty groups only, every contributing invocation,
│                      settled against the clean groups' name entries
│    Layout → Render (kit, per-file ∥)
├─ per plan, after every render:
│    Stage → Prepare   staged files and stale removals, then what
│                      each destination path contains
├─ Close               manifest merge, collisions, sweep, audit,
│                      checks (over records)
├─ plan Commits        dependency order, then state.Commit
└─ report
```

Concurrency per phase: phases are barriers. Load shards per unit,
with graph writes serialized per package. Annotate and each plan's
Generate run bucket-sequentially. Parallelism inside a bucket is the
workspace's opt-in, `Builder.Parallel`: one plugin's phase call runs
its matches on up to the worker count, and the dispatcher applies
every placement, slot append and finding of its matches in canonical
match order, so the count does not change the output
([06b-authoring.md](06b-authoring.md)). A stamp is not buffered,
since bag arbitration is already deterministic. Plans run in
parallel, except that a plan generates after every plan it depends
on has rendered. Render runs per file inside the kit. Close runs
single-threaded over immutable records. The plans commit one after
another, each after every plan it depends on. A dry run is this same
Run with the staging discarded.

## Build: the validation sequence

Build runs once, in the consumer's `main`, and produces the
immutable Workspace above, or an error naming *everything* that is
wrong at once. Build collects rather than stopping at the first
fault, because a consumer fixing a composition wants every fault at once
rather than an instalment plan. A Build that succeeds has resolved
every human-typed name in the composition, so nothing after it can
fail on a name.

The steps, in order, where each assumes the ones before it:

1. **Registries.** Language identities, metadata keys with their
   namespaces, groups, kinds and contracts, diagnostic codes and
   prefixes, capability labels, policy keys, and directive schemas.
   Each plugin registers through a handle bound to its name, so it
   registers keys only into namespaces it claimed and schemas only
   under its own name. A schema name registered twice is refused, and
   a name that two plugins register is ambiguous under its bare
   spelling.
2. **Plugin lowering.** Authoring-surface values lower to the SPI
   ([06b-authoring.md](06b-authoring.md)), subscriptions are
   collected as data, and per-role priorities and capability
   topology sort. A cycle is an error naming the bucket and its
   members.
3. **Options.** Every plugin's options schema populates from its
   config section, and a failure names the plugin and the field.
4. **Plans.** Each plan has exactly one backend and a registered
   target, and every template-claiming plugin declares that target.
   Layout refinements name plugins and tags that exist. The sources
   name a language that a frontend loads or a rules value declares,
   and patterns that name workspace directories. The dependencies
   name plans the composition declares and sort topologically, and a
   cycle is one fault naming every plan in it. Each workspace check
   reads plans the composition declares.
5. **Policies.** Selections validate against registered keys and
   choices, and each plan's total `Policy` is fixed here
   ([10-cross-language.md](10-cross-language.md)).
6. **The dispatch plan.** Subscriptions compile into the per-phase,
   per-bucket gate-to-handler tables the Run's dispatcher executes.
7. **The handshake.** One contract version, checked across every
   registered component
   ([15-compatibility.md](15-compatibility.md)).

The builder and the values it composes, pinned:

```go
func New() *Builder
func (b *Builder) Brand(brand output.Brand) *Builder // required: carriers, config, state, trailers
func (b *Builder) Output(open func() (output.Sink, error)) *Builder // a fresh sink per plan a run commits
func (b *Builder) Ledger(open func() (ledger.Ledger, error)) *Builder // the previous record, and this run's
func (b *Builder) Memo(m Memo) *Builder // the parse memo's cap and its ledger, none by default
func (b *Builder) Workspace(id string) *Builder // the manifest's name; the root's base name when empty
func (b *Builder) Parallel(workers int) *Builder // matches a phase call runs at once; 0 and 1 run sequentially
func (b *Builder) Frontends(fs ...plugin.Frontend) *Builder
func (b *Builder) Annotators(as ...plugin.Annotator) *Builder
func (b *Builder) Checks(cs ...plugin.WorkspaceCheck) *Builder // Close runs them in registration order
func (b *Builder) Plans(ps ...Plan) *Builder
func (b *Builder) Rules(rs ...rules.SourceRules) *Builder // one value per language; absent otherwise
func (b *Builder) Config(c Config) *Builder   // same structs the YAML maps onto
func (b *Builder) Build() (*Workspace, error) // collect-all; see above

type Plan struct {                // a value, not a plugin (D8)
    Name       string
    Sources    Sources            // what the generators see; the zero value admits everything
    DependsOn  []string           // the plans whose exports it reads: the topo edges
    Generators []plugin.Generator
    Backend    plugin.Backend     // exactly one, its target registered
    Layout     layout.Config
}

type Sources struct {             // conjunctive: every field that is set matches
    Lang     symbol.Lang          // the packages of one source language
    Packages []string             // workspace directories: ./svc, and ./svc/... below it
    Module   string               // the neutral gen.module fact
}

type ExportDoc struct {           // the versioned kernel schema of D99
    Plan    string
    Symbols []ExportedSymbol      // sorted by key, then by file
}
type ExportKey struct {           // what a dependent knows before the producer runs
    Origin symbol.Identity        // the declaration it derives from
    Plugin plugin.ID              // the plugin that emitted it
    Tag    string                 // the family it was emitted into
    Host   string                 // the emitted name of the declaration it is a member of
    Name   string                 // the name it declared, before respell
}
type ExportedSymbol struct {
    ExportKey
    Kind     symbol.Kind
    Spelling string               // what the producing plan's settle named it
    Package  symbol.Identity      // the file's package: import path and clause name
    File     string               // where it arrived
}
func (d ExportDoc) Find(k ExportKey) []ExportedSymbol // one binary search, no allocation
```

## Source scopes: mixed monorepos

`Plan.Sources` filters which source packages a plan's generators
see, because the Reader is scope-filtered. The read side is the
**union**: each frontend parses its files once, however many plans
read them.

```yaml
workspace:
  scope: ["./..."]
plans:
  - {name: go-services, sources: {lang: golang}, target: golang}
  - {name: go-mocks,    sources: {lang: golang}, target: golang}   # 2nd Go plan, different generators
  - {name: py-services, sources: {lang: python}, target: python}
  - {name: py-clients,  sources: {lang: golang}, target: python}   # cross-language plan
```

Several plans targeting one language, with different generator sets
and layouts, fall out of that for free. Annotators run once
over the union graph, because facts are per source language anyway
and detection dispatches per language.

The predicate vocabulary is closed and conjunctive: three optional
fields, and a package matches when every field that is present
matches.

```yaml
sources:
  lang: golang            # source language identity
  packages: ["./svc/..."] # workspace directories; a trailing /... adds every one below
  module: "billing"       # toolchain-module identity (the neutral gen.module fact)
```

Nothing else. No negation, and no unions of predicates. A plan that
needs another plugin scope is two plans. Growing the vocabulary is a
kernel change, additive under the compatibility policy. `golang`
here is the human spelling, resolved against the language registry
at Build, and a plan composed in Go uses the satellite's exported
identity ([03-projection.md](03-projection.md)).

A pattern is a workspace-relative, slash-separated directory with an
optional leading `./`, and a trailing `/...` extends it to every
directory below. `.`, `...` and `./...` name the whole tree. Build
refuses a language that no frontend loads and no rules value
declares, and a pattern that names no directory of a workspace tree:
an empty one, a `..` element, a backslash, an absolute path, and
`...` anywhere but at the end.

A package matches `packages` only when every one of its files is in
a named directory, so a scope admits a package whole or not at all.
A package that a store provides, such as the standard library's, is
in no workspace directory, so no pattern admits it. `module` admits
such a package only where its own `gen.module` fact names the module,
and a plan scoped by `lang` alone matches the store's packages of its
language, as an unscoped plan matches every package. Each run binds a
plan's sources to the frozen graph before the plan generates, and the
composition's fingerprint folds them.

## Plan exports: the one declared edge between plans

Reading a sibling plan's emit directly is forbidden. With N plans it
creates ordering constraints the workspace cannot infer, destroys
plan parallelism everywhere, and entangles the incrementality graphs
through edges nobody declared.

But one family of generators genuinely needs cross-plan knowledge:
**binding generators**, covering Rust-to-Python FFI, JNI, cgo and
wasm bindings. They must spell the names and signatures a sibling
plan generated, and working those out again by applying the same
naming rules and hoping drifts by construction.

So the coupling exists only in its explicit form. A plan may
**declare a dependency** on other plans by name. It generates after
each of them has rendered, reads their **exports**, and commits only
where each of them commits. An export is a typed, deterministic
summary of what a plan rendered: a frozen value rather than a window
into its emit graph. Plans sort topologically on their declared
dependencies, a cycle is a Build error naming every plan in it, plans
that declare nothing run fully in parallel, and the incrementality
engine gets a real edge instead of a hidden one.

The export is a versioned document under a kernel schema, and it is
public API, because binding correctness leans on it
([15-compatibility.md](15-compatibility.md)). Per exported symbol it
records the key a dependent joins on: the declaration it derives
from, the plugin that emitted it, the family it was emitted into,
the emitted name of the declaration it is a member of, and the name
that plugin declared before the settle respelled it. The key needs
every part. Two plugins can each emit a declaration from one source
declaration, and a lowering can derive declarations beside the
principal one. One generator can emit one name into two families,
and a stub and a mock of one interface give one method to two
receivers. The export also records the kind and **the spelling the
plan's settle chose**, with the package, meaning the import path and
the name of its package clause, and the file it arrived in.

The export records no signature. A generated declaration's types are
emit references, which no projection reads, so the kernel cannot
state them in TypeShape terms ([03-projection.md](03-projection.md)).
A binding generator that needs a declaration's type reads its
origin's shape through its own reader, which needs the origin's
package in its plan's scope.

A dependent reads spellings from the export rather than recomputing
naming conventions, which is the entire point: the producing plan's
settle is the only authority on what it named things. A dependent
invocation records each export it read, so an export that did not
change causes nobody to run again, and one that did re-runs exactly
the invocations that read it. A dependent that implements its role
directly reads every export its context hands over, so any changed
export runs it again.

A dependent never commits a file that refers to a declaration its
producer did not commit. Where a producer fails, its dependents
generate nothing and report a `FailedDependency` Info at the
producer's first Error, and their previous files and entries remain.

Considered and refused: **per-plan model transforms**, meaning
filtering or renaming the shared graph per plan before generation.
Transforms fork the one graph into N variants, and then everything
downstream has to ask "which variant" first, including explain,
invalidation edges, and cross-plan checks trying to compare like
with like. Every need a transform would serve already has a home:
filtering is a `Sources` scope, facts are annotators over the one
graph, and per-target spellings are lowering policy.

## Close

Close runs after every plan has staged, in four steps, on one
goroutine.

It merges the **workspace manifest**, recording plan to files, and
the state's commit writes the documents whose entries changed after
the plans commit. The record is what
stops a deleted plan leaving its generated files behind: a workspace
that no longer declares a plan deletes that plan's files.

It runs **path-collision detection** across the plan manifests,
producing a stable error rather than letting the last writer win.

It runs the **staleness sweep**, scoped per plan, because
"everything this plan did not produce is stale" is only true within
one plan's scope. It is also scoped per run: under narrowed patterns
([20-cli.md](20-cli.md)), a manifested file is deleted only when its
sources fall inside the narrowed scope and no longer produce it, or
when its producing plan or plugin left the composition.
Out-of-scope entries carry forward untouched, because a partial run
must not delete outputs it merely did not look at. Reconciling the
whole workspace is `prune`'s job.

**Audit mode** verifies the metadata completeness contracts
([04-metadata.md](04-metadata.md)).

Last, it runs the **cross-plan checks**, one after another in
registration order. Each `WorkspaceCheck` names the plans it reads
and reads their *records*, meaning their manifest entries and their
exports, with the graph and the facts, and never raw emit, which on a
warm run does not exist for a clean plan. So a check that runs reads
identical records cold and warm, the records of kept files included.
A warm run calls a check again only when a plan it reads rendered,
added or removed a file, when such a plan's export changed, or when
the check's read record meets the dirty set. Otherwise the run
reports the check's recorded findings. A check
that reads a failed plan does not run, and the run reports one
`FailedDependency` Info for it at the plan's first Error, because the
missing output is already that plan's Error and does not need
re-litigating once per check. A check's Error blocks every commit of
the run, as a collision does, because a claim across plans names no
plan at fault. Close runs no check after an Error in a phase every
plan shares, because the records of such a run are incomplete.

## Repositories with several workspaces

Every anticipated language has its own notion of a multi-project
build: `go.work`, Cargo workspaces, Maven and Gradle multi-module,
pnpm, uv. The rule, decided once:

**An eidos workspace is a config boundary rather than a toolchain
boundary.** One workspace deliberately spans N toolchain modules,
with one graph, one manifest and one sweep.

Frontends own toolchain-workspace resolution and report module
identity through the kernel-owned neutral keys `gen.module` and
`gen.moduleRoot`, which every frontend spells the same way. The raw
toolchain form, a go.mod path or a Maven artifact, stays in the
frontend's own namespace, which is the same normalize-plus-raw split
Visibility uses. Layout policies and Sources predicates match only
the neutral keys, which is what keeps module-aware kernel machinery
free of per-language tables. Generated files import correctly across
modules because the emit import machinery already qualifies by
package path.

A partial run is a scope, such as `./services/...`, rather than a
separate workspace. Per-module differences are plans with
directory-scoped Sources, rather than a nested config dialect.

A repository holding genuinely unrelated projects declares several
workspaces **explicitly, in one place**:

```yaml
workspaces:
  - {root: ./platform,  config: ./platform/.mygen.yaml}
  - {root: ./tools/gen, config: ./tools/gen/.mygen.yaml}
```

A config that declares `workspaces` declares nothing else. The list
is the whole file, so nobody has to ask which workspace a stray
top-level field belongs to.

Each workspace of the list loads only the tree under its root, writes
through a sink at its root that refuses a path with a `..` element,
and records its manifest in the state directory under its root, so
two workspaces over sibling roots share no file. The reader of the
list refuses two roots that nest. The outer root's load would read
the inner root's sources, and its plans would generate files inside
the inner root.

Nested configs are never implicitly independent workspaces, because
two things that each own a manifest and do not know about each other
is exactly how generated files end up orphaned. Nothing crosses a
workspace boundary: no facts, no exports, no checks. Two projects
that need each other's anything are telling you they are one
workspace with two plans, and the architecture offers nothing weaker.

## Config

The typed groups the YAML maps onto one-to-one, with a published
JSON Schema for editor completion, where the fluent builder is the
Go-native equal: `Identity` for brand and workspace ID, `Scope`,
`Cache` for the parse memo's size cap, where zero keeps no memo, and
the directory of its entries
([09-incrementality.md](09-incrementality.md)), and `Plans
[]PlanConfig{Name, Sources, Target, Generators, Layout, DependsOn}`.

`DryRun` resolves the full multi-plan picture, meaning buckets,
topological order, layouts and export edges, without executing
anything.

The workspace ID names the workspace in manifests and in
multi-workspace diagnostics, because merged CI output from a
`workspaces:` list has to say which workspace spoke. It defaults to
the root directory's name.
