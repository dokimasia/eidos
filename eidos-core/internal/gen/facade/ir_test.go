// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package facade_test

import (
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/gen/facade"
)

// Lowering owns what the renderer may assume: the curated
// packages parse in order, and what would defeat syntax-level
// re-export is refused before anything renders.
func TestIR(t *testing.T) {
	t.Parallel()

	t.Run("Lower", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers the kernel in curated order", func(t *testing.T) {
			t.Parallel()

			surfaces, err := facade.Lower(filepath.Join(repoRoot(t), facade.KernelDir))
			assert.NoError(t, err, "the kernel lowers")
			assert.Length(t, surfaces, len(facade.Surfaces), "one surface per curated entry")
			for i, ps := range surfaces {
				assert.Equal(t, ps.Rel, facade.Surfaces[i].Rel, "in curated order")
				assert.True(t, len(ps.Files) > 0, "each surface parsed its files")
			}
			assert.Equal(t, surfaces[0].KernelName, "eidos",
				"the root surface is the kernel's authoring package")
		})

		t.Run("refuses a dot import", func(t *testing.T) {
			t.Parallel()

			root := mini(t)
			poison(t, root, poisonRel, "package emit\n\nimport . \"strings\"\n")
			_, err := facade.Lower(filepath.Join(root, facade.KernelDir))
			assert.HasError(t, err, "a dot import erases the qualifier")
			assert.Contains(t, err.Error(), "dot import", "the refusal says why")
			assert.Contains(t, err.Error(), "poison.go:", "at the kernel position")
		})

		t.Run("refuses a package split", func(t *testing.T) {
			t.Parallel()

			root := mini(t)
			poison(t, root, poisonRel, "package emitx\n")
			_, err := facade.Lower(filepath.Join(root, facade.KernelDir))
			assert.HasError(t, err, "two package names in one directory")
			assert.Contains(t, err.Error(), "declared beside", "the refusal names the split")
			assert.Contains(t, err.Error(), "emitx", "and the name that arrived beside the package")
		})

		t.Run("reports a curated package it cannot parse", func(t *testing.T) {
			t.Parallel()

			root := mini(t)
			poison(t, root, poisonRel, "package emit\n\nfunc Broken(\n")
			_, err := facade.Lower(filepath.Join(root, facade.KernelDir))
			assert.HasError(t, err, "a curated package that does not parse is reported")
			assert.Contains(t, err.Error(), "go.dokimi.dev/eidos/core/emit",
				"naming the curated package it was reading")
			assert.Contains(t, err.Error(), "poison.go", "and the file inside it")
		})

		t.Run("refuses a root holding another module", func(t *testing.T) {
			t.Parallel()

			_, err := facade.Lower(filepath.Join(repoRoot(t), "eidos-lang"))
			assert.HasError(t, err, "a non-kernel module is not generated from")
			assert.Contains(t, err.Error(), "not the kernel", "the refusal names the mismatch")
		})

		t.Run("reports a tree holding no module", func(t *testing.T) {
			t.Parallel()

			_, err := facade.Lower(t.TempDir())
			assert.HasError(t, err, "a bare tree is reported")
		})
	})

	t.Run("FacadeImportPath", func(t *testing.T) {
		t.Parallel()

		t.Run("names the facade module for the root surface", func(t *testing.T) {
			t.Parallel()

			got := facade.Surface{Rel: "", Name: "sdk"}.FacadeImportPath()
			assert.Equal(t, got, facade.FacadeModule,
				"the root surface is the facade module itself, not a package under it")
		})

		t.Run("lays a nested kernel package flat under the facade", func(t *testing.T) {
			t.Parallel()

			got := facade.Surface{Rel: "backend/render", Name: "render"}.FacadeImportPath()
			assert.Equal(t, got, facade.FacadeModule+"/render",
				"the facade path is the package name alone, so a kernel regroup "+
					"moves no facade import path")
		})
	})
}
