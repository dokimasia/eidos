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

// The callable kinds are pinned field by field: the kinds a rule can
// match, the fields each declares, and the tags the generator reads.
func TestCallable(t *testing.T) {
	t.Parallel()

	t.Run("marks the callables a rule can match", func(t *testing.T) {
		t.Parallel()

		// A parameter and a return are subjects too: an authored value
		// is attached to one where a language's comments reach it.
		assertSubjects(t, "callable.go", "Function", "Method", "Param", "Return")
	})

	t.Run("gives a method a host", func(t *testing.T) {
		t.Parallel()

		_, owned := reflect.TypeFor[schema.Method]().FieldByName("Host")
		assert.True(t, owned,
			"a method names the declaration that contains it, which is what "+
				"lets the walk reach the owner through a tracked read")
	})

	t.Run("gives a function no host", func(t *testing.T) {
		t.Parallel()

		_, owned := reflect.TypeFor[schema.Function]().FieldByName("Host")
		assert.False(t, owned,
			"a function is declared outside any type, so it has no owner to name")
	})

	t.Run("declares a method's receiver as a parameter", func(t *testing.T) {
		t.Parallel()

		// A Kotlin extension function is declared in one place and
		// attaches to another, so the receiver and the attached type are
		// two fields.
		receiver, held := reflect.TypeFor[schema.Method]().FieldByName("Receiver")
		assert.True(t, held, "a method states an explicit receiver where one is written")
		assert.Equal(t, receiver.Type.String(), reflect.TypeFor[*schema.Param]().String(),
			"the receiver is a parameter, because that is what a language writes")
	})

	t.Run("declares a method's attached type as a type reference", func(t *testing.T) {
		t.Parallel()

		receives, held := reflect.TypeFor[schema.Method]().FieldByName("Receives")
		assert.True(t, held, "a method states the type it attaches to, where that differs from its host")
		assert.Equal(t, receives.Type.String(), reflect.TypeFor[*schema.TypeRef]().String(),
			"the attached type is a type reference, not a parameter")
	})

	t.Run("declares a parameter's default as source text", func(t *testing.T) {
		t.Parallel()

		// A generator that drops a default changes the callee's contract,
		// so the parameter states the spelling itself and no metadata key
		// does.
		field, held := reflect.TypeFor[schema.Param]().FieldByName("Default")
		assert.True(t, held, "a parameter states its default")
		assert.Equal(t, field.Type.String(), reflect.TypeFor[string]().String(),
			"as the source spelling, unevaluated: evaluating it is the language's job")
		assert.Equal(t, annotation(t, reflect.TypeFor[schema.Param](), "Default").side,
			model.SideBothToken,
			"on both models, because a generated parameter states one too")
	})

	t.Run("annotates every field from the vocabulary", func(t *testing.T) {
		t.Parallel()
		assertAnnotations(t, familyOf(t, "callable.go"))
	})

	t.Run("gives every recurring field one meaning", func(t *testing.T) {
		t.Parallel()
		assertConventions(t, familyOf(t, "callable.go"))
	})
}
