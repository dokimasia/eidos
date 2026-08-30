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
				"files read in name order, so two runs answer alike")
		})

		t.Run("reports a directory holding no hand-written file", func(t *testing.T) {
			t.Parallel()

			_, err := gosource.ParseDir(token.NewFileSet(), "testdata/empty", gosource.HandWritten)
			assert.HasError(t, err, "a directory holding no readable file is reported")
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
	})
}
