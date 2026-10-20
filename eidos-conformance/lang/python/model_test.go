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
	gotesting "go.dokimi.dev/eidos/lang/go/testing"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The extension of the files that the plan go-model writes, the module that
// the package path of the model starts with, and the file of the model.
const (
	goExt       = ".go"
	modelModule = "example.com/acme"
	modelPath   = "model/model.go"
)

// The declarations of the model that the cases expect, and the field at the
// level of the type of Order, which the model leaves out.
const (
	dataclassStruct   = "type Money struct {\n"
	tupleStruct       = "type Point struct {\n"
	dictStruct        = "type Address struct {\n"
	protocolInterface = "type Priced interface {\n"
	nestedEnum        = "type LineKind string\n"
	valueConst        = "ColorRed Color = \"red\"\n"
	nameConst         = "PermissionRead Permission = \"READ\"\n"
	classField        = "Instances"
)

// TestModel checks that the plan go-model writes a Go model of the data
// classes of the shop.
func TestModel(t *testing.T) {
	t.Parallel()

	t.Run("ModelPlan", func(t *testing.T) {
		t.Parallel()

		root := shopRoot(t)
		cleanRun(t, root, python.ShopPlans()...)
		model := files.Read(t, filepath.Join(root, filepath.FromSlash(modelPath)))

		t.Run("writes a model that the Go toolchain type-checks", func(t *testing.T) {
			t.Parallel()

			adapter := gotesting.New()
			if !toolchain.Require(t, adapter) {
				return
			}
			g := workspacetest.Generated(t, root, python.Brand, nil, goExt)
			g.Module = modelModule
			toolchain.AssertTypeChecks(t.Context(), t, adapter, g)
		})

		tests := []struct {
			name string
			want string
		}{
			{name: "writes a dataclass as a struct", want: dataclassStruct},
			{name: "writes a NamedTuple as a struct", want: tupleStruct},
			{name: "writes a TypedDict as a struct", want: dictStruct},
			{name: "writes a Protocol as an interface", want: protocolInterface},
			{name: "writes a nested enum under its flat name", want: nestedEnum},
			{name: "writes the value of a member of an enum of texts as the value of its constant", want: valueConst},
			{name: "writes the name of a member of an enum of integers as the value of its constant", want: nameConst},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Contains(t, model, tt.want, "the model declares the type")
			})
		}

		t.Run("writes no field at the level of the type", func(t *testing.T) {
			t.Parallel()

			assert.NotContains(t, model, classField, "an instance of Order has no field instances")
		})
	})
}
