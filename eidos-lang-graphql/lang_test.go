// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package graphql_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// syntaxAllocs is the comment syntax: its list of line openers.
const syntaxAllocs = 1

// TestLang pins the identity, the extensions and the comment forms of the
// satellite, because a change breaks compositions outside this module.
func TestLang(t *testing.T) {
	t.Parallel()

	t.Run("Lang", func(t *testing.T) {
		t.Parallel()

		t.Run("spells graphql", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, string(graphql.Lang), "graphql", "the language that every identity names")
		})
	})

	t.Run("CodePrefix", func(t *testing.T) {
		t.Parallel()

		t.Run("spells GQL", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, string(graphql.CodePrefix), "GQL", "the prefix of every diagnostic code")
		})
	})

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		t.Run("spells graphql", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, string(graphql.Name), "graphql", "the identity that findings report under")
		})
	})

	t.Run("Extension", func(t *testing.T) {
		t.Parallel()

		t.Run("spells .graphql", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, graphql.Extension, ".graphql", "the suffix of a GraphQL file")
		})
	})

	t.Run("SchemaExtension", func(t *testing.T) {
		t.Parallel()

		t.Run("spells .graphqls", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, graphql.SchemaExtension, ".graphqls", "the suffix of a schema file of gqlgen")
		})
	})

	t.Run("ShortExtension", func(t *testing.T) {
		t.Parallel()

		t.Run("spells .gql", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, graphql.ShortExtension, ".gql", "the short suffix of a GraphQL file")
		})
	})

	t.Run("ReservedPrefix", func(t *testing.T) {
		t.Parallel()

		t.Run("spells two underscores", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, graphql.ReservedPrefix, "__", "the prefix of the names of introspection")
		})
	})

	t.Run("DocumentsSegment", func(t *testing.T) {
		t.Parallel()

		t.Run("spells documents", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, graphql.DocumentsSegment, "documents",
				"the last segment of the package of the executable definitions")
		})
	})

	t.Run("Version", func(t *testing.T) {
		t.Parallel()

		t.Run("spells a version", func(t *testing.T) {
			t.Parallel()

			assert.NotEqual(t, graphql.Version, "", "the version that every unit key folds")
		})
	})

	t.Run("Target", func(t *testing.T) {
		t.Parallel()

		t.Run("spells the language", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, string(graphql.Target), string(graphql.Lang),
				"the emitter copies a reference of a GraphQL source as written into a plan of the target")
		})
	})

	t.Run("BackendVersion", func(t *testing.T) {
		t.Parallel()

		t.Run("spells a version", func(t *testing.T) {
			t.Parallel()

			assert.NotEqual(t, graphql.BackendVersion, "", "the version that the composition fingerprint folds")
		})
	})

	t.Run("Syntax", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the line comment", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, graphql.Syntax().Line, []string{"#"}, "the line comment of GraphQL")
		})

		t.Run("returns no block comment", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, graphql.Syntax().Blocks, "GraphQL has no block comment")
		})

		t.Run("reports no directive convention", func(t *testing.T) {
			t.Parallel()

			assert.False(t, graphql.Syntax().Directives, "a comment of GraphQL does not contain a tool directive")
		})
	})
}

// TestLangAllocs checks the ceiling of Syntax in the ordinary run, which
// does not run benchmarks.
func TestLangAllocs(t *testing.T) {
	var got plugin.CommentSyntax
	assert.MaxAllocs(t, func() { got = graphql.Syntax() }, syntaxAllocs, "Syntax allocates its list of line openers")
	assert.Length(t, got.Line, 1, "Syntax returns the line comment")
}

// BenchmarkLang measures the comment syntax that a composition reads once
// per frontend.
func BenchmarkLang(b *testing.B) {
	b.Run("Syntax", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(syntaxAllocs)
		defer c.End()
		var got plugin.CommentSyntax
		for c.Loop() {
			got = graphql.Syntax()
		}
		assert.Length(b, got.Line, 1, "Syntax returns the line comment")
	})
}
