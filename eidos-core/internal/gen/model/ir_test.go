// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package model_test

import (
	"path/filepath"
	"strings"
	"testing"

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

		t.Run("answers kinds in declaration order", func(t *testing.T) {
			t.Parallel()

			kinds, err := model.Lower("testdata/valid", "")
			if err != nil {
				t.Fatalf("Lower: unexpected error: %v", err)
			}
			var got []string
			for _, kind := range kinds {
				got = append(got, kind.Name)
			}
			if want := "Thing,Part"; strings.Join(got, ",") != want {
				t.Fatalf("kinds = %v, want %s", got, want)
			}
		})

		t.Run("lowers a tag into the field spec", func(t *testing.T) {
			t.Parallel()

			kinds, err := model.Lower("testdata/valid", "")
			if err != nil {
				t.Fatalf("Lower: unexpected error: %v", err)
			}
			fields := map[string]model.FieldSpec{}
			for _, field := range kinds[0].Fields {
				fields[field.Name] = field
			}

			if got := fields["Name"]; got.Side != model.SideBoth || got.Walk {
				t.Fatalf("Name = %+v, want side both and no walk", got)
			}
			if got := fields["Pos"]; got.Side != model.SideNode || got.Side.OnEmit() {
				t.Fatalf("Pos = %+v, want the node side only", got)
			}
			parts := fields["Parts"]
			if !parts.Walk || parts.Slot != "parts" {
				t.Fatalf("Parts = %+v, want walk and slot=parts", parts)
			}
			if parts.Elem != "Part" || !parts.Slice || parts.Type != "[]*Part" {
				t.Fatalf("Parts = %+v, want a slice of Part spelled []*Part", parts)
			}
			if decls := fields["Decls"]; !decls.IsSymbol || !decls.Walk {
				t.Fatalf("Decls = %+v, want the marker and walk", decls)
			}
		})

		t.Run("skips a field carrying no tag", func(t *testing.T) {
			t.Parallel()

			kinds, err := model.Lower("testdata/valid", "")
			if err != nil {
				t.Fatalf("Lower: unexpected error: %v", err)
			}
			for _, field := range kinds[0].Fields {
				if field.Name == "Untagged" {
					t.Fatal("Untagged was lowered: a field with no eidos tag is not a model field")
				}
			}
		})

		t.Run("lowers the kernel's own schema", func(t *testing.T) {
			t.Parallel()

			root, err := gosource.ModuleRoot(".")
			if err != nil {
				t.Fatalf("ModuleRoot: %v", err)
			}
			kinds, err := model.Lower(filepath.Join(root, model.SchemaDir), root)
			if err != nil {
				t.Fatalf("Lower: unexpected error: %v", err)
			}
			if len(kinds) != wantKinds {
				t.Fatalf("lowered %d kinds, want %d", len(kinds), wantKinds)
			}

			byName := map[string]model.KindSpec{}
			for _, kind := range kinds {
				byName[kind.Name] = kind
			}
			for _, name := range []string{"Package", "Struct", "Method", "Import", "Export"} {
				if _, ok := byName[name]; !ok {
					t.Fatalf("%s is missing from the lowered schema", name)
				}
			}

			var methods model.FieldSpec
			for _, field := range byName["Struct"].Fields {
				if field.Name == "Methods" {
					methods = field
				}
			}
			if methods.Slot != "methods" || methods.Elem != "Method" {
				t.Fatalf("Struct.Methods = %+v, want slot=methods and elem Method", methods)
			}
			if len(byName["Struct"].Doc) == 0 {
				t.Fatal("Struct carries no documentation: the schema's docblock was dropped")
			}
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
				{"declaration that is not a struct", "testdata/nonstruct", "Alias"},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					_, err := model.Lower(tt.dir, "")
					if err == nil {
						t.Fatalf("Lower(%s): error = nil, want non-nil", tt.dir)
					}
					if !strings.Contains(err.Error(), tt.want) {
						t.Fatalf("error = %q, want it to name %q", err, tt.want)
					}
					if !strings.Contains(err.Error(), ".go:") {
						t.Fatalf("error = %q, want a schema position", err)
					}
					if !strings.HasPrefix(err.Error(), "model: ") {
						t.Fatalf("error = %q, want the package prefix", err)
					}
				})
			}
		})
	})
}
