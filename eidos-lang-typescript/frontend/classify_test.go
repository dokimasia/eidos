// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	typescript "go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Whether a file takes part as a test is the consumer's call, so the
// stamp follows Jest's default match exactly.
func TestClassify(t *testing.T) {
	t.Parallel()

	t.Run("markTests", func(t *testing.T) {
		t.Parallel()

		paths := []struct {
			name string
			give string
			want int
		}{
			{name: "stamps a file under a __tests__ directory", give: "src/__tests__/a.ts", want: 1},
			{name: "stamps a file named test", give: "src/test.ts", want: 1},
			{name: "stamps a file named spec", give: "src/spec.ts", want: 1},
			{name: "stamps a file whose name ends in .test", give: "src/a.test.ts", want: 1},
			{name: "stamps a TSX file whose name ends in .spec", give: "src/a.spec.tsx", want: 1},
			{name: "stamps no other file", give: "src/testing.ts", want: 0},
		}
		for _, tt := range paths {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, testStamps(t, tt.give), tt.want, "Jest's default match decides")
			})
		}
	})
}

// testStamps parses one file at a path through the frontend's parse and
// classifiers, and returns how many test-file stamps it recorded.
func testStamps(tb testing.TB, path string) int {
	tb.Helper()

	f := frontend.New()
	tree := fstest.MapFS{path: {Data: []byte(exportClass)}}
	u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: path}}, tree, plugin.DepthFull,
		f.Syntax(), brand, diag.NewSink(), f.Name())
	assert.NoError(tb, f.Parse(tb.Context(), u), "the file parses")
	n := 0
	for _, s := range u.Graph().StampRecords() {
		if s.Stamp.Key == typescript.TestFileKey {
			n++
		}
	}
	return n
}
