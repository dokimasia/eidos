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
the cache, the diagnostic sink and the config. The consumer's binary
builds it, through the fluent builder or through YAML onto the same
typed config structs.

A **plan** is one write side: a generator set, a layout policy,
exactly one backend, one sink, a source scope and a name. Plans are
values, meaning declarable data that can be exported as presets,
rather than plugins ([06-plugins.md](06-plugins.md)).

**Freeze is enforced.** After Annotate the store seals, and a
structural write is refused with a stable code.

**Plans are isolated.** One plan failing does not abort its
siblings. The workspace joins the errors and reports status per
plan, and a plan's manifest slice commits only when that plan
succeeds.

**Outputs are never inputs.** A workspace does not read its own
generated files as source. Load excludes every path the workspace
can prove it owns, through a manifest entry or a provenance trailer
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

Four rules become structure rather than discipline.

Plugins are stateless values, because there is no run state outside
`Run` to hide anything in. `--watch` is many Runs over one
Workspace.

**The dispatch plan compiles at Build**, the way a query plan does,
because compile-time plugins make every subscription known at Build.
The run-time dispatcher is then a stateless executor over static
tables.

Those same tables are the ledger's invalidation metadata, so
dispatch and invalidation cannot disagree with each other.

The conductor is a function rather than a state machine: `Run()`
reads as straight-line code.

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

**The sealed state** is the persisted trio: the sealed graph, the
fact store and the artifact table, written one generation at a time,
with N+1 beside N and a swap on success. Facts persist because they
have to. Bags that died with the run would force full re-annotation
on every warm run, which is O(subjects) against the performance
target. Symbol IDs are local to a generation, and canonical
identities are the stored form.

**The commit protocol is two-phase, and its behaviour under a crash
is stated.** Every plan stages. A plan's `Commit`, meaning its
renames plus its manifest slice, runs only when that plan succeeds.
`Ledger.CommitRun` runs last, after every sink. So the engine's
memory never records an output that disk does not hold, and a crash
between the two fails conservatively: derive again, write identical
bytes, report `Unchanged`.

**The score**, meaning who runs what, in order:

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

Concurrency per phase: phases are barriers. Load shards per unit,
with graph writes serialized per package. Annotate runs
bucket-sequentially, with parallelism inside a bucket as an opt-in,
since bag arbitration is already deterministic. Plans run in
parallel except across export edges. Render runs per file inside the
kit. Close runs single-threaded over immutable records. A dry run is
this same Run with the staging discarded.

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
   Identical schema redeclarations unify, and conflicting ones are
   refused, naming both plugins.
2. **Plugin lowering.** Authoring-surface values lower to the SPI
   ([06b-authoring.md](06b-authoring.md)), subscriptions are
   collected as data, and per-role priorities and capability
   topology sort. A cycle is an error naming the bucket and its
   members.
3. **Options.** Every plugin's options schema populates from its
   config section, and a failure names the plugin and the field.
4. **Plans.** Per plan: exactly one backend and a registered target;
   every template-claiming plugin declares that target; layout
   refinements name plugins and tags that exist; Sources predicates
   resolve against the registries; and export dependencies
   topologically sort, where a cycle names both plans.
5. **Policies.** Selections validate against registered keys and
   choices, and each plan's total `Policy` is fixed here
   ([10-cross-language.md](10-cross-language.md)).
6. **The dispatch plan.** Subscriptions compile into the per-phase,
   per-bucket gate-to-handler tables the Run's dispatcher executes.
7. **The handshake.** One contract version, checked across every
   registered component
   ([15-compatibility.md](15-compatibility.md)).

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
    File      string              // where it arrived
}
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

Several plans targeting one language, with different generator sets,
layouts and sinks, fall out of that for free. Annotators run once
over the union graph, because facts are per source language anyway
and detection dispatches per language.

The predicate vocabulary is closed and conjunctive: three optional
fields, and a package matches when every field that is present
matches.

```yaml
sources:
  lang: golang            # source language identity
  packages: ["./svc/..."] # workspace-relative path globs
  module: "billing"       # toolchain-module identity (the neutral gen.module fact)
```

Nothing else. No negation, and no unions of predicates. A plan that
needs another plugin scope is two plans. Growing the vocabulary is a
kernel change, additive under the compatibility policy. `golang`
here is the human spelling, resolved against the language registry
at Build, and a plan composed in Go uses the satellite's exported
identity ([03-projection.md](03-projection.md)).

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
**publish an export**, which is a typed, deterministic summary of
what it generated: a frozen value rather than a window into its emit
graph. And a plan may **declare a dependency** on a named export.
Plans then sort topologically on declared dependencies, a cycle is a
Build error naming both plans, plans that declare nothing stay fully
parallel, and the incrementality engine gets a real edge instead of
a hidden one.

The export is a versioned document under a kernel schema, and it is
public API, because binding correctness leans on it
([15-compatibility.md](15-compatibility.md)). Per exported symbol it
records the kind, the canonical-type signature in TypeShape terms
([03-projection.md](03-projection.md)), and **the spelling the
plan's lowering chose**, meaning the qualified name, the import path
and the file it arrived in.

A dependent reads spellings from the export rather than recomputing
naming conventions, which is the entire point: the producing plan's
lowering is the only authority on what it named things. Exports hash
into their dependents' fingerprints, so an export that did not
change causes nobody to run again, and one that did re-runs exactly
its dependents. Each publishes beside the producing plan's manifest
slice.

Considered and refused: **per-plan model transforms**, meaning
filtering or renaming the shared graph per plan before generation.
Transforms fork the one graph into N variants, and then everything
downstream has to ask "which variant" first, including explain,
invalidation edges, and cross-plan checks trying to compare like
with like. Every need a transform would serve already has a home:
filtering is a `Sources` scope, facts are annotators over the one
graph, and per-target spellings are lowering policy.

## Close

Close runs after every plan.

It writes the **merged workspace manifest**, recording plan to
files, which is what stops a deleted plan leaving its generated
files behind: a workspace that no longer declares a plan deletes
that plan's files.

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

It runs the **cross-plan checks**. `WorkspaceCheck` plugins read the
*records*, meaning manifests, exports, graph and facts, and never
raw emit, which on a warm run does not exist for a clean plan. So
the checks run identically cold and warm, including over carried
records. They run over the plans that succeeded. A check whose claim
needs a failed plan's records reports one Info and stands down,
because the missing output is already that plan's Error and does not
need re-litigating once per check.

Finally, **audit mode** verifies the metadata completeness contracts
([04-metadata.md](04-metadata.md)).

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
`Cache` for enabled, directory override and memo size cap
([09-incrementality.md](09-incrementality.md)), and `Plans
[]PlanConfig{Name, Sources, Target, Generators, Layout, Sink,
Exports, DependsOn}`.

`DryRun` resolves the full multi-plan picture, meaning buckets,
topological order, layouts and export edges, without executing
anything.

The workspace ID names the workspace in manifests and in
multi-workspace diagnostics, because merged CI output from a
`workspaces:` list has to say which workspace spoke. It defaults to
the root directory's name.
