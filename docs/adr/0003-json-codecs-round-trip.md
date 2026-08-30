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

Decoding costs a design decision rather than a template. The models
hold interface-typed fields, such as a file's `Decls []Symbol`, and
`encoding/json` cannot unmarshal one without a kind discriminator and
generated dispatch. Whether to carry that machinery is what this
record decides.

## Decision

Generate a kind-discriminated round-trip codec, `EncodeJSON` and
`DecodeJSON`, for both models, because decoding is what lets a
fixture load from data, and the round-trip property test it allows is
the cheapest correctness check the encoders can get.

## Alternatives Considered

### Encode only

Generate marshalling and stop; no consumer reads symbol JSON back
today. It lost on what the saved template costs elsewhere. Without a
decoder there is no round-trip test, so golden files are all that
holds the encoders honest, and every fixture has to be written out by
hand in Go. The decode half is also where the kind discriminator gets
designed, and a discriminator designed apart from the encoding it has
to match will not reliably match it.

### Struct tags and plain encoding/json

Tag the generated structs and let the standard library do the work,
with no generated codec. It lost because `encoding/json` cannot
decide which concrete kind to allocate for a `Symbol`-typed field, so
plain unmarshalling stops there. Writing a custom `UnmarshalJSON` per
holder generates the same dispatch under another name.

## Consequences

**Positive:**

- A generated round-trip test (`decode(encode(x)) == x`) runs in the
  ordinary test suite for every kind on both sides.
- Fixtures and tooling can construct graphs from data rather than
  from Go code.

**Negative:**

- The codec shape (`{"kind":"Struct",...}`, identities as objects) is
  public API, and the compatibility policy holds it to additive
  change.
- Identities get two spellings: the object form inside symbol JSON,
  and the string form (`golang:svc/store.Store#Get(ctx,string)`) that
  manifests use. A reader has to know which artifact uses which.

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
