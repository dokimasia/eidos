// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package schema_test

import (
	"reflect"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/symbol/schema"
)

func TestEnum(t *testing.T) {
	t.Parallel()

	t.Run("the variant sets a rule can match", func(t *testing.T) {
		t.Parallel()

		// A variant belongs to its set and is reached by walking it,
		// rather than matched on its own.
		assertSubjects(t, "enum.go", "Enum", "Sum")
	})

	t.Run("a variant names the set that owns it", func(t *testing.T) {
		t.Parallel()

		for _, name := range []string{"EnumVariant", "SumVariant"} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				_, owned := everyKind()[name].FieldByName("Host")
				assert.True(t, owned,
					"a variant is an owned kind, so it carries its owner's identity")
			})
		}
		for _, name := range []string{"Enum", "Sum"} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				_, owned := everyKind()[name].FieldByName("Host")
				assert.False(t, owned,
					"a variant set is declared at top level and names no owner")
			})
		}
	})

	t.Run("the split from a sum goes by payload", func(t *testing.T) {
		t.Parallel()

		// A variant set where no variant carries fields is an Enum;
		// one where any variant does is a Sum. The types are what hold
		// the line: an enum variant has nowhere to put a payload.
		_, payload := reflect.TypeFor[schema.EnumVariant]().FieldByName("Fields")
		assert.False(t, payload,
			"an enum variant carries no payload, which is what makes it one")
		fields, held := reflect.TypeFor[schema.SumVariant]().FieldByName("Fields")
		assert.True(t, held, "a sum variant carries its payload")
		assert.Equal(t, fields.Type.String(), reflect.TypeFor[[]*schema.Field]().String(),
			"as fields, unnamed where the language writes them positionally")
	})

	t.Run("an enum declares state and behaviour of its own", func(t *testing.T) {
		t.Parallel()

		// A Java enum is a class. Most languages leave both empty, and
		// emptiness claims nothing.
		enum := reflect.TypeFor[schema.Enum]()
		fields, held := enum.FieldByName("Fields")
		assert.True(t, held, "an enum carries instance state")
		assert.Equal(t, fields.Type.String(), reflect.TypeFor[[]*schema.Field]().String(),
			"as the same field kind a struct carries")
		methods, held := enum.FieldByName("Methods")
		assert.True(t, held, "and behaviour")
		assert.Equal(t, methods.Type.String(), reflect.TypeFor[[]*schema.Method]().String(),
			"as the same method kind a struct carries")
	})
}
