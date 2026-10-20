// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package python_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/conformance/lang/python"
	pythonlang "go.dokimi.dev/eidos/lang/python"
	pythontesting "go.dokimi.dev/eidos/lang/python/testing"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The mark that opens a carrier of the fixture, the project file of the
// shop, its source root, and the module of the checks of the mirror and its
// source, which the toolchain adapter runs beside the mirror.
const (
	carrierMark = "#+" + string(python.Brand) + ":"
	projectFile = "pyproject.toml"
	sourceRoot  = "src/"
	checkModule = "test_shop.py"
	checkPath   = "testdata/shop/check/test_shop.py"
)

// The files of the shop whose mirror is not their source:
//
//   - compat.py has a check of the version, and the frontend loads the
//     branch that its options select. The model has no form for the check.
//     The module also has an asynchronous generator and the attributes that
//     __init__ assigns, which only the bodies state, and the model has no
//     body;
//   - legacy.py has a stub file beside it, so it declares nothing, because
//     a type checker reads the stub. It has no mirror;
//   - legacy.pyi and vendor.pyi are stub files. A composition renders the
//     Python target through one backend, so the mirror of a stub file is a
//     module, which separates its definitions with the blank lines of a
//     module;
//   - forms.py states the forms that the model writes in one spelling: type
//     comments, the import under TYPE_CHECKING, the check of the platform,
//     the functional forms of NamedTuple, TypedDict, an enum and namedtuple,
//     Optional, Union, string annotations and a relative import.
var unmirrored = []string{
	"src/shop/compat.py", legacyPath, "src/shop/legacy.pyi", "src/shop/vendor.pyi", "src/shop/forms.py",
}

// legacyPath is the source of the shop that a stub file beside it states
// the types of.
const legacyPath = "src/shop/legacy.py"

// TestMirror checks that the plan python-mirror writes each file of the shop
// as its source states it, that the frontend loads the mirror back with the
// stamps of its sources, and that the toolchain adapter validates what it
// writes.
func TestMirror(t *testing.T) {
	t.Parallel()

	t.Run("MirrorPlan", func(t *testing.T) {
		t.Parallel()

		t.Run("writes each file of the shop as its source states it", func(t *testing.T) {
			t.Parallel()

			root := shopRoot(t)
			mirrors := mirrorsOf(t, root)
			assert.NotEmpty(t, mirrors, "the shop has Python files")
			cleanRun(t, root, python.ShopPlans()...)
			for source, mirror := range mirrors {
				if slices.Contains(unmirrored, source) {
					continue
				}
				// A carrier is a directive of the run, which no declaration of
				// the model states, so the mirror does not restate it.
				var want strings.Builder
				for line := range strings.Lines(files.Read(t, filepath.Join(root, filepath.FromSlash(source)))) {
					if !strings.HasPrefix(strings.TrimSpace(line), carrierMark) {
						want.WriteString(line)
					}
				}
				written := files.Read(t, filepath.Join(root, filepath.FromSlash(mirror)))
				expect.Equal(t, bodyOf(t, written), want.String(), source+" is written as its source states it")
			}
		})

		t.Run("writes the mirror of each file again from the mirror", func(t *testing.T) {
			t.Parallel()

			root := shopRoot(t)
			mirrors := mirrorsOf(t, root)
			cleanRun(t, root, python.ShopPlans()...)
			// The second tree has each mirror without its frame at the path of
			// its source, because a load excludes the files that the workspace
			// stamped, and a module imports the modules of its package by
			// their paths.
			again := files.Tree{projectFile: files.Text(files.Read(t, filepath.Join(root, projectFile)))}
			firsts := map[string]string{}
			for source, mirror := range mirrors {
				first := bodyOf(t, files.Read(t, filepath.Join(root, filepath.FromSlash(mirror))))
				again[source] = files.Text(first)
				firsts[source] = first
			}
			second := files.Workspace(t, again)
			cleanRun(t, second, python.MirrorPlan())
			for source, first := range firsts {
				written := files.Read(t, filepath.Join(second, filepath.FromSlash(mirrors[source])))
				expect.Equal(t, bodyOf(t, written), first, source+" loads back with the stamps of its source")
			}
		})

		t.Run("writes a mirror that the toolchain adapter validates", func(t *testing.T) {
			t.Parallel()

			adapter := pythontesting.New(pythonlang.Version315)
			if !toolchain.Require(t, adapter) {
				return
			}
			root := shopRoot(t)
			mirrors := mirrorsOf(t, root)
			cleanRun(t, root, python.ShopPlans()...)
			// Each mirror is at the path of its source below the source root,
			// so the mirrors import one another as the sources do. The source
			// of a stub file declares nothing, so the project leaves it out.
			g := toolchain.Generated{Files: map[string][]byte{
				checkModule: []byte(files.Read(t, filepath.FromSlash(checkPath))),
			}}
			for source, mirror := range mirrors {
				if source != legacyPath {
					g.Files[strings.TrimPrefix(source, sourceRoot)] = []byte(
						files.Read(t, filepath.Join(root, filepath.FromSlash(mirror))))
				}
			}
			toolchain.AssertParses(t.Context(), t, adapter, g)
			toolchain.AssertTypeChecks(t.Context(), t, adapter, g)
			toolchain.AssertTestsPass(t.Context(), t, adapter, g)
		})
	})
}
