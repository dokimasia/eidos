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

func TestCallable(t *testing.T) {
	t.Parallel()

	t.Run("the callables a rule can match", func(t *testing.T) {
		t.Parallel()

		// A parameter and a return are subjects too: an authored
		// value sits on one where a language's comments reach it.
		assertSubjects(t, "callable.go", "Function", "Method", "Param", "Return")
	})

	t.Run("a method is owned and a function is not", func(t *testing.T) {
		t.Parallel()

		_, owned := reflect.TypeFor[schema.Method]().FieldByName("Host")
		assert.True(t, owned,
			"a method names the declaration that contains it, which is what "+
				"lets the walk reach the owner through a tracked read")
		_, free := reflect.TypeFor[schema.Function]().FieldByName("Host")
		assert.False(t, free,
			"a function is declared outside any type, so it has no owner to name")
	})

	t.Run("the receiver and the attached type answer separate questions", func(t *testing.T) {
		t.Parallel()

		// A Kotlin extension function is declared in one place and
		// attaches to another. Folding the two into one field would
		// make one of the answers a lie.
		method := reflect.TypeFor[schema.Method]()
		receiver, held := method.FieldByName("Receiver")
		assert.True(t, held, "a method carries an explicit receiver where one is written")
		assert.Equal(t, receiver.Type.String(), reflect.TypeFor[*schema.Param]().String(),
			"the receiver is a parameter, because that is what a language writes")

		receives, held := method.FieldByName("Receives")
		assert.True(t, held, "and the type it attaches to, where that differs from its host")
		assert.Equal(t, receives.Type.String(), reflect.TypeFor[*schema.TypeRef]().String(),
			"which is a type reference, not a parameter")
	})

	t.Run("a parameter carries its default as source text", func(t *testing.T) {
		t.Parallel()

		// A generator that drops a default changes the callee's
		// contract, so the spelling travels with the parameter rather
		// than living in metadata.
		field, held := reflect.TypeFor[schema.Param]().FieldByName("Default")
		assert.True(t, held, "a parameter carries its default")
		assert.Equal(t, field.Type.String(), reflect.TypeFor[string]().String(),
			"as the source spelling, unevaluated: evaluating it is the language's job")
		assert.Equal(t, annotation(t, reflect.TypeFor[schema.Param](), "Default").side,
			model.SideBothToken,
			"on both models, because a generated parameter states one too")
	})
}
