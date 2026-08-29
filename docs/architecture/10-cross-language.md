# Cross-language conversion

*Builds on: [03](03-projection.md) (the canonical shapes),
[08](08-workspace-and-plans.md) (plans and exports). Feeds:
[11](11-languages.md) (the lowering half of every satellite).*

One schema goes in, and a Go server and a TypeScript client come out
of a single workspace run. eidos does this without becoming a
transpiler and without a walker per language pair.

## The boundary

**eidos maps interfaces, never implementations.** It converts type
references, names, optionality, error models and enum
representations, which is what appears in generated declarations and
in delegating scaffolding. Function bodies stay out of scope
permanently. That sentence is what keeps this feature from turning
into a compiler.

## Hub and spoke, never pairwise

The pairwise approach writes one walker per (source, target) pair,
stamping target names and types onto source symbols. It costs
O(pairs), every walker re-implements the same projection of one type
system into another, and two walkers that share a target drift apart
the first time somebody corrects one of them.

The hub approach inverts that. A deliberately small canonical type
system sits in the middle. Each source language projects into it
once, and each target language lowers out of it once. Once both ends
exist, a new language pair costs no new code at all.

The hub is already in the kernel: the canonical **TypeShape**
vocabulary ([03-projection.md](03-projection.md)).

```
   go ──┐                        ┌── go
proto ──┤── TypeOf ─► canonical ─┤── Lowering ─► ts        (each language:
   ts ──┤            TypeShape   ├── python                 one arrow in,
 java ──┘                        └── java                   one arrow out)
```

Two layers sit around it.

### The canonical layer, in the kernel

Frontends project source types into canonical shapes through the
Tier-1 `TypeOf`. Each target language implements one `Lowering`
([11-languages.md](11-languages.md)):

```go
type Lowering interface {
    SpellType(s TypeShape, pol Policy) (Spelling, error)   // text + imports; error = refusal
    SpellOptional(inner Spelling) Spelling                 // *T / T? / T | undefined
    SpellZero(s TypeShape) (string, bool)
    ErrorModel() ErrorModel                                // what errors become here
    JoinName(word, base string) string
    SpellInstantiation(ref Spelling, args []Spelling) Spelling
}
```

A `Spelling` carries the rendered text plus the imports it needs, so
the same call that produces the text feeds the backend's per-file
`ImportSet`.

### The policy layer, as data

Some mappings have no correct answer, only a choice the project has
to make. A proto `int64` can reasonably become TypeScript `bigint`,
`string` or `number`. An unannotated Java reference needs somebody to
say whether to treat it as nullable.

The project makes those choices in workspace config. Each target
satellite records a default, and a directive can override one per
declaration at `directive` authority. A per-pair override table
exists only where the canonical path is genuinely wrong. The proto
scalar table is one of those, and it holds about twenty rows of data.
Nobody writes a walker for it.

`Policy`, the value a `Lowering` receives, is the resolved form of
all that. Resolution has four steps, and after Build there is no
fifth:

1. **Register.** A target satellite registers each policy key with
   its typed choices and a default: `ts.int64` accepts
   `bigint|string|number` and defaults to `bigint`;
   `java.nullabilityUnknown` accepts `nullable|nonnull`. Keys and
   choices export as generated constants, and the spellings above
   are what a human types.
2. **Select.** Workspace config picks a choice per key. Build checks
   every selection against the registered keys, and an unknown key
   or an invalid choice is a Build error naming the candidates.
3. **Override.** A directive on a declaration overrides at
   `directive` authority, checked against the registered choices at
   Freeze like any other directive param.
4. **Hand over.** The `Policy` a lowering receives is total: every
   registered key resolves to a default, a selection or an override.
   A lowering never sees an unresolved policy, so it never carries a
   fallback branch that quietly becomes the real default.

The contract, pinned:

```go
type PolicyKey string          // generated constant per registered key
type Choice string             // generated constant per choice

func RegisterPolicy(t rules.Target, k PolicyKey, choices []Choice, def Choice)

type Policy interface {
    Choice(k PolicyKey) Choice // total by construction after Build
}
```

## Refusal is first-class

A lowering that cannot spell a shape refuses, with a stable
diagnostic code, at the declaration that forced it. Go `chan T` has
no TypeScript spelling. A Union has no Go spelling. It never
guesses. This is rung 4 of the degradation ladder, and the
completeness rung tests it per language.

## Names across the boundary

Cross-language naming, such as proto `user_id` becoming Go `UserID`
and TypeScript `userId`, goes through the target's naming joins. The
result is stamped as target-namespace metadata on the source symbol,
such as `go.name`, at `plugin` authority. A consumer can therefore
override any single name at the declaration with a directive, and
`explain` shows where every spelling came from.

The timing matters. The stamps land during **Annotate**, written by
a naming annotator that each target language provides and the
workspace registers automatically whenever a plan targets that
language. Plans read the shared graph and never write it. Stamping
from inside a plan would break both the freeze and plan isolation,
because plans run in parallel. Two plans targeting one language
share the stamps, which is correct: a name is a fact about a symbol
and a target language, not about a plan.

## Binding generation

Some generators must reference generated artifacts on the other side
of a boundary: FFI bindings, JNI, cgo, wasm bindings. They do not
read a sibling plan's emit. They consume **plan exports**, the
typed, deterministic summaries that plans publish, with dependencies
declared and topologically ordered
([08-workspace-and-plans.md](08-workspace-and-plans.md)). Working
the names out again by applying the same naming rules and hoping
they match is rejected, because it drifts by construction.

## Consistency

Cross-language claims are checked rather than hoped for.
`WorkspaceCheck` plugins run at Close, read the records (manifests,
exports, graph and facts) and report a mismatch such as "proto
service method X has a Go handler and no TypeScript client method"
as a positioned diagnostic with a stable code.
