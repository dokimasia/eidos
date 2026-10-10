// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package schema_test

import (
	"reflect"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/gen/model"
	"go.dokimi.dev/eidos/core/symbol/schema"
)

// The member kinds are pinned field by field: the kinds a rule can
// match, the owner a field names, the value and type each states, and
// the tags the generator reads.
func TestMember(t *testing.T) {
	t.Parallel()

	t.Run("marks every member kind as a subject", func(t *testing.T) {
		t.Parallel()

		assertSubjects(t, "member.go", "Field", "Variable", "Constant")
	})

	t.Run("gives a field a host", func(t *testing.T) {
		t.Parallel()

		_, owned := reflect.TypeFor[schema.Field]().FieldByName("Host")
		assert.True(t, owned, "a field names the type that declares it")
	})

	t.Run("gives a top-level binding no host", func(t *testing.T) {
		t.Parallel()

		for _, name := range []string{"Variable", "Constant"} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				_, owned := everyKind()[name].FieldByName("Host")
				assert.False(t, owned,
					"a binding declared outside any type has no owner to name")
			})
		}
	})

	t.Run("declares a value expression as source text", func(t *testing.T) {
		t.Parallel()

		// Evaluating a Go iota expression or a Java constructor argument
		// is the language's job, not the model's.
		tests := map[string]reflect.Type{
			"Field":    reflect.TypeFor[schema.Field](),
			"Variable": reflect.TypeFor[schema.Variable](),
			"Constant": reflect.TypeFor[schema.Constant](),
		}
		for name, kind := range tests {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				field, held := kind.FieldByName("Value")
				assert.True(t, held, name+" states its value expression")
				assert.Equal(t, field.Type.String(), reflect.TypeFor[string]().String(),
					"as the source spelling, unevaluated")
				assert.Equal(t, annotation(t, kind, "Value").side, model.SideBothToken,
					"on both models, because a generator states one too")
			})
		}
	})

	t.Run("declares a member's type as a reference that can be absent", func(t *testing.T) {
		t.Parallel()

		// A variable declared without a type and an untyped constant
		// leave the type absent. A pointer states that absence apart from
		// the zero type.
		tests := map[string]reflect.Type{
			"Field":    reflect.TypeFor[schema.Field](),
			"Variable": reflect.TypeFor[schema.Variable](),
			"Constant": reflect.TypeFor[schema.Constant](),
		}
		for name, kind := range tests {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				field, held := kind.FieldByName("Type")
				assert.True(t, held, name+" states its type")
				assert.Equal(t, field.Type.String(), reflect.TypeFor[*schema.TypeRef]().String(),
					"as a reference that can be absent")
			})
		}
	})

	t.Run("annotates every field from the vocabulary", func(t *testing.T) {
		t.Parallel()
		assertAnnotations(t, familyOf(t, "member.go"))
	})

	t.Run("gives every recurring field one meaning", func(t *testing.T) {
		t.Parallel()
		assertConventions(t, familyOf(t, "member.go"))
	})
}
