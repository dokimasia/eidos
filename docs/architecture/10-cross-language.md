<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

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
   its typed choices and a default. `typescript.int64` accepts
   `bigint|string|number` and defaults to `bigint`, `typescript.duration`
   accepts `string|number` and defaults to `string`, and
   `java.nullabilityUnknown` accepts `nullable|nonnull`. Keys and
   choices export as generated constants, and the spellings above are
   what a human types.
2. **Select.** Workspace config picks a choice per key. Build checks
   every selection against the registered keys, and an unknown key
   or an invalid choice is a Build error naming the candidates.
3. **Override.** A directive on a declaration overrides at
   `directive` authority, checked against the registered choices at
   Freeze like any other directive param. A policy of the plan,
   `PolicySpec.Plan`, applies to every file of a plan and has no param:
   the render reads it, as the Python backend reads the version that its
   files run on.
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

## Refusal is fully supported

A lowering that cannot spell a shape refuses, with a stable
diagnostic code, at the declaration that forced it. Go `chan T` has
no TypeScript spelling. A Union has no Go spelling. It never
guesses. Rust refuses the top type, because Rust has no type of every
value. Java refuses the well-known `empty`, because Java has no type of
a value without data outside a declaration. Every target spells the
well-known `error`: Go as `error`, TypeScript as `Error`, Java as
`Exception` and Rust as `Box<dyn std::error::Error>`. This is level 4 of the
degradation scale, and the completeness check tests it per language.

A spoke spells every fact of a shape or refuses the shape. It never
writes a spelling with fewer constraints than the shape has. The
read-only mark of a list, an array, a tuple or a map is such a fact.
TypeScript writes `readonly T[]`, `readonly [A, B]`,
`Readonly<Record<K, V>>` and `ReadonlyMap<K, V>`. Go, Rust and Java
refuse each read-only collection that they write as writable, through
`spoke.ReadOnly`.

## Names across the boundary

Cross-language naming, such as proto `user_id` becoming Go `UserID`
and TypeScript `userId`, goes through the target's naming joins. The
result is stamped as target-namespace metadata on the source symbol,
such as `golang.name`, at `plugin` authority. A consumer can therefore
override any single name at the declaration with a directive, and
`explain` shows where every spelling came from.

Each target converts a name through the initialisms of its own
convention. Go writes an initialism in one case, so `user_id` becomes
`userID`. TypeScript, Java, Rust, protobuf and GraphQL write an acronym
as a word, as their style guides do, so `httpRow` becomes `HttpRow`.
Their conversion keeps a name that the source writes in its style,
such as `getById` and `toJSON`. A class that a generator mirrors from
an interface then implements each method under the name of the
interface, and a GraphQL field keeps the key of its response.

A nested declaration crosses under its flat name. `Identity.FlatName`
joins the parts of the declaration's owner chain and its name, and
writes the first letter of each part after the first in upper case, so
the nested `State` of the message `Session` is `SessionState` in every
target. Each spoke spells a translated reference with the referent's
flat name. A generator that mirrors a nested declaration emits it in
the file under that name, and the naming annotator stamps the respell
of that name on the nested declaration.

A generator translates the type of a value through `Emitter.Type`, and
a type that a declaration extends, implements or embeds, or that bounds
a type parameter, through `Emitter.Supertype`. A target spells the two
positions apart through `plugin.SupertypeSpeller`. Rust spells a value
of a trait as `Box<dyn Node>` and a supertrait as `Node`. Every other
target spells both positions alike.

A type parameter crosses under its name. The fold gives it an opaque
shape with the identity of the parameter, and `Emitter.Type` passes the
shape to the spoke at every depth of a type. The spokes of Go,
TypeScript, Java, Rust and Python spell it through `spoke.Param`, so a
list of `T` is `[]T` in Go and `list[T]` in Python. protobuf and
GraphQL have no type parameters, and their spokes refuse it. The hub
has a row for a type parameter and a row for a list of one. A generator
translates the type parameters of a declaration through
`Emitter.TypeParams`, which translates each bound as a supertype.

The timing matters. The stamps arrive during **Annotate**, written by
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
read a sibling plan's emit. Their plan declares a dependency on the
producing plan by name, and they read its **export**
([08-workspace-and-plans.md](08-workspace-and-plans.md)): every
declaration the producer rendered, keyed by the declaration it
derives from, the plugin, the family and the names it was emitted
under, and spelled with the name the producer's settle gave it, its
package's import path and its file. Working the names out again by
applying the same naming rules and hoping they match is rejected,
because it drifts by construction.

An export records no signature, because a generated declaration's
types are emit references that no projection reads. A binding
generator that needs a declaration's type reads the shape of the
declaration's origin through its own reader, which needs the
origin's package in its plan's scope.

## The bindings of gRPC

A plan that writes code against a protobuf service compiles against the
code of one gRPC binding. The lowering policy `golang.grpc` selects
`none`, `grpc-go` or `connect-go`, and `typescript.grpc` selects `none`
or `connect-es`. Under a binding, the spoke spells a reference to a
protobuf declaration as the code generator of the binding names it, and
marks the reference bound through `emit.TypeRef.Bound`. The plan does
not emit the referent of a bound reference. The layout qualifies the
reference by the package that the binder of the target returns
([18-routing-and-layout.md](18-routing-and-layout.md)).

A generator writes the handler of an rpc through `Emitter.Handler`. The
Emitter projects the rpc, and the `plugin.Binder` of the backend
returns the method in the form of the binding: the form of
protoc-gen-go-grpc, of protoc-gen-connect-go or of the `ServiceImpl` of
Connect-ES. Under `none`, and for a backend without a binder, the
method is the canonical method of the projection: the context, the
request and the response, and the failure under the error model
`raised`. A binder that cannot serve an rpc refuses it, and the Emitter
reports the refusal under `RefusedHandler` at the rpc.

## Consistency

Cross-language claims are checked rather than hoped for.
`WorkspaceCheck` plugins run at Close, read the records of the plans
they name (manifest entries and exports, with the graph and facts)
and report a mismatch such as "proto service method X has a Go
handler and no TypeScript client method" as a positioned diagnostic
with a stable code.
