// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package backendtest runs a renderer over a hand-built emit
// fixture and holds it to the checks a valueless render can prove.
// It is the harness a backend author tests against, before any
// sink or workspace exists.
//
// [Fixture] composes the plan as the renderer sees it: the emit
// store, the schedule, and the template trees, helpers and
// override declarations the composition would have handed over. A
// [Setup] builds the renderer under test with its fixture, fresh
// per call, so two calls build two isolated fixtures.
//
// # The checks
//
// [RunBackendSuite] composes the granular assertions:
// [AssertPopulatedFixture] refuses an empty store, because a suite
// over an empty store passes vacuously, [AssertDeterministicRender]
// holds two isolated renders to byte-equal files, [AssertSpeltKinds]
// refuses a kind the language cannot spell, [AssertPlacedContent]
// holds every body to arriving whole, and [AssertContinuedRender]
// holds the failure semantics —
// no fatal error for a file's problem, every finding positioned
// and attributed, and a file the formatter refused withheld from
// the values. Each assertion takes the [assert.TB] role, so its
// own failure path is testable.
//
// [AssertStamped] joins the render to the output contract: every
// rendered file stamps, the frame carries the body byte for byte,
// and it verifies whole under the same contract. It stands
// outside the suite, because a brand is the consumer's to state
// and a [Setup] carries none, so a satellite runs it beside the
// suite with the contract its own binary ships.
//
// # Dependency position
//
// core/backendtest imports core/plugin, core/render, core/output,
// core/diag, the assert module and the Go stdlib. It never imports
// the root package: the harness works directly on the SPI, so a
// kit-built backend and a hand-rolled renderer are held to the
// same checks.
package backendtest
