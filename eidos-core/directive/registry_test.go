// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package directive_test

import (
	"cmp"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/directive"
)

// The foreign tool the ignore cases opt out of: its plugin prefix, and
// one directive under it.
const (
	foreignPrefix directive.Name = "k8s:"
	foreignName   directive.Name = "k8s:deepcopy-gen"
)

// targetName is the spelling of the target whose directive the target
// cases register.
const targetName directive.Name = "typescript"

// The ceilings of the registry's construction and of its writes.
const (
	// newRegistryAllocs is one empty registry: the registry and its four
	// maps.
	newRegistryAllocs = 5
	// registerAllocs is one registration of a plugin's schema into an
	// empty registry: the schema's canonical spelling; the schema map's
	// first group, and the schema the map stores apart from the group
	// because a Schema is larger than 128 bytes; and the claimant map's
	// first group and the claimant list.
	registerAllocs = 5
	// registerTargetAllocs is one registration of a target's schema into
	// an empty registry: the set of targets and its first group; the
	// schema map's first group and the schema it stores apart; and the
	// claimant map's first group and the claimant list. A target's
	// canonical spelling is its name, so it does not allocate a spelling.
	registerTargetAllocs = 6
	// sealAllocs is one seal of the kernel's six schemas: the sorted
	// canonical spellings, and the first group of the constraint table.
	sealAllocs = 2
	// ignorePrefixAllocs is one ignore of a plugin prefix: the sorted
	// canonical spellings the prefix is checked against.
	ignorePrefixAllocs = 1
)

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
				name:   "returns an error for an override kernel schema",
				schema: directive.Schema{Name: directive.KernelMeta, Overrides: true, Doc: "overrides metadata"},
				want:   "override schema",
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
			{
				name: "returns an error for roles beside variants",
				schema: directive.Schema{
					Plugin: "shaper", Name: "shape", Doc: "classifies a callable",
					Roles:    []string{"client"},
					Variants: []directive.Variant{{Name: "writer", Doc: "writes a value"}},
				},
				want: "variants",
			},
			{
				name:   "returns an error for a variant name no carrier can spell",
				schema: varied(directive.Variant{Name: "wri.ter", Doc: "writes a value"}),
				want:   "wri.ter",
			},
			{
				name: "returns an error for a variant declared twice",
				schema: varied(
					directive.Variant{Name: "writer", Doc: "writes a value"},
					directive.Variant{Name: "writer", Doc: "writes another value"},
				),
				want: "writer",
			},
			{
				name:   "returns an error for a variant without documentation",
				schema: varied(directive.Variant{Name: "writer"}),
				want:   "writer",
			},
			{
				name:   "returns an error for a variant role requirement without roles",
				schema: varied(directive.Variant{Name: "tx", RolesRequired: true, Doc: "a transaction"}),
				want:   "tx",
			},
			{
				name: "returns an error for a variant role declared twice",
				schema: varied(directive.Variant{
					Name: "tx", Roles: []string{"begin", "begin"}, Doc: "a transaction",
				}),
				want: "begin",
			},
			{
				name: "returns an error for a variant param key no carrier can spell",
				schema: varied(directive.Variant{
					Name: "writer", Doc: "writes a value",
					Params: []directive.ParamSpec{{Key: "re.ads", Type: directive.TypeString, Doc: "the read fields"}},
				}),
				want: "re.ads",
			},
			{
				name: "returns an error for a variant param with the key of a schema param",
				schema: varied(directive.Variant{
					Name: "writer", Doc: "writes a value",
					Params: []directive.ParamSpec{{Key: "mode", Type: directive.TypeString, Doc: "the write mode"}},
				}),
				want: "mode",
			},
			{
				name: "returns an error for a variant param without documentation",
				schema: varied(directive.Variant{
					Name: "writer", Doc: "writes a value",
					Params: []directive.ParamSpec{{Key: "reads", Type: directive.TypeString}},
				}),
				want: "reads",
			},
			{
				name: "returns an error for a variant param scoped to a role that the variant does not declare",
				schema: varied(directive.Variant{
					Name: "tx", Roles: []string{"begin"}, Doc: "a transaction",
					Params: []directive.ParamSpec{
						{Key: "until", Type: directive.TypeString, Roles: []string{"commit"}, Doc: "the deadline"},
					},
				}),
				want: "commit",
			},
			{
				name: "returns an error for choices on a param that is not a string",
				schema: choosing(directive.ParamSpec{
					Key: "depth", Type: directive.TypeInt, Choices: []string{"1", "2"}, Doc: "the depth",
				}),
				want: "only a string param takes them",
			},
			{
				name: "returns an error for an empty choice",
				schema: choosing(directive.ParamSpec{
					Key: "int64", Type: directive.TypeString, Choices: []string{"bigint", ""}, Doc: "the spelling",
				}),
				want: "an empty choice",
			},
			{
				name: "returns an error naming a choice declared twice",
				schema: choosing(directive.ParamSpec{
					Key: "int64", Type: directive.TypeString, Doc: "the spelling",
					Choices: []string{"bigint", "bigint"},
				}),
				want: `choice "bigint" twice`,
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

		t.Run("registers an override plugin schema", func(t *testing.T) {
			t.Parallel()

			overriding := wellFormed("mockgen", "stub")
			overriding.Overrides = true
			assert.NoError(t, directive.NewRegistry().Register(overriding), "the schema registers")
		})

		t.Run("registers an open spec that names no key", func(t *testing.T) {
			t.Parallel()

			assert.NoError(t, directive.NewRegistry().Register(open(directive.ParamSpec{
				Type: directive.TypeReference, Resolution: directive.ResolveTypeInScope, Doc: "a witness",
			})), "the schema registers")
		})

		t.Run("registers one param key in two variants", func(t *testing.T) {
			t.Parallel()

			reads := []directive.ParamSpec{{Key: "reads", Type: directive.TypeString, Doc: "the read fields"}}
			assert.NoError(t, directive.NewRegistry().Register(varied(
				directive.Variant{Name: "writer", Params: reads, Doc: "writes a value"},
				directive.Variant{Name: "deleter", Params: reads, Doc: "deletes a value"},
			)), "each variant has keys of its own")
		})

		t.Run("registers a string param with choices", func(t *testing.T) {
			t.Parallel()

			assert.NoError(t, directive.NewRegistry().Register(choosing(directive.ParamSpec{
				Key: "int64", Type: directive.TypeString, Choices: []string{"bigint", "string"}, Doc: "the spelling",
			})), "the schema registers")
		})

		t.Run("returns an error for a plugin's schema of a target's name", func(t *testing.T) {
			t.Parallel()

			r := directive.NewRegistry()
			assert.NoError(t, r.RegisterTarget(target(targetName)), "the target's directive registers")
			err := r.Register(wellFormed("mockgen", targetName))
			assert.HasError(t, err, "the plugin's schema fails")
			assert.Contains(t, err.Error(), "kernel name", "the error is about a kernel name")
		})

		t.Run("returns an error after the seal", func(t *testing.T) {
			t.Parallel()

			assert.HasError(t, sealed(t).Register(wellFormed("mockgen", "stub")), "the registration fails")
		})
	})

	t.Run("RegisterTarget", func(t *testing.T) {
		t.Parallel()

		t.Run("registers a schema that the target's bare name addresses", func(t *testing.T) {
			t.Parallel()

			r := directive.NewRegistry()
			assert.NoError(t, r.RegisterTarget(target(targetName)), "the target's directive registers")
			s, held := r.ResolveName(targetName)
			assert.True(t, held, "the bare name resolves")
			expect.Equal(t, s.Canonical(), targetName, "the canonical spelling is the bare name")
			expect.Equal(t, s.Plugin, "", "the schema has no plugin")
		})

		t.Run("returns an error for an ignore of a target's name", func(t *testing.T) {
			t.Parallel()

			r := directive.NewRegistry()
			assert.NoError(t, r.RegisterTarget(target(targetName)), "the target's directive registers")
			err := r.Ignore(targetName)
			assert.HasError(t, err, "the ignore fails")
			assert.Contains(t, err.Error(), "kernel name", "the error is about a kernel name")
		})

		tests := []struct {
			name    string
			prepare func(r *directive.Registry) error
			schema  directive.Schema
			want    string
		}{
			{
				name:   "returns an error for a schema with a plugin",
				schema: wellFormed("typescript", targetName),
				want:   `plugin "typescript"`,
			},
			{
				name:    "returns an error for a target registered twice",
				prepare: func(r *directive.Registry) error { return r.RegisterTarget(target(targetName)) },
				schema:  target(targetName),
				want:    "registered twice",
			},
			{
				name:   "returns an error for one of the kernel's names",
				schema: target(directive.KernelSkip),
				want:   "kernel name",
			},
			{
				name:    "returns an error with the plugin's schema that claims the name",
				prepare: func(r *directive.Registry) error { return r.Register(wellFormed("mockgen", targetName)) },
				schema:  target(targetName),
				want:    "mockgen:" + string(targetName),
			},
			{
				name: "returns an error for a negatable schema",
				schema: func() directive.Schema {
					s := target(targetName)
					s.Negatable = true
					return s
				}(),
				want: "negatable",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				r := directive.NewRegistry()
				if tt.prepare != nil {
					assert.NoError(t, tt.prepare(r), "the registry is prepared")
				}
				err := r.RegisterTarget(tt.schema)
				assert.HasError(t, err, "the registration fails")
				assert.Contains(t, err.Error(), tt.want, "the error is about the fault")
				assert.HasPrefix(t, err.Error(), "directive: ", "the error has the package prefix")
			})
		}

		t.Run("leaves the name free after a refused schema", func(t *testing.T) {
			t.Parallel()

			r := directive.NewRegistry()
			refused := target(targetName)
			refused.Negatable = true
			assert.HasError(t, r.RegisterTarget(refused), "the negatable schema fails")
			assert.NoError(t, r.Ignore(targetName), "the name is not a kernel name")
		})

		t.Run("returns an error after the seal", func(t *testing.T) {
			t.Parallel()

			assert.HasError(t, sealed(t).RegisterTarget(target(targetName)), "the registration fails")
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

// The registry's construction, its writes and the claimant list
// allocate within their ceilings, and its reads allocate nothing, in
// the ordinary run, which runs no benchmark. A registration and a seal
// each take a registry built outside the count, because each changes the
// registry it writes. Each count keeps the first error of its calls,
// which cmp.Or returns without allocating. The check runs alone, because
// the count includes every goroutine's allocations.
func TestRegistryAllocs(t *testing.T) {
	var fresh *directive.Registry
	assert.MaxAllocs(t, func() { fresh = directive.NewRegistry() }, newRegistryAllocs,
		"NewRegistry allocates the registry and its maps")
	assert.False(t, fresh.Sealed(), "NewRegistry returns an open registry")

	stub := wellFormed("mockgen", "stub")
	var err error
	assert.MaxAllocsWithSetup(t, directive.NewRegistry,
		func(r *directive.Registry) { err = cmp.Or(err, r.Register(stub)) },
		registerAllocs, "Register allocates the schema's entries")
	assert.NoError(t, err, "every registration is admitted")

	typescript := target(targetName)
	assert.MaxAllocsWithSetup(t, directive.NewRegistry,
		func(r *directive.Registry) { err = cmp.Or(err, r.RegisterTarget(typescript)) },
		registerTargetAllocs, "RegisterTarget allocates the set of targets and the schema's entries")
	assert.NoError(t, err, "every target's registration is admitted")

	var faults []error
	assert.MaxAllocsWithSetup(t, func() *directive.Registry { return openRegistry(t, directive.Kernel()...) },
		func(r *directive.Registry) { faults = append(faults, r.Seal()...) },
		sealAllocs, "Seal allocates the sorted spellings and the constraint table")
	assert.Empty(t, faults, "every seal of the kernel's directives is clean")

	ignoring := openRegistry(t, directive.Kernel()...)
	assert.MaxAllocs(t, func() { err = cmp.Or(err, ignoring.Ignore(foreignPrefix)) }, ignorePrefixAllocs,
		"Ignore of a prefix allocates the sorted spellings it checks")
	assert.NoError(t, err, "the prefix is ignored")
	var ignored bool
	assert.MaxAllocs(t, func() { ignored = ignoring.Ignored(foreignName) }, 0, "Ignored allocates nothing")
	assert.True(t, ignored, "Ignored reports a name under the ignored prefix")

	r := sealed(t, wellFormed("mockgen", "stub"), wellFormed("stubgen", "stub"))
	var isSealed bool
	assert.MaxAllocs(t, func() { isSealed = r.Sealed() }, 0, "Sealed allocates nothing")
	assert.True(t, isSealed, "Sealed reports the seal")
	var resolved bool
	assert.MaxAllocs(t, func() { _, resolved = r.ResolveName("mockgen:stub") }, 0,
		"ResolveName allocates nothing")
	assert.True(t, resolved, "a prefixed spelling resolves")
	var candidates []directive.Name
	assert.MaxAllocs(t, func() { candidates = r.Candidates("stub") }, 1, "Candidates allocates the list it returns")
	assert.Equal(t, candidates, []directive.Name{"mockgen:stub", "stubgen:stub"},
		"Candidates returns both claimants in registration order")
}

// BenchmarkRegistry measures the registry's construction, the writes a
// composition makes, and resolution, which runs once per instance at
// validation and once per spelling at dispatch, so its cost scales with
// the directives in the workspace.
func BenchmarkRegistry(b *testing.B) {
	b.Run("NewRegistry", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(newRegistryAllocs)
		defer c.End()
		var got *directive.Registry
		for c.Loop() {
			got = directive.NewRegistry()
		}
		assert.False(b, got.Sealed(), "NewRegistry returns an open registry")
	})

	b.Run("Ignore", func(b *testing.B) {
		b.Run("a plugin prefix", func(b *testing.B) {
			r := openRegistry(b, directive.Kernel()...)
			c := bench.Start(b).Warmup(1).MaxAllocs(ignorePrefixAllocs)
			defer c.End()
			var err error
			for c.Loop() {
				err = r.Ignore(foreignPrefix)
			}
			assert.NoError(b, err, "a prefix no schema claims is ignored")
		})
	})

	b.Run("Ignored", func(b *testing.B) {
		r := openRegistry(b, directive.Kernel()...)
		assert.NoError(b, r.Ignore(foreignPrefix), "the prefix is ignored")
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got bool
		for c.Loop() {
			got = r.Ignored(foreignName)
		}
		assert.True(b, got, "a name under the ignored prefix is ignored")
	})

	b.Run("Register", func(b *testing.B) {
		b.Run("a schema into a new registry", func(b *testing.B) {
			stub := wellFormed("mockgen", "stub")
			c := bench.Start(b).MaxAllocs(registerAllocs)
			defer c.End()
			var (
				r   *directive.Registry
				err error
			)
			for c.Loop() {
				c.Excluding(func() { r = directive.NewRegistry() })
				err = r.Register(stub)
			}
			assert.NoError(b, err, "the schema registers")
		})
	})

	b.Run("RegisterTarget", func(b *testing.B) {
		b.Run("a target's schema into a new registry", func(b *testing.B) {
			typescript := target(targetName)
			c := bench.Start(b).MaxAllocs(registerTargetAllocs)
			defer c.End()
			var (
				r   *directive.Registry
				err error
			)
			for c.Loop() {
				c.Excluding(func() { r = directive.NewRegistry() })
				err = r.RegisterTarget(typescript)
			}
			assert.NoError(b, err, "the target's schema registers")
		})
	})

	b.Run("Seal", func(b *testing.B) {
		b.Run("the kernel's six schemas", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(sealAllocs)
			defer c.End()
			var (
				r      *directive.Registry
				faults []error
			)
			for c.Loop() {
				c.Excluding(func() { r = openRegistry(b, directive.Kernel()...) })
				faults = r.Seal()
			}
			assert.Empty(b, faults, "the kernel's schemas seal without a fault")
		})
	})

	b.Run("Sealed", func(b *testing.B) {
		r := sealed(b)
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got bool
		for c.Loop() {
			got = r.Sealed()
		}
		assert.True(b, got, "the registry is sealed")
	})

	b.Run("ResolveName", func(b *testing.B) {
		r := sealed(b, wellFormed("mockgen", "stub"))
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var (
			got  directive.Schema
			held bool
		)
		for c.Loop() {
			got, held = r.ResolveName("stub")
		}
		assert.True(b, held, "a unique bare spelling resolves")
		assert.Equal(b, got.Plugin, "mockgen", "to its one claimant")
	})

	b.Run("Candidates", func(b *testing.B) {
		b.Run("two claimants", func(b *testing.B) {
			r := sealed(b, wellFormed("mockgen", "stub"), wellFormed("stubgen", "stub"))
			c := bench.Start(b).MaxAllocs(1)
			defer c.End()
			var got []directive.Name
			for c.Loop() {
				got = r.Candidates("stub")
			}
			assert.Equal(
				b,
				got,
				[]directive.Name{"mockgen:stub", "stubgen:stub"},
				"both claimants in registration order",
			)
		})
	})
}

// openRegistry returns an open registry with the given schemas
// registered.
func openRegistry(tb assert.TB, schemas ...directive.Schema) *directive.Registry {
	tb.Helper()

	r := directive.NewRegistry()
	assert.Total(tb, r.Register, schemas, "the schema registers")
	return r
}

// target returns the directive of a target named name: a schema without
// a plugin whose name param and closed int64 param pass registration.
func target(name directive.Name) directive.Schema {
	return directive.Schema{
		Name: name,
		Params: []directive.ParamSpec{
			{Key: "name", Type: directive.TypeString, Doc: "the declaration's name in the target"},
			{
				Key: "int64", Type: directive.TypeString, Choices: []string{"bigint", "string", "number"},
				Doc: "the spelling of a 64-bit integer",
			},
		},
		Doc: "overrides how the target spells the declaration",
	}
}

// choosing returns a plugin's schema with spec as its one keyed param.
func choosing(spec directive.ParamSpec) directive.Schema {
	return directive.Schema{Plugin: "speller", Name: "spell", Params: []directive.ParamSpec{spec}, Doc: "spells a type"}
}

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
	assert.Total(tb, r.Register, directive.Kernel(), "the kernel schemas register first")
	assert.Total(tb, r.Register, schemas, "the fixture schema registers")
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

// varied returns a well-formed schema with the given variants.
func varied(variants ...directive.Variant) directive.Schema {
	s := wellFormed("shaper", "shape")
	s.Variants = variants
	return s
}
