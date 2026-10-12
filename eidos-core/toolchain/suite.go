// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package toolchain

import "testing"

// RunToolchainSuite runs the three checks of the kit over g with the
// adapter a: the generated output parses, it type-checks, and its own
// tests pass. It is the minimum that the harness of every satellite runs.
//
// The checks gate on the toolchain once, because a language such as
// TypeScript parses through its toolchain. Absent locally, the checks skip
// with the reason that the adapter gave. Absent in CI, they fail, so a
// regression cannot hide behind a missing compiler. Each check runs in a
// parallel subtest under the context of that subtest, and lays g out in a
// scratch directory of its own. No check changes a or g, so the checks
// share them. A satellite adds its own assertions in its own test, beside
// this call.
func RunToolchainSuite(t *testing.T, a Adapter, g Generated) {
	t.Helper()

	if Require(t, a) {
		t.Run("the generated output parses", func(t *testing.T) {
			t.Parallel()
			AssertParses(t.Context(), t, a, g)
		})
		t.Run("the generated output type-checks", func(t *testing.T) {
			t.Parallel()
			AssertTypeChecks(t.Context(), t, a, g)
		})
		t.Run("the generated tests pass", func(t *testing.T) {
			t.Parallel()
			AssertTestsPass(t.Context(), t, a, g)
		})
	}
}
