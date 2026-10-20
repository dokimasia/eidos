// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package python_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/conformance/lang/python"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/core/workspace/workspacetest"
)

// The goldens of the files that the plans of the shop write, the module that
// the edit changes, the field of Line before and after the edit, the plans
// of the fixture in their order, a root that a case does not create, and the
// mode of a module that does not write.
const (
	shopWant      = "testdata/shop/want"
	modelsPath    = "src/shop/models.py"
	intQuantity   = "quantity: int = 1"
	floatQuantity = "quantity: float = 1"
	modelName     = "go-model"
	clientName    = "ts-client"
	mirrorName    = "python-mirror"
	missingRoot   = "missing"
	readOnly      = 0o444
)

// TestShop checks that one workspace over a Python project writes a Go
// model, a TypeScript client and a mirror of the project, through the frame
// of the workspace and its warm path.
func TestShop(t *testing.T) {
	t.Parallel()

	t.Run("ComposeShop", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the workspace suite over the shop", func(t *testing.T) {
			t.Parallel()

			workspacetest.RunWorkspaceSuite(t, shopFixture(t))
		})

		t.Run("passes the warm suite over the shop with a field of Line made a float", func(t *testing.T) {
			t.Parallel()

			workspacetest.RunWarmColdSuite(t, shopFixture(t))
		})

		t.Run("writes the goldens of the shop fixture", func(t *testing.T) {
			t.Parallel()

			workspacetest.AssertGolden(t, shopFixture(t), t.TempDir(), shopWant)
		})
	})

	t.Run("ShopPlans", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the three plans of the fixture in their documented order", func(t *testing.T) {
			t.Parallel()

			plans := python.ShopPlans()
			names := make([]string, 0, len(plans))
			for _, p := range plans {
				names = append(names, p.Name)
			}
			assert.Equal(t, names, []string{mirrorName, modelName, clientName}, "the plans of the shop")
		})
	})

	t.Run("EditShop", func(t *testing.T) {
		t.Parallel()

		t.Run("makes the field quantity of Line a float", func(t *testing.T) {
			t.Parallel()

			src := files.Read(t, filepath.Join(filepath.FromSlash(shopTree), filepath.FromSlash(modelsPath)))
			root := files.Workspace(t, files.Tree{modelsPath: files.Text(src)})
			assert.NoError(t, python.EditShop(root), "the edit applies")
			files.Equal(t, os.DirFS(root), files.Tree{
				modelsPath: files.Text(strings.Replace(src, intQuantity, floatQuantity, 1)),
			}, "the edit changes the field and no other line")
		})

		t.Run("returns an error for a module without the field quantity", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, files.Tree{modelsPath: files.Text("class Line:\n    sku: str\n")})
			err := python.EditShop(root)
			assert.HasError(t, err, "a module without the field has nothing to change")
			assert.Contains(t, err.Error(), modelsPath, "the error contains the module")
		})

		t.Run("returns fs.ErrNotExist for a root without the module", func(t *testing.T) {
			t.Parallel()

			assert.ErrorIs(t, python.EditShop(t.TempDir()), fs.ErrNotExist, "an empty root has no module to read")
		})

		t.Run("returns fs.ErrNotExist for a root that does not exist", func(t *testing.T) {
			t.Parallel()

			assert.ErrorIs(t, python.EditShop(filepath.Join(t.TempDir(), missingRoot)), fs.ErrNotExist,
				"a missing root does not open")
		})

		t.Run("returns fs.ErrPermission for a module that does not write", func(t *testing.T) {
			t.Parallel()

			src := files.Read(t, filepath.Join(filepath.FromSlash(shopTree), filepath.FromSlash(modelsPath)))
			root := files.Workspace(t, files.Tree{modelsPath: files.Text(src).WithMode(readOnly)})
			assert.ErrorIs(t, python.EditShop(root), fs.ErrPermission, "a read-only module does not write")
		})
	})
}

// shopFixture returns the workspace suite's fixture of the shop composition:
// the tree, the composition, its three plans, the goldens of the files that
// they write, and the edit that makes a field of Line a float.
func shopFixture(t *testing.T) workspacetest.Fixture {
	t.Helper()

	return workspacetest.Fixture{
		Tree:    os.DirFS(filepath.FromSlash(shopTree)),
		Compose: func(root string) (*workspace.Builder, error) { return python.ComposeShop(root), nil },
		Plans:   python.ShopPlans,
		Want:    workspacetest.Want(t, shopWant),
		Edit:    python.EditShop,
	}
}
