// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	java "go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// marked is what java.testFile records for one test file: one stamp, true.
var marked = []any{true}

// Whether a file takes part as a test is the consumer's call, so the
// stamp follows the Maven and Gradle layout and Surefire's default
// includes exactly.
func TestClassify(t *testing.T) {
	t.Parallel()

	t.Run("markTests", func(t *testing.T) {
		t.Parallel()

		paths := []struct {
			name string
			give string
			want []any
		}{
			{name: "stamps a file under src/test/", give: "src/test/java/com/acme/Helper.java", want: marked},
			{name: "stamps a file under a module's src/test/", give: "store/src/test/java/Helper.java", want: marked},
			{name: "stamps a file whose name starts with Test", give: "src/main/java/TestHelper.java", want: marked},
			{name: "stamps a file whose name ends in Test", give: "src/main/java/HelperTest.java", want: marked},
			{name: "stamps a file whose name ends in Tests", give: "src/main/java/HelperTests.java", want: marked},
			{name: "stamps a file whose name ends in TestCase", give: "src/main/java/ATestCase.java", want: marked},
			{name: "stamps no file under a directory that only starts with src/test", give: "src/testing/Helper.java"},
			{name: "stamps no file whose name ends in test in lowercase", give: "src/main/java/Contest.java"},
		}
		for _, tt := range paths {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, testStamps(t, tt.give), tt.want, "the layout and Surefire's includes decide")
			})
		}
	})
}

// testStamps parses one file at a path through the frontend's parse and
// classifiers, and returns the values of the test-file stamps it
// recorded.
func testStamps(tb assert.TB, p string) []any {
	tb.Helper()

	gb, _ := parsedTree(tb, fstest.MapFS{p: {Data: []byte(publicClass)}}, p, plugin.DepthFull)
	return stampsOf(gb, java.TestFileKey)
}
