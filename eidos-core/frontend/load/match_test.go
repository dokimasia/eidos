// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load_test

import (
	"slices"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/frontend/load"
)

// testdataPath is a three-segment path the testdata claim's negation
// excludes.
const testdataPath = "pkg/testdata/x.go"

// testdataClaim is the claim the allocation check and the benchmark
// match against: every Go file, less the testdata trees.
var testdataClaim = []string{"**/*.go", "!**/testdata/**"}

// matchAllocs is one match of the two-pattern claim against the
// three-segment path: the compiled claim, one segment list for each
// pattern, and 3 for the growth of the path's segment list.
const matchAllocs = 6

// Selection is the file claim, so the glob grammar is pinned: whole
// paths, segment spanning, negation, order.
func TestMatch(t *testing.T) {
	t.Parallel()

	t.Run("Match", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			patterns []string
			give     string
			want     bool
		}{
			{
				name: "reports true for a ** spanning two segments", patterns: []string{"**/*.go"},
				give: "a/b/c.go", want: true,
			},
			{
				name: "reports true for a ** spanning no segment", patterns: []string{"**/*.go"},
				give: "c.go", want: true,
			},
			{
				name: "reports false for a star outside its segment", patterns: []string{"*.go"},
				give: "a/c.go", want: false,
			},
			{
				name: "reports true for a question mark inside a segment", patterns: []string{"a/?.go"},
				give: "a/c.go", want: true,
			},
			{
				name: "reports false for a path deeper than the pattern", patterns: []string{"a/*.go"},
				give: "a/b/c.go", want: false,
			},
			{
				name: "reports true for a path the negation leaves", patterns: testdataClaim,
				give: "pkg/x.go", want: true,
			},
			{
				name: "reports false for a path a later negation excludes", patterns: testdataClaim,
				give: testdataPath, want: false,
			},
			{
				name:     "reports true for a path a pattern after the negation claims",
				patterns: slices.Concat(testdataClaim, []string{"**/testdata/keep.go"}),
				give:     "a/testdata/keep.go", want: true,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, load.Match(tt.patterns, tt.give), tt.want, "the last matching pattern decides")
			})
		}
	})

	t.Run("checkPattern", func(t *testing.T) {
		t.Parallel()

		tree := func() fstest.MapFS {
			return fstest.MapFS{oneFile: {Data: []byte("package shared\n")}}
		}

		t.Run("returns an error for a pattern outside the glob grammar", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, tree(), with(claiming("[bad")))
			assert.Contains(t, err.Error(), "[bad", "naming the pattern")
			assert.Contains(t, err.Error(), "glob grammar",
				"a claim that cannot be read must not claim nothing")
		})

		t.Run("returns an error for a pattern that claims nothing", func(t *testing.T) {
			t.Parallel()

			err := refuse(t, tree(), with(claiming("!")))
			assert.Contains(t, err.Error(), "claims nothing",
				"a negation with nothing behind it is not a claim")
		})

		t.Run("returns an error for a segment no workspace-relative path contains", func(t *testing.T) {
			t.Parallel()

			for _, pattern := range []string{
				"/a/*.zz", "a//one.zz", "a/", "./a/one.zz", "a/../b/*.zz", "!**/./one.zz",
			} {
				err := refuse(t, tree(), with(claiming("**/*.zz", pattern)))
				assert.Contains(t, err.Error(), pattern, "naming the pattern")
				assert.Contains(t, err.Error(), "empty, . or .. segment",
					"a segment no path contains would claim nothing without a word")
			}
		})
	})
}

// A match allocates within its ceiling in the ordinary run, which runs
// no benchmark.
func TestMatchAllocs(t *testing.T) {
	claimed := true
	assert.MaxAllocs(t, func() { claimed = load.Match(testdataClaim, testdataPath) }, matchAllocs,
		"Match allocates the compiled claim and the path's segments")
	assert.False(t, claimed, "Match reports false for the excluded path")
}

// BenchmarkMatch measures one match of a claim with a negation.
func BenchmarkMatch(b *testing.B) {
	b.Run("Match", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(matchAllocs)
		defer c.End()
		got := true
		for c.Loop() {
			got = load.Match(testdataClaim, testdataPath)
		}
		assert.False(b, got, "the negation excludes the path")
	})
}

// claiming returns a frontend claiming exactly these patterns.
func claiming(patterns ...string) *frontendtest.Scripted {
	f := frontendtest.NewScripted()
	f.Sel = patterns
	return f
}
