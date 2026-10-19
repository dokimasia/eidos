// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package graphql_test

import (
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/conformance/lang/graphql"
	"go.dokimi.dev/eidos/core/workspace/workspacetest"
	gotesting "go.dokimi.dev/eidos/lang/go/testing"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The extension of the files that the plan go-model writes, the module that
// the package path of the model starts with, the file of the model, and the
// generator that reports the refusals of the model.
const (
	goExt                      = ".go"
	modelModule                = "example.com/acme"
	modelPath                  = "model/model.go"
	modelgenOrigin diag.Origin = "modelgen"
)

// The declarations of the model that the cases expect.
const (
	queryResolver  = "type QueryResolver interface {\n"
	stringEnumType = "type Availability string\n"
	stringValue    = "const AvailabilityInStock Availability = \"IN_STOCK\"\n"
	scalarType     = "type URL string\n"
	unionInterface = "type SearchResult interface {\n"
)

// TestModel checks that the plan go-model writes a Go model of the schema of
// the catalog.
func TestModel(t *testing.T) {
	t.Parallel()

	t.Run("ModelPlan", func(t *testing.T) {
		t.Parallel()

		// The carriers of the catalog name the typescript directive, which
		// the plan of the TypeScript backend registers, so every case runs
		// every plan of the catalog.
		root := catalogRoot(t)
		cleanRun(t, root, graphql.CatalogPlans()...)
		model := files.Read(t, filepath.Join(root, filepath.FromSlash(modelPath)))

		t.Run("writes a model that the Go toolchain type-checks", func(t *testing.T) {
			t.Parallel()

			adapter := gotesting.New()
			if !toolchain.Require(t, adapter) {
				return
			}
			g := workspacetest.Generated(t, root, graphql.Brand, nil, goExt)
			g.Module = modelModule
			toolchain.AssertTypeChecks(t.Context(), t, adapter, g)
		})

		tests := []struct {
			name string
			want string
		}{
			{name: "writes the resolver interface of a root operation type", want: queryResolver},
			{name: "writes an enum as a defined string type", want: stringEnumType},
			{name: "writes the name of a value of an enum as the value of its constant", want: stringValue},
			{name: "writes a custom scalar outside the table as a defined string type", want: scalarType},
			{name: "writes a union as an interface without methods", want: unionInterface},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Contains(t, model, tt.want, "the model declares the type")
			})
		}

		t.Run("reports RefusedType at each member whose type Go does not spell", func(t *testing.T) {
			t.Parallel()

			report := refusedRun(t, files.Workspace(t, files.Tree{
				undefinedSchema: files.Text(undefinedSource), undefinedDocument: files.Text(keyedSource),
			}))
			assert.Permutation(t, refusedAt(report, modelgenOrigin), []string{
				undefinedSchema + ":2", undefinedSchema + ":6", undefinedSchema + ":10", undefinedSchema + ":14",
				undefinedSchema + ":19", undefinedSchema + ":22", undefinedSchema + ":27", undefinedSchema + ":31",
			}, "the model refuses each translation of the undefined type, and withholds its declaration")
		})
	})
}
