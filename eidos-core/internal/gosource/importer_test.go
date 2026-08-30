// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

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

		t.Run("answers the same package on a second call", func(t *testing.T) {
			t.Parallel()

			imp, err := gosource.NewImporter(token.NewFileSet(), modRoot)
			assert.NoError(t, err, "the importer builds")
			first, err := imp.Import("example.test/fixture/lib")
			assert.NoError(t, err, "the first import answers")
			second, err := imp.Import("example.test/fixture/lib")
			assert.NoError(t, err, "and the second")
			assert.True(t, first == second,
				"one path answers one package, so type identities agree")
		})

		t.Run("delegates a standard-library path", func(t *testing.T) {
			t.Parallel()

			imp, err := gosource.NewImporter(token.NewFileSet(), modRoot)
			assert.NoError(t, err, "the importer builds")
			pkg, err := imp.Import("strings")
			assert.NoError(t, err, "a standard-library path delegates")
			assert.NotNil(t, pkg.Scope().Lookup("ToUpper"), "and resolves")
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
