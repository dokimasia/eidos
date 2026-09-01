// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package reference is the reference plugin ensemble: a complete,
// API-typical consumer of the kernel's authoring surface —
// directives, slots, templates, exports, multi-plan composition —
// kept deliberately ordinary so that it exercises the surfaces
// real plugins use, the way real plugins use them.
//
// # Canary
//
// The ensemble is the kernel's compatibility canary: it runs
// against kernel HEAD in the release ring, so an incompatible
// kernel change fails here before any tag exists. Compatibility
// is thereby executed against a real consumer, never asserted in
// a changelog.
//
// # Performance rig
//
// The ensemble drives the multi-plan and cross-language benchmark
// scenarios over the seeded synthetic corpora, so the ring that
// catches API breaks also catches speed regressions, budget-gated
// per release.
package reference
