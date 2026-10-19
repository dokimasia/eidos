// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package graphql_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/conformance/lang/graphql"
	"go.dokimi.dev/eidos/core/workspace/workspacetest"
)

// The schema that the edit changes, the field of Review before and after the
// edit, the plans of the fixture in their order, a root that a case does not
// create, and the mode of a schema that does not write.
const (
	reviewsPath  = "catalog/reviews/reviews.graphql"
	strictRating = "rating: Int!"
	looseRating  = "rating: Int"
	modelName    = "go-model"
	clientName   = "ts-client"
	mirrorName   = "graphql-mirror"
	missingRoot  = "missing"
	readOnly     = 0o444
)

// TestCatalog checks that one workspace over a GraphQL schema and its
// documents writes a Go model, a TypeScript client and a mirror of the
// schema, through the frame of the workspace and its warm path.
func TestCatalog(t *testing.T) {
	t.Parallel()

	t.Run("ComposeCatalog", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the workspace suite over the catalog", func(t *testing.T) {
			t.Parallel()

			workspacetest.RunWorkspaceSuite(t, catalogFixture(t))
		})

		t.Run("passes the warm suite over the catalog with a field of Review made nullable", func(t *testing.T) {
			t.Parallel()

			workspacetest.RunWarmColdSuite(t, catalogFixture(t))
		})

		t.Run("writes the goldens of the catalog fixture", func(t *testing.T) {
			t.Parallel()

			workspacetest.AssertGolden(t, catalogFixture(t), t.TempDir(), catalogWant)
		})
	})

	t.Run("CatalogPlans", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the three plans of the fixture in their documented order", func(t *testing.T) {
			t.Parallel()

			plans := graphql.CatalogPlans()
			names := make([]string, 0, len(plans))
			for _, p := range plans {
				names = append(names, p.Name)
			}
			assert.Equal(t, names, []string{modelName, clientName, mirrorName}, "the plans of the catalog")
		})
	})

	t.Run("EditCatalog", func(t *testing.T) {
		t.Parallel()

		t.Run("makes the field rating of Review nullable", func(t *testing.T) {
			t.Parallel()

			src := files.Read(t, filepath.Join(filepath.FromSlash(catalogTree), filepath.FromSlash(reviewsPath)))
			root := files.Workspace(t, files.Tree{reviewsPath: files.Text(src)})
			assert.NoError(t, graphql.EditCatalog(root), "the edit applies")
			files.Equal(t, os.DirFS(root), files.Tree{
				reviewsPath: files.Text(strings.Replace(src, strictRating, looseRating, 1)),
			}, "the edit changes the field and no other line")
		})

		t.Run("returns an error for a schema without the field rating", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, files.Tree{reviewsPath: files.Text("type Review {\n  id: ID!\n}\n")})
			err := graphql.EditCatalog(root)
			assert.HasError(t, err, "a schema without the field has nothing to change")
			assert.Contains(t, err.Error(), reviewsPath, "the error contains the schema")
		})

		t.Run("returns fs.ErrNotExist for a root without the schema", func(t *testing.T) {
			t.Parallel()

			assert.ErrorIs(t, graphql.EditCatalog(t.TempDir()), fs.ErrNotExist, "an empty root has no schema to read")
		})

		t.Run("returns fs.ErrNotExist for a root that does not exist", func(t *testing.T) {
			t.Parallel()

			assert.ErrorIs(t, graphql.EditCatalog(filepath.Join(t.TempDir(), missingRoot)), fs.ErrNotExist,
				"a missing root does not open")
		})

		t.Run("returns fs.ErrPermission for a schema that does not write", func(t *testing.T) {
			t.Parallel()

			src := files.Read(t, filepath.Join(filepath.FromSlash(catalogTree), filepath.FromSlash(reviewsPath)))
			root := files.Workspace(t, files.Tree{reviewsPath: files.Text(src).WithMode(readOnly)})
			assert.ErrorIs(t, graphql.EditCatalog(root), fs.ErrPermission, "a read-only schema does not write")
		})
	})
}
