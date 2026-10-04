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

// The container kinds are pinned field by field: none is a subject, the
// module boundary is read from source, and the tags follow the shared
// vocabulary.
func TestContainer(t *testing.T) {
	t.Parallel()

	t.Run("marks no container as a subject", func(t *testing.T) {
		t.Parallel()

		// A rule matches declarations, and a package, a file and an
		// import statement are where declarations are declared.
		assertSubjects(t, "container.go")
	})

	t.Run("reads the module boundary from source", func(t *testing.T) {
		t.Parallel()

		// A generated file's imports are a side effect of spelling types,
		// collected per file as the backend renders. A field visible to
		// the emit model would invite a generator to declare them instead.
		tests := map[string]reflect.Type{
			"Import":  reflect.TypeFor[schema.Import](),
			"Export":  reflect.TypeFor[schema.Export](),
			"Binding": reflect.TypeFor[schema.Binding](),
		}
		for name, kind := range tests {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				for field := range kind.Fields() {
					assert.Equal(t, annotation(t, kind, field.Name).side, model.SideNodeToken,
						name+"."+field.Name+" is read from source: the boundary is "+
							"parsed, not declared")
				}
			})
		}
	})

	t.Run("reads a package's files from source", func(t *testing.T) {
		t.Parallel()

		pkg := reflect.TypeFor[schema.Package]()
		assert.Equal(t, annotation(t, pkg, "Files").side, model.SideNodeToken,
			"a generated package is a set of files the layout routes, "+
				"never a parsed directory")
		assert.True(t, annotation(t, pkg, "Files").walk,
			"the files are inside the package, so the walk descends into them")
	})

	t.Run("declares a package's documentation on both models", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, annotation(t, reflect.TypeFor[schema.Package](), "Doc").side, model.SideBothToken,
			"documentation crosses both models, because a generator writes it")
	})

	t.Run("gives a binding a position of its own", func(t *testing.T) {
		t.Parallel()

		// A diagnostic about one unused name in a ten-name import points
		// at that name.
		_, held := reflect.TypeFor[schema.Binding]().FieldByName("Pos")
		assert.True(t, held, "a bound name locates itself")
	})

	t.Run("declares a binding's alias as a plain spelling", func(t *testing.T) {
		t.Parallel()

		alias, held := reflect.TypeFor[schema.Binding]().FieldByName("Alias")
		assert.True(t, held, "a binding states the local spelling where the statement renames it")
		assert.Equal(t, alias.Type.String(), reflect.TypeFor[string]().String(),
			"as a plain spelling: an unrenamed binding leaves it empty")
	})

	t.Run("annotates every field from the vocabulary", func(t *testing.T) {
		t.Parallel()
		assertAnnotations(t, familyOf(t, "container.go"))
	})

	t.Run("gives every recurring field one meaning", func(t *testing.T) {
		t.Parallel()
		assertConventions(t, familyOf(t, "container.go"))
	})
}
