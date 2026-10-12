// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package schema_test

import (
	"reflect"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/gen/model"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/symbol/schema"
)

// The tests pin the type machinery field by field. None of it is a
// subject, the walk is a tree, a value parameter states its own type
// and default, and the tags follow the shared vocabulary.
func TestTyperef(t *testing.T) {
	t.Parallel()

	t.Run("marks no type machinery as a subject", func(t *testing.T) {
		t.Parallel()

		// A reference, a parameter and an embed are parts of a
		// declaration. A walk of the declaration that contains them
		// visits them.
		assertSubjects(t, "typeref.go")
	})

	t.Run("declares no identity field on a reference", func(t *testing.T) {
		t.Parallel()

		// A reference is not a declaration, so only its target is an
		// identity.
		_, held := reflect.TypeFor[schema.TypeRef]().FieldByName("ID")
		assert.False(t, held, "a reference declares no identity field")
	})

	t.Run("declares a resolved target as an identity", func(t *testing.T) {
		t.Parallel()

		// An identity can be stored, compared and kept across runs, and a
		// pointer cannot. A read of the target goes through a tracked read.
		target, held := reflect.TypeFor[schema.TypeRef]().FieldByName("Target")
		assert.True(t, held, "a reference has the identity of its target")
		assert.Equal(t, target.Type.String(), reflect.TypeFor[symbol.Identity]().String(),
			"the target is an identity and not a pointer")
	})

	t.Run("leaves a resolved target out of the walk", func(t *testing.T) {
		t.Parallel()

		// Following a target would make the walk cyclic.
		assert.False(t, annotation(t, reflect.TypeFor[schema.TypeRef](), "Target").walk,
			"the walk does not follow the target, so the walk is a tree")
	})

	t.Run("walks the type arguments of a reference", func(t *testing.T) {
		t.Parallel()

		assert.True(t, annotation(t, reflect.TypeFor[schema.TypeRef](), "Args").walk,
			"the type arguments are inside the reference, so the walk descends")
	})

	t.Run("declares the asynchrony of a stream on both sides", func(t *testing.T) {
		t.Parallel()

		// A frontend states it from syntax, the fold reads it on the node
		// side, and a spoke writes it on the emit side.
		async, held := reflect.TypeFor[schema.TypeRef]().FieldByName("Async")
		assert.True(t, held, "a reference has a flag for an asynchronous stream")
		assert.Equal(t, async.Type.String(), reflect.TypeFor[bool]().String(), "the flag is a bool")
		assert.Equal(t, annotation(t, reflect.TypeFor[schema.TypeRef](), "Async").side, model.SideBothToken,
			"the flag is on both models")
	})

	t.Run("declares the host of an embed as an identity", func(t *testing.T) {
		t.Parallel()

		host, held := reflect.TypeFor[schema.Embed]().FieldByName("Host")
		assert.True(t, held, "an embed has the identity of the declaration that contains it")
		assert.Equal(t, host.Type.String(), reflect.TypeFor[symbol.Identity]().String(),
			"the host is an identity")
	})

	t.Run("leaves the host of an embed out of the walk", func(t *testing.T) {
		t.Parallel()

		assert.False(t, annotation(t, reflect.TypeFor[schema.Embed](), "Host").walk,
			"the walk does not follow the back-pointer")
	})

	t.Run("walks the embedded type of an embed", func(t *testing.T) {
		t.Parallel()

		assert.True(t, annotation(t, reflect.TypeFor[schema.Embed](), "Ref").walk,
			"the embed contains the embedded type, so the walk descends")
	})

	t.Run("declares the type of a value parameter as a reference that can be nil", func(t *testing.T) {
		t.Parallel()

		// Rust writes "<const N: usize>", where the parameter's argument
		// is a value. The field is empty for an ordinary type parameter.
		typ, held := reflect.TypeFor[schema.TypeParam]().FieldByName("Type")
		assert.True(t, held, "a const parameter has the type of its value")
		assert.Equal(t, typ.Type.String(), reflect.TypeFor[*schema.TypeRef]().String(),
			"the type is a reference, and nil for an ordinary type parameter")
	})

	t.Run("declares the default of a value parameter as source text", func(t *testing.T) {
		t.Parallel()

		value, held := reflect.TypeFor[schema.TypeParam]().FieldByName("DefaultValue")
		assert.True(t, held, "a const parameter has the spelling of its default")
		assert.Equal(t, value.Type.String(), reflect.TypeFor[string]().String(),
			"the default is source text, unlike a default type argument")
	})

	t.Run("declares a default type argument as a reference", func(t *testing.T) {
		t.Parallel()

		fallback, held := reflect.TypeFor[schema.TypeParam]().FieldByName("Default")
		assert.True(t, held, "a type parameter has a default type argument")
		assert.Equal(t, fallback.Type.String(), reflect.TypeFor[*schema.TypeRef]().String(),
			"the default is a reference, and nil where the language has no such form")
	})

	t.Run("annotates every field from the vocabulary", func(t *testing.T) {
		t.Parallel()
		assertAnnotations(t, familyOf(t, "typeref.go"))
	})

	t.Run("gives every recurring field one meaning", func(t *testing.T) {
		t.Parallel()
		assertConventions(t, familyOf(t, "typeref.go"))
	})
}
