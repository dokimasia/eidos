// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package model_test

import (
	"cmp"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/internal/gen/model"
	"go.dokimi.dev/eidos/core/internal/gosource"
)

// wantKinds is the number of declaration kinds in the schema. The
// number is a literal, so adding a kind takes an edit here as well
// as in the schema.
const wantKinds = 22

// validSchema is the fixture schema every contract case departs
// from.
const validSchema = "testdata/valid"

// lowerValidAllocs is one lowering of the fixture schema: its parse,
// its type-check and the schema of its two kinds, 290 in each of 24
// fresh processes but one, which counted 291.
const lowerValidAllocs = 290 + 1

// Lowering is where the annotation contract is enforced, so what it
// returns for a valid schema and what it refuses are contract.
func TestIR(t *testing.T) {
	t.Parallel()

	t.Run("Lower", func(t *testing.T) {
		t.Parallel()

		t.Run("returns kinds in declaration order", func(t *testing.T) {
			t.Parallel()

			schema, err := model.Lower(validSchema, "")
			assert.NoError(t, err, "the fixture schema lowers")
			got := make([]string, 0, len(schema.Kinds))
			for _, kind := range schema.Kinds {
				got = append(got, kind.Name)
			}
			assert.Equal(t, got, []string{"Thing", "Part"},
				"kinds come back in declaration order")
		})

		t.Run("returns an untagged side on both models", func(t *testing.T) {
			t.Parallel()

			name := fieldsOf(t)["Name"]
			assert.Equal(t, name.Side, model.SideBoth, "an untagged side arrives on both models")
			assert.False(t, name.Walk, "and is not walked untagged")
		})

		t.Run("returns a node-tagged field on the node model alone", func(t *testing.T) {
			t.Parallel()

			pos := fieldsOf(t)["Pos"]
			assert.Equal(t, pos.Side, model.SideNode, "a node-tagged field arrives on the node side")
			assert.False(t, pos.Side.OnEmit(), "and not on emit")
		})

		t.Run("reads every property of a walked slot", func(t *testing.T) {
			t.Parallel()

			parts := fieldsOf(t)["Parts"]
			assert.True(t, parts.Walk, "a walk tag marks the field traversed")
			assert.Equal(t, parts.Slot, "parts", "a slot tag names its accessor")
			assert.Equal(t, parts.Elem, "Part", "the element kind is read from the type")
			assert.True(t, parts.Slice, "so is the slice shape")
			assert.Equal(t, parts.Type, "[]*Part", "and the declared spelling")
		})

		t.Run("returns a marker-typed field as walked", func(t *testing.T) {
			t.Parallel()

			decls := fieldsOf(t)["Decls"]
			assert.True(t, decls.IsSymbol, "the marker type is recognized")
			assert.True(t, decls.Walk, "and walked")
		})

		t.Run("returns a fact tag as its constant suffix", func(t *testing.T) {
			t.Parallel()

			fields := fieldsOf(t)
			assert.Equal(t, fields["Async"].Fact, "Async", "a fact tag names the constant suffix")
			assert.Equal(t, fields["Name"].Fact, "", "a field without one states no fact")
		})

		t.Run("returns no field for a field without a tag", func(t *testing.T) {
			t.Parallel()

			assert.NotContains(t, fieldsOf(t), "Untagged", "a field with no eidos tag is not a model field")
		})

		t.Run("returns the kernel's own kinds", func(t *testing.T) {
			t.Parallel()

			root, err := gosource.ModuleRoot(".")
			assert.NoError(t, err, "the module root resolves")
			schema, err := model.Lower(filepath.Join(root, model.SchemaDir), root)
			assert.NoError(t, err, "the kernel's own schema lowers")
			assert.Length(t, schema.Kinds, wantKinds,
				"adding a kind is a deliberate edit here as well as in the schema")

			byName := map[string]model.KindSpec{}
			for _, kind := range schema.Kinds {
				byName[kind.Name] = kind
			}
			for _, name := range []string{"Package", "Struct", "Method", "Import", "Export"} {
				assert.Contains(t, byName, name, "every family representative lowers")
			}

			var methods model.FieldSpec
			for _, field := range byName["Struct"].Fields {
				if field.Name == "Methods" {
					methods = field
				}
			}
			assert.Equal(t, methods.Slot, "methods", "Struct.Methods names its slot")
			assert.Equal(t, methods.Elem, "Method", "and its element kind")
			assert.NotEmpty(t, byName["Struct"].Doc,
				"the schema's docblock arrives with the kind")
		})

		t.Run("returns the kernel's hand-written enums", func(t *testing.T) {
			t.Parallel()

			root, err := gosource.ModuleRoot(".")
			assert.NoError(t, err, "the module root resolves")
			schema, err := model.Lower(filepath.Join(root, model.SchemaDir), root)
			assert.NoError(t, err, "the kernel's own schema lowers")
			types := make([]string, 0, len(schema.Enums))
			for _, e := range schema.Enums {
				types = append(types, e.Type)
			}
			assert.Equal(t, types, []string{
				"Accessor", "Level", "Mutability", "TypeForm", "Variadic", "Variance", "Visibility",
			}, "every hand-written enum of the symbol package, sorted by type name")
		})

		t.Run("returns the enums of the symbol package the schema imports", func(t *testing.T) {
			t.Parallel()

			root := symbolModule{schema: enumSchema, symbol: toneSource}.write(t)
			schema, err := model.Lower(filepath.Join(root, model.SchemaDir), root)
			assert.NoError(t, err, "the schema lowers")
			assert.Equal(t, schema.Enums, []model.EnumSpec{{
				Type: "Tone",
				Values: []model.EnumValue{
					{Name: "ToneHigh", Value: "1"},
					{Name: "ToneLow", Value: "0"},
				},
			}}, "each constant's name and computed value, sorted by name")
		})

		t.Run("returns no enum for a string-typed constant", func(t *testing.T) {
			t.Parallel()

			root := symbolModule{schema: enumSchema, symbol: wideSource}.write(t)
			schema, err := model.Lower(filepath.Join(root, model.SchemaDir), root)
			assert.NoError(t, err, "the schema lowers")
			assert.Length(t, schema.Enums, 1, "the Tone enum alone: a string is no enum")
		})

		t.Run("returns no enum for a constant of another package's type", func(t *testing.T) {
			t.Parallel()

			root := symbolModule{schema: enumSchema, symbol: foreignSource}.write(t)
			schema, err := model.Lower(filepath.Join(root, model.SchemaDir), root)
			assert.NoError(t, err, "the schema lowers")
			assert.Length(t, schema.Enums, 1, "the Tone enum alone: time declares Duration")
		})

		t.Run("returns no enum a generated file declares", func(t *testing.T) {
			t.Parallel()

			root := symbolModule{schema: enumSchema, symbol: toneSource, generated: modeOneSource}.write(t)
			schema, err := model.Lower(filepath.Join(root, model.SchemaDir), root)
			assert.NoError(t, err, "the schema lowers")
			assert.Length(t, schema.Enums, 1, "the Tone enum alone: the generator writes Mode's kind")
		})

		t.Run("returns no enum for a schema that imports no symbol package", func(t *testing.T) {
			t.Parallel()

			root := schemaModule(t, reachSchema)
			schema, err := model.Lower(filepath.Join(root, model.SchemaDir), root)
			assert.NoError(t, err, "the schema lowers")
			assert.Empty(t, schema.Enums, "no symbol package, no enum")
		})

		t.Run("returns no enum without a module root", func(t *testing.T) {
			t.Parallel()

			schema, err := model.Lower(validSchema, "")
			assert.NoError(t, err, "the fixture schema lowers")
			assert.Empty(t, schema.Enums, "no module, no symbol package")
		})

		t.Run("returns an error for a schema directory it cannot read", func(t *testing.T) {
			t.Parallel()

			_, err := model.Lower("testdata/nonexistent", "")
			assert.HasError(t, err, "a schema directory it cannot read lowers nothing")
		})

		contract := []struct {
			name  string
			dir   string
			wants []string
		}{
			{
				name: "returns an error for an unknown tag token",
				dir:  "testdata/badtoken", wants: []string{"walkk"},
			},
			{
				name: "returns an error for an unknown side",
				dir:  "testdata/badside", wants: []string{"sideways", "unknown side"},
			},
			{
				name: "returns an error for a duplicate slot name",
				dir:  "testdata/dupslot", wants: []string{"parts"},
			},
			{
				name: "returns an error for a slot on a field that is not a slice",
				dir:  "testdata/slotnotslice", wants: []string{model.SlotPrefix},
			},
			{
				name: "returns an error for a walk on a field that is not a kind",
				dir:  "testdata/walkbadtype", wants: []string{model.WalkToken},
			},
			{
				name:  "returns an error for a walk on a shape the schema cannot reference",
				dir:   "testdata/walkbadshape",
				wants: []string{"Stream", "chan int", "cannot spell"},
			},
			{
				name:  "returns an error for a tagged embedded field",
				dir:   "testdata/embedtagged",
				wants: []string{"Thing embeds Base", "no embedded field"},
			},
			{
				name:  "returns an error for a sized array",
				dir:   "testdata/sizedarray",
				wants: []string{"Digest", "[32]byte", "cannot spell"},
			},
			{
				name:  "returns an error for a map",
				dir:   "testdata/maptype",
				wants: []string{"Meta", "map[string]string", "cannot spell"},
			},
			{
				name: "returns an error for a name on a field that is not a string",
				dir:  "testdata/namenotstring", wants: []string{"Count", "not a string"},
			},
			{
				name: "returns an error for a fact on a node-only field",
				dir:  "testdata/factnode", wants: []string{"emit-visible"},
			},
			{
				name: "returns an error for a fact that does not open with a capital",
				dir:  "testdata/factbadname", wants: []string{"suffix"},
			},
			{
				name: "returns an error for a fact with a character an identifier cannot contain",
				dir:  "testdata/factbadchar", wants: []string{"As-ync", "suffix"},
			},
			{
				name: "returns an error for a declaration that is not a struct",
				dir:  "testdata/nonstruct", wants: []string{"Alias"},
			},
			{
				name: "returns an error for a declaration that is not exported",
				dir:  "testdata/unexported", wants: []string{"thing", "not exported"},
			},
			{
				name: "returns an error for a declaration that is not a type",
				dir:  "testdata/notatype", wants: []string{"types and imports"},
			},
			{
				name: "returns an error for a subject without a node identity",
				dir:  "testdata/nosubjectid", wants: []string{"Thing", "cannot be dispatched"},
			},
		}
		for _, tt := range contract {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := model.Lower(tt.dir, "")
				assert.HasError(t, err, "a schema breaking the contract lowers nothing")
				text := err.Error()
				for _, want := range tt.wants {
					assert.Contains(t, text, want, "naming what broke it")
				}
				assert.That(t, text).
					Contains(".go:", "at a schema position").
					HasPrefix("model: ", "under the package prefix")
			})
		}
	})

	t.Run("OnNode", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give model.Side
			want bool
		}{
			{name: "reports true for a node-tagged field", give: model.SideNode, want: true},
			{name: "reports true for an untagged field", give: model.SideBoth, want: true},
			{name: "reports false for an emit-tagged field", give: model.SideEmit, want: false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.OnNode(), tt.want, "whether the field is on the node model")
			})
		}
	})

	t.Run("OnEmit", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give model.Side
			want bool
		}{
			{name: "reports true for an emit-tagged field", give: model.SideEmit, want: true},
			{name: "reports true for an untagged field", give: model.SideBoth, want: true},
			{name: "reports false for a node-tagged field", give: model.SideNode, want: false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.OnEmit(), tt.want, "whether the field is on the emit model")
			})
		}
	})
}

// A lowering of the fixture schema allocates within its ceiling, and a
// side's questions allocate nothing, in the ordinary run, which runs no
// benchmark. The count keeps the first error of its calls, which cmp.Or
// returns without allocating. The check runs alone, because the count
// includes every goroutine's allocations.
func TestIRAllocs(t *testing.T) {
	var err error
	assert.MaxAllocs(t, func() {
		_, lerr := model.Lower(validSchema, "")
		err = cmp.Or(err, lerr)
	}, lowerValidAllocs, "Lower allocates the parse, the type-check and the schema")
	assert.NoError(t, err, "the fixture schema lowers")
	side := model.SideBoth
	var onNode, onEmit bool
	assert.MaxAllocs(t, func() { onNode, onEmit = side.OnNode(), side.OnEmit() }, 0,
		"OnNode and OnEmit allocate nothing")
	assert.True(t, onNode, "an untagged field is on the node model")
	assert.True(t, onEmit, "and on the emit model")
}

// BenchmarkIR measures a lowering of the fixture schema after one
// lowering, and a side's questions.
func BenchmarkIR(b *testing.B) {
	b.Run("Lower", func(b *testing.B) {
		b.Run("the fixture schema", func(b *testing.B) {
			c := bench.Start(b).Warmup(1).MaxAllocs(lowerValidAllocs)
			defer c.End()
			var (
				schema model.Schema
				err    error
			)
			for c.Loop() {
				schema, err = model.Lower(validSchema, "")
			}
			assert.NoError(b, err, "the schema lowers")
			assert.Length(b, schema.Kinds, 2, "into its two kinds")
		})
	})

	side := model.SideBoth

	b.Run("Side.OnNode", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got bool
		for c.Loop() {
			got = side.OnNode()
		}
		assert.True(b, got, "an untagged field is on the node model")
	})

	b.Run("Side.OnEmit", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got bool
		for c.Loop() {
			got = side.OnEmit()
		}
		assert.True(b, got, "an untagged field is on the emit model")
	})
}

// fieldsOf returns the fixture schema's first kind's fields by name.
func fieldsOf(t *testing.T) map[string]model.FieldSpec {
	t.Helper()

	schema, err := model.Lower(validSchema, "")
	assert.NoError(t, err, "the fixture schema lowers")
	fields := map[string]model.FieldSpec{}
	for _, field := range schema.Kinds[0].Fields {
		fields[field.Name] = field
	}
	return fields
}
