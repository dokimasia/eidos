// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"os"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/lang/graphql/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/frontendtest"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// newAllocs is the cost of New: the list of line openers of the syntax,
// which the satellite root builds, the list of the claim, the list of key
// registrations and the frontend that the kit builds.
const newAllocs = 1 + 1 + 1 + 1

// New builds the frontend through the kit. The cases pin the identity, the
// claim, the codes and the roles of the frontend, and the suite of the
// kernel checks its contract over the schema tree.
func TestFrontend(t *testing.T) {
	t.Parallel()

	f := frontend.New()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a frontend that meets the conformance contract over the schema tree", func(t *testing.T) {
			t.Parallel()

			frontendtest.RunFrontendSuite(t, func(assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
				return frontend.New(), &frontendtest.Fixture{
					Sources: os.DirFS("testdata/schema"),
					Empty: fstest.MapFS{
						"empty/empty.graphql": {Data: []byte("# A document without a definition.\n")},
					},
					// A schema has no bodies and no unexported names, so a
					// signature-only root loads what a full one loads, and
					// the fixture lists no dropped identity.
					Signatures: []string{"dep"},
					Schemas:    []directive.Schema{tableSchema()},
				}
			})
		})

		t.Run("returns a frontend named after the satellite", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, f.Name(), graphql.Name, "findings report under the identity of the satellite")
		})

		t.Run("returns a frontend that registers the GraphQL keys under the identity of the language",
			func(t *testing.T) {
				t.Parallel()

				provider, provides := f.(plugin.KeyProvider)
				assert.True(t, provides, "the frontend registers keys")
				r := meta.NewRegistry()
				assert.NoError(t, provider.Keys(r.For(string(frontend.Lang))), "the keys register")
				_, held := r.Resolve(graphql.SelectionKey)
				assert.True(t, held, "the selection key is registered")
			})

		t.Run("returns a frontend in the language of the satellite", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, f.Lang(), graphql.Lang, "every loaded declaration is in the language of the satellite")
		})

		t.Run("returns a frontend in the language that the package restates", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, f.Lang(), frontend.Lang, "a caller with the frontend reads Lang")
		})

		t.Run("returns a frontend with the comment forms of the satellite", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, f.Syntax(), graphql.Syntax(), "the comment forms belong to the satellite")
		})

		t.Run("returns a frontend whose language cannot overload", func(t *testing.T) {
			t.Parallel()

			assert.False(t, f.Overloads(), "a type declares each field once by name")
		})

		t.Run("returns a frontend with the version of the satellite", func(t *testing.T) {
			t.Parallel()

			versioned, held := f.(plugin.Versioned)
			assert.True(t, held, "every unit key folds a version, so the frontend states one")
			assert.Equal(t, versioned.Version(), graphql.Version, "the version belongs to the satellite")
		})

		t.Run("returns a frontend that claims every file with an extension of GraphQL", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, f.Selection(), []string{
				"**/*" + graphql.Extension, "**/*" + graphql.SchemaExtension, "**/*" + graphql.ShortExtension,
			}, "the claim negates nothing")
		})
	})

	codes := []struct {
		name string
		give diag.Code
		want string
	}{
		{name: "UnparsedFile", give: frontend.UnparsedFile, want: "GQL-0001"},
		{name: "InvalidDocument", give: frontend.InvalidDocument, want: "GQL-0002"},
		{name: "UnreadConfig", give: frontend.UnreadConfig, want: "GQL-0003"},
		{name: "BadCarrier", give: frontend.BadCarrier, want: "GQL-0004"},
		{name: "UnaddressedCarrier", give: frontend.UnaddressedCarrier, want: "GQL-0005"},
	}
	for _, tt := range codes {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			t.Run("spells "+tt.want, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.String(), tt.want, "the code has the prefix of the satellite and its number")
			})
		})
	}
}

// The construction allocates the frontend that the kit builds. The
// ordinary run, which runs no benchmark, checks the ceiling here.
func TestFrontendAllocs(t *testing.T) {
	checkAllocs(t, frontendCalls())
}

// BenchmarkFrontend measures the construction of the frontend.
func BenchmarkFrontend(b *testing.B) {
	benchCalls(b, frontendCalls())
}

// frontendCalls returns a call of New.
func frontendCalls() []allocCall {
	var built plugin.Frontend
	return []allocCall{{
		name: "New", allocs: newAllocs,
		call:  func() { built = frontend.New() },
		check: func(tb assert.TB) { assert.Equal(tb, built.Name(), graphql.Name, "New returns the frontend") },
	}}
}

// tableSchema is the directive that the carriers of the schema tree write.
func tableSchema() directive.Schema {
	return directive.Schema{
		Plugin: "gen", Name: "table",
		Params: []directive.ParamSpec{{
			Key: "name", Type: directive.TypeString, Required: true,
			Doc: "the table that the declaration maps to",
		}},
		Doc: "maps a declaration onto a table",
	}
}
