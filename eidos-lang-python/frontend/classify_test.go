// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// TestClassify pins the files that the frontend stamps as tests: the files
// that the default collection of pytest finds.
func TestClassify(t *testing.T) {
	t.Parallel()

	t.Run("testFile", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want []any
		}{
			{name: "stamps a module whose stem starts with test_", give: "tests/test_order.py", want: []any{true}},
			{name: "stamps a module whose stem ends with _test", give: "shop/order_test.py", want: []any{true}},
			{name: "stamps the fixtures of a directory in conftest.py", give: "tests/conftest.py", want: []any{true}},
			{name: "stamps no stub of a test module", give: "tests/test_order.pyi"},
			{name: "stamps no module whose stem contains test elsewhere", give: "shop/contest.py"},
			{name: "stamps no module of a directory of tests", give: "tests/helpers.py"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				gbs, _ := parsedTree(t, fstest.MapFS{tt.give: {Data: []byte("x = 1\n")}}, plugin.DepthFull)
				assert.Length(t, gbs, 1, "the module is one unit")
				files := gbs[0].Packages()[0].Files
				assert.Length(t, files, 1, "the unit has one file")
				got := stampsOn(gbs[0], files[0], python.TestFileKey)
				assert.Equal(t, got, tt.want, "the default collection of pytest decides", assert.EquateEmpty())
			})
		}
	})
}
