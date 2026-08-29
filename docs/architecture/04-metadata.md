# Metadata

*Builds on: [02](02-symbol-model.md) (bags hang on symbols),
[03](03-projection.md) (namespaces follow language identity).
Feeds: [05](05-directives.md) (directive authority),
[09](09-incrementality.md) (read sets),
[16](16-diagnostics.md) (explain).*

The only channel between plugins. A shape detector concludes a
struct is a writer; a generator three stages later needs that
conclusion; the two ship in different modules and neither imports
the other. Whatever carries facts between them must create no
compile-time dependency, must survive a human overriding it, and
must say who won and why.

## The channel

- Facts are written into a typed `meta.Bag` under a `meta.Key[T]`.
- Every write carries an **authority** (`plugin < directive <
  manual`) and provenance; a human override at the source
  declaration beats any inference, independent of plugin order.
  `manual` is the reserved top rank for consumer tooling — a
  migration or fix-it script that must outrank even the directives
  it is rewriting — and nothing in a normal run writes at it.
- Plugins never call each other. A fact that must travel goes on the
  graph.
- Boolean keys are written only when the fact holds; absence is the
  negative; `false` is never stamped. That convention makes
  **deletion load-bearing**: negating a stamped boolean cannot be
  spelled as a lower value, so the kernel `meta` directive has a
  drop form — `+gen:meta drop=shape.comparable`, taking a key or a
  declared fact group — that removes the fact, or the group's
  every member (later stamps included), at `directive` authority.
  A drop outranks `plugin` stamps
  and loses to `manual` like any other directive write, and
  `explain` shows it as an authored deletion, not as never-written
  ([05-directives.md](05-directives.md)).
- Writes are deterministic under parallelism: the store serializes
  writes per bag, and the winner is decided by rank, never by
  arrival order —

  1. higher **authority** wins (`plugin < directive < manual`);
     `meta drop` is a directive-authority deletion like any other
     directive write;
  2. within an authority, the earlier **capability bucket** wins;
  3. within a bucket, **plugin name**, alphabetical — the same
     tie-break slots use;
  4. within a plugin, the **first claim wins**, in canonical match
     order (subject identity → directive-instance source order →
     insertion). A later claim with a different value is inert by
     design — that is precedence, not conflict: the shape
     catalog's detector ordering
     ([12-shape-catalog.md](12-shape-catalog.md)) is this rule
     doing its job. An identical re-stamp is idempotent, and is
     exactly what early cutoff prunes
     ([09-incrementality.md](09-incrementality.md)).

  `explain` shows every claim with the winner marked, so a losing
  write is visible, never mysterious. Two plugins stamping one key
  under opt-in parallelism produce the same winner as the serial
  run, which is what lets the warm≡cold rung hold with parallelism
  on.

Three failure modes are designed out of the channel from the start:
key collisions, silent absence, and opaque derivation. The next
three sections are those three, in order.

## Keys are declared artifacts

Every key is **registered** — type, owning namespace, semantics, and
the symbol kinds it may be stamped on. Two claimants to one key is a
workspace Build error naming both; without registration, a collision
surfaces as a wrong value far from either writer.

Registration returns the typed `meta.Key[T]` handle, and the handle
is what code holds — reads, writes, and read-set entries all go
through it. The key's string spelling appears only at the boundary
(`+gen:meta java.nullability=nonnull`, an `explain` argument) and
resolves against the registry when the carrier is parsed.

A key may also register into a **fact group**: a writer-declared
bundle name (`shape.writer`, naming every key that classification
stamps), generated as a constant like any registered name. Groups
exist because facts ship in families — negating a classification
means removing the family, and an author forced to enumerate a
writer's key inventory is coupled to exactly the internals the
visibility law hides ([06b-authoring.md](06b-authoring.md)). The group
name is public API; its membership is the writer's to grow.
`meta drop` accepts a key or a group, resolved against the
registry at Build — a typo names candidates, never a silent miss.

A key's type comes from a **closed value vocabulary** — `string`,
`int`, `bool`, `[]string`, and `symbol.Identity` — and nothing
else registers. The fact store persists values
([09-incrementality.md](09-incrementality.md)), `explain` renders
them, and the parity matrix tabulates them; any further type would
be a codec in public API, versioned with the sealed state and
breakable by any plugin. A fact needing structure is flat keys
under a fact group — which is what groups already model — and a
fact naming a declaration is an identity, never a string.

The registration contract, pinned:

```go
// FactValue is the closed vocabulary; nothing else serializes.
type FactValue interface {
    string | int64 | bool | []string | symbol.Identity
}

// KeyName is the interior form of a key's name — a generated
// constant per registered key, never a bare string in code.
type KeyName string

func Register[T FactValue](r *Registry, s KeySpec) Key[T]

type KeySpec struct {
    Name     KeyName         // generated constant; "shape.role" is boundary spelling
    Kinds    []symbol.Kind   // the kinds it may be stamped on
    Group    GroupID         // optional fact group (D66)
    Contract *Completeness   // optional: "on every X by phase Y"
    Doc      string          // semantics; feeds the parity matrix
}
```

Reads and writes flow through the effects of
[06b-authoring.md](06b-authoring.md) (`Fact`, `Stamp`) — the handle is
what code holds; the spec here is what registration declares.

Namespace ownership is per module: the kernel owns `gen.*` — the
cross-frontend neutral facts: authored values
([03-projection.md](03-projection.md)), module identity
([08-workspace-and-plans.md](08-workspace-and-plans.md)) — each
language satellite owns `<lang>.*` (`go.*`, `java.*`, `rust.*`),
eidos-plugin-shape owns `shape.*`, consumers own their own. The
kernel enforces ownership at Build.

## Completeness contracts

A fact nobody writes reads as absent, and absence is legitimate — so
a missing annotator would otherwise degrade output silently. A key
may therefore declare a **completeness contract** at registration
(`KeySpec.Contract`): "stamped on every callable-kinded symbol by
the end of the annotator phase." The
workspace **audit mode**
([08-workspace-and-plans.md](08-workspace-and-plans.md)) verifies
every contract after the phase it names and reports each violation
as a positioned diagnostic. A contract declares its severity: Error
for a promise downstream output depends on, Warning for an advisory
one ([16-diagnostics.md](16-diagnostics.md)). Silent degradation
becomes a red check either way.

## Provenance carries the read set

Provenance answers *who, at what authority, where* — and *derived
from what*: the (symbol, key) reads that produced the fact. One
mechanism, two consumers:

- `explain` walks it — "this stamp exists because of these three
  reads," across plans.
- The incrementality engine invalidates along it
  ([09-incrementality.md](09-incrementality.md)). The explain edge
  and the invalidation edge are the same edge, so neither can drift
  from the other.

## Language-specific metadata

The mechanism for everything the degradation ladder preserves but
the projection cannot hold ([03-projection.md](03-projection.md)):

- Frontends stamp their namespace: `java.annotation.*` lifted
  annotations, `rust.lifetimeParams`, `go.constraintTypeSet`,
  `java.nullability=unknown`, `kotlin.companion`, original spellings
  behind every normalization (visibility, package paths).
- Any plugin may *read* any namespace; in practice rung-2/3 facts
  are consumed by that language's bindings. Writing outside your
  owned namespace is refused at Build.
- Authority applies uniformly: `+gen:meta java.nullability=nonnull`
  at a declaration outranks the frontend's stamp, which is how a
  consumer corrects a library's missing annotations without forking
  it.

## The parity matrix

A CI artifact, per release: which kinds carry which keys, per
language, with cross-language concept rows (context-likeness,
error-ness, comparability, …) so a concept stamped by one frontend
and missing from another is a visible cell instead of a silent
degradation in every neutral consumer. Published beside the feature
matrix ([15-compatibility.md](15-compatibility.md)).
