// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package schema_test

import (
	"reflect"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/gen/model"
	"go.dokimi.dev/eidos/core/symbol/schema"
)

// The markers are the one part of the schema lowering matches by
// name rather than by structure, so renaming either compiles here
// and breaks the generator. The names are pinned against the
// generator's own constants.
func TestMarker(t *testing.T) {
	t.Parallel()

	t.Run("the markers carry the names lowering matches", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			typ  reflect.Type
			want string
		}{
			{
				name: "the heterogeneous-field marker",
				typ:  reflect.TypeFor[schema.Symbol](),
				want: model.MarkerName,
			},
			{
				name: "the body marker",
				typ:  reflect.TypeFor[schema.Body](),
				want: model.BodyMarkerName,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.typ.Name(), tt.want,
					"lowering reads the marker by name, so the spelling is contract")
				assert.Equal(t, tt.typ.Kind(), reflect.Interface,
					"a marker admits anything the field's contract allows")
				assert.Equal(t, tt.typ.NumMethod(), 0,
					"and declares no methods: it is read by name, never by structure")
			})
		}
	})

	t.Run("a field admitting any kind is typed by the Symbol marker", func(t *testing.T) {
		t.Parallel()

		// A field typed by a concrete kind accepts one; this marker is
		// what says the language genuinely admits anything there.
		tests := map[string]struct {
			kind  reflect.Type
			field string
		}{
			"File.Decls":      {reflect.TypeFor[schema.File](), "Decls"},
			"Struct.Types":    {reflect.TypeFor[schema.Struct](), "Types"},
			"Interface.Types": {reflect.TypeFor[schema.Interface](), "Types"},
		}
		for name, tt := range tests {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				kind := tt.kind
				field, held := kind.FieldByName(tt.field)
				assert.True(t, held, name+" is declared")
				assert.Equal(t, field.Type.Kind(), reflect.Slice,
					"a heterogeneous field holds a list of declarations")
				assert.Equal(t, field.Type.Elem().Name(), model.MarkerName,
					"typed by the marker, so every kind is admitted")
				assert.True(t, annotation(t, kind, field.Name).walk,
					"and walked, because the declarations sit inside this one")
			})
		}
	})

	t.Run("a callable's emit content is typed by the Body marker", func(t *testing.T) {
		t.Parallel()

		// Parsed bodies are out of scope entirely, so the field is
		// declared emit-side and the node model never sees it.
		for _, name := range []string{"Function", "Method"} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				kind := everyKind()[name]
				field, held := kind.FieldByName(model.BodyMarkerName)
				assert.True(t, held, name+" carries a body")
				assert.Equal(t, field.Type.Name(), model.BodyMarkerName,
					"typed by the marker, which resolves to the emit package's own Body")
				assert.Equal(t, annotation(t, kind, field.Name).side, model.SideEmitToken,
					"emit-side alone: the node model carries a signature and never a body")
			})
		}
	})
}
