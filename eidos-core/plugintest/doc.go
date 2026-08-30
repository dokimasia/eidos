// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package plugintest runs a plugin over a hand-built fixture and
// holds it to the conformance rungs no workspace is needed for. It
// is the harness a plugin author tests against, so its surface is
// as much contract as the builder's.
//
// [Fixture] composes the run's read side by hand: packages, raw
// and validated directives, stamped facts, a scope and seeded
// earlier-bucket units. [Fixture.Annotate] and [Fixture.Generate]
// run one plugin's phase over it and answer the [Result] to assert
// on. Nothing renders: the emitted units are compared as encoded
// bytes, which is what makes determinism checkable before any
// backend exists.
//
// # The rungs
//
// A [Setup] builds the plugin together with its fixture, the way a
// composition does, sharing one key registry and one graph.
// [RunPluginSuite] composes the granular assertions over it,
// skipping the roles and surfaces a plugin does not hold:
// [AssertStableDeclaration], [AssertOptionsSchema],
// [AssertDeterministicEmit], [AssertIdempotentAnnotate],
// [AssertPositionedDiagnostics] and
// [AssertAttributedEmit]. [AssertTwins] holds two spellings of one
// plugin to byte-equal emit, which is how the lowering guarantee
// is checked from the outside. Each assertion takes the
// [assert.TB] seat, so its own failure path is testable.
//
// # Dependency position
//
// core/plugintest imports core/plugin, core/diag, core/directive,
// core/emit, core/meta, core/node, core/store, core/symbol, the
// assert module and the Go stdlib. It never imports the root
// package: the harness works at the SPI floor, so a facade-built
// plugin and a hand-rolled one are held to the same rungs.
package plugintest
