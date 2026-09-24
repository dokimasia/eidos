// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package toolchain_test

import (
	"sync"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/toolchain"
)

// The suite is the minimum a satellite runs, and Require is the gate
// every toolchain assertion passes through, so both are contract.
func TestSuite(t *testing.T) {
	t.Parallel()

	t.Run("runs the three checks over a healthy toolchain", func(t *testing.T) {
		t.Parallel()

		var mu sync.Mutex
		runs := 0
		// The suite's checks run in parallel, so the count settles
		// after they finish, not when the call returns.
		t.Cleanup(func() {
			mu.Lock()
			defer mu.Unlock()
			assert.Equal(t, runs, 4, "the gate sets up once and each of the three checks sets up its own")
		})
		toolchain.RunToolchainSuite(t, func(toolchain.TB) (toolchain.Adapter, toolchain.Generated) {
			mu.Lock()
			defer mu.Unlock()
			runs++
			return scripted{report: toolchain.TestReport{Passed: 1}}, output()
		})
	})

	t.Run("Require", func(t *testing.T) {
		t.Parallel()

		t.Run("admits a toolchain that is there", func(t *testing.T) {
			t.Parallel()

			var r recorder
			assert.True(t, toolchain.Require(&r, scripted{}), "the toolchain runs")
			assert.False(t, r.failed(), "and nothing is reported")
			assert.Equal(t, r.skipped, "", "nor skipped")
		})

		t.Run("refuses no adapter at all", func(t *testing.T) {
			t.Parallel()

			var r recorder
			assert.False(t, toolchain.Require(&r, nil), "there is nothing to ask")
			assert.True(t, r.says("no adapter"), "which the refusal says")
		})
	})
}

// The gate reads a process-wide variable, so the suite without a
// toolchain runs on its own, apart from the parallel cases above.
func TestSuiteGate(t *testing.T) {
	t.Setenv("CI", "")

	var mu sync.Mutex
	runs := 0
	t.Run("parses without the toolchain and skips the checks that run one", func(t *testing.T) {
		toolchain.RunToolchainSuite(t, func(toolchain.TB) (toolchain.Adapter, toolchain.Generated) {
			mu.Lock()
			defer mu.Unlock()
			runs++
			return scripted{absent: "scriptedc is not on PATH"}, output()
		})
	})
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, runs, 2, "the parse check and the gate each set up once, and no gated check runs")
}
