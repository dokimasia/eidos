// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package gosource_test

import (
	"cmp"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/core/internal/gosource"
)

// goModName is the file every loader here reads a module out of.
const goModName = "go.mod"

// The ceilings of the module lookups over the fixture module.
const (
	// moduleRootAllocs is one search from the fixture's lib directory:
	// the absolute path, and a join and a stat for each of the two
	// go.mod candidates, the missing one's error included.
	moduleRootAllocs = 12
	// modulePathAllocs is one read of the fixture's go.mod: its path,
	// the read of its bytes, their copy as a string, and the fields of
	// its first line, which is the module directive.
	modulePathAllocs = 8
)

// A module is found by its go.mod and named by its module directive, so
// the search and the read are pinned.
func TestModule(t *testing.T) {
	t.Parallel()

	t.Run("ModuleRoot", func(t *testing.T) {
		t.Parallel()

		t.Run("finds the nearest enclosing module", func(t *testing.T) {
			t.Parallel()

			got, err := gosource.ModuleRoot(libDir)
			assert.NoError(t, err, "a directory inside a module resolves")
			assert.Equal(t, got, fixtureRoot(t), "to the nearest enclosing module")
		})

		t.Run("returns an absolute path", func(t *testing.T) {
			t.Parallel()

			got, err := gosource.ModuleRoot("testdata/mod")
			assert.NoError(t, err, "the module root resolves")
			assert.True(t, filepath.IsAbs(got), "and is absolute")
		})

		t.Run("returns an error for a directory outside any module", func(t *testing.T) {
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

			root := files.Workspace(t, files.Tree{
				goModName: files.Text(
					"// The module this fixture declares.\n\nmodule example.test/later\n\ngo 1.27.0\n",
				),
			})
			got, err := gosource.ModulePath(root)
			assert.NoError(t, err, "a directive below the first line reads")
			assert.Equal(t, got, "example.test/later",
				"the comment and the blank line above it are passed over")
		})

		t.Run("unquotes a quoted module path", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, files.Tree{goModName: files.Text("module \"example.test/quoted\"\n")})
			got, err := gosource.ModulePath(root)
			assert.NoError(t, err, "a quoted directive reads")
			assert.Equal(t, got, "example.test/quoted",
				"and comes back without its quotes, which are no part of the path")
		})

		t.Run("returns an error for a go.mod without a module directive", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, files.Tree{goModName: files.Text("go 1.27.0\n")})
			_, err := gosource.ModulePath(root)
			assert.HasError(t, err, "a go.mod without a module directive is reported")
			assert.Contains(t, err.Error(), filepath.Join(root, goModName),
				"naming the file it read to the end")
			assert.HasPrefix(t, err.Error(), "gosource: ", "under the package prefix")
		})

		t.Run("returns an error for a directory without a go.mod", func(t *testing.T) {
			t.Parallel()

			_, err := gosource.ModulePath("testdata/empty")
			assert.HasError(t, err, "a directory without a go.mod is reported")
			assert.HasPrefix(t, err.Error(), "gosource: ", "under the package prefix")
		})
	})
}

// The module lookups allocate within their ceilings in the ordinary
// run, which runs no benchmark. Each count keeps the first error of its
// calls, which cmp.Or returns without allocating.
func TestModuleAllocs(t *testing.T) {
	var (
		got string
		err error
	)
	assert.MaxAllocs(t, func() {
		var merr error
		got, merr = gosource.ModuleRoot(libDir)
		err = cmp.Or(err, merr)
	}, moduleRootAllocs, "ModuleRoot allocates the path and the stats")
	assert.NoError(t, err, "the directory is inside a module")
	assert.Equal(t, got, fixtureRoot(t), "ModuleRoot returns the fixture root")
	assert.MaxAllocs(t, func() {
		var merr error
		got, merr = gosource.ModulePath("testdata/mod")
		err = cmp.Or(err, merr)
	}, modulePathAllocs, "ModulePath allocates the read of the go.mod")
	assert.NoError(t, err, "the go.mod reads")
	assert.Equal(t, got, "example.test/fixture", "ModulePath returns the module directive")
}

// BenchmarkModule measures the search for the module a directory is in,
// and the read of the module's path.
func BenchmarkModule(b *testing.B) {
	b.Run("ModuleRoot", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(moduleRootAllocs)
		defer c.End()
		var (
			got string
			err error
		)
		for c.Loop() {
			got, err = gosource.ModuleRoot(libDir)
		}
		assert.NoError(b, err, "the directory is inside a module")
		assert.Equal(b, got, fixtureRoot(b), "ModuleRoot returns the fixture root")
	})

	b.Run("ModulePath", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(modulePathAllocs)
		defer c.End()
		var (
			got string
			err error
		)
		for c.Loop() {
			got, err = gosource.ModulePath("testdata/mod")
		}
		assert.NoError(b, err, "the go.mod reads")
		assert.Equal(b, got, "example.test/fixture", "ModulePath returns the module directive")
	})
}
