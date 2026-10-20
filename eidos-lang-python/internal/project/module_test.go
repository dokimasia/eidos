// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package project_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/python/internal/project"
)

// placeAllocs is the place of a file outside every root, two directories
// deep: the four joined paths of the modules of its two directories, and
// the growth of their list.
const placeAllocs = 4 + 2

// testUnit is the package directory of the test that the allocation test
// and the benchmark place.
const testUnit = "tests/unit"

// TestModule pins the root and the package that each file of a workspace
// takes.
func TestModule(t *testing.T) {
	t.Parallel()

	t.Run("Place", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the root of the project for a file below it", func(t *testing.T) {
			t.Parallel()

			p, err := project.Read(appFile, []byte("[project]\nname = \"app\"\n"))
			assert.NoError(t, err, "the project file decodes")
			root, probes := project.Place("app/src/shop/a.py", &p, func(string) bool { return false })
			assert.Equal(t, root, "app/src", "the source root of the project")
			assert.Empty(t, probes, "a root of the project depends on no further path")
		})

		tests := []struct {
			name       string
			giveFile   string
			givePkgs   []string
			wantRoot   string
			wantProbes []string
		}{
			{
				name: "returns the directory of a file outside every package directory", giveFile: "tools/run.py",
				wantRoot: "tools", wantProbes: []string{"tools/__init__.py", "tools/__init__.pyi"},
			},
			{
				name: "returns the parent of the highest package directory", giveFile: "tests/unit/test_a.py",
				givePkgs: []string{"tests", "tests/unit"}, wantRoot: ".",
				wantProbes: []string{
					"tests/unit/__init__.py", "tests/unit/__init__.pyi", "tests/__init__.py", "tests/__init__.pyi",
					"__init__.py", "__init__.pyi",
				},
			},
			{
				name:     "returns the directory that ends the run",
				giveFile: "lib/shop/models.py",
				givePkgs: []string{"lib/shop"},
				wantRoot: "lib",
				wantProbes: []string{
					"lib/shop/__init__.py",
					"lib/shop/__init__.pyi",
					"lib/__init__.py",
					"lib/__init__.pyi",
				},
			},
			{
				name: "returns the root of the workspace for a run up to the root", giveFile: "shop/a.py",
				givePkgs: []string{"shop", "."}, wantRoot: ".",
				wantProbes: []string{"shop/__init__.py", "shop/__init__.pyi", "__init__.py", "__init__.pyi"},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				root, probes := project.Place(tt.giveFile, nil, func(dir string) bool {
					return slices.Contains(tt.givePkgs, dir)
				})
				assert.Equal(t, root, tt.wantRoot, "the root above the run of package directories")
				assert.Equal(t, probes, tt.wantProbes, "the modules of each directory of the run")
			})
		}

		t.Run("returns the run for a file outside the roots of its project", func(t *testing.T) {
			t.Parallel()

			p, err := project.Read(appFile, []byte("[project]\nname = \"app\"\n"))
			assert.NoError(t, err, "the project file decodes")
			root, probes := project.Place("app/tests/test_a.py", &p, func(string) bool { return false })
			assert.Equal(t, root, "app/tests", "a test outside src takes the root of its run")
			assert.Equal(t, probes, []string{"app/tests/__init__.py", "app/tests/__init__.pyi"},
				"the modules of the directory that ends the run")
		})
	})

	t.Run("Package", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			giveFile string
			giveRoot string
			want     string
		}{
			{
				name: "returns the path below the root without the extension", giveFile: "src/shop/models.py",
				giveRoot: "src", want: "shop/models",
			},
			{
				name: "returns the path of a stub without the extension", giveFile: "src/shop/models.pyi",
				giveRoot: "src", want: "shop/models",
			},
			{
				name:     "returns the package of the directory for the module of a package directory",
				giveFile: "src/shop/__init__.py", giveRoot: "src", want: "shop",
			},
			{
				name:     "returns the package of the directory for the stub of a package directory",
				giveFile: "src/shop/__init__.pyi", giveRoot: "src", want: "shop",
			},
			{
				name:     "returns the package of the global scope for the module of a root",
				giveFile: "src/__init__.py", giveRoot: "src", want: "",
			},
			{
				name:     "returns the path of a file below the root of the workspace",
				giveFile: "shop/a.py", giveRoot: ".", want: "shop/a",
			},
			{
				name:     "returns the path of a file outside the root from the root of the workspace",
				giveFile: "lib/a.py", giveRoot: "src", want: "lib/a",
			},
			{
				name:     "returns the path of a file whose path continues the name of the root",
				giveFile: "srcx/a.py", giveRoot: "src", want: "srcx/a",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, project.Package(tt.giveFile, tt.giveRoot), tt.want, "the package of the module")
			})
		}
	})
}

// TestModuleAllocs checks the ceilings of the place and of the package in
// the ordinary run, which does not run benchmarks.
func TestModuleAllocs(t *testing.T) {
	t.Run("Place", func(t *testing.T) {
		t.Run("does not allocate for a file below a root of its project", func(t *testing.T) {
			p, err := project.Read(appFile, []byte(srcLayout))
			assert.NoError(t, err, "the project file decodes")
			none := func(string) bool { return false }
			root := ""
			assert.MaxAllocs(t, func() { root, _ = project.Place("app/src/shop/a.py", &p, none) }, 0,
				"Place does not allocate below a root")
			assert.Equal(t, root, "app/src", "the source root of the project")
		})

		t.Run("allocates the modules of the run of a file outside every root", func(t *testing.T) {
			unit := func(dir string) bool { return dir == testUnit }
			root := ""
			assert.MaxAllocs(t, func() { root, _ = project.Place("tests/unit/test_a.py", nil, unit) }, placeAllocs,
				"Place allocates the paths of the run")
			assert.Equal(t, root, "tests", "the root above the run")
		})
	})

	t.Run("Package", func(t *testing.T) {
		pkg := ""
		assert.MaxAllocs(t, func() { pkg = project.Package("src/shop/models.py", "src") }, 0,
			"Package does not allocate")
		assert.Equal(t, pkg, "shop/models", "the package of the module")
	})
}

// BenchmarkModule measures the place and the package that the partition
// and the parse take for each file.
func BenchmarkModule(b *testing.B) {
	b.Run("Place", func(b *testing.B) {
		unit := func(dir string) bool { return dir == testUnit }
		c := bench.Start(b).MaxAllocs(placeAllocs)
		defer c.End()
		root := ""
		for c.Loop() {
			root, _ = project.Place("tests/unit/test_a.py", nil, unit)
		}
		assert.Equal(b, root, "tests", "the root above the run")
	})

	b.Run("Package", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		pkg := ""
		for c.Loop() {
			pkg = project.Package("src/shop/models.py", "src")
		}
		assert.Equal(b, pkg, "shop/models", "the package of the module")
	})
}
