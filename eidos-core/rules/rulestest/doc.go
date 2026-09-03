// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package rulestest holds any language's rules to the properties
// the projection contract promises: fixtures in, checks the kernel
// owns.
//
// [RunRulesSuite] drives the projections over a loaded graph and
// composes the granular assertions, each of which also stands
// alone on the [go.dokimi.dev/assert.TB] role so its own failure
// path is testable: determinism across two calls with one view,
// totality of the fold and the callable mapping, a stated reason
// on every refusal and every gap, distinct sample halves, a
// witness list that is whole or absent, reads recorded on the view
// that was handed, and equal answers from parallel goroutines.
// [BenchRules] drives the four hot projections over a corpus under
// an allocation ceiling the caller pins from a profiled run.
//
// [Scripted] returns the rules of the scripted language the read
// side's suite ships, so the kernel proves the walks on a language
// it owns before any satellite arrives.
//
// # Dependency position
//
// core/rules/rulestest imports core/rules, core/frontend/frontendtest,
// core/store, core/meta, core/node, core/emit, core/directive,
// core/symbol, the bench module and the assert module; it drives
// the projections end to end, so it sits beside core/rules at the
// top and nothing beneath imports it back.
package rulestest
