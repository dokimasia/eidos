// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package model_test

import (
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/gen/model"
	"go.dokimi.dev/eidos/core/internal/gosource"
)

// wantKinds is the number of declaration kinds the schema holds.
// It is asserted rather than derived, so adding a kind is a
// deliberate edit here as well as in the schema.
const wantKinds = 23

func TestIR(t *testing.T) {
	t.Parallel()

	t.Run("Lower", func(t *testing.T) {
		t.Parallel()

		t.Run("returns kinds in declaration order", func(t *testing.T) {
			t.Parallel()

			kinds, err := model.Lower("testdata/valid", "")
			assert.NoError(t, err, "the fixture schema lowers")
			var got []string
			for _, kind := range kinds {
				got = append(got, kind.Name)
			}
			assert.Equal(t, got, []string{"Thing", "Part"},
				"kinds come back in declaration order")
		})

		t.Run("lowers a tag into the field spec", func(t *testing.T) {
			t.Parallel()

			kinds, err := model.Lower("testdata/valid", "")
			assert.NoError(t, err, "the fixture schema lowers")
			fields := map[string]model.FieldSpec{}
			for _, field := range kinds[0].Fields {
				fields[field.Name] = field
			}

			assert.Equal(t, fields["Name"].Side, model.SideBoth,
				"an untagged side arrives on both models")
			assert.False(t, fields["Name"].Walk, "and is not walked untagged")
			assert.Equal(t, fields["Pos"].Side, model.SideNode,
				"a node-tagged field arrives on the node side")
			assert.False(t, fields["Pos"].Side.OnEmit(), "and not on emit")
			parts := fields["Parts"]
			assert.True(t, parts.Walk, "a walk tag marks the field traversed")
			assert.Equal(t, parts.Slot, "parts", "a slot tag names its accessor")
			assert.Equal(t, parts.Elem, "Part", "the element kind is read from the type")
			assert.True(t, parts.Slice, "so is the slice shape")
			assert.Equal(t, parts.Type, "[]*Part", "and the declared spelling")
			assert.True(t, fields["Decls"].IsSymbol, "the marker type is recognized")
			assert.True(t, fields["Decls"].Walk, "and walked")
			assert.Equal(t, fields["Async"].Fact, "Async",
				"a fact tag names the constant suffix")
			assert.Equal(t, fields["Name"].Fact, "",
				"a field without one states no fact")
		})

		t.Run("skips a field carrying no tag", func(t *testing.T) {
			t.Parallel()

			kinds, err := model.Lower("testdata/valid", "")
			assert.NoError(t, err, "the fixture schema lowers")
			for _, field := range kinds[0].Fields {
				assert.NotEqual(t, field.Name, "Untagged",
					"a field with no eidos tag is not a model field")
			}
		})

		t.Run("lowers the kernel's own schema", func(t *testing.T) {
			t.Parallel()

			root, err := gosource.ModuleRoot(".")
			assert.NoError(t, err, "the module root resolves")
			kinds, err := model.Lower(filepath.Join(root, model.SchemaDir), root)
			assert.NoError(t, err, "the kernel's own schema lowers")
			assert.Length(t, kinds, wantKinds,
				"adding a kind is a deliberate edit here as well as in the schema")

			byName := map[string]model.KindSpec{}
			for _, kind := range kinds {
				byName[kind.Name] = kind
			}
			for _, name := range []string{"Package", "Struct", "Method", "Import", "Export"} {
				_, ok := byName[name]
				assert.True(t, ok, "every family representative lowers")
			}

			var methods model.FieldSpec
			for _, field := range byName["Struct"].Fields {
				if field.Name == "Methods" {
					methods = field
				}
			}
			assert.Equal(t, methods.Slot, "methods", "Struct.Methods carries its slot")
			assert.Equal(t, methods.Elem, "Method", "and its element kind")
			assert.NotEmpty(t, byName["Struct"].Doc,
				"the schema's docblock is carried with the kind")
		})

		t.Run("reports a schema directory it cannot read", func(t *testing.T) {
			t.Parallel()

			_, err := model.Lower("testdata/nonexistent", "")
			assert.HasError(t, err, "a schema directory it cannot read is reported")
		})

		t.Run("refuses a schema that breaks the contract", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				dir  string
				want string
			}{
				{"unknown tag token", "testdata/badtoken", "walkk"},
				{"duplicate slot name", "testdata/dupslot", "parts"},
				{"slot on a field that is not a slice", "testdata/slotnotslice", model.SlotPrefix},
				{"walk on a field that is not a kind", "testdata/walkbadtype", model.WalkToken},
				{"fact on a node-only field", "testdata/factnode", "emit-visible"},
				{"fact that is not an exported identifier", "testdata/factbadname", "suffix"},
				{"declaration that is not a struct", "testdata/nonstruct", "Alias"},
				{"declaration that is not a type", "testdata/notatype", "types and imports"},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					_, err := model.Lower(tt.dir, "")
					assert.HasError(t, err, "a schema breaking the contract is refused")
					assert.Contains(t, err.Error(), tt.want, "naming what broke it")
					assert.Contains(t, err.Error(), ".go:", "at a schema position")
					assert.HasPrefix(t, err.Error(), "model: ", "under the package prefix")
				})
			}
		})
	})
}
