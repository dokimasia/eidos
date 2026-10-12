// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package toolchain_test

import (
	"sync/atomic"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/toolchain"
)

// The suite is the minimum that a satellite runs, and Require is the gate
// that every toolchain assertion passes, so both are contract.
func TestSuite(t *testing.T) {
	t.Parallel()

	t.Run("RunToolchainSuite", func(t *testing.T) {
		t.Parallel()

		t.Run("runs each of the three checks over a healthy toolchain", func(t *testing.T) {
			t.Parallel()

			var layouts atomic.Int32
			// The checks of the suite run in parallel, so the count settles
			// after they finish, and not when the call returns.
			t.Cleanup(func() {
				assert.Equal(t, layouts.Load(), int32(3), "each of the three checks lays the fixture out once")
			})
			healthy := scripted{report: toolchain.TestReport{Passed: 1}, layouts: &layouts}
			toolchain.RunToolchainSuite(t, healthy, output())
		})
	})

	t.Run("Require", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for a toolchain that is present", func(t *testing.T) {
			t.Parallel()

			s := &gateTB{Recorder: assert.NewRecorder()}
			assert.True(t, toolchain.Require(s, scripted{}), "the toolchain runs")
			assert.Empty(t, s.Failures(), "the gate reports nothing")
			assert.Equal(t, s.skipped, "", "the gate skips nothing")
		})

		t.Run("records a failure for a missing adapter", func(t *testing.T) {
			t.Parallel()

			s := &gateTB{Recorder: assert.NewRecorder()}
			assert.False(t, toolchain.Require(s, nil), "there is nothing to ask")
			assert.Equal(t, coretest.Contracts(s.Failures()),
				[]string{"toolchain: an adapter answers whether its toolchain is present"},
				"the failure has the contract of the gate")
		})
	})
}

// The gate reads a variable of the process, so the suite without a
// toolchain runs on its own, apart from the parallel cases.
func TestSuiteGate(t *testing.T) {
	t.Setenv("CI", "")

	var layouts atomic.Int32
	t.Run("skips every check without the toolchain", func(t *testing.T) {
		toolchain.RunToolchainSuite(t, scripted{absent: "scriptedc is not on PATH", layouts: &layouts}, output())
	})
	assert.Equal(t, layouts.Load(), int32(0), "no check lays the fixture out")
}
