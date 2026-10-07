// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package gosource_test

import (
	"cmp"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/core/internal/gosource"
)

// The unparseable file a case writes to fault the parse step: a
// parameter list that never closes.
const (
	brokenName   = "broken.go"
	brokenSource = "package p\n\nfunc F(\n"
)

// libDir is the fixture module's package without imports, as a
// directory the tests read.
const libDir = "testdata/mod/lib"

// The ceilings of a load over the fixture's lib package, each into a
// new file set.
const (
	// parseDirAllocs is one parse of the package's one hand-written
	// file into a new file set: the set, the directory's entries, and
	// the file's syntax tree with its comments.
	parseDirAllocs = 59
	// loadAllocs is one load of the package from its hand-written file:
	// a new file set, the importer, the parse, and the type-check.
	loadAllocs = 117
)

// The loader reads a package's files in name order and type-checks them
// against the module, so the files it reads and the errors it returns
// are pinned.
func TestLoad(t *testing.T) {
	t.Parallel()

	t.Run("ParseDir", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the hand-written files alone", func(t *testing.T) {
			t.Parallel()

			fset := token.NewFileSet()
			files, err := gosource.ParseDir(fset, libDir, gosource.HandWritten)
			assert.NoError(t, err, "the fixture directory parses")
			assert.Equal(t, names(fset, files), []string{"lib.go"},
				"hand-written mode skips generated and test files")
		})

		t.Run("reads generated files in complete mode", func(t *testing.T) {
			t.Parallel()

			fset := token.NewFileSet()
			files, err := gosource.ParseDir(fset, libDir, gosource.Complete)
			assert.NoError(t, err, "the fixture directory parses")
			assert.Equal(t, names(fset, files), []string{"lib.gen.go", "lib.go"},
				"complete mode reads generated files too")
		})

		t.Run("reads files in name order", func(t *testing.T) {
			t.Parallel()

			fset := token.NewFileSet()
			files, err := gosource.ParseDir(fset, "testdata/mod/app", gosource.HandWritten)
			assert.NoError(t, err, "the fixture directory parses")
			assert.Equal(t, names(fset, files), []string{"app.go", "uses_stdlib.go"},
				"files read in name order, so two runs agree")
		})

		t.Run("returns an error for a directory without a hand-written file", func(t *testing.T) {
			t.Parallel()

			_, err := gosource.ParseDir(token.NewFileSet(), "testdata/empty", gosource.HandWritten)
			assert.HasError(t, err, "a directory holding no readable file is reported")
		})

		t.Run("returns an error for a file it cannot parse", func(t *testing.T) {
			t.Parallel()

			dir := files.Workspace(t, files.Tree{brokenName: files.Text(brokenSource)})
			_, err := gosource.ParseDir(token.NewFileSet(), dir, gosource.HandWritten)
			assert.HasError(t, err, "a file that does not parse is reported")
			assert.Contains(t, err.Error(), brokenName, "naming the file it could not read")
			assert.HasPrefix(t, err.Error(), "gosource: ", "under the package prefix")
		})

		t.Run("returns an error for a directory it cannot read", func(t *testing.T) {
			t.Parallel()

			_, err := gosource.ParseDir(token.NewFileSet(), "testdata/nonexistent", gosource.HandWritten)
			assert.HasError(t, err, "a directory that does not exist is reported")
			assert.Contains(t, err.Error(), "nonexistent", "naming the directory")
		})
	})

	t.Run("Load", func(t *testing.T) {
		t.Parallel()

		t.Run("type-checks a package against its module-local imports", func(t *testing.T) {
			t.Parallel()

			pkg, files, err := gosource.Load(
				token.NewFileSet(), "testdata/mod/app", "example.test/fixture/app", fixtureRoot(t),
				gosource.HandWritten,
			)
			assert.NoError(t, err, "the fixture package loads")
			assert.Length(t, files, 2, "with both hand-written files")
			assert.NotNil(t, pkg.Scope().Lookup("Holder"),
				"the package's own declarations type-check")
			assert.NotNil(t, pkg.Scope().Lookup("Upper"),
				"and its stdlib imports resolve")
		})

		t.Run("returns a package without the generated declarations", func(t *testing.T) {
			t.Parallel()

			pkg, _, err := gosource.Load(token.NewFileSet(), libDir, libPath, fixtureRoot(t), gosource.HandWritten)
			assert.NoError(t, err, "the fixture package loads")
			assert.Nil(t, pkg.Scope().Lookup("Generated"),
				"hand-written mode does not resolve what the generator produced")
		})

		t.Run("returns an error for a package that does not type-check", func(t *testing.T) {
			t.Parallel()

			_, _, err := gosource.Load(
				token.NewFileSet(), "testdata/mod/app", "example.test/fixture/app", "",
				gosource.HandWritten,
			)
			assert.HasError(t, err, "a package that does not type-check is reported")
		})

		t.Run("returns what resolved from a package that does not type-check", func(t *testing.T) {
			t.Parallel()

			pkg, files, err := gosource.Load(
				token.NewFileSet(), "testdata/mod/app", "example.test/fixture/app", "",
				gosource.Complete,
			)
			assert.NoError(t, err,
				"complete mode collects the type-checking errors instead of stopping")
			assert.Length(t, files, 2, "every file is still read")
			assert.NotNil(t, pkg.Scope().Lookup("Upper"),
				"and the names that resolved come back, so a dependency that does not "+
					"compile still yields its surface")
		})

		t.Run("resolves one import path to one package across nested loads", func(t *testing.T) {
			t.Parallel()

			pkg, _, err := gosource.Load(
				token.NewFileSet(), "testdata/mod/caller", "example.test/fixture/caller", fixtureRoot(t),
				gosource.HandWritten,
			)
			assert.NoError(t, err,
				"the caller's time.Time passes into the clock package's time.Time, "+
					"because the nested load resolves time to the same package")
			assert.NotNil(t, pkg.Scope().Lookup("Now"), "and the caller type-checks whole")
		})

		t.Run("returns an error for an import cycle through the module's packages", func(t *testing.T) {
			t.Parallel()

			_, _, err := gosource.Load(
				token.NewFileSet(), "testdata/mod/cycle/a", "example.test/fixture/cycle/a", fixtureRoot(t),
				gosource.HandWritten,
			)
			assert.HasError(t, err, "a cycle refuses rather than recursing")
			assert.Contains(t, err.Error(), "import cycle through example.test/fixture/cycle/a",
				"naming the path imported while it loads")
			assert.HasPrefix(t, err.Error(), "gosource: ", "under the package prefix")
		})

		t.Run("returns an error for a module root without a go.mod", func(t *testing.T) {
			t.Parallel()

			_, _, err := gosource.Load(
				token.NewFileSet(), "testdata/mod/app", "example.test/fixture/app", t.TempDir(),
				gosource.HandWritten,
			)
			assert.HasError(t, err, "a module root holding no go.mod is reported")
			assert.Contains(t, err.Error(), goModName, "naming the file it could not read")
			assert.HasPrefix(t, err.Error(), "gosource: ", "under the package prefix")
		})
	})
}

// A parse and a load of the fixture's lib package allocate within their
// ceilings in the ordinary run, which runs no benchmark. Each count
// keeps the first error of its calls, which cmp.Or returns without
// allocating. The check runs alone, because the count includes every
// goroutine's allocations.
func TestLoadAllocs(t *testing.T) {
	modRoot := fixtureRoot(t)
	var (
		parsed []*ast.File
		err    error
	)
	assert.MaxAllocs(t, func() {
		var perr error
		parsed, perr = gosource.ParseDir(token.NewFileSet(), libDir, gosource.HandWritten)
		err = cmp.Or(err, perr)
	}, parseDirAllocs, "ParseDir allocates the entries and the syntax tree")
	assert.NoError(t, err, "the directory parses")
	assert.Length(t, parsed, 1, "into its one hand-written file")
	var pkg *types.Package
	assert.MaxAllocs(t, func() {
		var lerr error
		pkg, _, lerr = gosource.Load(token.NewFileSet(), libDir, libPath, modRoot, gosource.HandWritten)
		err = cmp.Or(err, lerr)
	}, loadAllocs, "Load allocates the importer, the parse and the type-check")
	assert.NoError(t, err, "the package loads")
	assert.NotNil(t, pkg.Scope().Lookup("Value"), "with its declarations type-checked")
}

// BenchmarkLoad measures a parse and a load of the fixture's package
// without imports, each into a new file set.
func BenchmarkLoad(b *testing.B) {
	b.Run("ParseDir", func(b *testing.B) {
		b.Run("a directory of one hand-written file", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(parseDirAllocs)
			defer c.End()
			var (
				files []*ast.File
				err   error
			)
			for c.Loop() {
				files, err = gosource.ParseDir(token.NewFileSet(), libDir, gosource.HandWritten)
			}
			assert.NoError(b, err, "the directory parses")
			assert.Length(b, files, 1, "into its one hand-written file")
		})
	})

	b.Run("Load", func(b *testing.B) {
		b.Run("a package without imports", func(b *testing.B) {
			modRoot := fixtureRoot(b)
			c := bench.Start(b).MaxAllocs(loadAllocs)
			defer c.End()
			var (
				pkg *types.Package
				err error
			)
			for c.Loop() {
				pkg, _, err = gosource.Load(token.NewFileSet(), libDir, libPath, modRoot, gosource.HandWritten)
			}
			assert.NoError(b, err, "the package loads")
			assert.NotNil(b, pkg.Scope().Lookup("Value"), "with its declarations type-checked")
		})
	})
}

// names returns the base names of the parsed files, in their order.
func names(fset *token.FileSet, files []*ast.File) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, filepath.Base(fset.Position(f.Pos()).Filename))
	}
	return out
}
