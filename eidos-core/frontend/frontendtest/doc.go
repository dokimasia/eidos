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
// validation, no keys skips the stamp apply. Each skip is stated
// by the suite rather than passed in silence.
//
// # Dependency position
//
// core/frontend/frontendtest imports core/frontend/load,
// core/plugin, core/store, core/node, core/directive, core/meta,
// core/diag, core/symbol, core/position and the assert module; it drives the
// read side end to end, so it sits beside core/frontend/load at
// the top and nothing beneath imports it back.
package frontendtest
