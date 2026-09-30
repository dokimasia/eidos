// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package backendtest runs a renderer over a hand-built emit
// fixture and checks it against what a valueless render can prove.
// It is the harness a backend author tests against, before any
// sink or workspace exists.
//
// [Fixture] composes the plan as the renderer sees it: the emit
// store, the schedule, and the template trees, helpers and
// override declarations the composition would have handed over. A
// [Setup] builds the renderer under test with its fixture, fresh
// per call, so two calls build two isolated fixtures.
// [CanonicalFixture] is the kernel's own coverage fixture: it emits
// every kind an emit declaration takes at file level, so every
// satellite runs one set of declarations instead of hand-building
// its own, and meets each kind it spells, lowers or refuses.
//
// # The checks
//
// [RunBackendSuite] composes the granular assertions:
//
//   - [AssertPopulatedFixture] fails on an empty store, because a
//     suite over an empty store passes vacuously.
//   - [AssertDeterministicRender] fails unless two isolated renders
//     return byte-equal files.
//   - [AssertSpeltKinds] fails on a kind the language neither spells
//     nor refuses.
//   - [AssertPlacedContent] fails on a body that does not arrive
//     whole.
//   - [AssertSettledShape] fails on a settle that changes what its
//     seams may not.
//   - [AssertCoveredFacts] fails unless the declared coverage is
//     total and its refusals report once per statement.
//   - [AssertRenderedMembers] reads the bytes for a member a template
//     forgot.
//   - [AssertContinuedRender] fails on a breach of the failure
//     semantics: a fatal error for a file's problem, a finding
//     without a position or attribution, or a file the formatter
//     refused among the values.
//
// Each assertion takes the [assert.TB] role, so its own failure
// path is testable.
//
// [AssertStamped] joins the render to the output contract: every
// rendered file stamps, the frame contains the body byte for byte,
// and it verifies whole under the same contract. It runs outside
// the suite, because a brand is the consumer's to state and a
// [Setup] states none, so a satellite runs it beside the suite with
// the contract its own binary ships.
//
// # Dependency position
//
// core/backend/backendtest imports core/plugin,
// core/backend/render, core/output, core/emit, core/symbol,
// core/diag, the assert module and the Go stdlib. It never
// imports the root package: the harness works directly on the
// SPI, so a kit-built backend and a hand-rolled renderer face the
// same checks.
package backendtest
