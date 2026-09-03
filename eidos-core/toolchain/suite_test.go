// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package toolchain_test

import (
	"sync"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/toolchain"
)

// The suite is the floor a satellite runs, and Require is the gate
// every toolchain assertion passes through, so both are contract.
func TestSuite(t *testing.T) {
	t.Parallel()

	t.Run("runs the three checks over a healthy toolchain", func(t *testing.T) {
		t.Parallel()

		var mu sync.Mutex
		runs := 0
		// The suite's checks run in parallel, so the count settles
		// after they finish rather than when the call returns.
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
