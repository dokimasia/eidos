# The symbol model

*Builds on: [01](01-repos-and-kernel.md). Feeds:
[03](03-projection.md) (projections read symbols),
[07](07-rendering.md) (slots are schema-declared),
[09](09-incrementality.md) (identity keys the engine).*

eidos carries two concrete declaration models: `node`, what a
frontend produces from source, and `emit`, what generators produce
for backends to render. They share one vocabulary and differ by
envelope — node adds the read side (resolver, queries, freeze), emit
adds the write side (slots, targets, bodies). Neither is written by
hand: both generate from a single schema, which is what makes two
models cheap to keep and impossible to let drift.

## symbol/ — the vocabulary

Hand-written, small, slow-moving:

- **One `Kind` enum.** Every consumer that switches on kind shares
  one set of names across both models.
- **The walk interfaces**: `Symbol` (kind, position, docs, meta),
  `Membered` (fields/methods/embeds access), `Typed`, `Documented`.
  Everything neutral — walkers, differs, JSON codecs, doc audits,
  diagnostic printers — is written once against these.
- **`schema/`** — the definition of every declaration kind as plain
  Go structs, annotated for per-side behavior:

  ```go
  type Struct struct {
      Name    string     `eidos:"both"`
      Fields  []*Field   `eidos:"both,walk,slot=fields"`
      Methods []*Method  `eidos:"both,walk,slot=methods,owner"`
      Extends []*TypeRef `eidos:"both,walk"`
      // ...
  }
  ```

  The annotation vocabulary is closed: `both|node|emit` places a
  field per side; `walk` includes it in the generated traversal;
  `owner` makes it a rewire target; `slot=<name>` declares the
  emit-side slot ([07-rendering.md](07-rendering.md)). The generator
  rejects an unknown annotation — the vocabulary grows by kernel
  decision, not by typo.

## node/ and emit/ — generated supersets

A generator reads `symbol/schema` with `go/ast`/`go/types` and
emits, for both sides: the concrete kind structs, `Walk`,
`RewireOwners`, JSON encoding, and the mirror guards. Output is
committed; consumers never run the generator, and a schema edit plus
regen is the entire cost of a model change. Editing generated files
is a CI failure — the mirror guard reruns the tool and diffs the
tree.

The generator is **deliberately not an eidos workspace**. It would
need a Go frontend and backend, which live in `eidos-lang-go` — a
satellite that depends on the kernel. Kernel codegen depending on a
satellite inverts the layering (the kernel knows no language, even
at dev time) and cannot bootstrap: the first regen would need a
satellite that doesn't exist yet. So the tool is `internal/gen` in
the kernel — plain `go/ast` plus text/template, zero eidos
dependencies, small and stable by design. The framework's
self-hosting story lives where the dependency direction is legal:
eidos-plugin-shape generates its registries from specs through a
real eidos workspace ([12-shape-catalog.md](12-shape-catalog.md)).

Hand-written and not generated:

- `node/`: the resolver, the query surface (`Interfaces()`,
  `InterfaceByName`, …), and **freeze** — after the annotator phase
  the store seals and structural writes are refused with a stable
  diagnostic code
  ([08-workspace-and-plans.md](08-workspace-and-plans.md)). The
  query surface stays flat; the *derived* question — a type's
  effective member set across embeds and supertypes — is the
  `Members` projection ([03-projection.md](03-projection.md)),
  because its resolution rules are the language's.
- `emit/`: the slot machinery, `Target`, the `Ref` variants,
  `Expr`/`Stmt`, samples, provenance, and the typed builder API
  ([07-rendering.md](07-rendering.md)). Slot *declarations* — which
  slots each emit kind carries, including the standard body slots —
  are schema annotations and generate with the kinds.

## The kind inventory

Package, File, Import, Struct, Interface, Method, Field, Function,
Param, Return, Variable, Constant, Enum, EnumVariant, Sum,
SumVariant, Alias, TypeParam, TypeRef, Embed, Constraint.

What separates Struct from Interface is **instantiability**, not
which members each may declare: a Struct is a type values are made
of; an Interface is a shape values are checked against. That line
holds across the whole landscape — a class can be constructed, an
interface cannot — and both kinds carry fields and methods under
admit-with-empties, because interfaces in several languages are
property-majority (a TS `interface` body is mostly fields). One
source construct maps to one kind, stably, regardless of what its
body contains; a plugin asking "what contracts are there" finds
every interface by kind, never by filtering another kind on a
metadata key.

The model is sized against the language landscape up front
([11-languages.md](11-languages.md) names the forcing languages):

| Model element | Content |
|---|---|
| `TypeParam.Variance` | `Invariant \| In \| Out` (Kotlin/C#/Java wildcards; Go and Rust answer Invariant) |
| `Struct.Extends` / `Struct.Implements` | nominal supertypes, distinct from `Embeds` (compositional promotion). Go fills only Embeds; JVM languages only the former; Python fills Extends in MRO order |
| `Sum` kind | tagged variants with payloads (Rust data enums, sealed classes, proto `oneof`) — distinct from the untagged `Union` type shape. Each `SumVariant` carries a name and a field list; the projection rule is by payload: a variant set with no payloads projects as `Enum`, with any payload as `Sum` |
| `Visibility` | normalized `Public \| Package \| Protected \| Private \| Internal` on every declaration; the raw spelling stays in language metadata |
| `Level` on members | `Instance \| Type` (statics, companions). Go answers Instance always |
| `Method.HasDefault` | interface methods with default bodies (Java 8+, Kotlin, Rust default impls). Go leaves false |
| `Enum.Methods` | Java enums are classes with members; most languages leave it empty |
| `Package.Path` | hierarchical segments (Rust `mod` nesting and TS namespaces map into them; original spelling in language metadata) |

## Symbol identity

Every symbol has a **canonical identity**: package path, kind, name,
and a signature discriminator (overloads differ by it; a
discriminator-free language never produces two). Identity survives
reparsing — a reloaded file yields symbols with the same identities
wherever the declarations are unchanged — and everything that
references symbols *across time* keys on it: read-sets and
(symbol, key) invalidation
([09-incrementality.md](09-incrementality.md)), plan exports,
manifest provenance, drift records, `explain`. Pointer identity is a
run-local detail; the canonical identity is the contract.

## Resolution: Link, then Lookup

How anything crosses a file or package boundary, split along the
boundary-strings seam:

- **Link** is a kernel step at the end of Load, before Freeze
  ([08-workspace-and-plans.md](08-workspace-and-plans.md)). Once
  every frontend has finished, spellings resolve once: each
  `TypeRef` naming a declaration present in the graph — in-scope,
  or a signature-only dependency — gains a
  `Target symbol.Identity`. Builtins and truly-external types stay
  spelling-only, which is legitimate degradation, not failure; a
  consumer asks before relying. Link is a phase rather than an
  on-demand lookup because cross-package resolution needs the whole
  graph: two packages parse in parallel, and neither can resolve
  into the other until both exist. The resolution is
  language-assisted — the frontend recorded each file's import
  scope, and the language's `Resolve`
  ([03-projection.md](03-projection.md)) knows what a spelling
  denotes there — but the result is neutral: an identity.
- **Lookup** is the join, any phase after Link:
  `Reader.Lookup(Identity) (Symbol, ok)` answers across files,
  packages, and signature-only dependencies identically, and flows
  through the tracked Reader — a cross-package read becomes an
  incrementality edge like any other read
  ([09-incrementality.md](09-incrementality.md)).
- **Textual resolution** — a human's spelling in a directive
  reference param — is Tier-1 `rules.Resolve`, at validation time.
  Names are boundary strings; identities are the interior.
- **Cross-plan resolution does not exist.** Plans share this whole
  graph; they exchange *generated* artifacts only through exports
  ([08-workspace-and-plans.md](08-workspace-and-plans.md)).

## The Reader

Every phase context and every Match carries one: the tracked,
scope-filtered read handle — the only path reads take (the
ledger's `Track`,
[08-workspace-and-plans.md](08-workspace-and-plans.md)). The
surface, pinned:

```go
type Reader interface {
    ByKind(k symbol.Kind) iter.Seq[Symbol]     // enumeration
    Lookup(id symbol.Identity) (Symbol, bool)  // the join, post-Link
    PackageOf(id symbol.Identity) (*Package, bool)
    // Projections enter through Rules(), facts through Fact()
    // (06b-authoring.md) — every path records per the rule below.
}
```

**What a read records** — the grain rule invalidation lives on:

- A **targeted** read — `Lookup`, a symbol's members walked, a
  fact via `Fact` — records a per-identity edge (facts at the
  (symbol, key) grain,
  [09-incrementality.md](09-incrementality.md)).
- An **enumeration** — `ByKind`, a package's declaration list —
  records a set-membership edge: the reader re-wakes when a
  symbol of that kind or package is *added or removed*, never
  when one merely mutates. Mutation sensitivity comes from the
  per-identity edges the enumerator recorded for the symbols it
  actually touched while iterating.

The economics are the gate-free handler law's, extended to reads:
targeted reads are cheap precise edges, and an enumeration
honestly prices "asked for the whole set" as set-membership
sensitivity — no read can cost more than it observed. Scope
filtering composes: recorded edges are within the Reader's scope,
so an out-of-scope change cannot wake a plan that could never
have seen it.

## Model laws

- **Admit-with-empties.** A kind carries what any language in scope
  needs; languages leave the rest empty, and emptiness is never a
  claim.
- **Member lists are slices, never name-keyed maps.** Java, Kotlin,
  C#, and TS overload; a model keyed by name cannot hold their
  declarations. Languages that forbid overloads simply never produce
  two entries.
- **Origin is one-way.** emit symbols link to their originating node
  symbol; node symbols never reference emit. The read side cannot
  observe the write side.
- A new kind or field is a schema edit, nothing else.

## Why two models

The split is deliberate, twice over:

1. **The read side is frozen.** A generator cannot append into a
   source struct when the source model has no slots. The phase
   discipline is enforced by the compiler, not by convention.
2. **N plans share one node graph**
   ([08-workspace-and-plans.md](08-workspace-and-plans.md)). If emit
   borrowed node symbols by reference, one plan's generator mutating
   a shared symbol would corrupt a sibling plan's output
   nondeterministically. With separate models that bug does not
   compile.

The alternatives fail on inspection. A single merged model must
serve two masters and serves neither: parsed nodes carry authentic
positions and comments; synthesized nodes have neither, and every
rewrite pass inherits the misery of faking positions and
re-attaching comments. A base-type-plus-embedding design fails
structurally instead: `node.Struct` needs `Fields []*node.Field`,
and Go embedding cannot specialize element types down a mutually
recursive graph. The escape hatches — interface soup with assertions
at every use site, or type parameters threaded through the whole
graph — are worse for plugin authors than two concrete models. Two
generated models cost a schema and a regen; every alternative costs
an invariant.
