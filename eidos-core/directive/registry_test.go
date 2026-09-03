// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package directive_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/directive"
)

// wellFormed returns a schema that passes registration, for cases
// that vary one thing.
func wellFormed(plugin string, name directive.Name) directive.Schema {
	return directive.Schema{
		Plugin: plugin,
		Name:   name,
		Params: []directive.ParamSpec{
			{Key: "mode", Type: directive.TypeString, Doc: "the generation mode"},
		},
		Doc: "a fixture schema",
	}
}

// sealed returns a registry holding schemas, sealed without faults.
func sealed(tb assert.TB, schemas ...directive.Schema) *directive.Registry {
	tb.Helper()

	r := directive.NewRegistry()
	for _, s := range directive.Kernel() {
		assert.NoError(tb, r.Register(s), "the kernel schemas register first")
	}
	for _, s := range schemas {
		assert.NoError(tb, r.Register(s), "the fixture schema registers")
	}
	assert.Empty(tb, r.Seal(), "the registry seals without faults")
	return r
}

func TestRegistry(t *testing.T) {
	t.Parallel()

	t.Run("Ignore", func(t *testing.T) {
		t.Parallel()

		t.Run("silences a full name and a plugin prefix", func(t *testing.T) {
			t.Parallel()

			r := directive.NewRegistry()
			assert.NoError(t, r.Ignore("deepcopy-gen"), "a full name ignores")
			assert.NoError(t, r.Ignore("k8s:"), "a plugin prefix ignores")
			assert.Empty(t, r.Seal(), "an ignore covering nothing registered seals")
			assert.True(t, r.Ignored("deepcopy-gen"), "the name is opted out")
			assert.True(t, r.Ignored("k8s:openapi-gen"), "every name under the prefix is")
			assert.False(t, r.Ignored("kubebuilder:validation"), "another plugin's is not")
			assert.False(t, r.Ignored("k8s"), "nor the bare prefix spelled without its colon")
		})

		t.Run("refuses what would silence a schema", func(t *testing.T) {
			t.Parallel()

			r := directive.NewRegistry()
			assert.NoError(t, r.Register(wellFormed("mockgen", "stub")), "the schema registers")
			err := r.Ignore("mockgen:stub")
			assert.HasError(t, err, "ignoring a registered name is refused at the call")
			assert.Contains(t, err.Error(), "mockgen:stub", "naming it")
			err = r.Ignore("stub")
			assert.HasError(t, err, "and so is its bare spelling while one plugin claims it")
			err = r.Ignore("mockgen:")
			assert.HasError(t, err, "and a prefix covering it")
			assert.Contains(t, err.Error(), "mockgen:stub", "naming the schema it would silence")
			assert.HasError(t, r.Ignore(directive.KernelSkip), "a kernel name is never ignored")
			assert.HasError(t, r.Ignore(""), "an empty spelling ignores nothing")
			assert.HasError(t, r.Ignore(":"), "and a bare colon is one")
		})

		t.Run("refuses at the seal when the schema registers second", func(t *testing.T) {
			t.Parallel()

			r := directive.NewRegistry()
			assert.NoError(t, r.Ignore("mockgen:"), "the prefix ignores while nothing is under it")
			assert.NoError(t, r.Register(wellFormed("mockgen", "stub")), "then the schema registers")
			faults := r.Seal()
			assert.Length(t, faults, 1, "the seal reports the silenced schema")
			assert.Contains(t, faults[0].Error(), "mockgen:stub", "naming it")
			assert.HasError(t, r.Ignore("late"), "and nothing ignores after the seal")
		})
	})

	t.Run("Register", func(t *testing.T) {
		t.Parallel()

		t.Run("refuses what the contract refuses", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name   string
				schema directive.Schema
				want   string
			}{
				{
					name:   "a kernel name claimed by a plugin",
					schema: wellFormed("mockgen", directive.KernelSkip),
					want:   "skip",
				},
				{
					name:   "an empty plugin outside the kernel names",
					schema: wellFormed("", "stub"),
					want:   "kernel",
				},
				{
					name: "a reserved key among the params",
					schema: directive.Schema{
						Plugin: "mockgen", Name: "stub", Doc: "claims a reserved key",
						Params: []directive.ParamSpec{
							{Key: directive.ReservedOut, Type: directive.TypeString, Doc: "stolen"},
						},
					},
					want: string(directive.ReservedOut),
				},
				{
					name: "a param key declared twice",
					schema: directive.Schema{
						Plugin: "mockgen", Name: "stub", Doc: "declares tag twice",
						Params: []directive.ParamSpec{
							{Key: "mode", Type: directive.TypeString, Doc: "the first"},
							{Key: "mode", Type: directive.TypeInt, Doc: "the second"},
						},
					},
					want: "mode",
				},
				{
					name: "a role declared twice",
					schema: directive.Schema{
						Plugin: "mockgen", Name: "stub", Doc: "declares client twice",
						Roles: []string{"client", "client"},
					},
					want: "client",
				},
				{
					name: "a positional param carrying roles",
					schema: directive.Schema{
						Plugin: "mockgen", Name: "stub", Doc: "scopes a positional",
						Roles: []string{"client"},
						Positional: []directive.ParamSpec{
							{
								Key: "target", Type: directive.TypeString,
								Roles: []string{"client"}, Doc: "shifty",
							},
						},
					},
					want: "target",
				},
				{
					name: "an undeclared role on a param",
					schema: directive.Schema{
						Plugin: "mockgen", Name: "stub", Doc: "scopes to a ghost",
						Roles: []string{"client"},
						Params: []directive.ParamSpec{
							{
								Key: "mode", Type: directive.TypeString,
								Roles: []string{"server"}, Doc: "scoped",
							},
						},
					},
					want: "server",
				},
				{
					name: "a role requirement without roles",
					schema: directive.Schema{
						Plugin: "mockgen", Name: "stub", Doc: "demands nothing",
						RolesRequired: true,
					},
					want: "role",
				},
				{
					name: "the role key claimed as a param",
					schema: directive.Schema{
						Plugin: "mockgen", Name: "stub", Doc: "claims role",
						Params: []directive.ParamSpec{
							{Key: "role", Type: directive.TypeString, Doc: "stolen"},
						},
					},
					want: "role",
				},
				{
					name: "a positional that fails its own checks",
					schema: directive.Schema{
						Plugin: "mockgen", Name: "stub", Doc: "an untyped positional",
						Positional: []directive.ParamSpec{
							{Key: "target", Doc: "untyped"},
						},
					},
					want: "target",
				},
				{
					name: "a list of lists",
					schema: directive.Schema{
						Plugin: "mockgen", Name: "stub", Doc: "nests",
						Params: []directive.ParamSpec{
							{
								Key: "matrix", Type: directive.TypeList,
								ListOf: directive.TypeList, Doc: "too deep",
							},
						},
					},
					want: "list",
				},
				{
					name: "a schema without documentation",
					schema: directive.Schema{
						Plugin: "mockgen", Name: "stub",
					},
					want: "stub",
				},
				{
					name: "a param without documentation",
					schema: directive.Schema{
						Plugin: "mockgen", Name: "stub", Doc: "documented itself",
						Params: []directive.ParamSpec{
							{Key: "mode", Type: directive.TypeString},
						},
					},
					want: "mode",
				},
				{
					name: "a param without a type",
					schema: directive.Schema{
						Plugin: "mockgen", Name: "stub", Doc: "forgot the type",
						Params: []directive.ParamSpec{
							{Key: "mode", Doc: "untyped"},
						},
					},
					want: "mode",
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					r := directive.NewRegistry()
					err := r.Register(tt.schema)
					assert.HasError(t, err, "the contract refuses it")
					assert.Contains(t, err.Error(), tt.want, "naming what broke it")
					assert.HasPrefix(t, err.Error(), "directive: ", "under the package prefix")
				})
			}
		})

		t.Run("refuses one plugin claiming one name twice", func(t *testing.T) {
			t.Parallel()

			r := directive.NewRegistry()
			first := wellFormed("mockgen", "stub")
			first.Doc = "the first claimant"
			assert.NoError(t, r.Register(first), "the first registration arrives")

			second := wellFormed("mockgen", "stub")
			second.Doc = "the second claimant"
			err := r.Register(second)
			assert.HasError(t, err, "one plugin claims one name once")
			assert.Contains(t, err.Error(), "the first claimant", "naming the holder")
			assert.Contains(t, err.Error(), "the second claimant", "and the claimant")
		})

		t.Run("admits two plugins claiming one bare name", func(t *testing.T) {
			t.Parallel()

			r := directive.NewRegistry()
			assert.NoError(t, r.Register(wellFormed("mockgen", "stub")),
				"the first plugin claims stub")
			assert.NoError(t, r.Register(wellFormed("stubgen", "stub")),
				"and the second may too: the bare spelling becomes ambiguous, not refused")
		})

		t.Run("refuses a registration after the seal", func(t *testing.T) {
			t.Parallel()

			r := sealed(t)
			err := r.Register(wellFormed("mockgen", "stub"))
			assert.HasError(t, err, "sealing ends registration")
		})
	})

	t.Run("Seal", func(t *testing.T) {
		t.Parallel()

		t.Run("resolves constraints against what registered", func(t *testing.T) {
			t.Parallel()

			needs := wellFormed("mockgen", "stub")
			needs.Requires = []directive.Name{"index"}
			needs.ConflictsWith = []directive.Name{directive.KernelSkip}
			sealed(t, needs, wellFormed("indexer", "index"))
		})

		t.Run("collects every unknown name", func(t *testing.T) {
			t.Parallel()

			needs := wellFormed("mockgen", "stub")
			needs.Requires = []directive.Name{"nonexistent"}
			needs.ConflictsWith = []directive.Name{"alsomissing"}

			r := directive.NewRegistry()
			assert.NoError(t, r.Register(needs), "the schema registers")
			faults := r.Seal()
			assert.Length(t, faults, 2, "every unresolved name is one fault")
			assert.Contains(t, faults[0].Error(), "nonexistent", "naming the ghost")
		})

		t.Run("refuses a self-reference", func(t *testing.T) {
			t.Parallel()

			needs := wellFormed("mockgen", "stub")
			needs.Requires = []directive.Name{"stub"}

			r := directive.NewRegistry()
			assert.NoError(t, r.Register(needs), "the schema registers")
			assert.Length(t, r.Seal(), 1, "a schema cannot require itself")
		})
	})

	t.Run("ResolveName", func(t *testing.T) {
		t.Parallel()

		t.Run("a prefixed spelling addresses its owner's schema", func(t *testing.T) {
			t.Parallel()

			r := sealed(t, wellFormed("mockgen", "stub"), wellFormed("stubgen", "stub"))
			s, held := r.ResolveName("mockgen:stub")
			assert.True(t, held, "the prefixed spelling resolves")
			assert.Equal(t, s.Plugin, "mockgen", "to its owner")
		})

		t.Run("a unique bare spelling addresses the one claimant", func(t *testing.T) {
			t.Parallel()

			r := sealed(t, wellFormed("mockgen", "stub"))
			s, held := r.ResolveName("stub")
			assert.True(t, held, "an unambiguous bare name resolves")
			assert.Equal(t, s.Plugin, "mockgen", "to its claimant")
		})

		t.Run("an ambiguous bare spelling refuses and names candidates", func(t *testing.T) {
			t.Parallel()

			r := sealed(t, wellFormed("mockgen", "stub"), wellFormed("stubgen", "stub"))
			_, held := r.ResolveName("stub")
			assert.False(t, held, "two claimants make the bare spelling ambiguous")
			assert.Equal(t, r.Candidates("stub"),
				[]directive.Name{"mockgen:stub", "stubgen:stub"},
				"and the candidates are the prefixed spellings, in order")
		})

		t.Run("a kernel name resolves bare", func(t *testing.T) {
			t.Parallel()

			r := sealed(t)
			s, held := r.ResolveName(directive.KernelMeta)
			assert.True(t, held, "the kernel's bare spelling resolves")
			assert.Equal(t, s.Name, directive.KernelMeta, "to the kernel schema")
		})

		t.Run("a spelling nothing registered refuses", func(t *testing.T) {
			t.Parallel()

			r := sealed(t)
			_, held := r.ResolveName("nonexistent")
			assert.False(t, held, "an unclaimed spelling resolves to nothing")
			assert.Empty(t, r.Candidates("nonexistent"), "with no candidates to name")
		})
	})
	t.Run("Open", func(t *testing.T) {
		t.Parallel()

		open := func(spec directive.ParamSpec) directive.Schema {
			s := wellFormed("witnessy", "bind")
			s.Open = &spec
			return s
		}

		t.Run("admits a spec naming no key", func(t *testing.T) {
			t.Parallel()

			r := directive.NewRegistry()
			assert.NoError(t, r.Register(open(directive.ParamSpec{
				Type: directive.TypeReference, Resolution: directive.ResolveTypeInScope, Doc: "a witness",
			})), "the instance's own spelling is the key")
		})

		t.Run("refuses a spec naming a key", func(t *testing.T) {
			t.Parallel()

			r := directive.NewRegistry()
			err := r.Register(open(directive.ParamSpec{Key: "T", Type: directive.TypeInt, Doc: "a bound"}))
			assert.HasError(t, err, "an open spec's key is the instance's")
			assert.Contains(t, err.Error(), "names no key", "and the refusal says so")
		})

		t.Run("holds the spec to a param's own checks", func(t *testing.T) {
			t.Parallel()

			r := directive.NewRegistry()
			assert.HasError(t, r.Register(open(directive.ParamSpec{Type: directive.TypeInt})),
				"no doc refuses like any param")
			assert.HasError(t, r.Register(open(directive.ParamSpec{Doc: "untyped"})),
				"no type refuses like any param")
			assert.HasError(t, r.Register(open(directive.ParamSpec{
				Type: directive.TypeInt, Roles: []string{"ghost"}, Doc: "scoped",
			})), "an undeclared role refuses like any param")
		})
	})
}

// Resolution runs once per instance at validation and once per
// spelling at dispatch, so its cost is paid per directive in the
// workspace.
func BenchmarkRegistry(b *testing.B) {
	b.Run("ResolveName", func(b *testing.B) {
		b.ReportAllocs()

		r := sealed(b, wellFormed("mockgen", "stub"))
		for b.Loop() {
			if _, held := r.ResolveName("stub"); !held {
				b.Fatal("a unique bare spelling did not resolve")
			}
		}
	})
}
