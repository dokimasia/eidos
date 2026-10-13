// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package authored_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/authored"
	"go.dokimi.dev/eidos/core/diag"
)

func TestCodes(t *testing.T) {
	t.Parallel()

	t.Run("the witness refusal is a kernel code", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, authored.UnknownWitnessParam.Prefix, diag.KernelPrefix, "registered under the kernel")
		assert.Equal(t, authored.UnknownWitnessParam.Number, 41, "at its own number")
	})

	t.Run("the refusal of an empty sample is a kernel code", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, authored.EmptySample.Prefix, diag.KernelPrefix, "registered under the kernel")
		assert.Equal(t, authored.EmptySample.Number, 76, "at its own number")
	})

	t.Run("the refusal of an empty witness is a kernel code", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, authored.EmptyWitness.Prefix, diag.KernelPrefix, "registered under the kernel")
		assert.Equal(t, authored.EmptyWitness.Number, 77, "at its own number")
	})
}
