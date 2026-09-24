// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package toolchain

import "testing"

// Setup builds the adapter under test and the generated output it
// runs over, fresh per check, so no check reads another check's
// scratch project.
type Setup func(tb TB) (Adapter, Generated)

// RunToolchainSuite is the minimum every satellite's harness runs:
// the generated output parses, it type-checks, and its own tests
// pass.
//
// Parsing needs no toolchain, so the parse check runs on every
// machine. The two checks after it gate on the toolchain once:
// absent locally they skip with the reason the adapter gave, and
// absent in CI they fail, so a regression cannot hide behind a
// missing compiler. A satellite adds its own assertions in its own
// test, beside this call, because the kernel set is the minimum
// every satellite runs.
func RunToolchainSuite(t *testing.T, setup Setup) {
	t.Helper()

	t.Run("the generated output parses", func(t *testing.T) {
		t.Parallel()
		a, g := setup(t)
		AssertParses(t, a, g)
	})

	a, _ := setup(t)
	if !Require(t, a) {
		return
	}
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
