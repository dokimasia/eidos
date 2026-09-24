// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package rulestest checks any language's rules against the
// properties of the projection contract: a language brings its
// fixtures, and the kernel's checks run over them.
//
// [RunRulesSuite] drives the projections over a loaded graph and
// composes the granular assertions. Each assertion also runs alone
// on the [go.dokimi.dev/assert.TB] role, so its own failure path is
// testable. The checks are:
//
//   - [AssertDeterministic]: two bounds over one view return equal
//     values, so a memoised fold cannot hide a second derivation
//     that differs.
//   - [AssertTotal]: the fold returns a shape for every reference
//     and the callable mapping reports false for every other kind.
//   - [AssertRefusesWithReason]: every refusal and every gap names
//     its reason.
//   - [AssertDistinctSamples]: the two halves of a derived pair
//     differ at some depth.
//   - [AssertWitnesses]: a derived witness has a spelling, and
//     substitution rewrites a parameter's references into its
//     witness on a copy.
//   - [AssertRecorded]: a sample of a named type reads the
//     declaration through the view the language is handed.
//   - [AssertConcurrent]: parallel goroutines, each over its own
//     view, return the serial values.
//
// [BenchRules] drives the four hot projections over a corpus under
// an allocation ceiling the caller pins from a profiled run.
//
// [Scripted] returns the rules of the scripted language the read
// side's suite ships, so the kernel proves its walks on a language
// of its own, independent of any satellite.
//
// # Dependency position
//
// core/rules/rulestest imports core/rules, core/diag,
// core/directive, core/emit, core/frontend/frontendtest,
// core/frontend/load, core/meta, core/node, core/plugin,
// core/store, core/symbol, the assert module with its bench
// package, and the Go stdlib. It drives the projections end to end
// from a load. No package beneath core/rules imports it.
package rulestest
