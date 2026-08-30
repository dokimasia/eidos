---
adr: 0006
title: The owner back-pointer is an identity
status: Accepted
date: 2026-08-30
supersedes: none
superseded-by: none
---

# ADR-0006: The owner back-pointer is an identity

## Status

Accepted

## Context

An owned declaration names the declaration that holds it: a method
names its type, a field names its struct, an import binding names
its statement. The symbol model specification made that a pointer to
the owner and generated a `RewireOwners` pass to fill it after a
graph was assembled.

The schema already answers the same question differently everywhere
else. A `TypeRef` names what it resolves to with a
`symbol.Identity`, and the specification gives the reason: a key can
be stored, compared and carried across runs, and a pointer cannot.

Making the owner a pointer costs four accommodations. The generated
traversal has to be told never to follow it, or the walk stops being
a tree. A codec has to skip it, or encoding recurses forever. The
sealed state cannot serialize it at all. And every caller who
assembles declarations has to remember to run the pass.

It also hides a read. Reads flow through tracking readers so the
incrementality engine sees a plugin's real inputs. A raw pointer
lets a plugin ask which type declares a method without the reader
recording anything.

## Decision

Carry the owner as a `symbol.Identity` on the node model, set when a
frontend creates the child, and drop it from the emit model, because
a raw pointer is unstorable, unserializable and invisible to read
tracking. There is no rewiring pass.

## Alternatives Considered

### Keep the pointer and the RewireOwners pass

The specification's original shape: `Host Symbol` filled by a
generated pass over the graph.

It lost on the four accommodations above, and on the hidden read.
The pass also has to be remembered, and the architecture refuses
that class of rule elsewhere: a preference each generator has to
remember is a preference two of them will forget.

### Carry the identity on both models

Uniform across the two models. It lost because an emit declaration
has no identity of its own, only an origin naming the node symbol it
derives from, so an emit child's owner field would usually be zero
and mean nothing. A weaver reaches an emit value through its
parent's slot or through the origin symbol instead.

## Consequences

**Positive:**

- The generated traversal needs no exclusion rule, a codec needs no
  special case, and the sealed state can hold the field like any
  other.
- Reaching an owner goes through a tracked read, so it becomes an
  ordinary recorded edge.
- Two generated files and one schema annotation disappear: the
  rewiring pass on each model side, and the tag that drove it.

**Negative:**

- Reaching an owner costs a lookup rather than a dereference, and it
  needs a reader in hand. Code holding only a declaration can no
  longer walk upward on its own.
- A frontend has to set the owner when it creates a child. Forgetting
  leaves a zero identity, which reads as "no owner" rather than
  failing loudly.
- The specification and RFC-0001 described the pointer and the pass,
  so both change.

**Neutral:**

- The emit model loses a field it never populated meaningfully.

## References

- [02-symbol-model.md](../architecture/02-symbol-model.md), the
  symbol model specification
- [09-incrementality.md](../architecture/09-incrementality.md), read
  tracking and the edges it records
- [RFC-0001](../rfc/0001-symbol-model-contract.md), the schema and
  its field conventions
- [RFC-0002](../rfc/0002-model-generator.md), the generator and its
  output files
