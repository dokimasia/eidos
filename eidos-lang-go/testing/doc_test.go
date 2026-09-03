// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing_test

import (
	"testing"

	"go.dokimi.dev/assert"

	gotesting "go.dokimi.dev/eidos/lang/go/testing"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The harness is an adapter and nothing more, which is the claim
// the documentation makes and the kernel's set rests on.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("satisfies the kernel's adapter and adds one assertion", func(t *testing.T) {
		t.Parallel()

		assert.NotNil(t, adapterConformance, "New returns the adapter the kernel drives")
		assert.NotNil(t, vetsConformance, "and the Go-only assertion takes the kernel's own handle")
	})
}

// The harness's whole contract with the kernel, checked where it
// compiles rather than where a run reaches it: the adapter is the
// kernel's, and the added assertion takes the kernel's own handle.
var (
	adapterConformance toolchain.Adapter                                          = gotesting.New()
	vetsConformance    func(toolchain.TB, toolchain.Adapter, toolchain.Generated) = gotesting.AssertVets
)
