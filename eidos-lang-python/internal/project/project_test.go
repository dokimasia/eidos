// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package project_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/python/internal/project"
)

// The paths of the project files of the cases: one in a directory of the
// workspace and one at its root.
const (
	appFile  = "app/pyproject.toml"
	rootFile = "pyproject.toml"
)

// srcLayout is a project file of uv_build with the default module root,
// which the allocation test and the benchmark read.
const srcLayout = "[build-system]\nrequires = [\"uv_build\"]\nbuild-backend = \"uv_build\"\n\n" +
	"[project]\nname = \"shop\"\nversion = \"1.0.0\"\n"

// readAllocs is the read of srcLayout: the box of the document, sixteen
// allocations of the decoder for its tables, its keys, its strings and its
// array, and the joined path of src.
const readAllocs = 1 + 16 + 1

// TestProject pins the source roots that each build backend states and
// the root that each file takes.
func TestProject(t *testing.T) {
	t.Parallel()

	t.Run("Read", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			givePath  string
			give      string
			wantDir   string
			wantRoots []string
		}{
			{
				name: "returns the directory of the project file", givePath: appFile,
				give: "[project]\nname = \"app\"\n", wantDir: "app",
			},
			{
				name: "returns the root of the workspace for a project file at the root", givePath: rootFile,
				give: "[project]\nname = \"app\"\n", wantDir: ".",
			},
			{
				name: "returns the root of the package-dir of setuptools", givePath: appFile,
				give: "[build-system]\nbuild-backend = \"setuptools.build_meta\"\n" +
					"[tool.setuptools]\npackage-dir = {\"\" = \"src\"}\n",
				wantDir: "app", wantRoots: []string{"app/src"},
			},
			{
				name: "returns each where of the packages.find of setuptools", givePath: appFile,
				give: "[build-system]\nbuild-backend = \"setuptools.build_meta\"\n" +
					"[tool.setuptools.packages.find]\nwhere = [\"lib\", \"src\"]\n",
				wantDir: "app", wantRoots: []string{"app/lib", "app/src"},
			},
			{
				name: "returns the roots of setuptools for a file without a build-system", givePath: appFile,
				give:    "[tool.setuptools]\npackage-dir = {\"\" = \"lib\"}\n",
				wantDir: "app", wantRoots: []string{"app/lib"},
			},
			{
				name: "returns the roots of the legacy backend of setuptools", givePath: appFile,
				give: "[build-system]\nbuild-backend = \"setuptools.build_meta:__legacy__\"\n" +
					"[tool.setuptools]\npackage-dir = {\"\" = \"lib\"}\n",
				wantDir: "app", wantRoots: []string{"app/lib"},
			},
			{
				name: "returns each source of hatchling", givePath: appFile,
				give: "[build-system]\nbuild-backend = \"hatchling.build\"\n" +
					"[tool.hatch.build.targets.wheel]\nsources = [\"src\"]\n",
				wantDir: "app", wantRoots: []string{"app/src"},
			},
			{
				name: "returns each prefix that a table of sources of hatchling removes", givePath: appFile,
				give: "[build-system]\nbuild-backend = \"hatchling.build\"\n" +
					"[tool.hatch.build.targets.wheel.sources]\n\"src\" = \"\"\n\"lib/x\" = \"y\"\n",
				wantDir: "app", wantRoots: []string{"app/src"},
			},
			{
				name: "returns the parent of each package of hatchling", givePath: appFile,
				give: "[build-system]\nbuild-backend = \"hatchling.build\"\n" +
					"[tool.hatch.build.targets.wheel]\npackages = [\"src/shop\", \"src/tools\"]\n",
				wantDir: "app", wantRoots: []string{"app/src"},
			},
			{
				name: "returns the from of each package of poetry", givePath: appFile,
				give: "[build-system]\nbuild-backend = \"poetry.core.masonry.api\"\n" +
					"[tool.poetry]\npackages = [{include = \"shop\", from = \"src\"}]\n",
				wantDir: "app", wantRoots: []string{"app/src"},
			},
			{
				name: "returns the directory of the project for a package of poetry without a from",
				give: "[build-system]\nbuild-backend = \"poetry.core.masonry.api\"\n" +
					"[tool.poetry]\npackages = [{include = \"shop\"}, \"stray\"]\n",
				givePath: appFile, wantDir: "app", wantRoots: []string{"app"},
			},
			{
				name: "returns the module-root of uv_build", givePath: appFile,
				give: "[build-system]\nbuild-backend = \"uv_build\"\n" +
					"[tool.uv.build-backend]\nmodule-root = \"python\"\n",
				wantDir: "app", wantRoots: []string{"app/python"},
			},
			{
				name: "returns the package-dir of pdm-backend", givePath: appFile,
				give: "[build-system]\nbuild-backend = \"pdm.backend\"\n" +
					"[tool.pdm.build]\npackage-dir = \"lib\"\n",
				wantDir: "app", wantRoots: []string{"app/lib"},
			},
			{
				name: "returns the python-source of maturin", givePath: appFile,
				give: "[build-system]\nbuild-backend = \"maturin\"\n" +
					"[tool.maturin]\npython-source = \"python\"\n",
				wantDir: "app", wantRoots: []string{"app/python"},
			},
			{
				name: "returns no root for a backend without a table of roots", givePath: appFile,
				give: "[build-system]\nbuild-backend = \"flit_core.buildapi\"\n" +
					"[tool.setuptools]\npackage-dir = {\"\" = \"lib\"}\n",
				wantDir: "app",
			},
			{
				name: "returns no root for a key of another type", givePath: appFile,
				give: "[build-system]\nbuild-backend = \"uv_build\"\n" +
					"[tool.uv.build-backend]\nmodule-root = 1\n",
				wantDir: "app",
			},
			{
				name: "leaves out a root that leaves the workspace", givePath: appFile,
				give:    "[tool.setuptools.packages.find]\nwhere = [\"../..\", \"../../up\", \"src\"]\n",
				wantDir: "app", wantRoots: []string{"app/src"},
			},
			{
				name:     "returns each root once",
				givePath: appFile,
				give: "[tool.setuptools]\npackage-dir = {\"\" = \"src\"}\n" +
					"[tool.setuptools.packages.find]\nwhere = [\"src\"]\n",
				wantDir:   "app",
				wantRoots: []string{"app/src"},
			},
			{
				name: "returns the longest root first", givePath: appFile,
				give:    "[tool.setuptools.packages.find]\nwhere = [\"src\", \"lib/python\", \"lib\"]\n",
				wantDir: "app", wantRoots: []string{"app/lib/python", "app/lib", "app/src"},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := project.Read(tt.givePath, []byte(tt.give))
				assert.NoError(t, err, "the project file decodes")
				assert.Equal(t, got.Dir, tt.wantDir, "the directory of the project file")
				assert.Equal(t, got.Roots, tt.wantRoots, "the source roots of the project")
			})
		}

		names := []struct {
			name string
			give string
			want string
		}{
			{name: "returns the name of the table project", give: "[project]\nname = \"shop\"\n", want: "shop"},
			{
				name: "returns the name of the table of poetry for a file without the table project",
				give: "[tool.poetry]\nname = \"legacy\"\n",
				want: "legacy",
			},
			{
				name: "returns the name of the table project before the name of poetry",
				give: "[project]\nname = \"shop\"\n[tool.poetry]\nname = \"legacy\"\n",
				want: "shop",
			},
			{name: "returns the empty name for a file without a name", give: "[tool.setuptools]\nzip-safe = false\n"},
		}
		for _, tt := range names {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := project.Read(appFile, []byte(tt.give))
				assert.NoError(t, err, "the project file decodes")
				assert.Equal(t, got.Name, tt.want, "the name of the distribution")
			})
		}

		t.Run("returns an error for bytes that are not TOML", func(t *testing.T) {
			t.Parallel()

			_, err := project.Read(appFile, []byte("[project\n"))
			assert.HasError(t, err, "the decoder refuses the document")
			assert.Contains(t, err.Error(), appFile, "the error names the project file")
		})
	})

	t.Run("Root", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			give      string
			giveFile  string
			wantRoot  string
			wantBelow bool
		}{
			{
				name:     "returns the longest root that contains the file",
				give:     "[tool.setuptools.packages.find]\nwhere = [\"lib\", \"lib/python\"]\n",
				giveFile: "app/lib/python/shop/a.py", wantRoot: "app/lib/python", wantBelow: true,
			},
			{
				name:      "returns src for a file below src of a project without roots",
				give:      "[project]\nname = \"app\"\n",
				giveFile:  "app/src/shop/a.py",
				wantRoot:  "app/src",
				wantBelow: true,
			},
			{
				name: "reports false for a file outside src of a project without roots",
				give: "[project]\nname = \"app\"\n", giveFile: "app/tests/test_a.py",
			},
			{
				name:     "reports false for a file outside every root",
				give:     "[tool.setuptools]\npackage-dir = {\"\" = \"lib\"}\n",
				giveFile: "app/src/shop/a.py",
			},
			{
				name:     "reports false for a file whose path continues the name of a root",
				give:     "[tool.setuptools]\npackage-dir = {\"\" = \"lib\"}\n",
				giveFile: "app/library/a.py",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				p, err := project.Read(appFile, []byte(tt.give))
				assert.NoError(t, err, "the project file decodes")
				root, below := p.Root(tt.giveFile)
				assert.Equal(t, below, tt.wantBelow, "Root reports whether a root contains the file")
				assert.Equal(t, root, tt.wantRoot, "the source root of the file")
			})
		}

		t.Run("returns the root of the workspace for a root at the root of the workspace", func(t *testing.T) {
			t.Parallel()

			p, err := project.Read(rootFile, []byte("[build-system]\nbuild-backend = \"uv_build\"\n"+
				"[tool.uv.build-backend]\nmodule-root = \"\"\n"))
			assert.NoError(t, err, "the project file decodes")
			root, below := p.Root("shop/a.py")
			assert.True(t, below, "the root of the workspace contains every file")
			assert.Equal(t, root, ".", "the source root of the file")
		})

		t.Run("reports false for the zero Project", func(t *testing.T) {
			t.Parallel()

			_, below := project.Project{}.Root("src/a.py")
			assert.False(t, below, "the zero Project places no file")
		})
	})
}

// TestProjectAllocs checks the ceilings of the read and of the root in the
// ordinary run, which does not run benchmarks.
func TestProjectAllocs(t *testing.T) {
	t.Run("Read", func(t *testing.T) {
		src := []byte(srcLayout)
		var got project.Project
		var err error
		assert.MaxAllocs(t, func() { got, err = project.Read(appFile, src) }, readAllocs,
			"Read allocates the document and the joined path of src")
		assert.NoError(t, err, "the project file decodes")
		assert.Empty(t, got.Roots, "uv_build states no root without a module-root")
	})

	t.Run("Root", func(t *testing.T) {
		p, err := project.Read(appFile, []byte(srcLayout))
		assert.NoError(t, err, "the project file decodes")
		below := false
		assert.MaxAllocs(t, func() { _, below = p.Root("app/src/shop/models.py") }, 0, "Root does not allocate")
		assert.True(t, below, "src contains the file")
	})
}

// BenchmarkProject measures the read that the partition makes once for
// each project file and the parse once for each content of it, and the root
// that the partition takes for each file.
func BenchmarkProject(b *testing.B) {
	b.Run("Read", func(b *testing.B) {
		src := []byte(srcLayout)
		c := bench.Start(b).MaxAllocs(readAllocs)
		defer c.End()
		var err error
		for c.Loop() {
			_, err = project.Read(appFile, src)
		}
		assert.NoError(b, err, "the project file decodes")
	})

	b.Run("Root", func(b *testing.B) {
		p, err := project.Read(appFile, []byte(srcLayout))
		assert.NoError(b, err, "the project file decodes")
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		below := false
		for c.Loop() {
			_, below = p.Root("app/src/shop/models.py")
		}
		assert.True(b, below, "src contains the file")
	})
}
