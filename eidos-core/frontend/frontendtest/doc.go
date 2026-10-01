// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package frontendtest checks any frontend against the read side's
// contract: a fixture goes in, and the kernel's checks run over the
// loads it drives.
//
// [RunFrontendSuite] drives the load pipeline over a fixture and
// composes the granular assertions. Each assertion also runs alone
// against the [go.dokimi.dev/assert.TB] role, so a test can drive its
// failure path. The suite applies classification stamps and validates
// directives itself, at the point the workspace run does, because a
// load that skips validation accepts an invalid directive without a
// finding.
//
// # Skips
//
// A fixture that cannot state a check skips it, and the suite reports
// each skip as a skipped subtest:
//
//   - A fixture without signature roots skips the depth check.
//   - A fixture without schemas skips directive validation.
//   - A fixture without classification keys skips the stamp
//     application.
//   - A fixture that declares one package skips resolution across
//     packages.
//   - A fixture that loads every unit signature-only skips the key
//     comparison across depths.
//   - A frontend outside the [plugin.Dependent] role, or a fixture
//     without stores, skips the dependency check.
//   - A frontend outside the [plugin.Exporter] role, or a fixture that
//     lists no re-exported declaration, skips the re-export check.
//
// A granular assertion called directly over a fixture that cannot state
// it fails where the fixture broke a promise, such as classification
// keys declared and nothing stamped.
//
// The ownership check needs no fixture field. It frames a selected file
// under the load's own brand and under another brand, through the
// language's own comment syntax, and requires the load to refuse the
// first copy alone.
//
// # The scripted language
//
// [Scripted] is a small language whose grammar exercises every load
// phase, and the kernel's own tests run on it. [ScriptedDependent] and
// [ScriptedExporter] put it in the dependent and the exporter roles.
//
// # Dependency position
//
// core/frontend/frontendtest imports core/frontend/load, core/plugin,
// core/store, core/node, core/directive, core/meta, core/output,
// core/diag, core/symbol, core/position and the assert module. It
// drives the read side end to end, so it is at the top beside
// core/frontend/load, and no package beneath it imports it.
package frontendtest
