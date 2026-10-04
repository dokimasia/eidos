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

// The type machinery is pinned field by field: none of it is a subject,
// the walk stays a tree, a value parameter states its own type and
// default, and the tags follow the shared vocabulary.
func TestTyperef(t *testing.T) {
	t.Parallel()

	t.Run("marks no type machinery as a subject", func(t *testing.T) {
		t.Parallel()

		// A reference, a parameter and an embed are parts of a
		// declaration, reached by walking the declaration that contains
		// them.
		assertSubjects(t, "typeref.go")
	})

	t.Run("declares a resolved target as an identity", func(t *testing.T) {
		t.Parallel()

		// An identity can be stored, compared and kept across runs, and a
		// pointer cannot. Reaching the target goes through a tracked read.
		target, held := reflect.TypeFor[schema.TypeRef]().FieldByName("Target")
		assert.True(t, held, "a reference states what it resolves to")
		assert.Equal(t, target.Type.String(), reflect.TypeFor[symbol.Identity]().String(),
			"as an identity rather than a pointer")
	})

	t.Run("leaves a resolved target out of the walk", func(t *testing.T) {
		t.Parallel()

		// Following a target would make the walk cyclic.
		assert.False(t, annotation(t, reflect.TypeFor[schema.TypeRef](), "Target").walk,
			"the walk does not follow the target, which keeps it a tree")
	})

	t.Run("walks a reference's type arguments", func(t *testing.T) {
		t.Parallel()

		assert.True(t, annotation(t, reflect.TypeFor[schema.TypeRef](), "Args").walk,
			"the type arguments are inside the reference, so the walk descends")
	})

	t.Run("declares an embed's host as an identity", func(t *testing.T) {
		t.Parallel()

		host, held := reflect.TypeFor[schema.Embed]().FieldByName("Host")
		assert.True(t, held, "an embed names the declaration that contains it")
		assert.Equal(t, host.Type.String(), reflect.TypeFor[symbol.Identity]().String(),
			"as an identity")
	})

	t.Run("leaves an embed's host out of the walk", func(t *testing.T) {
		t.Parallel()

		assert.False(t, annotation(t, reflect.TypeFor[schema.Embed](), "Host").walk,
			"the back-pointer is not an edge the walk follows")
	})

	t.Run("walks an embed's embedded type", func(t *testing.T) {
		t.Parallel()

		assert.True(t, annotation(t, reflect.TypeFor[schema.Embed](), "Ref").walk,
			"the embed contains the embedded type, so the walk descends")
	})

	t.Run("declares a value parameter's type as a reference that can be absent", func(t *testing.T) {
		t.Parallel()

		// Rust writes "<const N: usize>", where the parameter's argument
		// is a value. The field is empty for an ordinary type parameter.
		typ, held := reflect.TypeFor[schema.TypeParam]().FieldByName("Type")
		assert.True(t, held, "a const parameter states the value's type")
		assert.Equal(t, typ.Type.String(), reflect.TypeFor[*schema.TypeRef]().String(),
			"as a reference that is absent for an ordinary type parameter")
	})

	t.Run("declares a value parameter's default as source text", func(t *testing.T) {
		t.Parallel()

		value, held := reflect.TypeFor[schema.TypeParam]().FieldByName("DefaultValue")
		assert.True(t, held, "a const parameter states its default spelling")
		assert.Equal(t, value.Type.String(), reflect.TypeFor[string]().String(),
			"as source text, apart from a default type argument")
	})

	t.Run("declares a default type argument as a reference", func(t *testing.T) {
		t.Parallel()

		fallback, held := reflect.TypeFor[schema.TypeParam]().FieldByName("Default")
		assert.True(t, held, "a type parameter states its default type argument")
		assert.Equal(t, fallback.Type.String(), reflect.TypeFor[*schema.TypeRef]().String(),
			"as a reference, nil where the language has no such form")
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
