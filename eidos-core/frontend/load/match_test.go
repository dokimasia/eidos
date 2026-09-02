// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load_test

import (
	"context"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/plugin"
)

// Selection is the file claim, so the glob grammar is pinned: whole
// paths, segment spanning, negation, order.
func TestMatch(t *testing.T) {
	t.Parallel()

	t.Run("matches whole workspace-relative paths", func(t *testing.T) {
		t.Parallel()

		assert.True(t, load.Match([]string{"**/*.go"}, "a/b/c.go"), "** spans segments")
		assert.True(t, load.Match([]string{"**/*.go"}, "c.go"), "including none")
		assert.False(t, load.Match([]string{"*.go"}, "a/c.go"), "one star stays in its segment")
		assert.True(t, load.Match([]string{"a/?.go"}, "a/c.go"), "path.Match inside a segment")
		assert.False(t, load.Match([]string{"a/*.go"}, "a/b/c.go"), "a short pattern claims no deeper path")
	})

	t.Run("negation carves a claim, in order", func(t *testing.T) {
		t.Parallel()

		claim := []string{"**/*.go", "!**/testdata/**"}
		assert.True(t, load.Match(claim, "pkg/x.go"), "claimed")
		assert.False(t, load.Match(claim, "pkg/testdata/x.go"), "carved out")
		assert.True(t, load.Match(append(claim, "**/testdata/keep.go"), "a/testdata/keep.go"),
			"the last match decides")
	})

	t.Run("refuses a pattern outside the grammar", func(t *testing.T) {
		t.Parallel()

		broken := frontendtest.NewScripted()
		broken.Sel = []string{"[bad"}
		_, _, err := load.Load(context.Background(), load.Config{
			FS:        fstest.MapFS{"x.zz": {Data: []byte("package p\n")}},
			Frontends: []plugin.Frontend{broken},
			Sink:      diag.NewSink(),
		})
		assert.HasError(t, err, "a claim that cannot be read must not claim nothing")
		assert.Contains(t, err.Error(), "[bad", "naming the pattern")
	})
}
