// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package graphql_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/graphql"
)

// TestDirectives pins the names of the built-in directives that the
// satellite reads, because the specification of GraphQL fixes them.
func TestDirectives(t *testing.T) {
	t.Parallel()

	t.Run("SpecifiedByDirective", func(t *testing.T) {
		t.Parallel()

		t.Run("spells specifiedBy", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, graphql.SpecifiedByDirective, "specifiedBy", "the directive of the URL of a scalar")
		})
	})

	t.Run("OneOfDirective", func(t *testing.T) {
		t.Parallel()

		t.Run("spells oneOf", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, graphql.OneOfDirective, "oneOf", "the directive of an input object of one field")
		})
	})

	t.Run("DeprecatedDirective", func(t *testing.T) {
		t.Parallel()

		t.Run("spells deprecated", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, graphql.DeprecatedDirective, "deprecated", "the directive of a deprecation")
		})
	})

	t.Run("IncludeDirective", func(t *testing.T) {
		t.Parallel()

		t.Run("spells include", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, graphql.IncludeDirective, "include", "the directive that includes a selected field")
		})
	})

	t.Run("SkipDirective", func(t *testing.T) {
		t.Parallel()

		t.Run("spells skip", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, graphql.SkipDirective, "skip", "the directive that skips a selected field")
		})
	})

	t.Run("DeferDirective", func(t *testing.T) {
		t.Parallel()

		t.Run("spells defer", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, graphql.DeferDirective, "defer", "the directive that defers the fields of a fragment")
		})
	})

	t.Run("NameSep", func(t *testing.T) {
		t.Parallel()

		t.Run("spells a colon and a space", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, graphql.NameSep, ": ", "the separator of a name and its value")
		})
	})

	t.Run("DirectiveSigil", func(t *testing.T) {
		t.Parallel()

		t.Run("spells @", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, graphql.DirectiveSigil, "@", "the sigil of the name of a directive")
		})
	})
}
