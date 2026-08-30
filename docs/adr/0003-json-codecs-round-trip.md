---
adr: 0003
title: JSON codecs round-trip
status: Accepted
date: 2026-08-30
supersedes: none
superseded-by: none
---

# ADR-0003: JSON codecs round-trip

## Status

Accepted

## Context

The symbol model specification lists JSON encoding among the
generated artifacts, and the open question was how much of a codec to
generate: encoding alone, or encoding and decoding both. The sealed
state is not the consumer here; it has its own standard-library
binary format. JSON serves fixtures, goldens and tooling.

Decoding is not free to add at zero design cost: the models hold
interface-typed fields (`Decls []Symbol`, `Host Symbol`), which
`encoding/json` cannot unmarshal without a kind discriminator and
generated dispatch. Whether to carry that machinery is the decision.
RFC-0001 specifies the codec surface this decision commits to.

## Decision

We will generate a kind-discriminated round-trip codec, `EncodeJSON`
and `DecodeJSON`, for both models, because decoding is what makes
fixtures loadable from data, and the round-trip property test it
enables is the cheapest generated correctness check the encoders can
have.

## Alternatives Considered

### Encode only

Generate marshalling and stop; no consumer reads symbol JSON back
today. It lost because it saves one template while
removing the round-trip test, leaving encoder correctness to rest on
golden files alone, and forcing every fixture to be hand-built in Go.
The decode half is where the kind-discriminator design gets settled,
and settling it apart from the encoding it has to match invites a
mismatch.

### Struct tags and plain encoding/json

Tag the generated structs and let the standard library do the work,
with no generated codec. It lost because the interface-typed fields
make plain unmarshalling impossible: `encoding/json` cannot decide
which concrete kind to allocate for a `Symbol`-typed field. A custom
`UnmarshalJSON` per holder is the same generated dispatch under a
different name.

## Consequences

**Positive:**

- A generated round-trip test (`decode(encode(x)) == x`) runs in the
  ordinary test suite for every kind on both sides.
- Fixtures and tooling can construct graphs from data rather than
  from Go code.

**Negative:**

- The codec shape (`{"kind":"Struct",...}`, identities as objects) is
  public API surface, held to additive change by the compatibility
  policy.
- Identities have two spellings in the world: the object form inside
  symbol JSON, and the string form
  (`golang:svc/store.Store#Get(ctx,string)`) that manifests use. A
  reader has to know which artifact uses which.

**Neutral:**

- The sealed state stays on its own binary format; this decision
  changes nothing about persistence or performance.

## References

- [02-symbol-model.md](../architecture/02-symbol-model.md), the
  symbol model specification listing JSON among the generated
  artifacts
- [09-incrementality.md](../architecture/09-incrementality.md), the
  sealed state's own binary format
- [15-compatibility.md](../architecture/15-compatibility.md), the
  compatibility policy
- [RFC-0001](../rfc/0001-symbol-model-contract.md), the codec surface
