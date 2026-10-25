<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# The authoring surface

*Builds on: [06](06-plugins.md) (the SPI everything here lowers to),
[04](04-metadata.md) (facts), [05](05-directives.md) (schemas).
Feeds: [07](07-rendering.md) (what handlers emit),
[09](09-incrementality.md) (gate tuples are its dirty-routing
keys).*

The role interfaces in [06](06-plugins.md) are the SPI: what the
workspace invokes, what the declarative host adapts onto, and the
base contract everything lowers to. Authors write against a second layer,
the kernel module's root package, and the whole design fits in one
sentence:

> Subscribers get rules, edges get kits, everything is a value, and
> everything lowers to the SPI.

Annotators and generators *subscribe*. They declare things, plus
rules pairing a trigger with a handler, and every handler is
`func(match, effect) error`.

Frontends and backends produce and finish the graph, so they get
**kits** instead ([11-languages.md](11-languages.md)): declarations
plus a handful of language callbacks, with the kit owning the shared steps
every satellite would otherwise repeat, and the correctness lessons
that go with it. Everything below covers the subscriber surface. The
kits live with the satellite anatomy.

**A declaration is a value.**
`NewPlugin(name).Version(v).Output(o)….Handle(rules…).Generator()`
builds a generator, and `Annotator()` and `Dual()` build the other two
roles. Identity, outputs, priorities and schemas are data, built once.
Only the handlers are functions.

## Triggers

**Triggers are indexed by the symbol model's subject kinds**:
`OnInterface`, `OnStruct`, `OnEnum`, `OnEnumVariant`, `OnSum`,
`OnFunction`, `OnMethod`, `OnParam`, `OnReturn`, `OnField`,
`OnConstant`, `OnVariable` and `OnAlias`.

Two structural triggers join them. `OnEmit(kind)` takes one emit
value produced by a plugin of an earlier turn in the same plan, which
is what a weaver subscribes to. `OnGraph` takes the whole graph, and
it is the pressure valve for logic that genuinely spans subjects.

`OnPackage` gives its handler the package with its files and their
declarations, so the handler checks those declarations as one set.
The rule runs once for each package of the scope. Under a `Directive`
wrapper it runs once for each gating instance of the directive on a
package. The handler reads the declarations
without a tracked read. So each invocation records the package edge,
and an edit of any declaration of the package runs the handler again.
A directive on a package is written in the doc of a Go package clause
or in the header of a TypeScript module
([05-directives.md](05-directives.md)).

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
side. Any node-kind trigger serves either role: a handler taking
`*Emitter` generates, and one taking `*Stamper` annotates. That is a
generic constraint over the two effect types, inferred at the call
site.

`OnGraph` and `OnEmit` take the Emitter only. A Stamper writes to
its subject's bag, which fixes every write's target from the
trigger, and `OnGraph` has no subject, so an annotator that needs
facts on many declarations subscribes to their kinds. `OnEmit` runs
per plan, and plans run in parallel, so a fact stamped on the emit
side would be invisible to Close's records, the sealed state and
sibling plans ([08-workspace-and-plans.md](08-workspace-and-plans.md)).
A fact about generated output is a fact on its *origin* symbol,
stamped during Annotate.

## Gates

**Two scoping wrappers** compose around any rules, including
`OnEmit`. `Directive(schema, rules…)` gates on a validated directive
and registers the schema. `Where(pred, rules…)` takes a predicate.
A key gate, `HasKey` or `KeyEquals`, reads a stamped fact. A value
gate reads the value that triggers the rule: `NameEquals` compares its
name, and `HasSlot` finds a named slot that an emitted body declares.
`Not` negates a predicate. Nested wrappers conjoin and two rules form a
disjunction, so the gates of a plugin state any selection over keys and
values.

A typed build gives the schema of each `Directive` wrapper the kinds
that an instance can affect: the kinds of the rules under the wrapper,
and for an override schema also the kinds of the plugin's bare and
fact-gated rules. The build leaves the kinds empty where one of those
rules is an `OnEmit` rule, and the plugin of such a rule states its
kinds in `Kinds` ([05-directives.md](05-directives.md)). The build
panics where an explicit `Kinds` leaves out the kind of a rule under the
wrapper.

For emit-side rules a key gate evaluates against the **origin**,
which is how a weaver subscribes to "methods emitted from
handler-classified symbols" without touching a handler. A value gate
evaluates against the emitted value, so the same weaver selects the
method `ServeHTTP` alone through `NameEquals`.

A bare rule runs for every subject in the plan's source scope. The
kernel `skip` directive ([05-directives.md](05-directives.md))
excludes a declaration from bare and fact-gated rules only, because
a directive-gated subject opted in explicitly and withdraws by
deleting the directive. A negated plugin directive gates no rule.
It excludes the declaration from the bare and fact-gated rules of
the plugin that registered its schema, as `skip plugin=` does.

**Gates are declarative, always.** A handler that starts by
filtering contains a gate that belongs in the rule. This is not
taste. It is what makes the subscriber model work at all.

Dispatch is an indexed engine that makes one pass: subjects by kind,
from the store's index; subjects by directive name, built for free
at Freeze, where validation already visits every directive; and
symbols by fact key, maintained at stamp time. A rule therefore
costs O(its matches) rather than O(graph), and only a bare rule,
which honestly asked for everything, costs the full scan. A negated
key gate and a value gate do not narrow the enumeration, so a rule
whose gates are of these two kinds alone costs the scan of its kind.

The gate tuples are also **subscription records**. `(kind,
directive)` and `(kind, originFact)` are exactly what the
incrementality engine routes dirtiness through
([09-incrementality.md](09-incrementality.md)), so it runs stubgen
again only when an Interface carrying `stub` changed. A gate hidden
inside a handler is selectivity the engine cannot see, which
degrades dispatch into scans and invalidation into running too much.
That is why it is banned rather than discouraged.

## Two-level lowering

The engine never reads authoring-surface values. A typed build lowers
them into the kernel form the engine indexes:

```go
package plugin

type Subscribed interface {          // what a typed build produces
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
everything in scope: honest, and costed accordingly, with
full-graph dispatch cold, full re-execution on any warm change, and
`stats` saying so.

The split keeps both promises at once. The engine indexes gates as
data without ever executing plugin code to discover them, and the
plugin that wants to parse everything stays a two-method affair.

A warm run hands a phase call a selection and a journal through its
context. The authoring surface runs the selected matches that its
gates still admit, evaluates every rule over the selection's candidate
subjects, and reports what each invocation read, placed, appended,
claimed and found to the journal in canonical match order
([09-incrementality.md](09-incrementality.md)). A hand-rolled plugin
ignores both, and the run records its call as one invocation.

## The Match

**The Match positions you without confining you.** Every match
carries the subject, the typed directive view when it is gated,
pre-bound `Rules()`, scoped reporting with the origin and position
already filled in, and the tracked `Reader()`. A per-subject handler
that needs a sibling declaration looks it up like anyone else,
instead of escaping to `OnGraph` for an ordinary lookup.

`SubjectMatch` is the interface of those methods, and every match
with a subject satisfies it. A function that handlers of two kinds
share takes a `SubjectMatch`, such as one that writes the double of an
interface and of a struct. A function that only reports takes its
reporting half, `Reporter`.

`m.Directives(name)` yields the validated instances of a directive of
the plugin on the subject, in source order. `Subjects(m, key)` yields the
subjects in scope on which a key reads present, in identity order, and
records a membership edge on the key. A subject that gains or loses the
key runs the handler again, and a change of a value on a subject does
not. `FactOf` reads each value and records the fact edge.

## The Emitter

**Every target is an accumulator, and the Emitter owns the bookkeeping.**
`File(tag?)`, `PackageFile(tag?)` and `PlanFile(tag?)` each return
*the* file for a given cardinality key and family, created on first
touch and appended to thereafter. Two interfaces in one source file
assemble one `_stub.go`. Per-value sentinel matches assemble one
per-file output. Package registries assemble across a package
([18-routing-and-layout.md](18-routing-and-layout.md)). One rule,
three keys.

`FileOf(id, tag?)` returns the per-source file of another declaration
of the subject's package. `File` of a package match returns the file
of the package clause. A package match that checks the error values of
its package keys its output on the file of the first error value
through `FileOf`. The invocation records the read of that declaration.
An identity that the package does not declare is refused under
`RefusedFile`, and the run fails.

Multiple files per plugin are families crossed with keys: declare
several tagged outputs, each with its own cardinality, and address
them independently from one handler.

A family that declares a file template claims its files, and it takes
records instead of declarations. `Out.Record(data, decls...)` places
declarations with the data of their match. The family's template then
writes each file from its records ([07-rendering.md](07-rendering.md)),
such as a test file with a test of each subject. Each record keeps the
data of its own call, so a subject can have one test for each instance
of a repeatable directive.

Targets, stems and provenance derive from the subject and the
family. `Mirror` copies a signature once, in the framework, so no
plugin repeats the copy, and leaves the receiver to the target: the
Go satellite's `PointerReceiver` names a pointer receiver against
the parameter names. Imports are collected at render.

A body passes each parameter on by its name. `naming.Params` of
eidos-lang names each parameter that a mirrored signature has without
a name. The Go satellite's `RenamePredeclared` renames a parameter or
a result that would hide a predeclared identifier, such as the
`append` that the body of a stub calls.

`MirrorTypeParams` copies the type parameters of a generic declaration
onto a declaration that a plugin derives from it. `TypeArgs` returns
those parameters as the arguments of a reference inside the derived
declaration, such as the interface that the mock of a generic interface
implements. `rules.Mentioned` returns the type parameters that the
references of one part of the declaration mention, with the parameters
that their bounds and defaults use. A declaration derived from that
part, such as the call record of a method of a stub, declares those
parameters alone.

Slot access is one `SlotView` type, which `Emitter.Slot` returns,
whether you are appending into your own method's prologue or, through
`OnEmit`, into another plugin's. An append into another plugin's value
names your plugin among the contributors of the unit that contains the
value. The frame of the file the value is written in, and the file's
manifest entry, then name the weaver beside the emitter
([17-output-and-determinism.md](17-output-and-determinism.md)). The
parameters and the results of a callable, the embeds of a struct or an
interface and the tag entries of a field are slots as well, so a
weaver changes a signature or a tag through the same view. A helper of
a backend takes such a slot by address, as in `{{params .Params}}`. A
template that ranges over the entries reads `.Items`, and tests for
entries with `.Len`.

The store of a plan seals every slot of a value that a unit places. A
direct `Append` or `Replace` on such a slot panics until the settle
starts, so the slot view is the only path into a value of the store. A
handler appends directly into a value that it builds itself, before it
places the value. The settle lifts the seal, so the lowering hooks of a
backend rewrite the declarations of their target.

## Ordering rules

These rules make dispatch safe to parallelize and keep the output
deterministic.

**Handlers are order-independent within a plugin.** No handler may
depend on another match of the same plugin having run first.

**A handler's writes apply when its phase call ends.** The dispatcher
buffers the declarations an invocation places, the values it appends
into slots, and the findings it reports. The buffers apply in canonical
match order once the phase call's rules have run, on any number of
workers. So a handler sees the plan's store and the slots as they were
when its phase call began, and it does not see its own appends. A
stamp arrives in the fact store at once, because arbitration ranks
claims by their canonical order and not by their arrival
([04-metadata.md](04-metadata.md)). The canonical order is the rule,
the subject and the gating instance. Every run computes it the same
way, so a claim a warm run keeps ranks against a new one.

**An accumulator file orders its contributions** by the source
position of their subject: the file, then the line, then the column. A
subject without a position, such as a declaration that a plugin
creates, comes after every subject with one. Contributions of one
position order by subject identity, then by directive-instance source
order, then by canonical match order and the order of the appends. So
a file lists its declarations in the order of the source, repeatable
instances arrive in the order the author wrote them, and neither
follows the order in which the dispatcher ran them. The provenance of
a unit, `Unit.Origins`, is sorted by identity.

## Annotators

**Annotators use the same grammar with the Stamper effect.** They
can be driven entirely by a directive, which is the `defaults`
family shape: a `Directive` wrapper over `OnField`, reading the
validated param and stamping the fact. They can be gated on facts.
They can be bare.

Three things keep the effect small and analyzable.

`Stamp(st, key, v)` writes with `plugin` authority and the plugin's
origin already bound.

**A Stamper writes only to its subject's bag, and to the bags of
what the subject declares.** A fact "about" a sibling is a fact on
the host whose value names the sibling. `StampOn(st, owned, key, v)`
writes to one of the subject's own parameters, returns or type
parameters, which a directive on the subject states something about
and which no trigger of their own would reach with that directive;
any other identity is refused. Every write's target is therefore
statically known from the trigger, which is what keeps invalidation
edges and audit mode analyzable. A plugin that wants facts on many
symbols subscribes to those kinds.

`Fact(m, key)` reads the subject's stamped facts through the tracked
reader, so an annotator depending on another annotator becomes an
incrementality edge automatically. The ordering between them is
declared through `Provides`, `Requires` and `After` on the builder
rather than hoped for. `Requires` names a capability whose provider
must be present and runs first. `After` names a capability whose
provider runs first where the composition contains it.

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

A plugin that serves more than one target embeds one directory of
templates for each, below `templates`. `TemplatesOf(files, target)`
declares the tree of a target from those files. A typed build panics
where the target is no name of a single directory, and where the
files have no directory `templates/<target>`.

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
a leak.

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

**A handler sees the directives of its own plugin alone.**
`m.Directive()` is the gating instance, and bare and fact-gated matches
get nil. `m.Directives(name)` yields the instances of a directive of the
plugin on the subject, such as each `excuse` beside the gating
`subject`. The directives of other plugins on a subject are invisible by
design. A directive's meaning belongs to the plugin that owns it, and
its effects are consumed through stamped facts rather than by reading
another plugin's annotations raw.

Anything else would make every directive's params public API for
every other plugin, which is compile-time coupling through a side
door. Contradictions between annotations are the schema layer's to
catch ([05-directives.md](05-directives.md)), for exactly this
reason.

The two-package weave, where one plugin owns a directive and
generates while another builds on the same subjects, is the positive
form of that rule, composed from three approved mechanisms.

The owner's annotator half **promotes its conclusions to declared
facts**, such as `handlergen.handler` under a namespaced key,
which works as a dual role through per-role priorities.

The weaver **declares its gate**:
`Where(HasKey(meta.Named[bool](handlergen.KeyHandler)), OnEmit(kind, weave))`, with
the predicate evaluating against the origin, per the rule that gates
are declarative. It appends into **slots** through `Emitter.Slot`: the
standard body slots, which every body has without a declaration from
the owner, or named slots the owner exported as constants.

And **`Requires(owner.Cap)`** turns the ordering into a scheduled
guarantee. A weaver that also serves compositions without the owner
declares `After(owner.Cap)`, which orders it after the owner where the
owner is present. In a composition without the owner, no plugin claims
the namespace of the keys that the owner registers. The workspace then
leaves the gate of the weaver unbound, so the gate does not select a
subject. A misspelled key still fails the build where a plugin claims
the namespace of the key. It also fails the build where each capability
that the weaver names in `After` has a provider. An owner whose
generator half requires the weaver declares its capability with
`ProvidesIn(plugin.RoleAnnotator, owner.Cap)`. The capability then
orders no generator, so the generator half of the owner runs after the
weaver.

A param that two plugins both want is either promoted to a fact by
its owner, or it belongs in the consumer's own directive. Sharing it
raw does not exist.

**One directive may gate many rules, and it registers once.** The
`Directive` wrapper both gates and registers its schema, and a
plugin declares each directive in one wrapper: the typed build of the
plugin panics on a second wrapper that declares the same directive. A
wrapper around no rules registers its schema and gates nothing, for a
directive that a handler of another rule reads through
`m.Directives(name)`. Its schema states its `Kinds`, unless it overrides,
because no rule gives the kinds that an instance may be written on. A name
that two plugins register is ambiguous under its bare spelling, so
a carrier writes it with its plugin prefix. The common multi-kind
plugin needs one wrapper:

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
each emit site. A declaration arrives in the test companion because it
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

A family whose files test their package from outside states
`External`. The Go target writes each test file of such a family into
the external test package of its directory, such as `store_test` beside
`store`, and qualifies each reference into the package under test. The
other families of the plugin keep the package of their directory.

```go
Output(Output{Tag: TagTest, Per: PerSource, Word: "check", External: true}) // package store_test
```

A test of an unexported declaration compiles only in the package under
test. A handler marks such a test through `Out.Internal`, and the flush
lists the origin of each marked declaration in `Unit.Inside`. The Go
target writes the marked declarations into the internal test file of the
family, `store_check_internal_test.go`, which declares `store`. The other
declarations of the family keep `store_test`.

```go
out := e.File(TagTest)
if s.Visibility != symbol.VisibilityPublic {
    out = out.Internal() // store_check_internal_test.go, package store
}
out.Append(check)
```

A handler reads the package of the routed file of a handle through
`Out.Package` before it records a declaration. The layout of the plan
previews the route of the subject of the handle: the routing directives
on the subject and on its package, and the configuration of the plan.
Under `out`, the package is the package of the directory of the route,
so the handler passes it to `Bound.Importable` and leaves out a
declaration that the file cannot refer to. After `Out.Internal`, the
package is the package of the internal test file of the family. A warm
run executes the invocation again where an edit can change the package
([18-routing-and-layout.md](18-routing-and-layout.md)).

```go
out := e.File(TagTest)
test, err := out.Package() // shapetest_test under out path=shapetest/
if err == nil && m.Rules().Importable(test, factory) == nil {
    out.Record(data, check)
}
```

## The lowering guarantee

This is the layer boundary's contract. Every rule set lowers to the
SPI through the typed build of its role: `Generator()` for
emit-bearing rules, `Annotator()` for stamp-bearing rules, and `Dual()`
for rules that do both, with per-role priorities intact. A typed build
panics where the rules do not fit its role, so the role is a compile-time
type and never an assertion. The workspace never knows which layer
authored a plugin, and the SPI stays public for whatever the facade does
not fit. The conformance suite holds both spellings of one plugin to
byte-equal output.

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
func (b *Builder) Requires(caps ...Capability) *Builder // present, and runs first
func (b *Builder) After(caps ...Capability) *Builder    // runs first where present
func (b *Builder) Options(cfg any) *Builder        // tagged struct; the workspace's
                                                   // Build populates it, errors name the field,
                                                   // and a schema:"required" field at its zero
                                                   // value fails Build
func (b *Builder) Handle(rules ...Rule) *Builder
func (b *Builder) Generator() Generator            // lowers to one role of the SPI,
func (b *Builder) Annotator() Annotator            // and panics where the rules
func (b *Builder) Dual() Dual                      // do not fit the role

type Output struct {
    Tag      string      // "" = primary
    Per      Cardinality // PerSource | PerPackage | PerPlan
    Word     string      // the family's word; the TARGET spells the join
                         // and extension (store_stub.go vs store.stub.ts)
    Template string      // the file template that claims the family's files;
}                        // "" leaves the files to the kind templates

// Presentation: neutral defaults at plugin level, per-target
// layering via options; never a config struct, never a word map.
func (b *Builder) Templates(tree fs.FS) *Builder
func (b *Builder) Funcs(fm template.FuncMap) *Builder
func (b *Builder) For(t rules.Target, opts ...TargetOption) *Builder
func Templates(tree fs.FS) TargetOption
func TemplatesOf(files fs.FS, t plugin.Target) TargetOption // the directory templates/<t> of files
func Funcs(fm template.FuncMap) TargetOption
func Overrides(fm template.FuncMap) TargetOption // the replace verb

// Effects: the handler's second parameter picks the role.
// Node-kind triggers only; OnGraph and OnEmit are Emitter-only.
type Effect interface{ *Emitter | *Stamper }

// Triggers: one constructor per subject kind, plus two structural.
func OnInterface[E Effect](h func(*InterfaceMatch, E) error) Rule
func OnEnum[E Effect](h func(*EnumMatch, E) error) Rule
// … OnStruct, OnEnumVariant, OnSum, OnFunction, OnMethod, OnParam,
//   OnReturn, OnField, OnConstant, OnVariable, OnAlias: the same
//   signature for each kind …
func OnEmit(k emit.Kind, h func(*EmitMatch, *Emitter) error) Rule
func OnGraph(h func(*GraphMatch, *Emitter) error) Rule
func OnPackage[E Effect](h func(*PackageMatch, E) error) Rule // each package of the scope

// Scoping wrappers compose around any rules.
func Directive(s directive.Schema, rules ...Rule) Rule // gate + register; without rules, register alone
func Gated(name directive.Name, rules ...Rule) Rule    // gate on a kernel directive
func Where(p Pred, rules ...Rule) Rule                 // gate on a predicate
func HasKey[T any](k meta.Key[T]) Pred                 // key gates read a fact
func KeyEquals[T comparable](k meta.Key[T], v T) Pred
func NameEquals(name string) Pred                      // value gates read the trigger's value
func HasSlot(name string) Pred                         // a named slot of an emitted body
func Not(p Pred) Pred

// Every *Match embeds the base surface:
//   Lang() symbol.Lang           Rules() rules.Bound  // the kernel's walks over the invocation's view
//   RulesFor(symbol.Lang) rules.Bound                 // another language's
//   Reader() *store.Reader       // incl. Lookup(Identity) after Link
//   Directive() *DirectiveView   // the gating instance; nil otherwise
//   Directives(name) iter.Seq[*DirectiveView] // the plugin's own instances on the subject
//   Kernel() meta.KernelKeys     // the kernel's own keys, for its facts
//   Errorf / Warnf(code, format, ...) // origin and position pre-bound
//   ErrorfAt(code, pos, format, ...)  // at a carrier line instead
// plus its subject field (.Interface, .Enum, .Method, .Package, …). The
// host of a method is the identity m.Method.Host, and the enumeration
// of a variant is the identity m.EnumVariant.Host. SubjectMatch is that
// surface as an interface, and Reporter is its reporting half.
// EmitMatch also carries Origin(), the emit value's source symbol,
// with tracked fact reads, so a weaver can ask whose output it is
// looking at without ever seeing another plugin's directive.
// Tracked fact reads: func Fact[T any](m Matcher, k meta.Key[T]) (T, bool)
// and func Subjects[T any](m Matcher, k meta.Key[T]) iter.Seq[symbol.Identity],
// the subjects of a key under a membership edge on the key

// Bodies a Mirror or Method accepts: the forms of 07 without a
// template, which Emitter.Ref below claims.
func Unimplemented(msgf string, a ...any) Body // target spells panic/throw
func Delegate(callee string, args ...Expr) Body
func Stmts(ss ...Stmt) Body                    // the scaffolding vocabulary

// Emitter: every target is an accumulator keyed by (key, family),
// and the write-side spellings are on it. The plan's target returns
// them, never the plugin's declaration. Every write is buffered and
// applies in canonical match order when the phase call ends.
func (e *Emitter) File(tag ...Tag) *Out                    // per source file
func (e *Emitter) FileOf(id symbol.Identity, tag ...Tag) *Out // the source file of id, of the package
func (e *Emitter) PackageFile(tag ...Tag) *Out             // per package
func (e *Emitter) PlanFile(tag ...Tag) *Out                // per plan
func (o *Out) Append(decls ...symbol.Symbol)               // declarations of the file
func (o *Out) Record(data any, decls ...symbol.Symbol)     // records of a claimed family, with their data
func (o *Out) Internal() *Out                              // later placements test their package from inside
func (o *Out) Package() (symbol.Identity, error)           // the package of the routed file, before the handler records
func (e *Emitter) Slot[T any](s *emit.Slot[T]) SlotView[T] // one slot's write view
func (v SlotView[T]) Append(values ...T)                   // names a weaver among the contributors
func (e *Emitter) JoinName(tail, head string) string       // head, then tail raised
func (e *Emitter) Ref(name string, data any) *emit.TemplateRef // resolves in this plugin's tree

// Stamper: authority and origin filled in by the dispatch.
func Stamp[T any](st *Stamper, k meta.Key[T], v T)
func StampOn[T any](st *Stamper, owned symbol.Identity, k meta.Key[T], v T) // a param, return or type param of the subject
```
