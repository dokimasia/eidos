// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package coretest_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/internal/coretest"
)

// Contracts projects a check's failure records onto what the kit cases
// compare: each record's contract, in record order.
func TestFailures(t *testing.T) {
	t.Parallel()

	t.Run("Contracts", func(t *testing.T) {
		t.Parallel()

		t.Run("returns each record's contract in record order", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "a check that breaks two contracts", func(tb assert.TB) {
				expect.Equal(tb, 1, 2, "the first contract")
				expect.True(tb, false, "the second contract")
			})
			assert.Equal(t, coretest.Contracts(got), []string{"the first contract", "the second contract"},
				"the contracts follow the records")
		})

		t.Run("returns nil for no record", func(t *testing.T) {
			t.Parallel()

			assert.Nil(t, coretest.Contracts(nil), "a check that broke nothing has no contract to name")
		})
	})
}
