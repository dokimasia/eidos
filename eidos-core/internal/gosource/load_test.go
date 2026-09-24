// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package gosource_test

import (
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/gosource"
)

// The unparseable file a case writes to fault the parse step: a
// parameter list that never closes.
const (
	brokenName   = "broken.go"
	brokenSource = "package p\n\nfunc F(\n"
)

func TestLoad(t *testing.T) {
	t.Parallel()

	t.Run("ParseDir", func(t *testing.T) {
		t.Parallel()

		t.Run("skips generated and test files", func(t *testing.T) {
			t.Parallel()

			fset := token.NewFileSet()
			files, err := gosource.ParseDir(fset, "testdata/mod/lib", gosource.HandWritten)
			assert.NoError(t, err, "the fixture directory parses")
			var got []string
			for _, f := range files {
				got = append(got, filepath.Base(fset.Position(f.Pos()).Filename))
			}
			assert.Equal(t, got, []string{"lib.go"},
				"hand-written mode skips generated and test files")
		})

		t.Run("reads generated files in complete mode", func(t *testing.T) {
			t.Parallel()

			fset := token.NewFileSet()
			files, err := gosource.ParseDir(fset, "testdata/mod/lib", gosource.Complete)
			assert.NoError(t, err, "the fixture directory parses")
			var got []string
			for _, f := range files {
				got = append(got, filepath.Base(fset.Position(f.Pos()).Filename))
			}
			assert.Equal(t, got, []string{"lib.gen.go", "lib.go"},
				"complete mode reads generated files too")
		})

		t.Run("reads files in name order", func(t *testing.T) {
			t.Parallel()

			fset := token.NewFileSet()
			files, err := gosource.ParseDir(fset, "testdata/mod/app", gosource.HandWritten)
			assert.NoError(t, err, "the fixture directory parses")
			var got []string
			for _, f := range files {
				got = append(got, filepath.Base(fset.Position(f.Pos()).Filename))
			}
			assert.Equal(t, got, []string{"app.go", "uses_stdlib.go"},
				"files read in name order, so two runs agree")
		})

		t.Run("reports a directory holding no hand-written file", func(t *testing.T) {
			t.Parallel()

			_, err := gosource.ParseDir(token.NewFileSet(), "testdata/empty", gosource.HandWritten)
			assert.HasError(t, err, "a directory holding no readable file is reported")
		})

		t.Run("reports a file it cannot parse", func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			assert.NoError(t,
				os.WriteFile(filepath.Join(dir, brokenName), []byte(brokenSource), 0o600),
				"the unparseable file writes")
			_, err := gosource.ParseDir(token.NewFileSet(), dir, gosource.HandWritten)
			assert.HasError(t, err, "a file that does not parse is reported")
			assert.Contains(t, err.Error(), brokenName, "naming the file it could not read")
			assert.HasPrefix(t, err.Error(), "gosource: ", "under the package prefix")
		})

		t.Run("reports a directory it cannot read", func(t *testing.T) {
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

			modRoot, err := filepath.Abs("testdata/mod")
			assert.NoError(t, err, "the module root resolves")
			pkg, files, err := gosource.Load(
				token.NewFileSet(), "testdata/mod/app", "example.test/fixture/app", modRoot,
				gosource.HandWritten,
			)
			assert.NoError(t, err, "the fixture package loads")
			assert.Length(t, files, 2, "with both hand-written files")
			assert.NotNil(t, pkg.Scope().Lookup("Holder"),
				"the package's own declarations type-check")
			assert.NotNil(t, pkg.Scope().Lookup("Upper"),
				"and its stdlib imports resolve")
		})

		t.Run("does not resolve generated declarations", func(t *testing.T) {
			t.Parallel()

			modRoot, err := filepath.Abs("testdata/mod")
			assert.NoError(t, err, "the module root resolves")
			pkg, _, err := gosource.Load(
				token.NewFileSet(), "testdata/mod/lib", "example.test/fixture/lib", modRoot,
				gosource.HandWritten,
			)
			assert.NoError(t, err, "the fixture package loads")
			assert.Nil(t, pkg.Scope().Lookup("Generated"),
				"hand-written mode does not resolve what the generator produced")
		})

		t.Run("reports a package that does not type-check", func(t *testing.T) {
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

			modRoot, err := filepath.Abs("testdata/mod")
			assert.NoError(t, err, "the module root resolves")
			pkg, _, err := gosource.Load(
				token.NewFileSet(), "testdata/mod/caller", "example.test/fixture/caller", modRoot,
				gosource.HandWritten,
			)
			assert.NoError(t, err,
				"the caller's time.Time passes into the clock package's time.Time, "+
					"because the nested load resolves time to the same package")
			assert.NotNil(t, pkg.Scope().Lookup("Now"), "and the caller type-checks whole")
		})

		t.Run("refuses an import cycle through the module's packages", func(t *testing.T) {
			t.Parallel()

			modRoot, err := filepath.Abs("testdata/mod")
			assert.NoError(t, err, "the module root resolves")
			_, _, err = gosource.Load(
				token.NewFileSet(), "testdata/mod/cycle/a", "example.test/fixture/cycle/a", modRoot,
				gosource.HandWritten,
			)
			assert.HasError(t, err, "a cycle refuses rather than recursing")
			assert.Contains(t, err.Error(), "import cycle through example.test/fixture/cycle/a",
				"naming the path imported while it loads")
			assert.HasPrefix(t, err.Error(), "gosource: ", "under the package prefix")
		})

		t.Run("reports a module root it cannot resolve imports against", func(t *testing.T) {
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
