---
adr: 0002
title: Identity carries the source language
status: Accepted
date: 2026-08-30
supersedes: none
superseded-by: none
---

# ADR-0002: Identity carries the source language

## Status

Accepted

## Context

The symbol model specification defines a canonical identity as
"package path, kind, name, and a signature discriminator". Every
worked example in the same document set spells a fifth part: the
end-to-end document and the determinism rules both write identities as
`golang:svc/store.Store#Get(ctx,string)`, with a language prefix the
field list never mentions.

The two readings collide in the situation eidos is built for: a mixed
monorepo, where a proto package and a Go package share one directory.
Under the four-part reading their declarations produce equal
identities, and read sets, exports, manifests, drift and explain all
key on identity. Two declarations that collide there are
indistinguishable to every one of those.

## Decision

Put the source language in `symbol.Identity` and in its string form,
because identities have to stay unique across languages that share
one workspace, and every documented example already spells the
prefix.

## Alternatives Considered

### The literal four-part identity

Package path, kind, name, discriminator, exactly as the
specification's field list says. It lost because a proto file and a
Go package in one directory can collide, and an identity that is not
unique does nothing else useful. It also contradicts every example
spelling the documents show.

### Language as metadata beside the identity

Keep the four-part key and stamp the language as a metadata fact on
the symbol. It lost because identity gets used as a standalone map
key and as a stored string: a read-set row, a manifest `sources`
entry, an export record. Someone reading a manifest in a fresh clone
has no graph to look the language up in, so the key would be
ambiguous where nothing can disambiguate it.

## Consequences

**Positive:**

- Identities are unique across a mixed workspace by construction, and
  the struct matches the spellings the manifests and the explain
  examples already use.

**Negative:**

- The key widens by one field, and every identity the engine handles
  in volume carries it. The interning commitments keep that off the
  hot path; the boundary form costs one more string.
- Stored identities depend on language names never changing meaning.
  Language names are registered API already, so this adds no new
  promise, but it does bind the stored state to it.

**Neutral:**

- The specification's field list and its examples disagreed. This
  record settles that on the examples' side.

## References

- [00-one-declaration-end-to-end.md](../architecture/00-one-declaration-end-to-end.md)
  and
  [17-output-and-determinism.md](../architecture/17-output-and-determinism.md),
  the worked identity spellings
- [02-symbol-model.md](../architecture/02-symbol-model.md), the
  four-part field list
- [21-decisions.md](../architecture/21-decisions.md), the decision
  log (D56, the interning commitments)
- [RFC-0001](../rfc/0001-symbol-model-contract.md), the `Identity`
  struct and string grammar
