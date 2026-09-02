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

func TestContainer(t *testing.T) {
	t.Parallel()

	t.Run("no container is a subject a rule matches", func(t *testing.T) {
		t.Parallel()

		// A rule fires on declarations, and a package, a file and an
		// import statement are where declarations live.
		assertSubjects(t, "container.go")
	})

	t.Run("the module boundary is read, never generated", func(t *testing.T) {
		t.Parallel()

		// A generated file's imports are a side effect of spelling
		// types, collected per file as the backend renders. A field
		// visible to the emit model would invite a generator to
		// declare them instead.
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

	t.Run("a package's files are read, and its documentation is not", func(t *testing.T) {
		t.Parallel()

		pkg := reflect.TypeFor[schema.Package]()
		assert.Equal(t, annotation(t, pkg, "Files").side, model.SideNodeToken,
			"a generated package is a set of files the layout routes, "+
				"never a parsed directory")
		assert.True(t, annotation(t, pkg, "Files").walk,
			"and the files sit inside it, so the walk descends into them")
		assert.Equal(t, annotation(t, pkg, "Doc").side, model.SideBothToken,
			"documentation crosses both models, because a generator writes it")
	})

	t.Run("a binding carries its own position", func(t *testing.T) {
		t.Parallel()

		// A diagnostic about one unused name in a ten-name import
		// points at that name.
		binding := reflect.TypeFor[schema.Binding]()
		_, held := binding.FieldByName("Pos")
		assert.True(t, held, "a bound name locates itself")
		alias, held := binding.FieldByName("Alias")
		assert.True(t, held, "and carries the local spelling where the statement renames it")
		assert.Equal(t, alias.Type.String(), reflect.TypeFor[string]().String(),
			"as a plain spelling: an unrenamed binding leaves it empty")
	})
}
