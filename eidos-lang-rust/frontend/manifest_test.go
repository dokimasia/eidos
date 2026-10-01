// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	rust "go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The [lib] sections the cases root a library outside src with: beside
// src, and at the package's own directory.
const (
	coreLib    = "[lib]\npath = \"core/lib.rs\"\n"
	packageLib = "[lib]\npath = \"lib.rs\"\n"
)

// Cargo's target layout and the manifest place every file of a package,
// so each target's root, name and members are pinned.
func TestManifest(t *testing.T) {
	t.Parallel()

	t.Run("crateName", func(t *testing.T) {
		t.Parallel()

		t.Run("replaces the package name's hyphens with underscores", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedSource(t, publicStruct)
			assert.Equal(t, packagesOf(gb), []string{crateName}, "Cargo's rule for a crate name")
		})
	})

	t.Run("targets", func(t *testing.T) {
		t.Parallel()

		t.Run("roots the library at the path its manifest states", func(t *testing.T) {
			t.Parallel()

			tree := manifestWith(coreLib, "core/lib.rs", "core/a.rs")
			assert.Equal(t, unitsOf(t, tree), [][]string{{"core/lib.rs", "core/a.rs"}},
				"the library's modules are below its root's directory")
		})

		t.Run("roots a declared binary at the path its manifest states", func(t *testing.T) {
			t.Parallel()

			tree := manifestWith("[[bin]]\nname = \"tool\"\npath = \"tools/main.rs\"\n", "tools/main.rs")
			tree["tools/main.rs"] = &fstest.MapFile{Data: []byte(publicStruct)}
			gb, _ := parsedTree(t, tree, "tools/main.rs", plugin.DepthFull, nil)
			assert.Equal(t, packagesOf(gb), []string{"tool"}, "the binary takes its declared name")
		})

		t.Run("roots a declared binary without a path under src/bin", func(t *testing.T) {
			t.Parallel()

			tree := manifestWith("[[bin]]\nname = \"tool\"\n", "src/bin/tool.rs")
			assert.Equal(t, unitsOf(t, tree), [][]string{{"src/bin/tool.rs"}}, "Cargo's default path")
		})

		t.Run("roots a file Cargo's layout places once when the manifest declares it too", func(t *testing.T) {
			t.Parallel()

			tree := manifestWith("[[bin]]\nname = \"tool\"\npath = \"src/bin/tool.rs\"\n", "src/bin/tool.rs")
			assert.Equal(t, unitsOf(t, tree), [][]string{{"src/bin/tool.rs"}}, "one target, one unit")
		})

		t.Run("roots each file under src/bin as a binary named after its stem", func(t *testing.T) {
			t.Parallel()

			tree := crateTree(map[string]string{"src/bin/a.rs": publicStruct})
			gb, _ := parsedTree(t, tree, "src/bin/a.rs", plugin.DepthFull, nil)
			assert.Equal(t, packagesOf(gb), []string{"a"}, "the binary's name")
		})

		t.Run("roots each directory's main.rs under src/bin as a binary named after the directory",
			func(t *testing.T) {
				t.Parallel()

				tree := crateTree(map[string]string{
					"src/bin/b/main.rs": "mod util;\n",
					"src/bin/b/util.rs": publicStruct,
				})
				gb, found := parsedTree(t, tree, "src/bin/b/main.rs", plugin.DepthFull, nil)
				assert.Empty(t, found, "the binary's module tree is linked")
				named[*node.Struct](t, fileIn(t, gb, "b/util").Decls, "A")
			})

		t.Run("roots each file under tests, examples and benches as a target", func(t *testing.T) {
			t.Parallel()

			tree := crateTree(map[string]string{"tests/t.rs": "", "examples/e.rs": "", "benches/b.rs": ""})
			assert.Equal(t, unitsOf(t, tree), [][]string{{"benches/b.rs"}, {"examples/e.rs"}, {"tests/t.rs"}},
				"each is a crate of its own")
		})

		t.Run("roots build.rs as the build script", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, crateTree(map[string]string{"build.rs": publicStruct}), "build.rs",
				plugin.DepthFull, nil)
			assert.Equal(t, packagesOf(gb), []string{"build_script_build"}, "Cargo's name for the build crate")
		})

		t.Run("roots no target for a file below a directory under src/bin", func(t *testing.T) {
			t.Parallel()

			tree := crateTree(map[string]string{"src/bin/c/d/e.rs": "", libRoot: ""})
			assert.Equal(t, unitsOf(t, tree), [][]string{{"src/bin/c/d/e.rs"}, {libRoot}},
				"the file is a crate of its own, and no module of the library")
		})
	})

	t.Run("owner", func(t *testing.T) {
		t.Parallel()

		t.Run("places a file under src in the src/main.rs binary of a package without a library",
			func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, unitsOf(t, crateTree(map[string]string{"src/main.rs": "", "src/a.rs": ""})),
					[][]string{{"src/main.rs", "src/a.rs"}}, "the binary's module")
			})

		t.Run("places a file under src in the library of a package with a binary too", func(t *testing.T) {
			t.Parallel()

			tree := crateTree(map[string]string{libRoot: "", "src/main.rs": "", "src/a.rs": ""})
			assert.Equal(t, unitsOf(t, tree), [][]string{{libRoot, "src/a.rs"}, {"src/main.rs"}},
				"the library's module")
		})

		t.Run("places a file under src in a crate of its own for a package without src/lib.rs or src/main.rs",
			func(t *testing.T) {
				t.Parallel()

				tree := crateTree(map[string]string{"src/a.rs": "", "src/bin/tool.rs": ""})
				assert.Equal(t, unitsOf(t, tree), [][]string{{"src/a.rs"}, {"src/bin/tool.rs"}},
					"a binary under src/bin has no module under src")
			})

		t.Run("places a file beside a library rooted at the package's directory in the library", func(t *testing.T) {
			t.Parallel()

			tree := manifestWith(packageLib, "lib.rs", "f/mod.rs")
			assert.Equal(t, unitsOf(t, tree), [][]string{{"lib.rs", "f/mod.rs"}}, "a module below the root's directory")
		})

		t.Run("places a file under src in a crate of its own when the library is rooted beside src",
			func(t *testing.T) {
				t.Parallel()

				tree := manifestWith(coreLib, "core/lib.rs", "src/a.rs")
				assert.Equal(t, unitsOf(t, tree), [][]string{{"core/lib.rs"}, {"src/a.rs"}},
					"no module of a library is outside its root's directory")
			})

		t.Run("places a file under src in the src/main.rs binary when the library is rooted above src",
			func(t *testing.T) {
				t.Parallel()

				tree := manifestWith(packageLib, "lib.rs", "src/main.rs", "src/a.rs")
				assert.Equal(t, unitsOf(t, tree), [][]string{{"lib.rs"}, {"src/main.rs", "src/a.rs"}},
					"the deeper directory's target")
			})

		t.Run("places a test file in a crate of its own when the library is rooted at the package's directory",
			func(t *testing.T) {
				t.Parallel()

				tree := manifestWith(packageLib, "lib.rs", "tests/t.rs")
				assert.Equal(t, unitsOf(t, tree), [][]string{{"lib.rs"}, {"tests/t.rs"}},
					"an integration test is a target of its own")
			})

		t.Run("places a file below a directory under src/bin in a crate of its own when the library is "+
			"rooted at the package's directory", func(t *testing.T) {
			t.Parallel()

			tree := manifestWith(packageLib, "lib.rs", "src/bin/c/d/e.rs")
			assert.Equal(t, unitsOf(t, tree), [][]string{{"lib.rs"}, {"src/bin/c/d/e.rs"}},
				"Cargo's layout reserves src/bin for binaries, so no file there is a module of the library")
		})

		t.Run("places a directory under tests that no target roots in a unit of its own, its mod.rs first",
			func(t *testing.T) {
				t.Parallel()

				tree := crateTree(map[string]string{"tests/common/mod.rs": "", "tests/common/a.rs": ""})
				assert.Equal(t, unitsOf(t, tree), [][]string{{"tests/common/mod.rs", "tests/common/a.rs"}},
					"the shared module")
			})

		t.Run("places a directory under tests without a mod.rs in a unit rooted at its first file",
			func(t *testing.T) {
				t.Parallel()

				tree := crateTree(map[string]string{"tests/common/a.rs": "", "tests/common/b.rs": ""})
				assert.Equal(t, unitsOf(t, tree), [][]string{{"tests/common/a.rs", "tests/common/b.rs"}},
					"the files in path order")
			})

		t.Run("places any other file of a package in a crate named below the package's", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, crateTree(map[string]string{"scripts/tool.rs": publicStruct}),
				"scripts/tool.rs", plugin.DepthFull, nil)
			assert.Equal(t, packagesOf(gb), []string{crateName + "/scripts/tool"}, "the file's path below the crate")
		})
	})

	t.Run("tests", func(t *testing.T) {
		t.Parallel()

		t.Run("reports false for an example", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, crateTree(map[string]string{"examples/e.rs": publicStruct}), "examples/e.rs",
				plugin.DepthFull, nil)
			assert.Empty(t, stampsOf(gb, string(rust.TestKey)), "an example's files are not test files")
		})

		t.Run("reports false for a shared module under examples", func(t *testing.T) {
			t.Parallel()

			gb, _ := parsedTree(t, crateTree(map[string]string{"examples/common/mod.rs": publicStruct}),
				"examples/common/mod.rs", plugin.DepthFull, nil)
			assert.Empty(t, stampsOf(gb, string(rust.TestKey)), "only a module under tests is a test module")
		})
	})
}

// manifestWith returns a tree of a manifest that names the fixture
// package and states more, and the given empty files.
func manifestWith(more string, files ...string) fstest.MapFS {
	tree := fstest.MapFS{manifestPath: {Data: []byte(manifestSrc + more)}}
	for _, f := range files {
		tree[f] = &fstest.MapFile{}
	}
	return tree
}
