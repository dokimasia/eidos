// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package toolchain_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/toolchain"
)

// A run over nothing proves nothing, so the fixture and the report
// each state their own emptiness, and every assertion reads it.
func TestAdapter(t *testing.T) {
	t.Parallel()

	t.Run("Generated", func(t *testing.T) {
		t.Parallel()

		t.Run("IsEmpty", func(t *testing.T) {
			t.Parallel()

			assert.True(t, toolchain.Generated{}.IsEmpty(), "a fixture carrying no file is empty")
			assert.False(t, toolchain.Generated{Files: map[string][]byte{"a.go": nil}}.IsEmpty(),
				"and one carrying a file is not, whatever the file holds")
		})
	})

	t.Run("TestReport", func(t *testing.T) {
		t.Parallel()

		t.Run("OK", func(t *testing.T) {
			t.Parallel()

			assert.False(t, toolchain.TestReport{}.OK(), "a report of no case is not a pass")
			assert.True(t, toolchain.TestReport{Passed: 1, Skipped: 2}.OK(),
				"one passing case and no failure is a pass")
			assert.False(t, toolchain.TestReport{Passed: 3, Failed: 1}.OK(), "and one failure fails it")
		})
	})
}
