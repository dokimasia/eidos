# The authoring surface

*Builds on: [06](06-plugins.md) (the SPI everything here lowers to),
[04](04-metadata.md) (facts), [05](05-directives.md) (schemas).
Feeds: [07](07-rendering.md) (what handlers emit),
[09](09-incrementality.md) (gate tuples are its dirty-routing
keys).*

The role interfaces in [06](06-plugins.md) are the SPI: what the
workspace invokes, what the declarative host adapts onto, and the
floor everything lowers to. Authors write against a second layer,
the kernel module's root package, and the whole design fits in one
sentence:

> Subscribers get rules, edges get kits, everything is a value, and
> everything lowers to the SPI.

Annotators and generators *subscribe*. They declare things, plus
rules pairing a trigger with a handler, and every handler is
`func(match, effect) error`.

Frontends and backends produce and finish the graph, so they get
**kits** instead ([11-languages.md](11-languages.md)): declarations
plus a handful of language callbacks, with the kit owning the ritual
every satellite would otherwise repeat, and the correctness lessons
that go with it. Everything below covers the subscriber surface. The
kits live with the satellite anatomy.

**A declaration is a value.**
`NewPlugin(name).Version(v).Output(o)….Handle(rules…).Build()`.
Identity, outputs, priorities and schemas are data, built once. Only
the handlers are functions.

## Triggers

**Triggers are indexed by the symbol model's subject kinds**:
`OnInterface`, `OnStruct`, `OnEnum`, `OnSum`, `OnFunction`,
`OnMethod`, `OnField`, `OnConstant`, `OnVariable` and `OnAlias`.

Two structural triggers join them. `OnEmit(kind)` takes one emit
value produced by an earlier-bucket plugin in the same plan, which
is what a weaver subscribes to. `OnGraph` takes the whole graph, and
it is the pressure valve for logic that genuinely spans subjects.

The kind-indexed set is bounded by the model rather than by use
cases, and it generates from `symbol/schema` alongside the kinds
([02-symbol-model.md](02-symbol-model.md)), so a new kind
automatically gets its trigger and its `Match` type. The anti-sprawl
rule therefore guards *mechanisms*, meaning new wrapper kinds and
new effect types, rather than coverage of kinds.

**Semantic categories are deliberately not triggers.** There is no
`OnError`, because being an error is a fact: an annotator stamps it,
which is `ErrorValueRules` earning its keep, and a generator
subscribes with `Where`. Kinds describe structure and facts describe
meaning, and the trigger names never mix the two.

## Effects

**The handler picks the effect, not the trigger**, on the source
side. Any node-kind trigger and `OnGraph` serve either role: a
handler taking `*Emitter` generates, and one taking `*Stamper`
annotates. That is a generic constraint over the two effect types,
inferred at the call site.

`OnEmit` is the exception, and takes the Emitter only. Emit is per
plan and plans run in parallel, so a fact on the emit side would
live in a second universe that Close's records, the sealed state and
sibling plans never see
([08-workspace-and-plans.md](08-workspace-and-plans.md)). A fact
about generated output is a fact on its *origin* symbol, stamped
during Annotate, and Build rejects an `OnEmit` rule with a Stamper
handler, naming the rule.

## Gates

**Two scoping wrappers** compose around any rules, including
`OnEmit`. `Directive(schema, rules…)` gates on a validated directive
and registers the schema. `Where(pred, rules…)` gates on stamped
facts, through `HasKey` and `KeyEquals`.

For emit-side rules the predicate evaluates against the **origin**,
which is how a weaver subscribes to "methods emitted from
handler-classified symbols" without touching a handler.

A bare rule runs for every subject in the plan's source scope. The
kernel `skip` directive ([05-directives.md](05-directives.md))
excludes a declaration from bare and fact-gated rules only, because
a directive-gated subject opted in explicitly and withdraws by
deleting the directive.

**Gates are declarative, always.** A handler that starts by
filtering contains a gate that belongs in the rule. This is not
taste. It is what makes the subscriber model work at all.

Dispatch is an indexed engine that makes one pass: subjects by kind,
from the store's index; subjects by directive name, built for free
at Freeze, where validation already visits every directive; and
symbols by fact key, maintained at stamp time. A rule therefore
costs O(its matches) rather than O(graph), and only a bare rule,
which honestly asked for everything, pays full price.

The gate tuples are also **subscription records**. `(kind,
directive)` and `(kind, originFact)` are exactly what the
incrementality engine routes dirtiness through
([09-incrementality.md](09-incrementality.md)), so it runs stubgen
again only when an Interface carrying `stub` changed. A gate hidden
inside a handler is selectivity the engine cannot see, which
degrades dispatch into scans and invalidation into running too much.
That is why it is banned rather than discouraged.

## Two-level lowering

The engine never reads authoring-surface values. `Build` lowers them
into the kernel form the engine indexes:

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
`Subscribed` entirely. That is one implicit subscription to
everything in scope: honest, and priced accordingly, with
full-graph dispatch cold, full re-execution on any warm change, and
`stats` saying so.

The split keeps both promises at once. The engine indexes gates as
data without ever executing plugin code to discover them, and the
plugin that wants to parse everything stays a two-method affair.

## The Match

**The Match positions you without confining you.** Every match
carries the subject, the typed directive view when it is gated,
pre-bound `Rules()`, scoped reporting with the origin and position
already filled in, and the tracked `Reader()`. A per-subject handler
that needs a sibling declaration looks it up like anyone else,
instead of escaping to `OnGraph` for an ordinary lookup.

## The Emitter

**Every target is an accumulator, and the Emitter owns the ritual.**
`File(tag?)`, `PackageFile(tag?)` and `PlanFile(tag?)` each return
*the* file for a given cardinality key and family, created on first
touch and appended to thereafter. Two interfaces in one source file
assemble one `_stub.go`. Per-value sentinel matches assemble one
per-file output. Package registries assemble across a package
([18-routing-and-layout.md](18-routing-and-layout.md)). One rule,
three keys.

Multiple files per plugin are families crossed with keys: declare
several tagged outputs, each with its own cardinality, and address
them independently from one handler.

Targets, stems and provenance derive from the subject and the
family. `Mirror` carries the correctness lessons once, in the
framework, rather than copied into every plugin: choosing the
receiver against parameter names, and inferring imports. Slot access
is one `SlotView` type, whether you are appending into your own
method's prologue or, through `OnEmit`, into another plugin's.

## Two ordering rules

These make dispatch safe to parallelize and the output
deterministic.

**Handlers are order-independent within a plugin.** No handler may
depend on another match of the same plugin having run first.

**Accumulator files order contributions** by subject identity, then
by directive-instance source order, then by insertion. So repeatable
instances land in the order the author wrote them, and never in the
order the dispatcher happened to run them. Metadata writes obey the
same canonical order through first-claim-wins arbitration
([04-metadata.md](04-metadata.md)).

## Annotators

**Annotators use the same grammar with the Stamper effect.** They
can be driven entirely by a directive, which is the `defaults`
family shape: a `Directive` wrapper over `OnField`, reading the
validated param and stamping the fact. They can be gated on facts.
They can be bare.

Three things keep the effect small and analyzable.

`Stamp(st, key, v)` writes with `plugin` authority and the plugin's
origin already bound.

**A Stamper writes only to its subject's bag.** A fact "about" a
sibling is a fact on the host whose value names the sibling. Every
write's target is therefore statically known from the trigger, which
is what keeps invalidation edges and audit mode analyzable. A plugin
that wants facts on many symbols subscribes to those kinds.

`Fact(m, key)` reads the subject's stamped facts through the tracked
reader, so an annotator depending on another annotator becomes an
incrementality edge automatically. The ordering between them is
declared through `Provides` and `Requires` on the builder rather
than hoped for.

## Target universality

**A plugin is target-universal in the neutral lane, and per-target
by declaration outside it.** A handler reads *source*-side
projections through its Match and emits *neutral* values through its
Emitter, so a plugin using only the builder, the scaffolding
vocabulary and the backend's kind templates serves every target
language with no changes.

The lane's edge is the body vocabulary. A body beyond scaffolding
lives in a `TemplateRef`, templates are per target, and from that
point the plugin is per-target **by declaration**, through
`For(target, …)`, rather than by accident. The neutral lane is for
declaration-shaped output.

## Presentation

**Presentation is options, layered from defaults to targets.** What
templates need, enumerated: their data, meaning emit values and
`TemplateRef` payloads; the language funcmap, registered by the
backend under one documented name ([07-rendering.md](07-rendering.md));
the plugin's own helpers; overrides, which are the replace verb;
partials; slot markers; and imports for free when they spell types.

The declaration surface serves exactly that list. Plugin-level
`Templates(tree)` and `Funcs(fm)` are the neutral defaults, where
helpers are pure functions over emit values. `For(target,
Templates(...), Funcs(...), Overrides(...))` layers per-target
specializations over those defaults. A plugin with no `For` stays
target-universal, and a plan targeting a language that a
template-claiming plugin never declared fails at **Build**, naming
the plugin and the target, rather than at render.

One composition rule keeps imports working: a helper that produces
type spellings composes the language funcmap's spelling helpers
rather than bypassing them, because those feed the per-file
`ImportSet` as a side effect.

There is deliberately no per-target word map. Filenames take the
family's neutral `Word` with the *target's* join and extension, so
`store_stub.go` against `store.stub.ts`
([18-routing-and-layout.md](18-routing-and-layout.md)). Naming a
generated identifier goes through the target's `JoinName`
([10-cross-language.md](10-cross-language.md)), which the Emitter
exposes. Anything subtler is presentation and belongs in templates
or helpers.

That split applies the Source and Target rule to the facade's own
API: **read-side questions live on the Match, and write-side
spellings on the Emitter.** A target fact in a neutral declaration is
a leak, and plugintest checks for it.

## Directives

**The directive binding is made when you declare it, never
discovered.** `Directive(schema, rules…)` welds the schema, the
trigger and the handler into one rule. When it runs, the dispatcher
binds the validated instance that *caused the call* into
`m.Directive()`, which is why typed param access needs no paranoia
about existence beyond what the schema already states.

Under a `Repeatable` schema ([05-directives.md](05-directives.md))
the handler runs once per instance, and each match carries its one
instance, so `m.Directive()` stays singular and handlers stay
simple. `Name()` disambiguates the rare registration where one
handler serves two schemas.

**A handler sees only its own gating instance.** Bare and fact-gated
matches get nil, and the *other* directives on a subject, meaning
other plugins' annotations, are invisible by design. A directive's
meaning belongs to the plugin that owns it, and its effects are
consumed through stamped facts rather than by reading a stranger's
annotations raw.

Anything else would make every directive's params public API for
every other plugin, which is compile-time coupling through a side
door. Contradictions between annotations are the schema layer's to
catch ([05-directives.md](05-directives.md)), for exactly this
reason.

The two-package weave, where one plugin owns a directive and
generates while another builds on the same subjects, is the positive
form of that rule, composed from three approved mechanisms.

The owner's annotator half **promotes its conclusions to declared
facts**, such as `handlergen.isHandler` under a namespaced key,
which works as a dual role through per-role priorities.

The weaver **declares its gate**:
`Where(HasKey(handlergen.KeyIsHandler), OnEmit(kind, weave))`, with
the predicate evaluating against the origin, per the rule that gates
are declarative. It appends into **slots**: the standard body slots,
which need no foresight from the owner, or named slots the owner
exported as constants.

And **`Requires(owner.Cap)`** turns the ordering into a scheduled
guarantee.

A param that two plugins both want is either promoted to a fact by
its owner, or it belongs in the consumer's own directive. Sharing it
raw does not exist.

**One directive may gate many rules, and it registers once.** The
`Directive` wrapper both gates and carries its schema, and Build
unifies identical redeclarations while refusing conflicting ones.
The common multi-kind plugin is therefore one wrapper:

```go
Handle(Directive(Schema(),      // registered once
    OnStruct(docStruct),        // gated four ways
    OnInterface(docInterface),
    OnEnum(docEnum),
    OnField(docField),
))
```

## Routing

**Routing is addressed, never inferred.** Families are independent
accumulators under the same key, and the handler names the family at
each emit site. A declaration lands in the test companion because it
was written through the `TagTest` handle, not because anything
guessed:

```go
Output(Output{Per: PerPackage, Word: "suite"})                 // suite.go
Output(Output{Tag: TagTest, Per: PerPackage, Word: "suite"})   // suite_test.go
// in the handler:
harness(m, out.PackageFile())            // → suite.go
entry(m, out.PackageFile(TagTest))       // → suite_test.go
```

The target spells the join of word and tag per its own ecosystem
([18-routing-and-layout.md](18-routing-and-layout.md)).

## The lowering guarantee

This is the layer boundary's contract. Every rule set lowers to the
SPI: emit-bearing rules to Generator, stamp-bearing rules to
Annotator, and rules that do both to both, with per-role priorities
intact. The workspace never knows which layer authored a plugin, and
the SPI stays public for whatever the facade does not fit. The
conformance suite holds both spellings of one plugin to byte-equal
output.

The contract, written out. These signatures are terse enough to pin,
and the kind-indexed constructors are shown as two, since each
subject kind has one of the same shape, generated with the schema:

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
// layering via options; never a config struct, never a word map.
func (b *Builder) Templates(tree fs.FS) *Builder
func (b *Builder) Funcs(fm template.FuncMap) *Builder
func (b *Builder) For(t rules.Target, opts ...TargetOption) *Builder
func Templates(tree fs.FS) TargetOption
func Funcs(fm template.FuncMap) TargetOption
func Overrides(fm template.FuncMap) TargetOption // the replace verb

// Effects: the handler's second parameter picks the role.
// Source-side triggers only; OnEmit is Emitter-only.
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
// EmitMatch also carries Origin(), the emit value's source symbol,
// with tracked fact reads, so a weaver can ask whose output it is
// looking at without ever seeing a stranger's directive.
// Tracked fact reads: func Fact[T any](m Matcher, k meta.Key[T]) (T, bool)

// Bodies a Mirror or Method accepts: the four forms of 07.
func Unimplemented(msgf string, a ...any) Body // target spells panic/throw
func Delegate(callee string, args ...Expr) Body
func Stmts(ss ...Stmt) Body                    // the scaffolding vocabulary
func Ref(name string, data any) Body           // TemplateRef, emitter's tree

// Emitter: every target is an accumulator keyed by (key, family),
// and the write-side spellings live here. The plan's target answers
// them, never the plugin's declaration.
func (e *Emitter) File(tag ...Tag) *FileBuilder        // per source file
func (e *Emitter) PackageFile(tag ...Tag) *FileBuilder // per package
func (e *Emitter) PlanFile(tag ...Tag) *FileBuilder    // per plan
func (e *Emitter) JoinName(word, base string) string   // target's join

// Stamper: authority and origin filled in by the dispatch.
func Stamp[T any](st *Stamper, k meta.Key[T], v T)
```
