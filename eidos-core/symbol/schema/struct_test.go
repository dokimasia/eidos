// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package schema_test

import (
	"reflect"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/symbol/schema"
)

// The structural kinds are pinned field by field: the kinds a rule can
// match, the member lists a struct shares with an interface, the
// supertype relations, and the tags the generator reads.
func TestStruct(t *testing.T) {
	t.Parallel()

	t.Run("marks every structural kind as a subject", func(t *testing.T) {
		t.Parallel()

		assertSubjects(t, "struct.go", "Struct", "Interface", "Alias")
	})

	t.Run("gives a struct the member lists of an interface", func(t *testing.T) {
		t.Parallel()

		// One source construct maps to one kind whatever its body
		// declares, so both kinds declare the same member lists, and the
		// split is about whether a language can make a value.
		for _, member := range []string{"Fields", "Methods", "Types"} {
			t.Run(member, func(t *testing.T) {
				t.Parallel()

				list, declared := reflect.TypeFor[schema.Struct]().FieldByName(member)
				assert.True(t, declared, "a struct declares "+member)
				shape, declared := reflect.TypeFor[schema.Interface]().FieldByName(member)
				assert.True(t, declared, "an interface declares "+member)
				assert.Equal(t, shape.Type.String(), list.Type.String(),
					"under the same type: the two never differ by which "+
						"members they may declare")
			})
		}
	})

	t.Run("declares no implements list on an interface", func(t *testing.T) {
		t.Parallel()

		// An interface that names another widens its own contract, which
		// is Extends.
		_, held := reflect.TypeFor[schema.Interface]().FieldByName("Implements")
		assert.False(t, held,
			"an interface naming another is widening its contract, which is Extends")
	})

	t.Run("declares no abstract flag on an interface", func(t *testing.T) {
		t.Parallel()

		_, held := reflect.TypeFor[schema.Interface]().FieldByName("Abstract")
		assert.False(t, held,
			"nothing instantiates an interface, so no flag states that it cannot be")
	})

	t.Run("declares an implements list on a struct", func(t *testing.T) {
		t.Parallel()

		_, held := reflect.TypeFor[schema.Struct]().FieldByName("Implements")
		assert.True(t, held, "a struct names the contracts it satisfies")
	})

	t.Run("declares embeds apart from supertypes", func(t *testing.T) {
		t.Parallel()

		// Embedding promotes members and nominal supertyping inherits
		// them. A language fills what it has, and one field for both
		// would make a Go generator guess.
		embeds, held := reflect.TypeFor[schema.Struct]().FieldByName("Embeds")
		assert.True(t, held, "compositional promotion has its own field")
		assert.Equal(t, embeds.Type.String(), reflect.TypeFor[[]*schema.Embed]().String(),
			"a list of embeds, which resolve by a language rule rather than a model one")
	})

	t.Run("declares supertypes as type references", func(t *testing.T) {
		t.Parallel()

		extends, held := reflect.TypeFor[schema.Struct]().FieldByName("Extends")
		assert.True(t, held, "nominal supertyping has a field of its own")
		assert.Equal(t, extends.Type.String(), reflect.TypeFor[[]*schema.TypeRef]().String(),
			"a list of type references")
	})

	t.Run("declares an alias's target as a reference that can be absent", func(t *testing.T) {
		t.Parallel()

		// A Rust trait's "type Item;" and a Swift associatedtype are names
		// the implementation supplies, so an associated type is an alias
		// whose target is absent rather than empty.
		target, held := reflect.TypeFor[schema.Alias]().FieldByName("Target")
		assert.True(t, held, "an alias states the type it names")
		assert.Equal(t, target.Type.String(), reflect.TypeFor[*schema.TypeRef]().String(),
			"as a reference that can be absent")
	})

	t.Run("annotates every field from the vocabulary", func(t *testing.T) {
		t.Parallel()
		assertAnnotations(t, familyOf(t, "struct.go"))
	})

	t.Run("gives every recurring field one meaning", func(t *testing.T) {
		t.Parallel()
		assertConventions(t, familyOf(t, "struct.go"))
	})
}
