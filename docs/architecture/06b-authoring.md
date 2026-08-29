# The authoring surface

*Builds on: [06](06-plugins.md) (the SPI everything here lowers
to), [04](04-metadata.md) (facts), [05](05-directives.md)
(schemas). Feeds: [07](07-rendering.md) (what handlers emit),
[09](09-incrementality.md) (gate tuples are its dirty-routing
keys).*

The role interfaces of [06](06-plugins.md) are the SPI: what the
workspace invokes, what the declarative host adapts onto, and the
floor everything lowers to. Authors write against a second layer,
the kernel module's root package, and the whole design is one
frame:

> **Subscribers get rules; edges get kits; everything is a value;
> everything lowers to the SPI.**

Annotators and generators *subscribe* — declarations plus
(trigger → handler) rules, every handler
`func(match, effect) error`. Frontends and backends are the graph's
*producer* and *finisher* — they get **kits**
([11-languages.md](11-languages.md)): declarations plus a handful
of language callbacks, with the cross-satellite ritual and its
correctness lessons owned by the kit. Everything below covers the
subscriber surface; the kits live with the satellite anatomy.

**Declarations are a value**: `NewPlugin(name).Version(v).Output(o)…
.Handle(rules…).Build()`. Identity, outputs, priorities, schemas —
data built once; only handlers are functions.

**Triggers are indexed by the symbol model's subject kinds** —
`OnInterface`, `OnStruct`, `OnEnum`, `OnSum`, `OnFunction`,
`OnMethod`, `OnField`, `OnConstant`, `OnVariable`, `OnAlias` —
plus two structural ones: `OnEmit(kind)` (one emit value produced by an
earlier-bucket plugin in the same plan — the weaver's subject) and
`OnGraph` (the whole graph, the pressure valve for genuinely
cross-subject logic). The kind-indexed set is bounded by the model,
not by use cases, and generates from `symbol/schema` with the kinds
([02-symbol-model.md](02-symbol-model.md)) — a new kind
automatically gets its trigger and its `Match` type. The
anti-sprawl rule therefore guards *mechanisms* (new wrapper kinds,
new effect types), not kind coverage.

Semantic categories are deliberately not triggers: there is no
`OnError`, because error-ness is a **fact** — an annotator stamps
it (that is `ErrorValueRules` earning its keep) and a generator
subscribes with `Where`. Kinds are structure; facts are meaning;
the two axes never mix in the trigger names.

**Effects are chosen by the handler, not the trigger** — on the
source side. Any node-kind trigger and `OnGraph` serve either
role: a handler taking `*Emitter` generates, one taking `*Stamper`
annotates — a generic constraint over the two effect types,
inferred at the call site. `OnEmit` is the exception: it takes the
Emitter only. Emit is per-plan and plans run in parallel — an
emit-side fact would live in a second universe that Close's
records, the sealed state, and sibling plans never see
([08-workspace-and-plans.md](08-workspace-and-plans.md)) — so a
fact about generated output is a fact on its *origin* symbol,
stamped during Annotate, and Build rejects an `OnEmit` rule with a
Stamper handler, naming the rule.

**Two scoping wrappers**, composable around any rules — `OnEmit`
included: `Directive(schema, rules…)` gates on a validated
directive and registers the schema; `Where(pred, rules…)` gates on
stamped facts (`HasKey`, `KeyEquals`) — for emit-side rules the
predicate evaluates against the **origin**, which is how a weaver
subscribes to "methods emitted from handler-classified symbols"
without touching a handler. A bare rule fires for every subject in
the plan's source scope; the kernel `skip` directive
([05-directives.md](05-directives.md)) excludes a declaration from
bare and fact-gated rules only — a directive-gated subject opted in
explicitly, and its opt-out is deleting the directive.

**The gate-free handler law.** Gates are declarative, always: a
handler that begins by filtering is a gate that belongs in the
rule. This is not taste — it is what makes the subscriber model
real. Dispatch is an **indexed engine, one pass**: subjects by kind
(the store's index), subjects by directive name (built free at
Freeze, where validation already visits every directive), symbols
by fact key (maintained at stamp time). A rule costs O(its
matches), never O(graph); only bare rules — which honestly asked
for everything — pay full price. And the gate tuples are
**subscription records**: `(kind, directive)` and `(kind,
originFact)` are exactly what the incrementality engine routes
dirtiness through ([09-incrementality.md](09-incrementality.md)) —
wake stubgen only when an Interface carrying `stub` changed. A gate
hidden inside a handler is selectivity the engine cannot see: it
degrades dispatch to scans and invalidation to over-waking, which
is why it is banned rather than discouraged.

**Two-level lowering.** The engine never reads authoring-surface
values; `Build` lowers them to the kernel form the engine indexes:

```go
package plugin

type Subscribed interface {          // what Build produces
    Generator
    Subscriptions() []Subscription   // the gate tuples, as data
}
type Subscription struct {
    Rule      RuleID                 // stable within the plugin
    Kind      node.Kind              // or KindAny for OnGraph
    Directive string                 // "" when ungated
    FactKey   meta.KeyID             // zero when ungated
    Phase     Phase                  // Annotate | Generate | Emit
}
```

A hand-rolled plugin may implement bare `Generator` and skip
`Subscribed` entirely — that is one implicit subscription to
everything in scope, honest and priced: full-graph dispatch cold,
full re-execution on any warm change, and `stats` says so. The
split keeps both promises at once: the engine indexes gates as
data without ever executing plugin code to discover them, and the
parse-everything plugin stays a two-method affair.

**The Match positions you; it never imprisons you.** Every match
carries the subject, the typed directive view when gated, pre-bound
`Rules()`, scoped reporting (origin and position filled in), and the
tracked `Reader()` — a per-subject handler that needs a sibling
declaration looks it up like anyone else, instead of fleeing to
`OnGraph` for ordinary lookups.

**The Emitter owns the ritual and its lessons — and every target
is an accumulator.** `File(tag?)`, `PackageFile(tag?)`, and
`PlanFile(tag?)` each return THE file for (cardinality key, family)
— created on first touch, appended thereafter: two interfaces in
one source file assemble one `_stub.go`; per-value sentinel matches
assemble one per-file output; package registries assemble across a
package ([18-routing-and-layout.md](18-routing-and-layout.md)).
One law, three keys. Multiple files per plugin are families ×
keys: declare several tagged outputs, each with its own
cardinality, and address them independently from one handler.
Targets, stems, and provenance derive from the subject and the
family; `Mirror` carries the correctness lessons — the receiver
chosen against parameter names, import inference — once, in the
framework, instead of copied into every plugin. Slot access is one
`SlotView` type whether appending into your own method's prologue
or, through `OnEmit`, into another plugin's.

**Two laws** make the dispatch safe to parallelize and the output
deterministic: handlers are order-independent within a plugin — no
handler may depend on another match of the same plugin having run —
and accumulator files order contributions by **subject identity,
then directive-instance source order, then insertion** — so
repeatable instances land as the author wrote them, and never as
the dispatcher happened to run them. Metadata writes obey the same
canonical order through first-claim-wins arbitration
([04-metadata.md](04-metadata.md)).

**Annotators are the same grammar with the Stamper effect** — fully
directive-drivable (the `defaults`-family shape: a `Directive`
wrapper over `OnField`, reading the validated param, stamping the
fact), fact-gatable, or bare. What makes the effect small and
analyzable:

- `Stamp(st, key, v)` writes with `plugin` authority and the
  plugin's origin pre-bound.
- **The subject-only law**: a Stamper writes only to its subject's
  bag. A fact "about" a sibling is a fact on the host whose value
  names the sibling. Every write's target is statically known from
  the trigger — which is what keeps invalidation edges and audit
  mode analyzable; a plugin wanting facts on many symbols
  subscribes to those kinds.
- `Fact(m, key)` reads the subject's stamped facts through the
  tracked reader, so annotator-reads-annotator dependencies become
  incrementality edges automatically — and ordering between them is
  declared through `Provides`/`Requires` on the builder, never
  hoped.

**Plugins are target-universal in the neutral lane, per-target by
declaration outside it.** A handler reads *source*-side projections
through its Match and emits *neutral* values through its Emitter —
so a plugin using only the builder, the scaffolding vocabulary, and
backend kind templates serves every target language with zero
plugin changes. The lane's edge is the body vocabulary: a body
beyond scaffolding lives in a `TemplateRef`, templates are
per-target, and from that point the plugin is per-target *by
declaration* — honestly, through `For(target, …)` — never
accidentally. The neutral lane is for declaration-shaped output.

**Presentation is options with default→target layering.** What
templates need, enumerated: their data (emit values and
`TemplateRef` payloads), the language funcmap (backend-registered,
one documented name — [07-rendering.md](07-rendering.md)), the
plugin's own helpers, overrides (the replace verb), partials, slot
markers, and imports-for-free when they spell types. The
declaration surface serves exactly that list:

- Plugin-level `Templates(tree)` and `Funcs(fm)` are the neutral
  defaults — helpers are pure functions over emit values.
- `For(target, Templates(...), Funcs(...), Overrides(...))` layers
  per-target specializations over the defaults; a plugin with no
  `For` stays target-universal, and a plan targeting a language a
  template-claiming plugin never declared fails at **Build**,
  naming plugin and target — never at render.
- The composition law that keeps imports working: a helper that
  produces type spellings composes the language funcmap's spelling
  helpers rather than bypassing them, because those feed the
  per-file `ImportSet` as a side effect.

There is deliberately no per-target word map: filenames take the
family's neutral `Word` with the *target* spelling join and
extension (`store_stub.go` versus `store.stub.ts`,
[18-routing-and-layout.md](18-routing-and-layout.md));
generated-identifier naming goes through the target's `JoinName`
([10-cross-language.md](10-cross-language.md)), exposed on the
Emitter; anything subtler is presentation and lives in templates or
helpers. The split is the Source/Target law applied to the facade's
own API — **read-side questions on the Match, write-side spellings
on the Emitter** — and a target fact in a neutral declaration is a
leak plugintest checks for.

**The directive binding is made at declaration time, never
discovered.** `Directive(schema, rules…)` welds schema, trigger,
and handler into one rule; when it fires, the dispatcher binds the
validated instance that *caused the call* into `m.Directive()` —
which is why typed param access needs no existence paranoia beyond
what the schema states. Under a `Repeatable` schema
([05-directives.md](05-directives.md)) the handler fires once per
instance, each match carrying its one instance; `m.Directive()`
stays singular and handlers stay simple. `Name()` disambiguates the
rare one-handler-two-schemas registration.

**The visibility law**: a handler sees only its own gating
instance. Bare and fact-gated matches get nil, and the *other*
directives on a subject — other plugins' annotations — are
invisible by design: a directive's meaning belongs to its owning
plugin, and its effects are consumed through stamped facts, never
by reading a stranger's annotations raw. Anything else makes every
directive's params de-facto public API for every other plugin —
compile-time coupling through a side door. Contradictions between
annotations are the schema layer's to catch
([05-directives.md](05-directives.md)), for exactly this reason.

The two-package weave — one plugin owns a directive and generates,
another builds on the same subjects — is the visibility law's
positive form, three approved mechanisms composed: the owner's
annotator half **promotes its conclusions to declared facts**
(`handlergen.isHandler`, a namespaced key — dual-role via per-role
priorities); the weaver **declares its gate** —
`Where(HasKey(handlergen.KeyIsHandler), OnEmit(kind, weave))`, the
predicate evaluating against the origin, per the gate-free handler
law — and appends into **slots**: standard body slots needing zero
foresight, or named slots the owner exported as constants; and
**`Requires(owner.Cap)`** makes the ordering a scheduled guarantee.
A param two plugins want is either promoted to a fact by its owner
or belongs in the consumer's own directive; raw sharing does not
exist.

**One directive may gate many rules; it registers once.** The
`Directive` wrapper both gates and carries its schema; Build
unifies identical redeclarations and refuses conflicting ones. The
common multi-kind plugin is therefore one wrapper:

```go
Handle(Directive(Schema(),      // registered once
    OnStruct(docStruct),        // gated four ways
    OnInterface(docInterface),
    OnEnum(docEnum),
    OnField(docField),
))
```

**Routing is addressed, never inferred.** Families are independent
accumulators under the same key, and the handler names the family
at each emit site — a decl lands in the test companion because it
was written through the `TagTest` handle, not because anything
guessed:

```go
Output(Output{Per: PerPackage, Word: "suite"})                 // suite.go
Output(Output{Tag: TagTest, Per: PerPackage, Word: "suite"})   // suite_test.go
// in the handler:
harness(m, out.PackageFile())            // → suite.go
entry(m, out.PackageFile(TagTest))       // → suite_test.go
```

The target spells the (word, tag) join per its ecosystem
([18-routing-and-layout.md](18-routing-and-layout.md)).

**The lowering guarantee** is the layer boundary's contract: every
rule set lowers to the SPI (emit-bearing rules to Generator,
stamp-bearing to Annotator, both to both, per-role priorities
intact); the workspace never knows which layer authored a plugin;
and the SPI stays public for what the facade doesn't fit. The
conformance suite holds both spellings of one plugin to byte-equal
output.

The contract, written out — signatures this terse are stable enough
to pin (kind-indexed constructors elided to two; each subject kind
has one of the same shape, generated with the schema):

```go
// Declarations: a plugin is a value.
func NewPlugin(name string) *Builder
func (b *Builder) Version(v string) *Builder
func (b *Builder) Output(o Output) *Builder        // repeatable: families
func (b *Builder) Priority(r Role, p int) *Builder // per role
func (b *Builder) Provides(caps ...Capability) *Builder
func (b *Builder) Requires(caps ...Capability) *Builder
func (b *Builder) Options(cfg any) *Builder        // tagged struct; populated
                                                   // at Build, errors name the field
func (b *Builder) Handle(rules ...Rule) *Builder
func (b *Builder) Build() Plugin                   // lowers to the SPI

type Output struct {
    Tag  string      // "" = primary
    Per  Cardinality // PerSource | PerPackage | PerPlan
    Word string      // the family's word; the TARGET spells the join
}                    // and extension (store_stub.go vs store.stub.ts)

// Presentation: neutral defaults at plugin level, per-target
// layering via options — never a config struct, never a word map.
func (b *Builder) Templates(tree fs.FS) *Builder
func (b *Builder) Funcs(fm template.FuncMap) *Builder
func (b *Builder) For(t rules.Target, opts ...TargetOption) *Builder
func Templates(tree fs.FS) TargetOption
func Funcs(fm template.FuncMap) TargetOption
func Overrides(fm template.FuncMap) TargetOption // the replace verb

// Effects: the handler's second parameter picks the role —
// source-side triggers only; OnEmit is Emitter-only.
type Effect interface{ *Emitter | *Stamper }

// Triggers: one constructor per subject kind, plus two structural.
func OnInterface[E Effect](h func(*InterfaceMatch, E) error) Rule
func OnEnum[E Effect](h func(*EnumMatch, E) error) Rule
// … OnStruct, OnSum, OnFunction, OnMethod, OnField, OnConstant,
//   OnVariable, OnAlias: same shape per kind …
func OnEmit(k emit.Kind, h func(*EmitMatch, *Emitter) error) Rule
func OnGraph[E Effect](h func(*GraphMatch, E) error) Rule

// Scoping wrappers compose around any rules.
func Directive(s directive.Schema, rules ...Rule) Rule // gate + register
func Where(p Pred, rules ...Rule) Rule                 // gate on facts
func HasKey[T any](k meta.Key[T]) Pred
func KeyEquals[T comparable](k meta.Key[T], v T) Pred

// Every *Match embeds the base surface:
//   Lang() rules.Source          Rules() rules.SourceRules
//   Reader() *store.Reader       // incl. Lookup(Identity) after Link
//   Directive() *DirectiveView   // the gating instance; nil otherwise
//   Errorf / Warnf(code, format, ...) // origin and position pre-bound
// plus its subject field (.Interface, .Enum, {.Host, .Method}, …).
// EmitMatch additionally carries Origin() — the emit value's source
// symbol, with tracked fact reads — so a weaver can ask whose
// output it is looking at without ever seeing a stranger's
// directive.
// Tracked fact reads: func Fact[T any](m Matcher, k meta.Key[T]) (T, bool)

// Bodies a Mirror/Method accepts — the four forms of 07:
func Unimplemented(msgf string, a ...any) Body // target spells panic/throw
func Delegate(callee string, args ...Expr) Body
func Stmts(ss ...Stmt) Body                    // the scaffolding vocabulary
func Ref(name string, data any) Body           // TemplateRef, emitter's tree

// Emitter: every target is an accumulator keyed by (key, family),
// and the write-side spellings live here — the plan's target
// answers them, never the plugin's declaration.
func (e *Emitter) File(tag ...Tag) *FileBuilder        // per source file
func (e *Emitter) PackageFile(tag ...Tag) *FileBuilder // per package
func (e *Emitter) PlanFile(tag ...Tag) *FileBuilder    // per plan
func (e *Emitter) JoinName(word, base string) string   // target's join

// Stamper: authority and origin filled by the dispatch.
func Stamp[T any](st *Stamper, k meta.Key[T], v T)
```
