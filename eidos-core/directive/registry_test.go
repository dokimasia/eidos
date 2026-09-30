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

// sealed returns a sealed registry containing the kernel schemas
// and the given ones, failing the test on a registration fault.
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

// ignoring returns a sealed registry that ignores one full name and
// one plugin prefix.
func ignoring(tb assert.TB) *directive.Registry {
	tb.Helper()

	r := directive.NewRegistry()
	assert.NoError(tb, r.Ignore("deepcopy-gen"), "the full name is ignored")
	assert.NoError(tb, r.Ignore("k8s:"), "the plugin prefix is ignored")
	assert.Empty(tb, r.Seal(), "the registry seals without faults")
	return r
}

// negatableKernel returns the kernel's meta schema declared
// negatable.
func negatableKernel() directive.Schema {
	s := directive.Kernel()[0]
	s.Negatable = true
	return s
}

// open returns a well-formed schema that types undeclared keys by
// spec.
func open(spec directive.ParamSpec) directive.Schema {
	s := wellFormed("witnessy", "bind")
	s.Open = &spec
	return s
}

func TestRegistry(t *testing.T) {
	t.Parallel()

	t.Run("Ignore", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error naming a registered name", func(t *testing.T) {
			t.Parallel()

			r := directive.NewRegistry()
			assert.NoError(t, r.Register(wellFormed("mockgen", "stub")), "the schema registers")
			err := r.Ignore("mockgen:stub")
			assert.HasError(t, err, "the ignore fails")
			assert.Contains(t, err.Error(), "mockgen:stub", "the error names the schema")
		})

		t.Run("returns an error naming the schema a prefix covers", func(t *testing.T) {
			t.Parallel()

			r := directive.NewRegistry()
			assert.NoError(t, r.Register(wellFormed("mockgen", "stub")), "the schema registers")
			err := r.Ignore("mockgen:")
			assert.HasError(t, err, "the ignore fails")
			assert.Contains(t, err.Error(), "mockgen:stub", "the error names the schema")
		})

		t.Run("returns an error for the bare spelling of a registered name", func(t *testing.T) {
			t.Parallel()

			r := directive.NewRegistry()
			assert.NoError(t, r.Register(wellFormed("mockgen", "stub")), "the schema registers")
			assert.HasError(t, r.Ignore("stub"), "the ignore fails")
		})

		t.Run("returns an error naming a bare name two plugins claim", func(t *testing.T) {
			t.Parallel()

			r := directive.NewRegistry()
			assert.NoError(t, r.Register(wellFormed("mockgen", "stub")), "the first claimant registers")
			assert.NoError(t, r.Register(wellFormed("fakegen", "stub")), "the second claimant registers")
			err := r.Ignore("stub")
			assert.HasError(t, err, "the ignore fails")
			assert.Contains(t, err.Error(), "stub", "the error names the spelling")
		})

		tests := []struct {
			name string
			give directive.Name
		}{
			{name: "returns an error for a kernel name", give: directive.KernelSkip},
			{name: "returns an error for an empty spelling", give: ""},
			{name: "returns an error for a bare colon", give: ":"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.HasError(t, directive.NewRegistry().Ignore(tt.give), "the ignore fails")
			})
		}

		t.Run("returns an error after the seal", func(t *testing.T) {
			t.Parallel()

			assert.HasError(t, ignoring(t).Ignore("late"), "the ignore fails")
		})
	})

	t.Run("Ignored", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give directive.Name
			want bool
		}{
			{name: "reports true for an ignored full name", give: "deepcopy-gen", want: true},
			{name: "reports true for a name under an ignored prefix", give: "k8s:openapi-gen", want: true},
			{name: "reports false for another plugin's name", give: "kubebuilder:validation", want: false},
			{name: "reports false for the prefix without its colon", give: "k8s", want: false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, ignoring(t).Ignored(tt.give), tt.want, "the opt-out is pinned")
			})
		}
	})

	t.Run("Register", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name   string
			schema directive.Schema
			want   string
		}{
			{
				name:   "returns an error for a kernel name claimed by a plugin",
				schema: wellFormed("mockgen", directive.KernelSkip),
				want:   "skip",
			},
			{
				name:   "returns an error for an empty plugin outside the kernel names",
				schema: wellFormed("", "stub"),
				want:   "kernel",
			},
			{
				name:   "returns an error for a negatable kernel schema",
				schema: negatableKernel(),
				want:   "negatable",
			},
			{
				name: "returns an error for a reserved key among the params",
				schema: directive.Schema{
					Plugin: "mockgen", Name: "stub", Doc: "claims a reserved key",
					Params: []directive.ParamSpec{
						{Key: directive.ReservedOut, Type: directive.TypeString, Doc: "stolen"},
					},
				},
				want: string(directive.ReservedOut),
			},
			{
				name: "returns an error for a param key declared twice",
				schema: directive.Schema{
					Plugin: "mockgen", Name: "stub", Doc: "declares mode twice",
					Params: []directive.ParamSpec{
						{Key: "mode", Type: directive.TypeString, Doc: "the first"},
						{Key: "mode", Type: directive.TypeInt, Doc: "the second"},
					},
				},
				want: "mode",
			},
			{
				name: "returns an error for a role declared twice",
				schema: directive.Schema{
					Plugin: "mockgen", Name: "stub", Doc: "declares client twice",
					Roles: []string{"client", "client"},
				},
				want: "client",
			},
			{
				name: "returns an error for a positional param with roles",
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
				name: "returns an error for an undeclared role on a param",
				schema: directive.Schema{
					Plugin: "mockgen", Name: "stub", Doc: "scopes to a role it does not declare",
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
				name: "returns an error for a role requirement without roles",
				schema: directive.Schema{
					Plugin: "mockgen", Name: "stub", Doc: "demands nothing",
					RolesRequired: true,
				},
				want: "role",
			},
			{
				name: "returns an error for the role key declared as a param",
				schema: directive.Schema{
					Plugin: "mockgen", Name: "stub", Doc: "claims role",
					Params: []directive.ParamSpec{
						{Key: "role", Type: directive.TypeString, Doc: "stolen"},
					},
				},
				want: "role",
			},
			{
				name: "returns an error for an untyped positional param",
				schema: directive.Schema{
					Plugin: "mockgen", Name: "stub", Doc: "an untyped positional",
					Positional: []directive.ParamSpec{
						{Key: "target", Doc: "untyped"},
					},
				},
				want: "target",
			},
			{
				name: "returns an error for a list of lists",
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
				name: "returns an error for a schema without documentation",
				schema: directive.Schema{
					Plugin: "mockgen", Name: "stub",
				},
				want: "stub",
			},
			{
				name: "returns an error for a param without documentation",
				schema: directive.Schema{
					Plugin: "mockgen", Name: "stub", Doc: "documented itself",
					Params: []directive.ParamSpec{
						{Key: "mode", Type: directive.TypeString},
					},
				},
				want: "mode",
			},
			{
				name: "returns an error for a param without a type",
				schema: directive.Schema{
					Plugin: "mockgen", Name: "stub", Doc: "forgot the type",
					Params: []directive.ParamSpec{
						{Key: "mode", Doc: "untyped"},
					},
				},
				want: "mode",
			},
			{
				name: "returns an error for a list param without an element type",
				schema: directive.Schema{
					Plugin: "listgen", Name: "collect", Doc: "collects the named members",
					Params: []directive.ParamSpec{
						{Key: "members", Type: directive.TypeList, Doc: "the collected members"},
					},
				},
				want: "members",
			},
			{
				name: "returns an error for a reference param without a resolution kind",
				schema: directive.Schema{
					Plugin: "mockgen", Name: "stub", Doc: "names a target",
					Params: []directive.ParamSpec{
						{Key: "target", Type: directive.TypeReference, Doc: "the stubbed target"},
					},
				},
				want: "target",
			},
			{
				name: "returns an error for a list of references without a resolution kind",
				schema: directive.Schema{
					Plugin: "mockgen", Name: "stub", Doc: "names targets",
					Params: []directive.ParamSpec{
						{
							Key: "targets", Type: directive.TypeList,
							ListOf: directive.TypeReference, Doc: "the stubbed targets",
						},
					},
				},
				want: "targets",
			},
			{
				name:   "returns an error for a name no carrier can spell",
				schema: wellFormed("mockgen", "st.ub"),
				want:   "st.ub",
			},
			{
				name:   "returns an error for a plugin prefix no carrier can spell",
				schema: wellFormed("gen.sample", "stub"),
				want:   "gen.sample",
			},
			{
				name: "returns an error for a param key no carrier can spell",
				schema: directive.Schema{
					Plugin: "mockgen", Name: "stub", Doc: "names a dotted key",
					Params: []directive.ParamSpec{
						{Key: "max.len", Type: directive.TypeInt, Doc: "the bound"},
					},
				},
				want: "max.len",
			},
			{
				name:   "returns an error for an open spec that names a key",
				schema: open(directive.ParamSpec{Key: "T", Type: directive.TypeInt, Doc: "a bound"}),
				want:   "names no key",
			},
			{
				name:   "returns an error for an open spec without documentation",
				schema: open(directive.ParamSpec{Type: directive.TypeInt}),
				want:   "semantics",
			},
			{
				name:   "returns an error for an untyped open spec",
				schema: open(directive.ParamSpec{Doc: "untyped"}),
				want:   "type",
			},
			{
				name: "returns an error for an open spec scoped to an undeclared role",
				schema: open(directive.ParamSpec{
					Type: directive.TypeInt, Roles: []string{"ghost"}, Doc: "scoped",
				}),
				want: "ghost",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				err := directive.NewRegistry().Register(tt.schema)
				assert.HasError(t, err, "the registration fails")
				assert.Contains(t, err.Error(), tt.want, "the error names what broke the contract")
				assert.HasPrefix(t, err.Error(), "directive: ", "the error has the package prefix")
			})
		}

		t.Run("returns an error naming both docs for a name one plugin registers twice", func(t *testing.T) {
			t.Parallel()

			r := directive.NewRegistry()
			first := wellFormed("mockgen", "stub")
			first.Doc = "the first claimant"
			assert.NoError(t, r.Register(first), "the first registration succeeds")

			second := wellFormed("mockgen", "stub")
			second.Doc = "the second claimant"
			err := r.Register(second)
			assert.HasError(t, err, "the second registration fails")
			assert.Contains(t, err.Error(), "the first claimant", "the error names the first doc")
			assert.Contains(t, err.Error(), "the second claimant", "the error names the second doc")
		})

		t.Run("registers one bare name for two plugins", func(t *testing.T) {
			t.Parallel()

			r := directive.NewRegistry()
			assert.NoError(t, r.Register(wellFormed("mockgen", "stub")), "the first plugin registers stub")
			assert.NoError(t, r.Register(wellFormed("stubgen", "stub")), "the second plugin registers stub")
		})

		t.Run("registers a negatable plugin schema", func(t *testing.T) {
			t.Parallel()

			negatable := wellFormed("mockgen", "stub")
			negatable.Negatable = true
			assert.NoError(t, directive.NewRegistry().Register(negatable), "the schema registers")
		})

		t.Run("registers an open spec that names no key", func(t *testing.T) {
			t.Parallel()

			assert.NoError(t, directive.NewRegistry().Register(open(directive.ParamSpec{
				Type: directive.TypeReference, Resolution: directive.ResolveTypeInScope, Doc: "a witness",
			})), "the schema registers")
		})

		t.Run("returns an error after the seal", func(t *testing.T) {
			t.Parallel()

			assert.HasError(t, sealed(t).Register(wellFormed("mockgen", "stub")), "the registration fails")
		})
	})

	t.Run("Seal", func(t *testing.T) {
		t.Parallel()

		t.Run("returns no fault for constraints that resolve", func(t *testing.T) {
			t.Parallel()

			needs := wellFormed("mockgen", "stub")
			needs.Requires = []directive.Name{"index"}
			needs.ConflictsWith = []directive.Name{directive.KernelSkip}
			sealed(t, needs, wellFormed("indexer", "index"))
		})

		t.Run("returns one fault per unknown name", func(t *testing.T) {
			t.Parallel()

			needs := wellFormed("mockgen", "stub")
			needs.Requires = []directive.Name{"nonexistent"}
			needs.ConflictsWith = []directive.Name{"alsomissing"}

			r := directive.NewRegistry()
			assert.NoError(t, r.Register(needs), "the schema registers")
			faults := r.Seal()
			assert.Length(t, faults, 2, "each unknown name is one fault")
			assert.Contains(t, faults[0].Error(), "nonexistent", "the fault names the unknown name")
		})

		t.Run("returns a fault for a self-reference", func(t *testing.T) {
			t.Parallel()

			needs := wellFormed("mockgen", "stub")
			needs.Requires = []directive.Name{"stub"}

			r := directive.NewRegistry()
			assert.NoError(t, r.Register(needs), "the schema registers")
			assert.Length(t, r.Seal(), 1, "the self-reference is one fault")
		})

		t.Run("returns a fault naming a schema an earlier ignore covers", func(t *testing.T) {
			t.Parallel()

			r := directive.NewRegistry()
			assert.NoError(t, r.Ignore("mockgen:"), "the prefix is ignored while nothing is under it")
			assert.NoError(t, r.Register(wellFormed("mockgen", "stub")), "the schema registers")
			faults := r.Seal()
			assert.Length(t, faults, 1, "the covered schema is one fault")
			assert.Contains(t, faults[0].Error(), "mockgen:stub", "the fault names the schema")
		})
	})

	t.Run("ResolveName", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the schema a prefixed spelling names", func(t *testing.T) {
			t.Parallel()

			r := sealed(t, wellFormed("mockgen", "stub"), wellFormed("stubgen", "stub"))
			s, held := r.ResolveName("mockgen:stub")
			assert.True(t, held, "the spelling resolves")
			assert.Equal(t, s.Plugin, "mockgen", "the schema is the named plugin's")
		})

		t.Run("returns the one claimant of a bare spelling", func(t *testing.T) {
			t.Parallel()

			r := sealed(t, wellFormed("mockgen", "stub"))
			s, held := r.ResolveName("stub")
			assert.True(t, held, "the spelling resolves")
			assert.Equal(t, s.Plugin, "mockgen", "the schema is the claimant's")
		})

		t.Run("returns the kernel schema for a kernel name", func(t *testing.T) {
			t.Parallel()

			s, held := sealed(t).ResolveName(directive.KernelMeta)
			assert.True(t, held, "the spelling resolves")
			assert.Equal(t, s.Name, directive.KernelMeta, "the schema is the kernel's")
		})

		t.Run("reports false for a bare spelling two plugins claim", func(t *testing.T) {
			t.Parallel()

			r := sealed(t, wellFormed("mockgen", "stub"), wellFormed("stubgen", "stub"))
			_, held := r.ResolveName("stub")
			assert.False(t, held, "the spelling is ambiguous")
		})

		t.Run("reports false for a spelling nothing registered", func(t *testing.T) {
			t.Parallel()

			_, held := sealed(t).ResolveName("nonexistent")
			assert.False(t, held, "the spelling resolves to nothing")
		})
	})

	t.Run("Candidates", func(t *testing.T) {
		t.Parallel()

		t.Run("returns every claimant's prefixed spelling in registration order", func(t *testing.T) {
			t.Parallel()

			r := sealed(t, wellFormed("mockgen", "stub"), wellFormed("stubgen", "stub"))
			assert.Equal(t, r.Candidates("stub"),
				[]directive.Name{"mockgen:stub", "stubgen:stub"},
				"the candidates are in registration order")
		})

		t.Run("returns nothing for a spelling nothing registered", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, sealed(t).Candidates("nonexistent"), "there is no candidate")
		})
	})
}

// Resolution runs once per instance at validation and once per
// spelling at dispatch, so its cost scales with the directives in
// the workspace.
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
