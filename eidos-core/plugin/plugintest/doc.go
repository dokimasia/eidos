// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package plugintest runs a plugin over a hand-built fixture and
// applies the conformance checks that need no workspace. It is the
// harness a plugin author tests against, so its surface is as much
// contract as the builder's.
//
// [Fixture] composes the run's read side by hand: packages, raw
// and validated directives, stamped facts, a scope, the seeded units
// of earlier turns and the worker count each phase call runs
// on. [Fixture.Annotate] and [Fixture.Generate] run one plugin's
// phase over it and return the [Result] to assert on. Nothing
// renders. The checks compare the emitted units as encoded bytes,
// with the contributors of each unit, so determinism is checkable
// without a backend. [Fixture.LoadDependencies] adds the packages of a
// store, on whose declarations a rule of the generate phase does not run,
// as in a run.
//
// [Loaded] builds a fixture from real source instead: it loads a
// [Source] tree through its frontends, registers the keys of the
// frontends and the plugins, files the classification stamps of the load,
// and validates the directives of the plugins, as a workspace prepares
// its first phase call. It adds each package that a dependency round
// loaded, beside the packages that the source lists. [Fixture.Layout]
// returns the input of the layout of a plan over a fixture, so a spec of
// a backend routes the emitted units as a workspace routes them.
//
// # The checks
//
// A [Setup] builds the plugin together with its fixture, the way a
// composition does, sharing one key registry and one graph.
// [RunPluginSuite] composes the granular assertions over it:
// [AssertPopulatedFixture], [AssertStableDeclaration],
// [AssertOptionsSchema], [AssertTemplates],
// [AssertDeterministicEmit], [AssertAttributedEmit],
// [AssertIdempotentAnnotate], [AssertCleanRun], [AssertEmptyRun],
// [AssertParallelDispatch], [AssertSelective],
// [AssertPositionedDiagnostics] and [AssertNoStructuralWrites]. A check
// of a role or a surface that the plugin does not implement runs as a
// skipped subtest with the reason.
//
// [AssertStableDeclaration] reads every declaration that the workspace
// reads off a plugin, repeatedly from one build and once from a second
// build, and requires the same values each time. It reads the template
// declarations for each target of the fixture and for a target that no
// backend claims. [AssertCleanRun] requires a run over the fixture
// without an Error, and [AssertEmptyRun] requires a run over the
// fixture without its graph, its directives, its facts and its emit
// store to complete without a panic, an error or an Error.
// [AssertParallelDispatch] runs the plugin on one worker and on eight
// and requires the same emit, facts and findings. Under the race
// detector it also exposes state a handler writes outside its effects.
// [AssertSelective] runs the plugin whole, then under a selection of
// every match it journaled, and requires the same emit, facts and
// findings. A warm run assumes the same of every handler.
// [Fixture.Select] and [Fixture.Journal] hand a selection and
// a journal to each phase call. [AssertTwins] checks the lowering
// guarantee from the outside. It requires byte-equal emit from two
// spellings of one plugin. Each assertion takes the [assert.TB] role,
// so its own failure path is testable.
//
// # Panics
//
// The kernel recovers no panic of a plugin, so a panic surfaces in this
// kit before the plugin ships. [Fixture.Annotate] and [Fixture.Generate]
// run the phase call under [assert.NotPanics], and the declaration
// check reads the declarations under it too. A panic therefore fails
// the check that made the call, and the other checks of the binary
// still report.
//
// # Dependency position
//
// core/plugintest imports core/plugin, core/backend/render,
// core/diag, core/directive, core/emit, core/frontend/load, core/layout,
// core/meta, core/node, core/output, core/position, core/rules,
// core/store, core/symbol, the assert module and the Go stdlib. It imports the load
// for [Loaded] alone, and the SDK facade re-exports no load. It never
// imports the root package: the harness works directly on the SPI, so a
// facade-built plugin and a hand-rolled one pass the same checks.
package plugintest
