// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package backendtest runs a renderer over a hand-built emit
// fixture and checks it against what a valueless render can prove.
// It is the harness that a backend author tests with, without a sink
// or a workspace.
//
// [Fixture] is the plan in the form that the renderer receives. It
// contains the emit store, the schedule, and the template trees,
// helpers and override declarations that a composition hands over. A
// [Setup] builds the renderer under test with its fixture, fresh per
// call, so two calls build two isolated fixtures. [CanonicalFixture]
// is the coverage fixture of the kernel. It emits every kind of emit
// declaration at file level, so every satellite runs one set of
// declarations instead of a set of its own. Each satellite meets every
// kind that it spells, lowers or refuses.
//
// # The checks
//
// [RunBackendSuite] composes the granular assertions:
//
//   - [AssertPopulatedFixture] fails on an empty store, because a
//     suite over an empty store passes vacuously.
//   - [AssertEmptyRender] renders an empty store, and fails where the
//     render panics, returns an error, returns a file or reports an
//     Error.
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
//   - [AssertRenderedMembers] fails on a member that a template leaves
//     out of the rendered bytes.
//   - [AssertContinuedRender] fails on a breach of the failure
//     semantics: a fatal error for a file's problem, a finding
//     without a position or attribution, or a file the formatter
//     refused among the values.
//
// Each assertion takes the [assert.TB] role, so its own failure
// path is testable.
//
// [Rendered] settles and renders the fixture of one setup, and returns
// the files and the findings. [RenderSettled] also fails on an Error, so
// the tests of a satellite pin its rendered bytes. [Fixture.Context]
// lowers a fixture to the context of one render call, for a test that
// renders through a context of its own. A fixture with a
// [Fixture.Layout] routes its store through the layout of a plan, so its
// files are the files that a workspace writes for the same units, with
// the packages of their families and the qualified references.
//
// [AssertStamped] joins the render to the output contract: every
// rendered file stamps, the frame contains the body byte for byte,
// and it verifies whole under the same contract. It runs outside the
// suite, because the consumer chooses the brand and a [Setup] has
// none. A satellite runs it beside the suite with the contract of its
// own binary.
//
// # Dependency position
//
// core/backend/backendtest imports core/plugin,
// core/backend/render, core/layout, core/output, core/emit,
// core/symbol, core/diag, the assert module and the Go stdlib. It
// never imports the root package: the harness works directly on the
// SPI, so a kit-built backend and a hand-rolled renderer face the
// same checks.
package backendtest
