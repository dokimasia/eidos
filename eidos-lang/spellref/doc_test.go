// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spellref_test

import (
	"os"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
)

// The package documentation is the register's contract, so its
// required section is pinned.
func TestDoc(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("doc.go")
	assert.NoError(t, err, "the package doc exists")
	assert.True(t, strings.Contains(string(src), "# Dependency position"),
		"and states its dependency position")
}
