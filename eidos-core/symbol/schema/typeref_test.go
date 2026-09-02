// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package schema_test

import (
	"reflect"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/symbol/schema"
)

func TestTyperef(t *testing.T) {
	t.Parallel()

	t.Run("no type machinery is a subject a rule matches", func(t *testing.T) {
		t.Parallel()

		// A reference, a parameter and an embed are parts of a
		// declaration, reached by walking the declaration that holds
		// them.
		assertSubjects(t, "typeref.go")
	})

	t.Run("a resolved target is named, not walked", func(t *testing.T) {
		t.Parallel()

		// Following a target would make the walk cyclic. An identity
		// can be stored, compared and carried across runs; a pointer
		// cannot, and reaching the target goes through a tracked read.
		reference := reflect.TypeFor[schema.TypeRef]()
		target, held := reference.FieldByName("Target")
		assert.True(t, held, "a reference carries what it resolves to")
		assert.Equal(t, target.Type.String(), reflect.TypeFor[symbol.Identity]().String(),
			"as an identity rather than a pointer")
		assert.False(t, annotation(t, reference, "Target").walk,
			"and the walk does not follow it, which is what keeps it a tree")

		assert.True(t, annotation(t, reference, "Args").walk,
			"the type arguments sit inside the reference, so the walk descends")
	})

	t.Run("an embed names its host without walking to it", func(t *testing.T) {
		t.Parallel()

		embed := reflect.TypeFor[schema.Embed]()
		host, held := embed.FieldByName("Host")
		assert.True(t, held, "an embed names the declaration it sits in")
		assert.Equal(t, host.Type.String(), reflect.TypeFor[symbol.Identity]().String(),
			"as an identity")
		assert.False(t, annotation(t, embed, "Host").walk,
			"and the back-pointer is not an edge the walk follows")
		assert.True(t, annotation(t, embed, "Ref").walk,
			"the embedded type is, because the embed contains it")
	})

	t.Run("a value parameter carries a type and a default of its own", func(t *testing.T) {
		t.Parallel()

		// Rust writes "<const N: usize>", where the parameter's
		// argument is a value. Both fields stay empty for an ordinary
		// type parameter.
		parameter := reflect.TypeFor[schema.TypeParam]()
		typ, held := parameter.FieldByName("Type")
		assert.True(t, held, "a const parameter carries the value's type")
		assert.Equal(t, typ.Type.String(), reflect.TypeFor[*schema.TypeRef]().String(),
			"as a reference that is absent for an ordinary type parameter")

		value, held := parameter.FieldByName("DefaultValue")
		assert.True(t, held, "and its default spelling")
		assert.Equal(t, value.Type.String(), reflect.TypeFor[string]().String(),
			"as source text, which is a different question from a default type argument")

		fallback, held := parameter.FieldByName("Default")
		assert.True(t, held, "the default type argument is that other question")
		assert.Equal(t, fallback.Type.String(), reflect.TypeFor[*schema.TypeRef]().String(),
			"and is a reference, nil where the language has no such form")
	})
}
