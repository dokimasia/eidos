// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package gosource_test

import (
	"cmp"
	"go/token"
	"go/types"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/internal/gosource"
)

// libPath is the import path of the fixture module's package without
// imports.
const libPath = "example.test/fixture/lib"

// The ceilings of an importer over the fixture module.
const (
	// newImporterAllocs is one importer: the importer, the compiler's
	// source importer, its cache and its loading set, and the read of
	// the module's go.mod.
	newImporterAllocs = 13
	// importAllocs is one first import of the fixture's lib package: the
	// parse of its two files, the generated one included, and their
	// type-check.
	importAllocs = 168
)

// The importer resolves module-local packages from source and
// delegates the rest, so its cache, its cycle refusal and its
// delegation are pinned.
func TestImporter(t *testing.T) {
	t.Parallel()

	modRoot := fixtureRoot(t)

	t.Run("NewImporter", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error for a module root without a go.mod", func(t *testing.T) {
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
			pkg, err := imp.Import(libPath)
			assert.NoError(t, err, "a module-local package imports from disk")
			assert.NotNil(t, pkg.Scope().Lookup("Value"), "with its declarations resolved")
		})

		t.Run("resolves a dependency's generated declarations", func(t *testing.T) {
			t.Parallel()

			imp, err := gosource.NewImporter(token.NewFileSet(), modRoot)
			assert.NoError(t, err, "the importer builds")
			pkg, err := imp.Import(libPath)
			assert.NoError(t, err, "the dependency imports")
			assert.NotNil(t, pkg.Scope().Lookup("Generated"),
				"a dependency's generated declarations resolve: hand-written code "+
					"there may refer to what its own generator produced")
		})

		t.Run("returns the same package on a second call", func(t *testing.T) {
			t.Parallel()

			imp, err := gosource.NewImporter(token.NewFileSet(), modRoot)
			assert.NoError(t, err, "the importer builds")
			first, err := imp.Import(libPath)
			assert.NoError(t, err, "the first import returns")
			second, err := imp.Import(libPath)
			assert.NoError(t, err, "and the second")
			assert.Equal(t, first, second,
				"one path returns one package, so type identities agree", assert.ByIdentity())
		})

		t.Run("delegates a standard-library path", func(t *testing.T) {
			t.Parallel()

			imp, err := gosource.NewImporter(token.NewFileSet(), modRoot)
			assert.NoError(t, err, "the importer builds")
			pkg, err := imp.Import("strings")
			assert.NoError(t, err, "a standard-library path delegates")
			assert.NotNil(t, pkg.Scope().Lookup("ToUpper"), "and resolves")
		})

		t.Run("returns an error for a path imported while it loads", func(t *testing.T) {
			t.Parallel()

			imp, err := gosource.NewImporter(token.NewFileSet(), modRoot)
			assert.NoError(t, err, "the importer builds")
			_, err = imp.Import("example.test/fixture/cycle/a")
			assert.HasError(t, err, "an import cycle refuses rather than recursing")
			assert.Contains(t, err.Error(), "import cycle", "naming the defect")

			_, err = imp.Import(libPath)
			assert.NoError(t, err, "and the refusal belongs to that import alone")
		})

		t.Run("returns an error for a module-local path that does not exist", func(t *testing.T) {
			t.Parallel()

			imp, err := gosource.NewImporter(token.NewFileSet(), modRoot)
			assert.NoError(t, err, "the importer builds")
			_, err = imp.Import("example.test/fixture/missing")
			assert.HasError(t, err, "a module-local path that does not exist is reported")
		})
	})
}

// An importer, a first import and a cached import allocate within their
// ceilings in the ordinary run, which runs no benchmark. Each first
// import takes an importer built outside the count, because an import
// fills the cache it reads. Each count keeps the first error of its
// calls, which cmp.Or returns without allocating. The check runs alone,
// because the count includes every goroutine's allocations.
func TestImporterAllocs(t *testing.T) {
	modRoot, fset := fixtureRoot(t), token.NewFileSet()
	var (
		imp *gosource.Importer
		err error
	)
	assert.MaxAllocs(t, func() {
		var nerr error
		imp, nerr = gosource.NewImporter(fset, modRoot)
		err = cmp.Or(err, nerr)
	}, newImporterAllocs, "NewImporter allocates the importer and reads the go.mod")
	assert.NoError(t, err, "the importer builds")
	fresh := func() *gosource.Importer {
		built, nerr := gosource.NewImporter(token.NewFileSet(), modRoot)
		assert.NoError(t, nerr, "the importer builds")
		return built
	}
	var pkg *types.Package
	assert.MaxAllocsWithSetup(t, fresh, func(i *gosource.Importer) {
		var ierr error
		pkg, ierr = i.Import(libPath)
		err = cmp.Or(err, ierr)
	}, importAllocs, "Import allocates the parse and the type-check of a first import")
	assert.NoError(t, err, "every first import returns")
	assert.Equal(t, pkg.Path(), libPath, "the imported package")
	assert.MaxAllocs(t, func() {
		_, ierr := imp.Import(libPath)
		err = cmp.Or(err, ierr)
	}, 0, "Import allocates nothing for a package imported before")
	assert.NoError(t, err, "the cached import returns")
}

// BenchmarkImporter measures an importer's construction, a first import
// of a module-local package, and a cached import.
func BenchmarkImporter(b *testing.B) {
	modRoot := fixtureRoot(b)

	b.Run("NewImporter", func(b *testing.B) {
		fset := token.NewFileSet()
		c := bench.Start(b).MaxAllocs(newImporterAllocs)
		defer c.End()
		var (
			got *gosource.Importer
			err error
		)
		for c.Loop() {
			got, err = gosource.NewImporter(fset, modRoot)
		}
		assert.NoError(b, err, "the importer builds")
		assert.NotNil(b, got, "NewImporter returns the importer")
	})

	b.Run("Import", func(b *testing.B) {
		b.Run("a module-local package", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(importAllocs)
			defer c.End()
			var (
				imp *gosource.Importer
				pkg *types.Package
				err error
			)
			for c.Loop() {
				c.Excluding(func() { imp, err = gosource.NewImporter(token.NewFileSet(), modRoot) })
				pkg, err = imp.Import(libPath)
			}
			assert.NoError(b, err, "the package imports")
			assert.Equal(b, pkg.Path(), libPath, "Import returns the package")
		})

		b.Run("a package imported before", func(b *testing.B) {
			imp, err := gosource.NewImporter(token.NewFileSet(), modRoot)
			assert.NoError(b, err, "the importer builds")
			// The warm-up call imports the package into the cache.
			c := bench.Start(b).Warmup(1).MaxAllocs(0)
			defer c.End()
			var pkg *types.Package
			for c.Loop() {
				pkg, err = imp.Import(libPath)
			}
			assert.NoError(b, err, "the package imports")
			assert.Equal(b, pkg.Path(), libPath, "from the cache")
		})
	})
}

// fixtureRoot returns the absolute root of the fixture module.
func fixtureRoot(tb testing.TB) string {
	tb.Helper()

	root, err := filepath.Abs("testdata/mod")
	assert.NoError(tb, err, "the fixture module root resolves")
	return root
}
