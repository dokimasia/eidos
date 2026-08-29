# The symbol model

*Builds on: [01](01-repos-and-kernel.md). Feeds:
[03](03-projection.md) (projections read symbols),
[07](07-rendering.md) (slots are declared in the schema),
[09](09-incrementality.md) (identity keys the engine).*

eidos carries two concrete declaration models. `node` is what a
frontend produces from source. `emit` is what generators produce for
backends to render. They share one vocabulary and differ in what
wraps it: node adds the read side (resolver, queries, freeze), emit
adds the write side (slots, targets, bodies). Nobody writes either
by hand. Both generate from one schema, which is what makes two
models cheap to keep and impossible to let diverge.

## symbol/: the vocabulary

Hand-written, small, and slow to change:

- **One `Kind` enum.** Every consumer that switches on kind uses one
  set of names across both models.
- **The walk interfaces**: `Symbol` (kind, position, docs, meta),
  `Membered` (access to fields, methods and embeds), `Typed` and
  `Documented`. Everything neutral is written once against these:
  walkers, differs, JSON codecs, doc audits, diagnostic printers.
- **`schema/`**, which defines every declaration kind as plain Go
  structs, annotated for per-side behaviour:

  ```go
  type Struct struct {
      Name    string     `eidos:"both"`
      Fields  []*Field   `eidos:"both,walk,slot=fields"`
      Methods []*Method  `eidos:"both,walk,slot=methods,owner"`
      Extends []*TypeRef `eidos:"both,walk"`
      // ...
  }
  ```

  The annotation vocabulary is closed. `both|node|emit` places a
  field on one side or both. `walk` includes it in the generated
  traversal. `owner` makes it a rewire target. `slot=<name>`
  declares the emit-side slot ([07-rendering.md](07-rendering.md)).
  The generator rejects an annotation it does not know, so the
  vocabulary grows by kernel decision rather than by typo.

## node/ and emit/: generated supersets

A generator reads `symbol/schema` with `go/ast` and `go/types`, then
writes, for both sides: the concrete kind structs, `Walk`,
`RewireOwners`, JSON encoding and the mirror guards. The output is
committed. Consumers never run the generator, and changing the model
costs a schema edit plus a regeneration. Editing a generated file
fails CI, because the mirror guard reruns the tool and diffs the
tree.

**The generator is deliberately not an eidos workspace.** It would
need a Go frontend and backend, which live in `eidos-lang-go`, a
satellite that depends on the kernel. Kernel code generation
depending on a satellite would invert the layering, since the kernel
knows no language even at development time, and it could never
bootstrap: the first regeneration would need a satellite that does
not exist yet. So the tool is `internal/gen` inside the kernel,
plain `go/ast` plus text/template, with no eidos dependencies, small
and stable on purpose. The framework does build itself with itself,
but only where the dependency direction allows it:
eidos-plugin-shape generates its registries from specs through a
real eidos workspace ([12-shape-catalog.md](12-shape-catalog.md)).

Hand-written rather than generated:

- `node/`: the resolver, the query surface (`Interfaces()`,
  `InterfaceByName`, and so on), and **freeze**. After the annotator
  phase the store seals, and a structural write is refused with a
  stable diagnostic code
  ([08-workspace-and-plans.md](08-workspace-and-plans.md)). The
  query surface stays flat. The derived question, a type's effective
  member set across embeds and supertypes, is the `Members`
  projection ([03-projection.md](03-projection.md)), because
  resolving it means applying the language's rules.
- `emit/`: the slot machinery, `Target`, the `Ref` variants, `Expr`
  and `Stmt`, samples, provenance, and the typed builder API
  ([07-rendering.md](07-rendering.md)). Which slots each emit kind
  carries, including the standard body slots, is declared in the
  schema and generated with the kinds.

## The kind inventory

Package, File, Import, Struct, Interface, Method, Field, Function,
Param, Return, Variable, Constant, Enum, EnumVariant, Sum,
SumVariant, Alias, TypeParam, TypeRef, Embed, Constraint.

Struct and Interface differ by whether you can instantiate them, not
by which members they may declare. A Struct is a type you make
values of. An Interface is a shape you check values against. That
line holds across the whole landscape, because a class can be
constructed and an interface cannot. Both kinds carry fields and
methods, because interfaces in several languages are mostly
properties: a TypeScript `interface` body usually holds fields. One
source construct maps to one kind, always, whatever its body
contains. A plugin asking "what contracts are there" finds every
interface by kind, and never by filtering another kind on a metadata
key.

The model was sized against the language landscape up front, and
[11-languages.md](11-languages.md) names the languages that forced
each addition:

| Model element | Content |
|---|---|
| `TypeParam.Variance` | `Invariant \| In \| Out` (Kotlin, C# and Java wildcards; Go and Rust answer Invariant) |
| `Struct.Extends` / `Struct.Implements` | nominal supertypes, kept separate from `Embeds`, which is compositional promotion. Go fills only Embeds, JVM languages only the former, Python fills Extends in MRO order |
| `Sum` kind | tagged variants with payloads: Rust data enums, sealed classes, proto `oneof`. Distinct from the untagged `Union` type shape. Each `SumVariant` carries a name and a field list, and the projection rule goes by payload: a variant set with no payloads projects as `Enum`, one with any payload as `Sum` |
| `Visibility` | normalized to `Public \| Package \| Protected \| Private \| Internal` on every declaration. The raw spelling stays in language metadata |
| `Level` on members | `Instance \| Type`, covering statics and companions. Go always answers Instance |
| `Method.HasDefault` | interface methods with default bodies: Java 8 and later, Kotlin, Rust default impls. Go leaves it false |
| `Enum.Methods` | Java enums are classes with members. Most languages leave it empty |
| `Package.Path` | hierarchical segments. Rust `mod` nesting and TypeScript namespaces map into them, and the original spelling stays in language metadata |

## Symbol identity

Every symbol has a **canonical identity**: package path, kind, name,
and a signature discriminator. Overloads differ by the
discriminator, and a language without overloads never produces two.

Identity survives reparsing. A reloaded file yields symbols with the
same identities wherever the declarations did not change. Everything
that refers to symbols across time keys on it: read sets and
(symbol, key) invalidation
([09-incrementality.md](09-incrementality.md)), plan exports,
manifest provenance, drift records and `explain`. Pointer identity
is a detail local to one run. The canonical identity is the
contract.

## Resolution: Link, then Lookup

How anything crosses a file or package boundary:

- **Link** is a kernel step at the end of Load, before Freeze
  ([08-workspace-and-plans.md](08-workspace-and-plans.md)). Once
  every frontend has finished, spellings resolve once: each
  `TypeRef` that names a declaration present in the graph, whether
  in scope or in a signature-only dependency, gains a
  `Target symbol.Identity`. Builtins and genuinely external types
  keep only their spelling. That is legitimate degradation rather
  than failure, and a consumer asks before relying on a target.

  Link is a phase rather than an on-demand lookup because
  cross-package resolution needs the whole graph: two packages parse
  in parallel, and neither can resolve into the other until both
  exist. The language helps, since the frontend recorded each file's
  import scope and the language's `Resolve`
  ([03-projection.md](03-projection.md)) knows what a spelling means
  there, but the result is neutral: an identity.
- **Lookup** is the join, in any phase after Link.
  `Reader.Lookup(Identity) (Symbol, ok)` answers the same way across
  files, packages and signature-only dependencies, and it goes
  through the tracked Reader, so a cross-package read becomes an
  incrementality edge like any other
  ([09-incrementality.md](09-incrementality.md)).
- **Textual resolution**, meaning a human's spelling in a directive
  reference param, is Tier-1 `rules.Resolve` at validation time.
  Names are what humans type; identities are what the system holds.
- **Cross-plan resolution does not exist.** Plans share this graph.
  They exchange generated artifacts only through exports
  ([08-workspace-and-plans.md](08-workspace-and-plans.md)).

## The Reader

Every phase context and every Match carries one: the tracked,
scope-filtered read handle, and the only path a read takes. The
ledger's `Track` provides it
([08-workspace-and-plans.md](08-workspace-and-plans.md)). The
surface, pinned:

```go
type Reader interface {
    ByKind(k symbol.Kind) iter.Seq[Symbol]     // enumeration
    Lookup(id symbol.Identity) (Symbol, bool)  // the join, post-Link
    PackageOf(id symbol.Identity) (*Package, bool)
    // Projections enter through Rules(), facts through Fact()
    // (06b-authoring.md). Every path records per the rule below.
}
```

**What a read records**, which is the grain invalidation runs on:

- A **targeted** read records a per-identity edge. That covers
  `Lookup`, walking a symbol's members, and reading a fact through
  `Fact`, where facts record at the (symbol, key) grain
  ([09-incrementality.md](09-incrementality.md)).
- An **enumeration** records a set-membership edge. That covers
  `ByKind` and a package's declaration list. The reader runs again
  when a symbol of that kind or package is added or removed, and
  never when one merely changes. Sensitivity to changes comes from
  the per-identity edges the enumerator recorded for the symbols it
  actually touched while iterating.

The economics match the gate-free handler law, extended to reads. A
targeted read is a cheap precise edge, and an enumeration honestly
prices "I asked for the whole set" as sensitivity to membership. No
read costs more than what it observed. Scope filtering composes,
because recorded edges stay inside the Reader's scope, so a change
outside a plan's scope cannot re-run a plan that could never have
seen it.

## Model laws

- **Admit with empties.** A kind carries what any language in scope
  needs, other languages leave the rest empty, and emptiness never
  claims anything.
- **Member lists are slices, never maps keyed by name.** Java,
  Kotlin, C# and TypeScript overload, and a model keyed by name
  cannot hold their declarations. Languages that forbid overloads
  simply never produce two entries.
- **Origin points one way.** An emit symbol links to the node symbol
  it came from, and a node symbol never refers to emit. The read
  side cannot observe the write side.
- Adding a kind or a field is a schema edit and nothing else.

## Why two models

The split does two jobs.

First, **the read side is frozen**. A generator cannot append into a
source struct, because the source model has no slots. The compiler
enforces the phase discipline instead of a convention.

Second, **N plans share one node graph**
([08-workspace-and-plans.md](08-workspace-and-plans.md)). If emit
borrowed node symbols by reference, one plan's generator changing a
shared symbol would corrupt a sibling plan's output in ways that
vary between runs. With separate models that bug does not compile.

The alternatives fail on inspection. A single merged model has to
serve two masters and serves neither: parsed nodes carry real
positions and comments, synthesized nodes have neither, and every
rewrite pass then has to fake positions and re-attach comments. A
base type with embedding fails structurally instead, because
`node.Struct` needs `Fields []*node.Field` and Go embedding cannot
specialize element types down a mutually recursive graph. The ways
out of that, interface soup with type assertions at every use site
or type parameters threaded through the whole graph, are both worse
for plugin authors than two concrete models. Two generated models
cost a schema and a regeneration. Every alternative costs an
invariant.
