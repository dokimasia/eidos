// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package gosource_test

import (
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/gosource"
)

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

		t.Run("answers an absolute path", func(t *testing.T) {
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
			assert.NoError(t, err, "a module with a go.mod answers")
			assert.Equal(t, got, "example.test/fixture", "its module directive")
		})

		t.Run("reports a directory holding no go.mod", func(t *testing.T) {
			t.Parallel()

			_, err := gosource.ModulePath("testdata/empty")
			assert.HasError(t, err, "a directory holding no go.mod is reported")
			assert.HasPrefix(t, err.Error(), "gosource: ", "under the package prefix")
		})
	})
}
