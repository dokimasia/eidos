// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package shape classifies callables so that check-generating
// consumers can act on the classification instead of re-deriving
// it.
//
// Shape stamps; consumers check. The catalog asserts nothing about
// behavior — it stamps facts in the shape namespace at plugin
// authority, and a wrong inference is a one-line directive at the
// declaration, never a fork.
//
// # One mechanism, three forms
//
// Every classification declares its form, and the form pins the
// semantics nothing downstream can re-derive:
//
//   - shape — exactly one per callable; inferable by a detector;
//     first claim wins, with precedence declared where signatures
//     overlap.
//   - mixin — any number per callable; declared only; every stamp
//     accumulates.
//   - contract — one per (instance, role); declared only; roles
//     with arity bind by protocol name within a package.
//
// A callable carries all three at once under disjoint key
// prefixes: a Commit method is writer-shaped, holds the commit
// role of a tx contract, and is atomic.
//
// # Language neutrality
//
// Detectors read only the neutral Callable projection and the
// canonical type shapes; parameter resolution goes through the
// neutral resolve rules. One detector therefore serves every
// language that reaches Tier 1 — the catalog grows by
// classification, never by language.
//
// # Spec-first
//
// The source of truth is one spec per classification — claim,
// observation, param schema, falsifiability, counterexample
// obligations — under a published JSON Schema. Registries, typed
// name constants, param constants, and directive schemas generate
// from the specs; the spec is also the published documentation,
// rendered verbatim.
package shape
