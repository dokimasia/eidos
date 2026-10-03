// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package plugintest runs a plugin over a hand-built fixture and
// applies the conformance checks that need no workspace. It is the
// harness a plugin author tests against, so its surface is as much
// contract as the builder's.
//
// [Fixture] composes the run's read side by hand: packages, raw
// and validated directives, stamped facts, a scope, seeded
// earlier-bucket units and the worker count each phase call runs
// on. [Fixture.Annotate] and [Fixture.Generate] run one plugin's
// phase over it and return the [Result] to assert on. Nothing
// renders: the emitted units are compared as encoded bytes, the
// contributors each unit names included, which is what makes
// determinism checkable before any backend exists.
//
// # The checks
//
// A [Setup] builds the plugin together with its fixture, the way a
// composition does, sharing one key registry and one graph.
// [RunPluginSuite] composes the granular assertions over it,
// skipping the roles and surfaces a plugin does not implement:
// [AssertPopulatedFixture], [AssertStableDeclaration],
// [AssertOptionsSchema], [AssertTemplates],
// [AssertDeterministicEmit], [AssertIdempotentAnnotate],
// [AssertParallelDispatch], [AssertSelective],
// [AssertPositionedDiagnostics], [AssertNoStructuralWrites] and
// [AssertAttributedEmit]. [AssertParallelDispatch] runs the plugin on
// one worker and on eight and requires the same emit, facts and
// findings. Under the race detector it also exposes state a handler
// writes outside its effects. [AssertSelective] runs the plugin whole,
// then under a selection of every match it journaled, and requires the
// same emit, facts and findings, which a warm run assumes of every
// handler. [Fixture.Select] and [Fixture.Journal] hand a selection and
// a journal to each phase call. [AssertTwins] requires byte-equal emit
// from two spellings of one plugin, which is how the lowering
// guarantee is checked from the outside. Each assertion takes the
// [assert.TB] role, so its own failure path is testable.
//
// # Dependency position
//
// core/plugintest imports core/plugin, core/backend/render,
// core/diag, core/directive, core/emit, core/meta, core/node,
// core/rules, core/store, core/symbol, the assert module and the
// Go stdlib.
// It never imports the root package: the harness works directly
// on the SPI, so a facade-built plugin and a hand-rolled one pass
// the same checks.
package plugintest
