// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
)

// The package documentation is part of the contract: it states the
// dependency position every other package relies on, and the
// hermeticity that position promises applies over the whole import
// graph.
func TestDoc(t *testing.T) {
	t.Parallel()

	t.Run("states its dependency position", func(t *testing.T) {
		t.Parallel()

		src, err := os.ReadFile("doc.go")
		assert.NoError(t, err, "the package comment is on disk")
		assert.True(t, strings.Contains(string(src), "# Dependency position"),
			"the package comment states its dependency position")
	})

	t.Run("never imports os/exec", func(t *testing.T) {
		t.Parallel()

		// The toolchain that runs this test lists the package's
		// transitive dependencies; the frontend itself runs nothing.
		out, err := exec.CommandContext(t.Context(),
			"go", "list", "-deps", "-f", "{{.ImportPath}}", ".").Output()
		assert.NoError(t, err, "the toolchain lists the package's dependencies")
		deps := strings.Split(strings.TrimSpace(string(out)), "\n")
		assert.True(t, slices.Contains(deps, "github.com/bufbuild/protocompile/parser"),
			"the listing includes the parser the frontend is built on")
		assert.False(t, slices.Contains(deps, "os/exec"),
			"a frontend never executes project code or build tools")
	})
}
