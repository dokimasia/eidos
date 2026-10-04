// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package schema_test

import (
	"reflect"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/symbol/schema"
)

// The variant-set kinds are pinned field by field: the sets a rule can
// match, the owner a variant names, the payload that splits a sum from
// an enum, and the tags the generator reads.
func TestEnum(t *testing.T) {
	t.Parallel()

	t.Run("marks the variant sets a rule can match", func(t *testing.T) {
		t.Parallel()

		// A variant belongs to its set and is reached by walking it,
		// rather than matched on its own.
		assertSubjects(t, "enum.go", "Enum", "Sum")
	})

	t.Run("gives a variant the host of its set", func(t *testing.T) {
		t.Parallel()

		for _, name := range []string{"EnumVariant", "SumVariant"} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				_, owned := everyKind()[name].FieldByName("Host")
				assert.True(t, owned,
					"a variant is an owned kind, so it states its owner's identity")
			})
		}
	})

	t.Run("gives a variant set no host", func(t *testing.T) {
		t.Parallel()

		for _, name := range []string{"Enum", "Sum"} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				_, owned := everyKind()[name].FieldByName("Host")
				assert.False(t, owned,
					"a variant set is declared at top level and names no owner")
			})
		}
	})

	t.Run("declares no payload on an enum variant", func(t *testing.T) {
		t.Parallel()

		// A variant set where no variant states fields is an Enum, and
		// one where any variant does is a Sum. An enum variant has no
		// field to put a payload in.
		_, payload := reflect.TypeFor[schema.EnumVariant]().FieldByName("Fields")
		assert.False(t, payload, "an enum variant states no payload, which is what makes it one")
	})

	t.Run("declares a sum variant's payload as fields", func(t *testing.T) {
		t.Parallel()

		fields, held := reflect.TypeFor[schema.SumVariant]().FieldByName("Fields")
		assert.True(t, held, "a sum variant states its payload")
		assert.Equal(t, fields.Type.String(), reflect.TypeFor[[]*schema.Field]().String(),
			"as fields, unnamed where the language writes them positionally")
	})

	t.Run("declares an enum's instance state", func(t *testing.T) {
		t.Parallel()

		// A Java enum is a class. Most languages leave the list empty,
		// and an empty list claims nothing.
		fields, held := reflect.TypeFor[schema.Enum]().FieldByName("Fields")
		assert.True(t, held, "an enum states instance state")
		assert.Equal(t, fields.Type.String(), reflect.TypeFor[[]*schema.Field]().String(),
			"as the field kind a struct declares")
	})

	t.Run("declares an enum's behaviour", func(t *testing.T) {
		t.Parallel()

		methods, held := reflect.TypeFor[schema.Enum]().FieldByName("Methods")
		assert.True(t, held, "an enum states behaviour")
		assert.Equal(t, methods.Type.String(), reflect.TypeFor[[]*schema.Method]().String(),
			"as the method kind a struct declares")
	})

	t.Run("annotates every field from the vocabulary", func(t *testing.T) {
		t.Parallel()
		assertAnnotations(t, familyOf(t, "enum.go"))
	})

	t.Run("gives every recurring field one meaning", func(t *testing.T) {
		t.Parallel()
		assertConventions(t, familyOf(t, "enum.go"))
	})
}
