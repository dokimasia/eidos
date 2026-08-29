# Cross-language conversion

*Builds on: [03](03-projection.md) (the canonical shapes),
[08](08-workspace-and-plans.md) (plans and exports). Feeds:
[11](11-languages.md) (the lowering half of every satellite).*

One schema in, a Go server and a TS client out of one workspace run —
without eidos becoming a transpiler and without a walker per language
pair.

## The boundary

**eidos maps interfaces, never implementations.** Type references,
names, optionality, error models, enum representations — what appears
in generated declarations and delegating scaffolding. Function bodies
are permanently out of scope. This sentence is what keeps the feature
from becoming a compiler.

## Hub and spoke, never pairwise

The pairwise shape — a hand-written walker per (source, target) pair,
stamping target names and types onto source symbols — is O(pairs),
and every walker re-implements the same projection of one type system
into another; two walkers sharing a target drift apart the first time
either is corrected. The hub shape inverts it: a deliberately small
canonical type system in the middle, each source language projecting
*into* it once, each target language lowering *out of* it once. A new
language pair then costs **zero new code** once both ends exist.

The hub already exists in the kernel: the canonical **TypeShape**
vocabulary ([03-projection.md](03-projection.md)):

```
   go ──┐                        ┌── go
proto ──┤── TypeOf ─► canonical ─┤── Lowering ─► ts        (each language:
   ts ──┤            TypeShape   ├── python                 one arrow in,
 java ──┘                        └── java                   one arrow out)
```

Two layers around it:

### Canonical layer (kernel)

- Frontends project source types **into** canonical shapes (Tier 1
  `TypeOf`).
- Each target language implements one **`Lowering`**
  ([11-languages.md](11-languages.md)) — the contract shape:

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

  A `Spelling` carries rendered text plus the imports it requires,
  so the backend's per-file `ImportSet` is fed by the same call that
  produced the text.

### Policy layer (data)

The genuinely contested mappings are not facts, they are project
policy: proto `int64` → TS `bigint` vs `string` vs `number`;
nullability-unknown treatment for unannotated Java. Policies live in
workspace config with a recorded default in the target satellite,
overridable per directive at `directive` authority. Per-pair override
tables exist only where the canonical path is genuinely wrong — the
proto scalar table is ~20 rows of data, not a walker.

`Policy`, the value a `Lowering` receives, is the *resolved* form
of that. Resolution has four steps, and after Build there is no
fifth:

1. **Register.** A target satellite registers each policy key with
   its typed choices and a default (`ts.int64:
   bigint|string|number`, default `bigint`;
   `java.nullabilityUnknown: nullable|nonnull`). Keys and choices
   export as generated constants; the spellings below are boundary
   forms.
2. **Select.** Workspace config picks choices per key. Build
   validates every selection against the registered keys — an
   unknown key or invalid choice is a Build error naming the
   candidates.
3. **Override.** A per-declaration directive overrides at
   `directive` authority, validated against the registered choices
   at Freeze like any directive param.
4. **Hand over.** The `Policy` a lowering receives is total: every
   registered key resolves to default, selection, or override. A
   lowering never sees an unresolved policy, so it never carries a
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

A lowering that cannot spell a shape **refuses with a stable
diagnostic code** at the declaration that forced it: Go `chan T` has
no TS spelling; a Union has no Go spelling. It never guesses. Rung 4
of the degradation ladder, tested per language by the completeness
rung.

## Names across the boundary

Cross-language naming (proto `user_id` → Go `UserID` → TS `userId`)
flows through the target's naming joins, with the result stamped as
target-namespace metadata on the source symbol (`go.name`) at
`plugin` authority — so a consumer overrides any single name at the
declaration with a directive, and `explain` shows where every
spelling came from.

*When* matters: the stamps land during **Annotate**, by a naming
annotator each target language provides and the workspace registers
automatically when any plan targets that language. Plans read the
shared graph and never write it — stamping from inside a plan would
break both the freeze and plan isolation, since plans run in
parallel. Two plans targeting one language share the stamps, which
is correct: a name is a fact about (symbol, target language), not
about a plan.

## Binding generation

Generators that must reference *generated* artifacts on the other
side of a boundary (FFI bindings, JNI, cgo, wasm bindings) do not
read sibling emit; they consume **plan exports** — the typed,
deterministic summaries plans publish, with dependencies declared and
topo-ordered
([08-workspace-and-plans.md](08-workspace-and-plans.md)). Convention
recomputation ("apply the same naming rules and hope") is rejected as
drift-by-construction.

## Consistency

Cross-language claims are checked, not hoped for: `WorkspaceCheck`
plugins at Close read the records — manifests, exports, graph and
facts — and report drift ("proto
service method X has a Go handler and no TS client method") as
positioned diagnostics with stable codes.
