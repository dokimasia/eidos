// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package gosource_test

import (
	"go/token"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/gosource"
)

func TestImporter(t *testing.T) {
	t.Parallel()

	modRoot, err := filepath.Abs("testdata/mod")
	assert.NoError(t, err, "the fixture module root resolves")

	t.Run("NewImporter", func(t *testing.T) {
		t.Parallel()

		t.Run("reports a module root holding no go.mod", func(t *testing.T) {
			t.Parallel()

			_, err := gosource.NewImporter(token.NewFileSet(), t.TempDir())
			assert.HasError(t, err, "a module root holding no go.mod is reported")
			assert.Contains(t, err.Error(), goModName, "naming the file it could not read")
			assert.HasPrefix(t, err.Error(), "gosource: ", "under the package prefix")
		})
	})

	t.Run("Import", func(t *testing.T) {
		t.Parallel()

		t.Run("resolves a module-local package from disk", func(t *testing.T) {
			t.Parallel()

			imp, err := gosource.NewImporter(token.NewFileSet(), modRoot)
			assert.NoError(t, err, "the importer builds")
			pkg, err := imp.Import("example.test/fixture/lib")
			assert.NoError(t, err, "a module-local package imports from disk")
			assert.NotNil(t, pkg.Scope().Lookup("Value"), "with its declarations resolved")
		})

		t.Run("resolves a dependency's generated declarations", func(t *testing.T) {
			t.Parallel()

			imp, err := gosource.NewImporter(token.NewFileSet(), modRoot)
			assert.NoError(t, err, "the importer builds")
			pkg, err := imp.Import("example.test/fixture/lib")
			assert.NoError(t, err, "the dependency imports")
			assert.NotNil(t, pkg.Scope().Lookup("Generated"),
				"a dependency's generated declarations resolve: hand-written code "+
					"there may refer to what its own generator produced")
		})

		t.Run("returns the same package on a second call", func(t *testing.T) {
			t.Parallel()

			imp, err := gosource.NewImporter(token.NewFileSet(), modRoot)
			assert.NoError(t, err, "the importer builds")
			first, err := imp.Import("example.test/fixture/lib")
			assert.NoError(t, err, "the first import returns")
			second, err := imp.Import("example.test/fixture/lib")
			assert.NoError(t, err, "and the second")
			assert.True(t, first == second,
				"one path returns one package, so type identities agree")
		})

		t.Run("delegates a standard-library path", func(t *testing.T) {
			t.Parallel()

			imp, err := gosource.NewImporter(token.NewFileSet(), modRoot)
			assert.NoError(t, err, "the importer builds")
			pkg, err := imp.Import("strings")
			assert.NoError(t, err, "a standard-library path delegates")
			assert.NotNil(t, pkg.Scope().Lookup("ToUpper"), "and resolves")
		})

		t.Run("refuses a path imported while it loads", func(t *testing.T) {
			t.Parallel()

			imp, err := gosource.NewImporter(token.NewFileSet(), modRoot)
			assert.NoError(t, err, "the importer builds")
			_, err = imp.Import("example.test/fixture/cycle/a")
			assert.HasError(t, err, "an import cycle refuses rather than recursing")
			assert.Contains(t, err.Error(), "import cycle", "naming the defect")

			_, err = imp.Import("example.test/fixture/lib")
			assert.NoError(t, err, "and the refusal belongs to that import alone")
		})

		t.Run("reports a module-local path that does not exist", func(t *testing.T) {
			t.Parallel()

			imp, err := gosource.NewImporter(token.NewFileSet(), modRoot)
			assert.NoError(t, err, "the importer builds")
			_, err = imp.Import("example.test/fixture/missing")
			assert.HasError(t, err, "a module-local path that does not exist is reported")
		})
	})
}
