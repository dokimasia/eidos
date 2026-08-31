// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package backendtest runs a renderer over a hand-built emit
// fixture and holds it to the rungs a valueless render can prove.
// It is the harness a backend author tests against, before any
// sink or workspace exists.
//
// [Fixture] composes the plan as the renderer sees it: the emit
// store, the schedule, and the template trees, helpers and
// override declarations the composition would have handed over. A
// [Setup] builds the renderer under test with its fixture, fresh
// per call, so two calls answer two isolated worlds.
//
// # The rungs
//
// [RunBackendSuite] composes the granular assertions:
// [AssertInhabitedFixture] refuses an empty store, because a suite
// over an empty world passes vacuously, [AssertDeterministicRender]
// holds two isolated renders to byte-equal files, [AssertSpeltKinds]
// refuses a kind the language cannot spell, [AssertPlacedContent]
// holds every body to landing whole, and [AssertContinuedRender]
// holds the failure semantics —
// no fatal error for a file's problem, every finding positioned
// and attributed, and a file the formatter refused withheld from
// the values. Each assertion takes the [assert.TB] seat, so its
// own failure path is testable. The generated-file header and the
// provenance trailer are the output contract's, and their rungs
// join the suite with it.
//
// # Dependency position
//
// core/backendtest imports core/plugin, core/render, core/diag,
// the assert module and the Go stdlib. It never imports the root
// package: the harness works at the SPI floor, so a kit-built
// backend and a hand-rolled renderer are held to the same rungs.
package backendtest
