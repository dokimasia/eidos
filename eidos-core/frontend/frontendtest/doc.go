// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package frontendtest holds any frontend to the read side's
// contract: fixtures in, checks the kernel owns.
//
// [RunFrontendSuite] drives the load pipeline over a fixture and
// composes the granular assertions, each of which also stands
// alone on the [go.dokimi.dev/assert.TB] role so its own failure
// path is testable. The suite stands in for the workspace until
// the two join: it applies classification stamps and validates
// directives itself, at the same point the workspace run does,
// because a stand-in that skips validation re-opens the
// silent-acceptance window.
//
// A fixture that cannot state a check skips it: no signature
// roots skips the depth check, no schemas skips directive
// validation, no keys skips the stamp apply, one package skips
// resolution across packages, and a fixture loading every unit
// signature-only skips the key comparison across depths. The suite
// states each skip as a skipped subtest rather than passing it in
// silence. A granular assertion called over a fixture that cannot
// state it fails where its precondition is a promise the fixture
// broke: classification keys declared and nothing stamped. The
// ownership check
// states nothing: it frames a selected file under the load's own
// brand and under another's itself, through the language's own
// comment syntax, and holds the load to refusing the first alone.
//
// # Dependency position
//
// core/frontend/frontendtest imports core/frontend/load,
// core/plugin, core/store, core/node, core/directive, core/meta,
// core/output, core/diag, core/symbol, core/position and the
// assert module; it drives the
// read side end to end, so it sits beside core/frontend/load at
// the top and nothing beneath imports it back.
package frontendtest
