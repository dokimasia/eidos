// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package python_test

import (
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/conformance/lang/python"
	"go.dokimi.dev/eidos/core/workspace/workspacetest"
	tstesting "go.dokimi.dev/eidos/lang/typescript/testing"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The extension of the files that the plan ts-client writes, and the file
// of the client.
const (
	tsExt      = ".ts"
	clientPath = "client/client.ts"
)

// The declarations of the client that the cases expect, and the alias with
// a type parameter, which the client leaves out.
const (
	recordInterface  = "export interface Order {\n"
	wireProperty     = "  placed: Date;\n"
	optionalProperty = "  note?: string | undefined;\n"
	dictProperty     = "  postcode?: string | undefined;\n"
	readonlyProperty = "  readonly price: Money;\n"
	protocolMethod   = "  discounted(percent: bigint): Money;\n"
	textEnum         = "  RED = \"red\",\n"
	plainAlias       = "export type Lines = Line[];\n"
	newTypeAlias     = "export type Sku = string;\n"
	genericAlias     = "Pair"
)

// TestClient checks that the plan ts-client writes the TypeScript types of
// the data classes of the shop.
func TestClient(t *testing.T) {
	t.Parallel()

	t.Run("ClientPlan", func(t *testing.T) {
		t.Parallel()

		root := shopRoot(t)
		cleanRun(t, root, python.ShopPlans()...)
		client := files.Read(t, filepath.Join(root, filepath.FromSlash(clientPath)))

		t.Run("writes a client that tsc type-checks", func(t *testing.T) {
			t.Parallel()

			adapter := tstesting.New()
			if !toolchain.Require(t, adapter) {
				return
			}
			g := workspacetest.Generated(t, root, python.Brand, nil, tsExt)
			toolchain.AssertTypeChecks(t.Context(), t, adapter, g)
		})

		tests := []struct {
			name string
			want string
		}{
			{name: "writes a dataclass as an interface", want: recordInterface},
			{name: "writes the key of a field as the name of its property", want: wireProperty},
			{name: "writes a field whose type is optional as an optional property", want: optionalProperty},
			{name: "writes a key of a TypedDict that a value may lack as an optional property", want: dictProperty},
			{name: "writes a property of a Protocol as a readonly property", want: readonlyProperty},
			{name: "writes a method of a Protocol as a method", want: protocolMethod},
			{name: "writes the value of a member of an enum of texts as its value", want: textEnum},
			{name: "writes a type alias as an alias", want: plainAlias},
			{name: "writes a NewType as an alias of the type that it wraps", want: newTypeAlias},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Contains(t, client, tt.want, "the client declares the type")
			})
		}

		t.Run("writes no alias with a type parameter", func(t *testing.T) {
			t.Parallel()

			assert.NotContains(t, client, genericAlias, "the kernel translates no type parameter inside a tuple")
		})
	})
}
