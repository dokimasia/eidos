// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package directive_test

import (
	"testing"

	"go.dokimi.dev/eidos/core/internal/coretest"
)

// The package documentation is part of the contract: it states the
// dependency position every other package relies on.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("states its dependency position", func(t *testing.T) {
		t.Parallel()
		coretest.AssertDependencyPosition(t)
	})
}
