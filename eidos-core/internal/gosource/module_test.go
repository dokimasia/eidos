// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package gosource_test

import (
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/gosource"
)

// goModName is the file every loader here reads a module out of.
const goModName = "go.mod"

// goMod writes one go.mod into a fresh module root and returns the
// root, so a case can read a directive the committed fixture does
// not hold.
func goMod(t *testing.T, body string) string {
	t.Helper()

	root := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(root, goModName), []byte(body), 0o600),
		"the go.mod writes")
	return root
}

func TestModule(t *testing.T) {
	t.Parallel()

	t.Run("ModuleRoot", func(t *testing.T) {
		t.Parallel()

		t.Run("finds the nearest enclosing module", func(t *testing.T) {
			t.Parallel()

			got, err := gosource.ModuleRoot("testdata/mod/lib")
			assert.NoError(t, err, "a directory inside a module resolves")
			want, err := filepath.Abs("testdata/mod")
			assert.NoError(t, err, "the fixture root resolves")
			assert.Equal(t, got, want, "to the nearest enclosing module")
		})

		t.Run("returns an absolute path", func(t *testing.T) {
			t.Parallel()

			got, err := gosource.ModuleRoot("testdata/mod")
			assert.NoError(t, err, "the module root resolves")
			assert.True(t, filepath.IsAbs(got), "and is absolute")
		})

		t.Run("reports a directory outside any module", func(t *testing.T) {
			t.Parallel()

			_, err := gosource.ModuleRoot("/")
			assert.HasError(t, err, "a directory outside any module is reported")
		})
	})

	t.Run("ModulePath", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the module directive", func(t *testing.T) {
			t.Parallel()

			got, err := gosource.ModulePath("testdata/mod")
			assert.NoError(t, err, "a module with a go.mod returns")
			assert.Equal(t, got, "example.test/fixture", "its module directive")
		})

		t.Run("reads the directive below the lines above it", func(t *testing.T) {
			t.Parallel()

			root := goMod(t, "// The module this fixture declares.\n\nmodule example.test/later\n\ngo 1.27.0\n")
			got, err := gosource.ModulePath(root)
			assert.NoError(t, err, "a directive below the first line reads")
			assert.Equal(t, got, "example.test/later",
				"the comment and the blank line above it are passed over")
		})

		t.Run("unquotes a quoted module path", func(t *testing.T) {
			t.Parallel()

			got, err := gosource.ModulePath(goMod(t, "module \"example.test/quoted\"\n"))
			assert.NoError(t, err, "a quoted directive reads")
			assert.Equal(t, got, "example.test/quoted",
				"and comes back without its quotes, which are no part of the path")
		})

		t.Run("reports a go.mod holding no module directive", func(t *testing.T) {
			t.Parallel()

			root := goMod(t, "go 1.27.0\n")
			_, err := gosource.ModulePath(root)
			assert.HasError(t, err, "a go.mod holding no module directive is reported")
			assert.Contains(t, err.Error(), filepath.Join(root, goModName),
				"naming the file it read to the end")
			assert.HasPrefix(t, err.Error(), "gosource: ", "under the package prefix")
		})

		t.Run("reports a directory holding no go.mod", func(t *testing.T) {
			t.Parallel()

			_, err := gosource.ModulePath("testdata/empty")
			assert.HasError(t, err, "a directory holding no go.mod is reported")
			assert.HasPrefix(t, err.Error(), "gosource: ", "under the package prefix")
		})
	})
}
