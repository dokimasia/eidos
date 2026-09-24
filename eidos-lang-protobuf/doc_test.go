// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protobuf_test

import (
	"os"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
)

// The package documentation states the read-only shape and the
// dependency position, both of which the rest of the system relies
// on.
func TestDoc(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("doc.go")
	assert.NoError(t, err, "the package comment is on disk")
	text := string(src)

	t.Run("states its dependency position", func(t *testing.T) {
		t.Parallel()

		assert.True(t, strings.Contains(text, "# Dependency position"),
			"the package comment states its dependency position")
	})

	t.Run("states the read-only shape", func(t *testing.T) {
		t.Parallel()

		assert.True(t, strings.Contains(text, "Read-only"),
			"the package comment states the read-only shape as a contract")
		for _, absent := range []string{"lowering", "backend"} {
			assert.True(t, strings.Contains(text, absent),
				"the comment names "+absent+" among what the module does not ship")
		}
	})

	t.Run("ships no package the read-only shape forbids", func(t *testing.T) {
		t.Parallel()

		entries, err := os.ReadDir(".")
		assert.NoError(t, err, "the module directory reads")
		var packages []string
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && e.Name() != "testdata" {
				packages = append(packages, e.Name())
			}
		}
		assert.Equal(t, packages, []string{"frontend", "rules"},
			"a read-only language ships a frontend and rules, and nothing else")
	})
}
