// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package toolchain

import "testing"

// Setup builds the adapter under test and the generated output it
// runs over, fresh per check, so one check's scratch project never
// reaches another.
type Setup func(tb TB) (Adapter, Generated)

// RunToolchainSuite is the floor every satellite's harness runs:
// the generated output parses, it type-checks, and its own tests
// pass.
//
// The whole suite gates on the toolchain once: absent locally it
// skips with the reason the adapter gave, and absent in CI it
// fails, so a regression cannot hide behind a missing compiler. A
// satellite adds its own assertions beside this call rather than
// inside it, because the kernel set is the floor rather than the
// ceiling.
func RunToolchainSuite(t *testing.T, setup Setup) {
	t.Helper()

	a, _ := setup(t)
	if !Require(t, a) {
		return
	}

	t.Run("the generated output parses", func(t *testing.T) {
		t.Parallel()
		a, g := setup(t)
		AssertParses(t, a, g)
	})
	t.Run("the generated output type-checks", func(t *testing.T) {
		t.Parallel()
		a, g := setup(t)
		AssertTypeChecks(t, a, g)
	})
	t.Run("the generated tests pass", func(t *testing.T) {
		t.Parallel()
		a, g := setup(t)
		AssertTestsPass(t, a, g)
	})
}
