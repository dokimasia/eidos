# Workspaces and plans

*Builds on: [06](06-plugins.md), [07](07-rendering.md). Feeds:
[09](09-incrementality.md) (the engine runs this frame),
[10](10-cross-language.md) (exports),
[17](17-output-and-determinism.md) (manifest and sweep),
[18](18-routing-and-layout.md) (layout per plan).*

The composition frame. Rendering a language needs exactly one answer
for import resolution, formatting, and layout — so one backend per
rendering pass is an invariant worth keeping. But a project is
rarely one rendering pass: a schema becomes a Go server and a TS
client; a monorepo generates Go and Python side by side. Running
independent tools per language means parsing the same source N
times, no place to check cross-language consistency, and output
trackers that don't know about each other — the orphan shape, where
a deleted configuration leaves its generated files behind forever.

The frame that resolves this: **one workspace, N plans; one backend
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
([02-symbol-model.md](02-symbol-model.md)): after every frontend
finishes, type spellings resolve once to canonical identities,
which is what makes cross-file and cross-package lookup a plain
join for every later phase.

- **Workspace**: N frontends, the annotator set, N named plans, the
  cache, the diag sink, config. Built by the consumer's binary via
  the fluent builder or YAML onto the same typed config structs.
- **Plan**: one write side — a generator set, a layout policy,
  exactly one backend, one sink, a **source scope**, a name. Plans
  are *values* (declarable data, exportable as presets), not plugins
  ([06-plugins.md](06-plugins.md)).
- **Freeze** is enforced: after Annotate the store seals; structural
  writes are refused with a stable code.
- **Isolation**: a plan failing doesn't abort siblings; the
  workspace joins errors with per-plan status; a plan's manifest
  slice commits only on that plan's success.
- **Outputs are never inputs.** A workspace does not read its own
  generated files as source. Load excludes every path the
  workspace can prove it owns — a manifest entry, or the
  provenance trailer
  ([17-output-and-determinism.md](17-output-and-determinism.md)) —
  at the fingerprint gate, before any parse. Without the law the
  second run parses the first run's output, bare rules fire on
  generated symbols, and warm≡cold degenerates into
  run-N ≠ run-N+1; and no consumer could express the exclusion
  themselves, because the scope vocabulary below has no negation.
  Files another tool generated are ordinary input — parsed and
  classified, never excluded
  ([11-languages.md](11-languages.md)); only the self-loop is
  closed.

## The runtime architecture: Workspace and Run

The frame above is semantics; this is the machine. One load-bearing
split: **the composition is immutable, the run owns every mutable
thing.**

```
Workspace (built once, survives forever)
├─ composition   plugins (lowered), plans (values), config
├─ registries    schemas, meta keys, diag codes, capabilities, languages
├─ interner A    composition-static names → dense IDs
└─ dispatch plan per phase, per bucket: gate→handler tables — COMPILED

Run (per invocation; holds the workspace lock)
├─ ledger        the engine's run side: fingerprints, read-sets,
│                dirty work-lists, sealed-state open/commit
├─ graph         node symbols · freeze · kind + directive indexes
├─ facts         bags · fact-key index · stamp arbitration
├─ interner B    run-local symbol IDs
├─ dispatcher    stateless executor of the compiled plan
├─ plans[i]      emit store · layout · render · staged sink ·
│                manifest slice · export     (isolated, parallel)
└─ diag          the one collector
```

Four laws become structure instead of discipline: plugins are
stateless values (there is no run state outside `Run` to smuggle
anything into — `--watch` is many Runs over one Workspace); the
**dispatch plan is compiled at Build** like a query plan, since
compile-time plugins make every subscription known at Build — the
run-time dispatcher is a stateless executor over static tables; the
same tables are the ledger's invalidation metadata, so dispatch and
invalidation cannot disagree; and the conductor is a **function,
not a state machine** — `Run()` is the score as straight-line code.

The seams, as contracts:

```go
type Ledger interface {
    BeginRun(ctx context.Context) (*RunPlan, error) // stat-first sweep +
                                                    // sealed-state open
    Track(r Reader, at ArtifactID) Reader           // the only read path
    Artifacts(p PlanID) (dirty []ArtifactID, carried []manifest.Entry)
    CommitRun(ctx context.Context) error            // AFTER sinks commit
}
type RunPlan struct {
    ParseUnits []UnitWork // (unit, Depth) to (re)parse
    CarryUnits []UnitRef  // graph regions opened from sealed state
}
type Graph interface {
    AddUnit(u ParsedUnit) error                    // Load only
    Link(frontier []UnitRef) (dirty SymbolSet, err error)
    Freeze()                                       // builds directive index
    ByKind(symbol.Kind) SubjectSet
    ByDirective(name string) SubjectSet
    Lookup(symbol.ID) (Symbol, bool)
}
type Facts interface {
    Stamp(sym symbol.ID, k KeyID, v Value, at Authority, by PluginID)
    ByKey(KeyID) SubjectSet
    Of(symbol.ID) TrackedBag
}
type EmitStore interface {                 // one per plan
    Add(v emit.Node) error                 // Generate only; frozen at Layout
    ByTarget() iter.Seq2[Target, []emit.Node]
    AppendSlot(host emit.Node, s SlotName, v emit.Node, p Provenance) error
}
type Dispatcher struct{ tables PhaseTables }       // Build's output
// One call serves cold (subjects=all) and warm (subjects=an
// artifact's contributor set); yields matches in canonical order:
// (bucket, plugin, rule, subject identity, instance).
func (d *Dispatcher) MatchesOver(ph Phase, bucket int, s SubjectSet) iter.Seq[MatchWork]
type PlanExec interface {
    Generate(dirty []ArtifactID, d *Dispatcher, g Graph, f Facts) error
    LayoutAndRender(ctx context.Context) ([]StagedFile, error)
    Stage([]StagedFile) error   // temp files, not yet visible
    Export() (ExportDoc, error)
    Commit() ([]manifest.Entry, error) // atomic renames + slice
}
```

**The sealed state** is the persisted trio — sealed graph, fact
store, artifact table — written generationally (N+1 beside N, swap
on success). Facts persist because they must: bags that die with the
run would force full re-annotation every warm run, O(subjects)
against the envelope. Symbol IDs are generation-local; canonical
identities are the stored form.

**The commit protocol is two-phase and its crash posture is
stated**: every plan stages; a plan's `Commit` (renames + manifest
slice) runs only on its success; `Ledger.CommitRun` runs last, after
all sinks — the engine's memory never records an output disk does
not hold, and a crash between the two fails conservative:
re-derive, rewrite identical bytes, `Unchanged`.

**The score** — who runs everything, in order:

```
Workspace.Run (the only conductor; cli `run` calls it, main calls cli)
├─ ledger.BeginRun     stat-first sweep → RunPlan
├─ Load                kit units, parallel; carried regions opened
├─ Link                dirty frontier from the spelling diff
├─ Freeze              directive index built here, free
├─ Annotate            dispatcher over frontier-intersecting gates;
│                      early cutoff against persisted fact values
├─ per plan (topo on exports, else parallel):
│    Generate          dirty artifacts only, whole contributor sets
│    Layout → Render (kit, per-file ∥) → Stage
├─ Close               manifest merge, collisions, sweep, checks
│                      (over records), audit
├─ plan Commits        then ledger.CommitRun
└─ report
```

Concurrency, per phase: phases are barriers; Load shards per unit
with per-package graph writes; Annotate is bucket-sequential with
opt-in parallelism inside a bucket (bag arbitration is already
deterministic); plans parallel except export edges; render per-file
inside the kit; Close single-threaded over immutable records.
Dry-run is this same Run with staging discarded.

## Build: the validation ladder

Build runs once, in the consumer's `main`, and produces the
immutable Workspace above — or an error that names *everything*
wrong at once. Build collects; it never stops at the first fault,
because a consumer fixing a composition wants the whole bill, not
an installment plan. A Build that returns success has resolved
every boundary string in the composition; nothing after it can
fail on a name.

The ladder, in order — each step assumes the ones before it:

1. **Registries.** Language identities, metadata keys (namespaces,
   groups, kinds, contracts), diagnostic codes and prefixes,
   capability labels, policy keys, and directive schemas —
   identical schema redeclarations unify, conflicting ones are
   refused naming both plugins.
2. **Plugin lowering.** Authoring-surface values lower to the SPI
   ([06b-authoring.md](06b-authoring.md)); subscriptions are
   collected as data; per-role priorities and capability topology
   sort — a cycle is an error naming the bucket and its members.
3. **Options.** Every plugin's options schema populates from its
   config section; a failure names the plugin and the field.
4. **Plans.** Per plan: exactly one backend and a registered
   target; every template-claiming plugin declares that target;
   layout refinements name known plugins and tags; Sources
   predicates resolve against the registries; export dependencies
   topo-sort — a cycle names both plans.
5. **Policies.** Selections validate against registered keys and
   choices; each plan's total `Policy` is fixed here
   ([10-cross-language.md](10-cross-language.md)).
6. **The dispatch plan.** Subscriptions compile into the
   per-phase, per-bucket gate→handler tables the Run's dispatcher
   executes.
7. **The handshake.** One contract version checked across every
   registered component ([15-compatibility.md](15-compatibility.md)).

The builder and the two values it composes, pinned:

```go
func New() *Builder
func (b *Builder) Frontends(fs ...plugin.Frontend) *Builder
func (b *Builder) Annotators(as ...plugin.Annotator) *Builder
func (b *Builder) Checks(cs ...plugin.WorkspaceCheck) *Builder
func (b *Builder) Plans(ps ...Plan) *Builder
func (b *Builder) Config(c Config) *Builder   // same structs the YAML maps onto
func (b *Builder) Build() (*Workspace, error) // collect-all; see above

type Plan struct {                // a value, not a plugin (D8)
    Name       string
    Sources    Sources            // lang / packages / module — conjunctive
    Target     rules.Target
    Generators []plugin.Generator
    Backend    plugin.Backend
    Layout     LayoutConfig
    Sink       sink.Sink
    Exports    []ExportName       // what it publishes
    DependsOn  []ExportName       // the topo edges
}

type ExportDoc struct {           // the versioned kernel schema of D34
    Plan    string
    Symbols []ExportedSymbol
}
type ExportedSymbol struct {
    Kind      symbol.Kind
    Signature TypeSig             // canonical TypeShape terms
    Spelling  string              // what the producing lowering named it
    Import    string              // the path a dependent qualifies with
    File      string              // where it landed
}
```

## Source scopes: mixed monorepos

`Plan.Sources` filters which source packages a plan's generators see
(the Reader is scope-filtered). The read side is the **union** —
each frontend parses its files once, however many plans read them:

```yaml
workspace:
  scope: ["./..."]
plans:
  - {name: go-services, sources: {lang: golang}, target: golang}
  - {name: go-mocks,    sources: {lang: golang}, target: golang}   # 2nd Go plan, different generators
  - {name: py-services, sources: {lang: python}, target: python}
  - {name: py-clients,  sources: {lang: golang}, target: python}   # cross-language plan
```

Multiple same-target plans (different generator sets, layouts,
sinks) fall out for free. Annotators run once on the union graph —
facts are per-source-language anyway, and detection dispatches per
language.

The predicate vocabulary is closed and conjunctive — three fields,
all optional, a package matches when every present field matches:

```yaml
sources:
  lang: golang            # source language identity
  packages: ["./svc/..."] # workspace-relative path globs
  module: "billing"       # toolchain-module identity (the neutral gen.module fact)
```

Nothing else — no negation, no unions of predicates. A plan needing
a stranger scope is two plans. Growing the vocabulary is a kernel
change, additive under the compatibility policy. `golang` here is
boundary spelling, resolved against the language registry at Build;
a plan composed in Go uses the satellite's exported identity
([03-projection.md](03-projection.md)).

## Plan exports: the declared cross-plan edge

Raw sibling-emit reads are forbidden: with N plans they create order
constraints the workspace cannot infer, destroy plan parallelism
globally, and entangle the incrementality graphs through edges
nobody declared. But one legitimate family needs cross-plan
knowledge — **binding generators** (Rust↔Python FFI, JNI, cgo, wasm
bindings) — which must spell the names and signatures a sibling plan
generated, and recomputing them by convention ("apply the same
naming rules and hope") is drift-by-construction.

So the coupling exists only in its explicit form:

- A plan may **publish an export**: a typed, deterministic summary
  of what it generated. A frozen value, never a window into its emit
  graph.
- A plan may **declare a dependency** on a named export. Plans
  topo-sort on declared dependencies; a cycle is a Build error
  naming both plans; undeclared plans stay fully parallel; the
  incrementality engine gets a real edge instead of a hidden one.

The export itself is a versioned document under a kernel schema —
public API, since binding correctness leans on it
([15-compatibility.md](15-compatibility.md)). Per exported symbol it
records: the kind, the canonical-type signature (TypeShape terms,
[03-projection.md](03-projection.md)), and the **spelling the plan's
lowering chose** — qualified name, import path, and the file it
landed in. A dependent reads spellings from the export rather than
recomputing naming conventions, which is the entire point: the
producing plan's lowering is the only authority on what it named
things. Exports hash into dependents' fingerprints — an export that
didn't change wakes nobody, one that did re-runs exactly its
dependents — and publish beside the producing plan's manifest slice.

Considered and refused: **per-plan model transforms** — filtering or
renaming the shared graph per plan before generation. Transforms
fork the one graph into N variants, and everything downstream —
explain, invalidation edges, cross-plan checks comparing like with
like — would have to answer "which variant?" first. Each need a
transform serves already has a home: filtering is `Sources` scopes,
facts are annotators on the one graph, per-target spellings are
lowering policy.

## Close

Runs after all plans:

- **Merged workspace manifest**, recording plan → files. This closes
  the orphan failure mode: a workspace that no longer declares a
  plan sweeps that plan's files.
- **Path-collision detection** across plan manifests — a stable
  error, never last-writer-wins.
- **Staleness sweep**, scoped per plan ("everything this plan didn't
  produce is stale" is only true within a plan's scope) — and per
  run: under narrowed patterns ([20-cli.md](20-cli.md)), a
  manifested file is swept only when its sources fall inside the
  narrowed scope and no longer produce it, or its producing plan
  or plugin left the composition. Out-of-scope entries carry
  forward untouched; a partial run must not eat outputs it merely
  didn't look at. Whole-workspace reconciliation is `prune`'s job.
- **Cross-plan checks**: `WorkspaceCheck` plugins read the
  *records* — manifests, exports, graph and facts — never raw emit,
  which on a warm run does not exist for clean plans. Checks
  therefore run identically on cold and warm runs, over carried
  records included. They run over the plans that *succeeded*; a
  check whose claim needs a failed plan's records reports one Info
  and stands down — the missing output is already that plan's
  Error, not something to re-litigate once per check.
- **Audit mode**: metadata completeness contracts verified
  ([04-metadata.md](04-metadata.md)).

## Multi-workspace repositories

Every anticipated language has a native multi-project notion
(`go.work`, Cargo workspaces, Maven/Gradle multi-module, pnpm, uv).
The rule, decided once:

**An eidos workspace is a config boundary, not a toolchain
boundary.** One workspace deliberately spans N toolchain modules —
one graph, one manifest, one sweep. Frontends own
toolchain-workspace resolution and surface module identity through
the kernel-owned neutral keys `gen.module` and `gen.moduleRoot` —
one spelling every frontend writes; the raw toolchain form (a
go.mod path, a Maven artifact) stays in the frontend's own
namespace, the same normalize-plus-raw split as Visibility. Layout
policies and Sources predicates match the neutral keys only, which
is what keeps module-aware kernel machinery free of per-language
tables; generated files import correctly across modules because
the emit import machinery already qualifies by package path.
Partial runs are scoping (`./services/...`), not separate
workspaces; per-module heterogeneity is plans with directory-scoped
Sources, not a nested config dialect.

A repo holding genuinely unrelated projects declares multiple
workspaces **explicitly, in one place**:

```yaml
workspaces:
  - {root: ./platform,  config: ./platform/.mygen.yaml}
  - {root: ./tools/gen, config: ./tools/gen/.mygen.yaml}
```

A config that declares `workspaces` declares nothing else — the list
is the whole file, so there is never a question of which workspace a
stray top-level field belongs to. Nested configs are never
implicitly independent workspaces: two things that each own a
manifest and don't know about each other is exactly the orphan
shape. Nothing crosses workspace boundaries — no facts, no exports,
no checks. Two projects needing each other's anything is the signal
they are one workspace with two plans, and the architecture offers
nothing weaker.

## Config

The typed groups the YAML maps 1:1 onto (published JSON Schema for
editor completion; the fluent builder is the Go-native equal):
`Identity` (brand, workspace ID), `Scope`, `Cache` (enabled, dir
override, memo size cap — [09-incrementality.md](09-incrementality.md)),
`Plans []PlanConfig{Name, Sources, Target, Generators, Layout, Sink,
Exports, DependsOn}`. `DryRun` resolves the full multi-plan
picture — buckets, topo order, layouts, export edges — without
executing. The workspace ID names the workspace in manifests and
in multi-workspace diagnostics — merged CI output from a
`workspaces:` list must say which workspace spoke — and defaults
to the root directory's name.
