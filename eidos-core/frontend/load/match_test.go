// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/frontend/load"
)

// claiming returns a frontend claiming exactly these patterns.
func claiming(patterns ...string) *frontendtest.Scripted {
	f := frontendtest.NewScripted()
	f.Sel = patterns
	return f
}

// Selection is the file claim, so the glob grammar is pinned: whole
// paths, segment spanning, negation, order.
func TestMatch(t *testing.T) {
	t.Parallel()

	t.Run("Match", func(t *testing.T) {
		t.Parallel()

		t.Run("matches whole workspace-relative paths", func(t *testing.T) {
			t.Parallel()

			assert.True(t, load.Match([]string{"**/*.go"}, "a/b/c.go"), "** spans segments")
			assert.True(t, load.Match([]string{"**/*.go"}, "c.go"), "including none")
			assert.False(t, load.Match([]string{"*.go"}, "a/c.go"), "one star stays in its segment")
			assert.True(t, load.Match([]string{"a/?.go"}, "a/c.go"), "path.Match inside a segment")
			assert.False(t, load.Match([]string{"a/*.go"}, "a/b/c.go"),
				"a short pattern claims no deeper path")
		})

		t.Run("negation carves a claim, in order", func(t *testing.T) {
			t.Parallel()

			claim := []string{"**/*.go", "!**/testdata/**"}
			assert.True(t, load.Match(claim, "pkg/x.go"), "claimed")
			assert.False(t, load.Match(claim, "pkg/testdata/x.go"), "carved out")
			assert.True(t, load.Match(append(claim, "**/testdata/keep.go"), "a/testdata/keep.go"),
				"the last match decides")
		})
	})

	t.Run("checkPattern", func(t *testing.T) {
		t.Parallel()

		tree := func() fstest.MapFS {
			return fstest.MapFS{oneFile: {Data: []byte("package shared\n")}}
		}

		t.Run("refuses a pattern outside the grammar", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, tree(), with(claiming("[bad")))
			assert.Contains(t, err.Error(), "[bad", "naming the pattern")
			assert.Contains(t, err.Error(), "glob grammar",
				"a claim that cannot be read must not claim nothing")
		})

		t.Run("refuses a pattern that claims nothing", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, tree(), with(claiming("!")))
			assert.Contains(t, err.Error(), "claims nothing",
				"a negation with nothing behind it is not a claim")
		})
	})
}
