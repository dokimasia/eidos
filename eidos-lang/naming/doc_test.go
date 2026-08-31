// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package naming_test

import (
	"os"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
)

// The package documentation is part of the contract: it states the
// dependency position every other package relies on.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("states its dependency position", func(t *testing.T) {
		t.Parallel()

		src, err := os.ReadFile("doc.go")
		assert.NoError(t, err, "the package comment is on disk")
		assert.True(t, strings.Contains(string(src), "# Dependency position"),
			"a file carries the package comment with its dependency position")
	})
}
