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

func TestMember(t *testing.T) {
	t.Parallel()

	t.Run("every member kind is a subject a rule can match", func(t *testing.T) {
		t.Parallel()

		assertSubjects(t, "member.go", "Field", "Variable", "Constant")
	})

	t.Run("a field is owned and a top-level binding is not", func(t *testing.T) {
		t.Parallel()

		_, owned := reflect.TypeFor[schema.Field]().FieldByName("Host")
		assert.True(t, owned, "a field names the type that holds it")
		for _, name := range []string{"Variable", "Constant"} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				_, held := everyKind()[name].FieldByName("Host")
				assert.False(t, held,
					"a binding declared outside any type has no owner to name")
			})
		}
	})

	t.Run("a value expression travels as source text", func(t *testing.T) {
		t.Parallel()

		// Evaluating a Go iota expression or a Java constructor
		// argument is the language's job, not the model's.
		tests := map[string]reflect.Type{
			"Field":    reflect.TypeFor[schema.Field](),
			"Variable": reflect.TypeFor[schema.Variable](),
			"Constant": reflect.TypeFor[schema.Constant](),
		}
		for name, kind := range tests {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				field, held := kind.FieldByName("Value")
				assert.True(t, held, name+" carries its value expression")
				assert.Equal(t, field.Type.String(), reflect.TypeFor[string]().String(),
					"as the source spelling, unevaluated")
				assert.Equal(t, annotation(t, kind, "Value").side, model.SideBothToken,
					"on both models, because a generator states one too")
			})
		}
	})

	t.Run("a type the source states none of is nil, not empty", func(t *testing.T) {
		t.Parallel()

		// A pointer is what lets an inferred variable and an untyped
		// constant say "the source stated none" rather than "the type
		// is the zero one".
		tests := map[string]reflect.Type{
			"Field":    reflect.TypeFor[schema.Field](),
			"Variable": reflect.TypeFor[schema.Variable](),
			"Constant": reflect.TypeFor[schema.Constant](),
		}
		for name, kind := range tests {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				field, held := kind.FieldByName("Type")
				assert.True(t, held, name+" carries its type")
				assert.Equal(t, field.Type.String(), reflect.TypeFor[*schema.TypeRef]().String(),
					"as a reference that can be absent")
			})
		}
	})
}
