// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package schema_test

import (
	"reflect"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/symbol/schema"
)

func TestStruct(t *testing.T) {
	t.Parallel()

	t.Run("every structural kind is a subject a rule can match", func(t *testing.T) {
		t.Parallel()

		assertSubjects(t, "struct.go", "Struct", "Interface", "Alias")
	})

	t.Run("a struct and an interface differ by instantiation alone", func(t *testing.T) {
		t.Parallel()

		// One source construct maps to one kind whatever its body
		// holds, so both carry the same member lists and the split
		// stays about whether a language can make a value.
		for _, member := range []string{"Fields", "Methods", "Types"} {
			t.Run(member, func(t *testing.T) {
				t.Parallel()

				held, declared := reflect.TypeFor[schema.Struct]().FieldByName(member)
				assert.True(t, declared, "a struct declares "+member)
				shape, declared := reflect.TypeFor[schema.Interface]().FieldByName(member)
				assert.True(t, declared, "and so does an interface")
				assert.Equal(t, shape.Type.String(), held.Type.String(),
					"under the same type: the two never differ by which "+
						"members they may declare")
			})
		}
	})

	t.Run("an interface declares no implementation", func(t *testing.T) {
		t.Parallel()

		// An interface naming another widens its own contract, which
		// is Extends; and nothing instantiates one, so there is
		// nothing for Abstract to say.
		shape := reflect.TypeFor[schema.Interface]()
		_, held := shape.FieldByName("Implements")
		assert.False(t, held,
			"an interface naming another is widening its contract, which is Extends")
		_, held = shape.FieldByName("Abstract")
		assert.False(t, held,
			"nothing instantiates an interface, so no flag says it cannot be")
		_, held = reflect.TypeFor[schema.Struct]().FieldByName("Implements")
		assert.True(t, held, "a struct is what names the contracts it satisfies")
	})

	t.Run("the three supertype relations stay apart", func(t *testing.T) {
		t.Parallel()

		// Embedding promotes members and nominal supertyping inherits
		// them. A language fills what it has, and folding the two
		// would make a Go generator guess.
		kind := reflect.TypeFor[schema.Struct]()
		embeds, held := kind.FieldByName("Embeds")
		assert.True(t, held, "compositional promotion has its own field")
		assert.Equal(t, embeds.Type.String(), reflect.TypeFor[[]*schema.Embed]().String(),
			"holding embeds, which resolve by a language rule rather than a model one")
		extends, held := kind.FieldByName("Extends")
		assert.True(t, held, "nominal supertyping has another")
		assert.Equal(t, extends.Type.String(), reflect.TypeFor[[]*schema.TypeRef]().String(),
			"holding type references")
	})

	t.Run("an alias without a target is an associated type", func(t *testing.T) {
		t.Parallel()

		// A Rust trait's "type Item;" and a Swift associatedtype are
		// names the implementation supplies, so the target has to be
		// absent rather than empty.
		target, held := reflect.TypeFor[schema.Alias]().FieldByName("Target")
		assert.True(t, held, "an alias carries the type it names")
		assert.Equal(t, target.Type.String(), reflect.TypeFor[*schema.TypeRef]().String(),
			"as a reference that can be absent")
	})
}
