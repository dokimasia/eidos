// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package python_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/conformance/lang/python"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/core/workspace/workspacetest"
	tstesting "go.dokimi.dev/eidos/lang/typescript/testing"
	eidos "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The extension of the files that the plan ts-client writes, the file of the
// client, and the generator that reports the refusals of the client.
const (
	tsExt                       = ".ts"
	clientPath                  = "client/client.ts"
	clientgenOrigin diag.Origin = "clientgen"
)

// A module that a case adds to the shop: an alias whose type parameter has
// complex as its bound, which TypeScript has no spelling of, and the line of
// the alias.
const (
	numbersPath   = "src/shop/numbers.py"
	numbersSource = "from typing import TypeAlias, TypeVar\n\nN = TypeVar(\"N\", bound=complex)\n\n" +
		"Pairs: TypeAlias = tuple[N, N]\n"
	numbersAlias = 5
)

// The declarations of the client that the cases expect.
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
	genericAlias     = "export type Pair<T_co extends Line = Line> = readonly [T_co, T_co];\n"
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
			{name: "writes an alias with a type parameter as a generic alias", want: genericAlias},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Contains(t, client, tt.want, "the client declares the type")
			})
		}

		t.Run("reports RefusedType at an alias whose type parameter TypeScript does not spell", func(t *testing.T) {
			t.Parallel()

			root := shopRoot(t)
			files.Write(t, root, files.Tree{numbersPath: files.Text(numbersSource)})
			w, err := python.ComposeShop(root).Plans(python.ShopPlans()...).Build()
			assert.NoError(t, err, "the composition builds")
			report, err := w.Run(t.Context(), workspace.Input{Tree: os.DirFS(root)})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the refusal fails the run")
			var refused []string
			for d := range report.Sink.All() {
				if d.Code == eidos.RefusedType && d.Origin == clientgenOrigin {
					refused = append(refused, fmt.Sprintf("%s:%d", d.Pos.File, d.Pos.Line))
				}
			}
			assert.Equal(t, refused, []string{fmt.Sprintf("%s:%d", numbersPath, numbersAlias)},
				"the client refuses the bound of the type parameter once, at the alias that it withholds")
		})
	})
}
