// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package graphql_test

import (
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/conformance/lang/graphql"
	"go.dokimi.dev/eidos/core/workspace/workspacetest"
	tstesting "go.dokimi.dev/eidos/lang/typescript/testing"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The extension of the files that the plan ts-client writes, the file of the
// client, the generator that reports the refusals of the client, and a
// document that selects a field that the schema does not declare under a
// type condition.
const (
	tsExt                       = ".ts"
	clientPath                  = "client/client.ts"
	clientgenOrigin diag.Origin = "clientgen"
	bogusPath                   = "catalog/documents/bogus.graphql"
	bogusSource                 = "query Bogus {\n  search(text: \"lamp\") {\n    ... on Product {\n      bogus\n    }\n  }\n}\n"
)

// The declarations of the client that the cases expect.
const (
	carriedName    = "export interface CatalogProduct {\n"
	carriedPolicy  = "  added?: string | undefined;\n"
	typenameKey    = "  __typename: \"Product\";\n"
	searchUnion    = "export type SearchQuerySearch = SearchQuerySearchProduct | SearchQuerySearchVendor;\n"
	browseVars     = "export interface BrowseVariables {\n"
	nullableVar    = "  withVendor?: boolean | undefined;\n"
	discriminantOf = "kind:"
)

// The files that a case adds to the catalog: a schema of a type whose name
// and field contain an abbreviation, and a query that selects the type
// through a union. The client declares the field and the interface of the
// type in the selection under these lines.
const (
	linksSchemaPath = "catalog/schema/links.graphql"
	linksSchema     = "type HttpLink {\n  imageUrl: URL\n}\n\n" +
		"union LinkResult = HttpLink | Vendor\n\n" +
		"extend type Query {\n  links: [LinkResult!]!\n}\n"
	linksDocumentPath = "catalog/documents/links.graphql"
	linksDocument     = "query Links {\n  links {\n    __typename\n    ... on HttpLink {\n      imageUrl\n    }\n  }\n}\n"
	imageURLProperty  = "  imageUrl?: URL | undefined;\n"
	linksUnion        = "export type LinksQueryLinks = LinksQueryLinksHttpLink | LinksQueryLinksVendor;\n"
)

// TestClient checks that the plan ts-client writes the TypeScript types of
// the schema of the catalog and of its operations.
func TestClient(t *testing.T) {
	t.Parallel()

	t.Run("ClientPlan", func(t *testing.T) {
		t.Parallel()

		root := catalogRoot(t)
		cleanRun(t, root, graphql.ClientPlan())
		client := files.Read(t, filepath.Join(root, filepath.FromSlash(clientPath)))

		t.Run("writes a client that tsc type-checks", func(t *testing.T) {
			t.Parallel()

			adapter := tstesting.New()
			if !toolchain.Require(t, adapter) {
				return
			}
			toolchain.AssertTypeChecks(t.Context(), t, adapter,
				workspacetest.Generated(t, root, graphql.Brand, nil, tsExt))
		})

		tests := []struct {
			name string
			want string
		}{
			{name: "writes the name that a carrier states on a type", want: carriedName},
			{name: "writes the type that a carrier chooses for the policy of a field", want: carriedPolicy},
			{name: "writes the response key __typename with the name of its type", want: typenameKey},
			{name: "writes a selection of particular types as a union of their interfaces", want: searchUnion},
			{name: "writes the variables of an operation", want: browseVars},
			{name: "writes a nullable variable as an optional property", want: nullableVar},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Contains(t, client, tt.want, "the client declares the type")
			})
		}

		t.Run("writes no discriminant that a response does not contain", func(t *testing.T) {
			t.Parallel()

			assert.NotContains(t, client, discriminantOf, "a response discriminates by __typename alone")
		})

		t.Run("writes the names with an abbreviation as the TypeScript respell writes them", func(t *testing.T) {
			t.Parallel()

			root := catalogRoot(t)
			files.Write(t, root, files.Tree{
				linksSchemaPath: files.Text(linksSchema), linksDocumentPath: files.Text(linksDocument),
			})
			cleanRun(t, root, graphql.ClientPlan())
			expect.That(t, files.Read(t, filepath.Join(root, filepath.FromSlash(clientPath)))).
				Contains(imageURLProperty, "the property has the key of the field without a computed name").
				Contains(linksUnion, "each interface of the selection writes the abbreviation of its type as one word")
		})

		t.Run("reports RefusedType at each member whose type TypeScript does not spell", func(t *testing.T) {
			t.Parallel()

			report := refusedRun(t, files.Workspace(t, files.Tree{
				undefinedSchema: files.Text(undefinedSource), undefinedDocument: files.Text(keyedSource),
			}))
			assert.Permutation(t, refusedAt(report, clientgenOrigin), []string{
				undefinedSchema + ":2", undefinedSchema + ":10", undefinedSchema + ":14", undefinedSchema + ":19",
				undefinedSchema + ":22", undefinedSchema + ":27", undefinedSchema + ":34",
				undefinedDocument + ":1", undefinedDocument + ":2",
			}, "the client refuses each translation of the undefined type, and withholds its declaration")
		})

		t.Run("reports RefusedType at an undeclared field of a type condition", func(t *testing.T) {
			t.Parallel()

			root := catalogRoot(t)
			files.Write(t, root, files.Tree{bogusPath: files.Text(bogusSource)})
			assert.Equal(t, refusedAt(refusedRun(t, root), clientgenOrigin), []string{bogusPath + ":4"},
				"the client refuses the field that the validator does not type")
		})
	})
}
