# Metadata

*Builds on: [02](02-symbol-model.md) (bags hang on symbols),
[03](03-projection.md) (namespaces follow language identity).
Feeds: [05](05-directives.md) (directive authority),
[09](09-incrementality.md) (read sets),
[16](16-diagnostics.md) (explain).*

Metadata is the only channel between plugins. A shape detector
concludes that a struct is a writer. A generator three stages later
needs that conclusion. The two ship in different modules and neither
imports the other. Whatever carries the fact between them must
create no compile-time dependency, must survive a human overriding
it, and must be able to say who won and why.

## The channel

Facts go into a typed `meta.Bag` under a `meta.Key[T]`.

Every write carries an **authority**, ordered `plugin < directive <
manual`, and provenance. A human who overrides a fact at the source
declaration beats any inference, whatever order the plugins ran in.
`manual` is reserved for consumer tooling, meaning a migration or a
fix-it script that has to outrank even the directives it is
rewriting. Nothing in a normal run writes at that authority.

Plugins never call each other. A fact that has to move between plugins goes on the
graph.

**A boolean key is written only when the fact holds.** Absence is
the negative, and `false` is never stamped. That makes deletion
load-bearing, because you cannot negate a stamped boolean by writing
a lower value. So the kernel `meta` directive has a drop form,
`+gen:meta drop=shape.comparable`, which takes a key or a declared
fact group and removes the fact, or every member of the group
including stamps that arrive later. A drop writes at `directive`
authority: it outranks a `plugin` stamp and loses to `manual`, like
any other directive write. `explain` shows it as a deletion somebody
authored rather than as a fact nobody wrote
([05-directives.md](05-directives.md)).

**Writes stay deterministic under parallelism.** The store
serializes writes per bag, and rank decides the winner rather than
arrival order:

1. Higher **authority** wins, ordered `plugin < directive < manual`.
   A `meta drop` is a directive-authority deletion like any other
   directive write.
2. Within one authority, the earlier **capability bucket** wins.
3. Within a bucket, **plugin name**, alphabetically, which is the
   same tie-break slots use.
4. Within a plugin, the **first claim wins**, in canonical match
   order: subject identity, then directive-instance source order,
   then insertion.

A later claim carrying a different value does nothing, by design.
That is precedence rather than conflict, and the shape catalog's
detector ordering ([12-shape-catalog.md](12-shape-catalog.md)) is
this rule doing its job. An identical re-stamp changes nothing, and
that is exactly what early cutoff prunes
([09-incrementality.md](09-incrementality.md)).

`explain` shows every claim with the winner marked, so a losing
write stays visible instead of mysterious. Two plugins stamping one
key under opt-in parallelism produce the same winner as a serial
run, which is what lets the warm≡cold check hold with parallelism
switched on.

The channel designs out three failure modes: colliding keys, silent
absence, and derivation nobody can trace. The next three sections
take them in that order.

## Keys are declared artifacts

Every key is **registered**, with its type, its owning namespace,
its semantics, and the symbol kinds it may be stamped on. Two
claimants to one key is a workspace Build error naming both. Without
registration, a collision shows up as a wrong value a long way from
either writer.

Registration returns the typed `meta.Key[T]` handle, and the handle
is what code holds: reads, writes and read-set entries all go
through it. The key's string spelling appears only at the boundary,
in `+gen:meta java.nullability=nonnull` or as an `explain` argument,
and it resolves against the registry when the carrier is parsed.

A key may also register into a **fact group**, which is a bundle
name its writer declares, such as `shape.writer` naming every key
classification stamps. The group name generates as a constant like
any registered name.

Groups exist because facts ship in families. Negating a
classification means removing the family, and an author forced to
list a writer's keys by hand is coupled to exactly the internals the
visibility rule hides ([06b-authoring.md](06b-authoring.md)). The
group name is public API, and its membership is the writer's to
grow. `meta drop` accepts a key or a group, resolved against the
registry at Build, so a typo names the candidates instead of missing
silently.

A key's type comes from a **closed vocabulary**: `string`, `int`,
`bool`, `[]string` and `symbol.Identity`. Nothing else registers.
The fact store persists these values
([09-incrementality.md](09-incrementality.md)), `explain` renders
them, and the parity matrix tabulates them. Any further type would
put a codec in public API, versioned with the sealed state and
breakable by any plugin. A fact that needs structure becomes flat
keys under a fact group, which is what groups already model, and a
fact that names a declaration carries an identity rather than a
string.

The registration contract, pinned:

```go
// FactValue is the closed vocabulary; nothing else serializes.
type FactValue interface {
    string | int64 | bool | []string | symbol.Identity
}

// KeyName is the interior form of a key's name: a generated
// constant per registered key, never a bare string in code.
type KeyName string

func Register[T FactValue](r *Registry, s KeySpec) Key[T]

type KeySpec struct {
    Name     KeyName         // generated constant; "shape.role" is the boundary spelling
    Kinds    []symbol.Kind   // the kinds it may be stamped on
    Group    GroupID         // optional fact group (D66)
    Contract *Completeness   // optional: "on every X by phase Y"
    Doc      string          // semantics; feeds the parity matrix
}
```

Reads and writes flow through the effects in
[06b-authoring.md](06b-authoring.md), meaning `Fact` and `Stamp`.
The handle is what code holds, and the spec here is what
registration declares.

**Namespaces are owned per module.** The kernel owns `gen.*`, which
holds the neutral facts every frontend writes: authored values
([03-projection.md](03-projection.md)) and module identity
([08-workspace-and-plans.md](08-workspace-and-plans.md)). Each
language satellite owns `<lang>.*`, so `go.*`, `java.*` and
`rust.*`. eidos-plugin-shape owns `shape.*`. Consumers own their
own. Build enforces the ownership.

## Completeness contracts

A fact nobody writes reads as absent, and absence is legitimate. So
a missing annotator would otherwise degrade the output silently.

A key may therefore declare a **completeness contract** at
registration, through `KeySpec.Contract`, such as "stamped on every
callable-kinded symbol by the end of the annotator phase". The
workspace **audit mode**
([08-workspace-and-plans.md](08-workspace-and-plans.md)) checks
every contract after the phase it names and reports each violation
as a positioned diagnostic.

A contract declares its own severity: Error when downstream output
depends on the promise, Warning when the promise is advisory
([16-diagnostics.md](16-diagnostics.md)). Either way, degrading
silently turns into a red check.

## Provenance carries the read set

Provenance returns who wrote a fact, at what authority and where,
and what they derived it from, meaning the (symbol, key) reads that
produced it. One mechanism serves two consumers.

`explain` walks it, so a reader can see that a stamp exists because
of these three reads, across plans. The incrementality engine
invalidates along it
([09-incrementality.md](09-incrementality.md)). The explain edge and
the invalidation edge are the same edge, so neither can drift from
the other.

## Language-specific metadata

This is the mechanism for everything the degradation scale
preserves but the projection cannot hold
([03-projection.md](03-projection.md)).

Frontends stamp their own namespace: lifted annotations under
`java.annotation.*`, plus `rust.lifetimeParams`,
`go.constraintTypeSet`, `java.nullability=unknown`,
`kotlin.companion`, and the original spelling behind every
normalization, such as visibility and package paths.

Any plugin may read any namespace. In practice, facts on checks 2 and
3 get read by that language's bindings. Build refuses a write
outside the namespace you own.

Authority applies uniformly, so `+gen:meta java.nullability=nonnull`
at a declaration outranks the frontend's stamp. That is how a
consumer corrects a library's missing annotations without forking
it.

## The parity matrix

A CI artifact, published per release: which kinds carry which keys,
per language, with cross-language concept rows for things like
context-likeness, error-ness and comparability. A concept one
frontend stamps and another does not becomes a visible cell instead
of a silent degradation in every neutral consumer. It publishes
beside the feature matrix
([15-compatibility.md](15-compatibility.md)).
