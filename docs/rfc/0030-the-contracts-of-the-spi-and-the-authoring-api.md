---
rfc: 0030
title: The contracts of the SPI and the authoring API
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-12
updated: 2026-10-12
discussion: none
supersedes: none
superseded-by: none
produces-adr: 0022, 0023
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0030: The contracts of the SPI and the authoring API

## Summary

A type can satisfy an interface of the kernel and still fail workspace Build,
because Build asserts methods that the interface does not declare. A plugin can
write into the output of another plugin without the buffered path that names
the writer, and a built plugin changes when a workspace builds it. We propose
to put every method that Build requires into the interface that declares the
role, to give the authoring API one write path per effect, and to keep a built
plugin unchanged. The changes break the SPI and the authoring API. The SDK has
no tagged release, so no consumer pays a major version for them.

## Motivation

### Build refuses at run time a type that the compiler accepts

The workspace package makes 36 interface assertions outside its tests. Seventeen
of them assert a method that every plan calls, so a type without the method
compiles and then fails Build:

| Interface | Required by | Assertions |
|---|---|---|
| `plugin.Renderer`, `FileSpeller`, `SyntaxProvider` | every plan that writes output | 6 |
| `plugin.TypeSpeller`, `Packager`, `PolicyProvider` | every plan of the backend | 5 |
| `plugin.Respeller` | the settle of every plan | 1 |
| `plugin.Versioned` on a frontend | the key of every unit | 3 |
| `output.Overwriter` | every sink that writes over a drifted file | 1 |
| `ledger.Locker` | one writer per workspace | 1 |

`plugin.Settle` asserts `Lowerer` and `Respeller` once more. The run asserts two
of the backend's interfaces again, with error paths that Build made
unreachable. Two load sites discard the result of the `Versioned` assertion with
the comment "asserted when the load began". The kit returns `plugin.Backend`,
which declares `Target` alone, although every backend that the kit builds
implements all the methods above. The backend suite fails a renderer that
does not implement `render.Coverer`, which no interface requires.

The authoring API has the same defect. `Builder.Build` returns `plugin.Plugin`.
The caller asserts the role. Twenty-four production sites and 68 test sites
write `Build().(plugin.Generator)` or `Build().(plugin.Annotator)`. Twenty-two of
the production sites discard the second result. A rule set of the wrong effect
then yields a nil plugin without a compiler error.

### Writes that bypass the effects

`Emitter.Slot(s).Append` buffers a write with its invocation, applies it in
canonical match order when the phase call ends, and names the plugin among the
contributors of the unit. `emit.Slot.Append` writes at once and names nobody.
Both compile inside an `OnEmit` handler, which receives the value of another
plugin. A direct write from matches that run in parallel is a data race. The
weaver example of the authoring RFC uses the direct path.

A parameter, a return, an embed and a struct tag are plain fields of the emit
model. The doc comment of `Field.Tag` tells a later plugin to reshape the tag
through `OnEmit`, which is a direct write. The header of a generated file then
does not list the weaver.

A handler can also read a fact without recording the read: `rules.View` exports
the run's raw fact store, and the read set misses every read through it.

### A built plugin changes after Build

Workspace Build writes the ids of named key handles into the predicates of a
built plugin. A plugin instance then belongs to one workspace, and every
reference constructor returns a new instance. The authoring document describes
a plugin as a value that is built once.

### Why the kernel, and why one RFC

Every item changes an exported identifier of the kernel or of the SDK facade
that re-exports it. A consumer cannot work around a contract that the kernel
does not declare, and a change of one interface changes every satellite. One
RFC gives the review one view of the whole break, and one release contains it.

## Detailed design

### Invariants

1. A type that satisfies an interface of the kernel passes every check that
   Build makes on the role of that interface.
2. The workspace asserts an interface only for a role that a plugin may lack.
3. Every write of a plugin into a value that the plan's store contains is
   buffered with its invocation, applies in canonical match order and names the
   plugin among the contributors of its unit.
4. Every fact read of a plugin goes through a read that the read set records.
5. A built plugin does not change after Build.

### Components

| Package | Change | Items |
|---|---|---|
| `core/plugin` | `Backend` declares the methods of a plan. `Frontend` embeds `Versioned`. `KeyBinder` returns a bound plugin. `CapabilityProvider` gains `After`. `ContractVersion` | W4.1, W4.2, W4.15, W4.17, W4.19 |
| `core/backend` | `Build` returns the kit's `Backend`. One backend type replaces four | W4.1, W4.6, W4.7, W4.9 |
| `core/output` | `Sink` declares `Overwrite`. `Overwriter` goes | W4.3 |
| `core/ledger` | `Ledger` declares `Lock`. `Locker` goes | W4.4 |
| `core/workspace` | Build refuses a ledger without an output and a partial provider. 17 assertions go | W4.5, W4.8, W4.16 |
| `core` | One typed build per role. `GraphMatch` reports through `ErrorfAt`, `WarnfAt` and `InfofAt`. `JoinName` names its positions | W4.10, W4.24, W4.25 |
| `core/emit` | A slot of a value in the store refuses a direct append. Tags, embeds, parameters and returns are slots | W4.11, W4.12 |
| `core/rules` | `View` hides its fact store. `Bound` reaches every optional capability. The kernel substitutes. `SourceRules.SamplesOf` becomes `DeriveSamples` | W4.13, W4.20, W4.21, W4.22 |
| `eidos-sdk` | Regenerated facade | all |

### The backend contract

`plugin.Backend` declares every method that a plan calls on its backend:

```go
// Backend renders the files of one target. A plan calls every method of
// the interface, so a type that implements Backend passes the checks that
// workspace Build makes on the role. The kit in package backend builds a
// Backend from declared parts, and an exotic backend implements it
// directly.
type Backend interface {
    Plugin
    // Target is the target that the backend renders.
    Target() Target
    Renderer
    FileSpeller
    SyntaxProvider
    TypeSpeller
    Packager
    PolicyProvider
    Lowerer
    Respeller
}
```

The kit declares its own interface, so the backend suite can require the
coverage and the refusals that only a kit backend has:

```go
// Backend is a backend that the kit built: the SPI's backend, the fact
// coverage that the render's guard reads, and the kinds that the language
// refuses.
type Backend interface {
    plugin.Backend
    render.Coverer
    render.Refuser
}

// Build lowers the declaration to a Backend. It panics on a declaration
// defect, as it does now.
func (b *Builder) Build() Backend
```

One unexported type implements the kit's `Backend`, and the types
`loweringBackend`, `respellingBackend` and `settlingBackend` go. A method whose
part the backend does not declare behaves as follows:

| Method | Without a declared part |
|---|---|
| `Lower` | returns the declaration alone |
| `Respell` | returns the name unchanged |
| `SpellType` | returns an error that the target does not spell the types of other languages |
| `PackageAt` | returns the origin package of the file |
| `Policies` | returns nil |

`plugin.Settle` calls `Lower` and `Respell` without an assertion, so a backend
without hooks walks the store once more in the settle. All four
language backends declare both hooks, so only test backends take that walk.
`backendtest` takes the kit's `Backend`, and the suite stops asserting
`render.Coverer`.

### The frontend contract

`plugin.Frontend` embeds `Versioned`. Its other methods do not change. The
check in `frontendFaults`, the check in `load.Load` and the two discarded
assertions go, because the compiler refuses a frontend without a version.

### The sink contract

`output.Sink` declares `Overwrite`, and `output.Overwriter` goes. The disk sink,
the memory sink and the tee implement it already.

```go
type Sink interface {
    Write(path string, body []byte) error
    Delete(path string) error
    // Overwrite lets Commit write over the files of the verdicts given,
    // FoundDrifted, FoundForeign or both.
    //
    // Error modes: a verdict other than FoundDrifted and FoundForeign, a
    // call after Prepare, and ErrFinished after Commit or Discard.
    Overwrite(found ...Found) error
    Prepare() ([]Change, error)
    Commit() ([]Written, error)
    Discard() error
}
```

### The ledger contract

The lock moves into `ledger.Ledger`, and `ledger.Locker` goes. `Dir` and `Mem`
implement it already, so the workspace takes the lock of every ledger and the
branch that runs without one goes:

```go
// Lock takes the ledger's lock for h, and returns the function that
// releases it. Lock does not wait: where another holder has the lock, it
// returns a *LockedError that names that holder.
//
// Error modes: a *LockedError, which wraps ErrLocked, the context's error,
// and the operating system's error where the lock's files cannot be
// created, opened or written.
Lock(ctx context.Context, h Holder) (release func() error, err error)
```

`GroupWriter` remains an optional interface. A ledger without it writes each
blob of a group alone, which costs time and loses nothing.

A composition that declares a ledger and no output never opens the ledger,
because the lock, the record and the parse memo all need a run with an output.
Build refuses the combination with a fault that states both options.

### The assertions that remain

After the changes above, the workspace asserts only roles that a plugin may
lack:

| Interface | Assertions |
|---|---|
| `plugin.CapabilityProvider` | 4 |
| `plugin.TemplateProvider` | 3 |
| `plugin.Annotator`, `plugin.Generator` | 4 |
| `plugin.KeyProvider` | 2 |
| `plugin.OutputProvider`, `OptionsProvider`, `KeyBinder`, `DirectiveProvider`, `Subscribed`, `StoreLocator` | 6 |

That is 19 assertions of 36.

### A typed build per role

`Builder.Build` goes. Three methods build the plugin as one role each, and each
panics at construction where the rules do not match the role:

```go
// Generator builds the plugin as a generator. It panics where a rule
// stamps facts or where no rule emits, because such a plugin is not a
// generator alone.
func (b *Builder) Generator() plugin.Generator

// Annotator builds the plugin as an annotator. It panics where a rule
// emits or where no rule stamps.
func (b *Builder) Annotator() plugin.Annotator

// Dual builds a plugin that annotates and generates. It panics unless
// rules of both effects exist.
func (b *Builder) Dual() Dual

// Dual is a plugin with an annotator half and a generator half, each with
// its own priority.
type Dual interface {
    plugin.Annotator
    plugin.Generator
}
```

The 24 production sites become `g := sdk.NewPlugin(ID)...Generator()`, and the
68 test sites change the same way. `Build` keeps no deprecated alias, because
the SDK has no tagged release and an alias keeps the discarded assertion
compiling.

### One write path into the value of another plugin

A slot of a value that the plan's store contains refuses a direct append:

```go
// Append adds values to the end of the slot. It panics on a slot of a
// value that the plan's store contains, because a write there goes through
// the slot view of the emitter, which buffers it and names its writer.
func (s *Slot[T]) Append(values ...T)
```

The store seals every slot of a declaration when the phase call that placed it
applies its effects. The seal is one flag that the store shares with every slot
of the values that it contains. The store sets the flag while the phase calls of
a plan run, and clears it when the settle starts, so a backend's `Lower` and
`Respell` hooks rewrite the declarations of their target. `Slot` gains
`Replace`, which a lowering that rewrites a list calls, and which panics on a
sealed slot as `Append` does:

```go
// Replace sets the contents of the slot to values. It panics on a sealed
// slot, as Append does.
func (s *Slot[T]) Replace(values ...T)
```

The emitter applies a buffered append through
`emit.AppendSealed`, which the documentation reserves to the kernel and which
the repository's forbidigo rule refuses outside `core`. A handler that builds a
value of its own appends to it directly, as every plugin of the reference
ensemble does now. A direct append into a sealed slot panics instead of
reporting a finding, because a write that the store keeps without its writer
breaks the header of the file either way.

### Slots for tags, embeds, parameters and returns

The emit schema turns four fields into slots:

| Kind | Field now | Field proposed |
|---|---|---|
| `Field` | `Tag string` | `Tags Slot[*TagEntry]` |
| `Struct`, `Interface` | `Embeds []*Embed` | `Embeds Slot[*Embed]` |
| `Function`, `Method` | `Params []*Param` | `Params Slot[*Param]` |
| `Function`, `Method` | `Returns []*Return` | `Returns Slot[*Return]` |

```go
// TagEntry is one entry of a struct tag: a key and its value, which a
// target joins into its tag syntax, such as json:"name,omitempty" in Go.
type TagEntry struct {
    Key   string `json:"key,omitzero"`
    Value string `json:"value,omitzero"`
}
```

A weaver appends through `Emitter.Slot`, so the header of the file names it. A
backend template reads `.Params.Items` and `.Returns.Items`, as it reads
`.Fields.Items` now. The Go backend joins the entries of `Tags` into the text of
the tag. The other three targets refuse a tag as they do now.

### One tracked fact read

`rules.View` keeps its fact store, its read set and the kernel's keys in
unexported fields, which `rules.NewView` sets. A plugin reads a fact through the
match, with `Fact` for its subject and `FactOf` for another identity. A rules
implementation reads a fact through `rules.Fact` with a handle, or through
`rules.FactNamed` by the name of the key, on the view that it receives. Every
one of these reads records into the read set.

```go
// NewView returns a view over decls and facts, whose fact reads record
// into reads. A nil reads reads untracked, which only the kernel's own
// walks do.
func NewView(decls *store.Reader, facts *meta.Facts, reads meta.Recorder, kernel meta.KernelKeys) View
```

### A plugin that does not change after Build

`KeyBinder.BindKeys` returns a bound plugin, and the workspace keeps that value:

```go
// KeyBinder binds the named key handles of a plugin to the registry of one
// workspace. BindKeys returns a plugin with the bound ids, and leaves the
// receiver unchanged, so one built plugin serves any number of workspaces.
//
// Error modes: one error for each name that r does not contain under the
// value type of its handle.
type KeyBinder interface {
    BindKeys(r *meta.Registry) (Plugin, error)
}
```

A constructor can then return one shared instance. The reference constructors
keep returning a new one, because nothing requires a change there.

### Partial providers

Build reports a plugin whose method set contains part of a provider interface,
and names the methods that it lacks. A plugin with `Priority` and `Provides` and
without `Requires` then fails Build, where the workspace now reads none of the
three. The check covers the providers of more than one method,
`CapabilityProvider` and `TemplateProvider`. It compares the name and the
signature of each method through `reflect.Type.MethodByName`.

### The contract version

```go
// ContractVersion is the version of the contract between the kernel and
// its components: the SPI, the authoring API and the formats that cross
// the binary boundary. A change that breaks a component raises it.
const ContractVersion = 1
```

Go links one version of the kernel into a binary, so a compiled component
cannot have another contract version. The handshake checks what crosses the
binary boundary:

- The declarative manifest states `contract`, and the loader refuses a manifest
  of another version with a fault that states the plugin and both versions. A
  manifest that states no contract reads as version 1, the first version.
- The header of the sealed state records the version, and a run over a state of
  another version runs cold.
- `version` prints it.

### Ordering apart from presence

`Requires` states that a capability must be present and must run first.
`After` states only the order, for a capability that may be absent:

```go
type CapabilityProvider interface {
    Priority(r Role) int
    Provides() []Capability
    Requires() []Capability
    // After returns the capabilities whose providers run before the
    // plugin where the composition contains them. Build checks no
    // presence for them.
    After() []Capability
}
```

The builder gains `After(caps ...Capability)`. debugweaver weaves the methods
that repogen and mockgen placed, and it does not need either plugin to be
present. It
declares `After(repogen.Capability, mockgen.Capability)` in place of `Requires`,
so a composition of TypeScript plans does not need either plugin.

### One access path for optional rules

`Bound` gains a method for each optional capability that a plugin asserts now
through `m.Rules().Source().(rules.X)`, each with an ok result:

```go
// Tag returns the value of a field's tag under key, and reports false
// where the field has none or the language does not declare tags.
func (b Bound) Tag(f *node.Field, key string) (string, bool)

// SentinelName returns the name of the error value that a generator
// derives from base, and reports false where the language declares no
// error values.
func (b Bound) SentinelName(base string) (string, bool)

// IsSentinelName reports whether an identifier follows the language's
// convention for an error value, and false where the language declares
// no error values.
func (b Bound) IsSentinelName(ident string) bool
```

`Source` stays for the required `SourceRules`. A plugin then asserts no rules
capability.

### Substitution in the kernel

A reference to a type parameter targets the parameter, so the kernel can
substitute without a language predicate. `GenericsRules.Substitute` goes, and
`rules.Substitute` walks the references:

```go
// Substitute returns ref with each reference to a parameter of params
// replaced by its argument in args, copying what it rewrites. A reference
// whose target is not one of params returns unchanged.
func Substitute(ref *node.TypeRef, params []*node.TypeParam, args []*node.TypeRef) *node.TypeRef
```

The member walk and `Witnesses` bind generic contributors without an
assertion, and the reference builder writes its checks for a struct without
type parameters on every language.

### Derived samples under their own name

`SourceRules.SamplesOf` becomes `DeriveSamples`. `Bound.SamplesOf` keeps its
name and reads authored values first, so one name has one meaning.

### The positioned reports of a graph match

A graph match has no subject, so every report has a position. Its methods
take the names that the other matches give their positioned forms:
`ErrorfAt`, `WarnfAt` and `InfofAt`. `Errorf`, `Warnf` and `Infof` go from
`GraphMatch`.

### The positions of JoinName

`JoinName` keeps its order, and its parameters take the names of their
positions:

```go
// JoinName returns head followed by tail, with the first letter of tail
// raised, so the boundary between them survives: JoinName("stub", "store")
// returns storeStub.
func (*Emitter) JoinName(tail, head string) string
```

Every call site compiles unchanged, because both parameters are strings in the
same order.

### Migration

| Consumer | Change | Sites |
|---|---|---|
| Plugins of the reference ensemble, the shape catalog and the declarative host | `Generator()`, `Annotator()` or `Dual()` in place of `Build()` with an assertion | 24 |
| Tests that build plugins | the same | 68 |
| Hand-written backends in tests | embed a kit backend, or implement the nine methods | 8 |
| Templates that read parameters and returns | `.Params.Items`, `.Returns.Items` | the four languages' kind templates |
| Generators that set parameters, returns, embeds and tags | `Append` on the slot | the reference ensemble and the declarative host |
| debugweaver | `After` in place of `Requires` | 1 |

### Failure handling

| Failure | Where it shows |
|---|---|
| A backend, frontend, sink or ledger without a required method | the compiler |
| A plugin built as the wrong role | a panic in its constructor, at the first Build of any test |
| A direct append into a value of the store | a panic in the handler, at the first run of any test |
| A partial provider | a Build fault that names the missing methods |
| A ledger without an output | a Build fault that names both options |
| A declarative manifest of another contract version | a load fault that names the plugin and both versions |

## Alternatives considered

### Keep the optional interfaces and document them

Each optional role keeps its own interface, and the documentation lists what
Build requires.

**Why not:** the defect is that the compiler accepts a type that Build refuses.
Documentation does not move the refusal to the compiler, and 17 assertions with
their unreachable error paths stay.

### A capability set on each component

A component returns a bit set of the roles that it implements, and the
workspace dispatches on the set.

**Why not:** a set that a method returns is a second statement of what the
method set already states, and the two can disagree. The compiler checks the
method set alone.

### A builder that is generic over the effect

`NewPlugin[E Effect](name)` returns a `Builder[E]`, whose `Build` returns the
role of `E`.

**Why not:** a dual plugin has rules of both effects, so it needs a second
builder type or two builders that share a name. Three methods on one builder
cover the three roles with one set of declaration methods.

### Detect a direct append after the phase call

The dispatcher records the lengths of the slots of each value that it hands to
an `OnEmit` handler, and reports a slot that grew without a buffered append.

**Why not:** the record costs a pass over every matched value per phase call,
and the finding arrives after the race. A sealed slot panics at the write.

## Drawbacks

- Every hand-written backend implements nine more methods. Eight test doubles in
  seven files change.
- The four slots change the emit schema: the four languages' kind templates, the
  vocabulary helpers that take parameters and returns, and every generator that
  sets one of the four fields.
- A backend without hooks walks the store once more in the settle.
- `Build` goes from the authoring API, so 92 sites change, 68 of them in tests.
- `emit.AppendSealed` is an exported function that only the kernel may call.
  The forbidigo rule enforces that inside the repository, and nothing enforces
  it in a consumer's module.

## Unresolved and future work

- A contract version for a component compiled into another binary is not
  proposed here, because Go links one kernel per binary.
- A read-only view of the value that an `OnEmit` handler receives is not
  proposed here. The emit kinds have exported fields, so Go cannot express it.

## References

| What | Where |
|---|---|
| D42, one writer per workspace | ../architecture/21-decisions.md |
| D93, a reference to a type parameter targets the parameter | ../architecture/21-decisions.md |
| The handshake as a step of workspace Build | ../architecture/08-workspace-and-plans.md |
| One contract version across every registered component | ../architecture/15-compatibility.md |
| A plugin as a value built once | ../architecture/06b-authoring.md |
| forbidigo | https://github.com/ashanbrown/forbidigo |
