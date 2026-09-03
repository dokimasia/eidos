// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing_test

import (
	"testing"

	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The kernel's assertion set runs over a generated fixture through
// the Go toolchain, which is what the harness exists to make
// possible: the assertions and their wording are the kernel's, and
// only the adapter beneath them is Go's.
func TestSuite(t *testing.T) {
	t.Parallel()

	toolchain.RunToolchainSuite(t, func(toolchain.TB) (toolchain.Adapter, toolchain.Generated) {
		return adapter(), healthy()
	})
}
