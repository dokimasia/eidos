// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta_test

import (
	"cmp"
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/symbol"
)

// The registrants and namespaces the cases claim under, and the
// phrase a message names the composition by.
const (
	shapePlugin     = "eidos-plugin-shape"
	rivalPlugin     = "rival"
	shapeNamespace  = "shape"
	genNamespace    = "gen"
	compositionName = "the composition"
)

// The allocations of building a registry and of each registration,
// which TestRegistryAllocs checks in the ordinary run and
// BenchmarkRegistry in a benchmark run.
const (
	// newRegistryAllocs is an empty registry: the composition's handle,
	// the registrations every handle shares, and their three maps.
	newRegistryAllocs = 5
	// forAllocs is a registrant's handle.
	forAllocs = 1
	// firstNamespaceAllocs is the first namespace a registry claims: the
	// first entry of the namespace map.
	firstNamespaceAllocs = 1
	// firstKeyAllocs is the first key a registry registers outside a
	// group: the spec list, the type list and the first entry of the
	// name map.
	firstKeyAllocs = 3
	// firstGroupedKeyAllocs is the first key a registry registers into a
	// group: what firstKeyAllocs counts, the first entry of the group
	// map and the group's member list.
	firstGroupedKeyAllocs = 5
)

// Registration is how a plugin claims its namespace and its keys, so
// what each registration refuses and what each lookup returns are
// contract.
func TestRegistry(t *testing.T) {
	t.Parallel()

	t.Run("NewRegistry", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a registry without a key", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, slices.Collect(meta.NewRegistry().Keys()), "nothing is registered")
		})
	})

	t.Run("For", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a handle on the same registrations", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			shape := r.For(shapePlugin)
			assert.NoError(t, shape.ClaimNamespace(shapeNamespace), "the plugin claims its namespace")
			role, err := meta.Register[string](shape, meta.KeySpec{
				Name: "shape.role", Doc: "the classified role",
			})
			assert.NoError(t, err, "the key registers through the plugin's handle")

			id, known := r.Resolve("shape.role")
			assert.True(t, known, "the composition's handle resolves the key")
			assert.Equal(t, id, role.ID(), "the composition's handle resolves the same id")
		})
	})

	t.Run("ClaimNamespace", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error naming both registrants for a namespace claimed twice", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, r.For(shapePlugin).ClaimNamespace(shapeNamespace),
				"the first claim succeeds")

			err := r.For(rivalPlugin).ClaimNamespace(shapeNamespace)
			assert.HasError(t, err, "the second claim fails")
			assert.Contains(t, err.Error(), shapePlugin, "the error names the first registrant")
			assert.Contains(t, err.Error(), rivalPlugin, "the error names the second registrant")
			assert.HasPrefix(t, err.Error(), "meta: ", "the error has the package prefix")
		})

		t.Run("names the composition for a claim through the composition's handle", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, r.ClaimNamespace(shapeNamespace), "the composition's claim succeeds")

			err := r.For(shapePlugin).ClaimNamespace(shapeNamespace)
			assert.HasError(t, err, "the plugin's claim fails")
			assert.Contains(t, err.Error(), compositionName, "the error names the composition")
		})

		t.Run("returns nil for a namespace that its registrant claims again", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, r.For(shapePlugin).ClaimNamespace(shapeNamespace), "the first claim succeeds")
			assert.NoError(t, r.For(shapePlugin).ClaimNamespace(shapeNamespace), "the repeated claim succeeds")
			got, held := r.Claimant(shapeNamespace)
			assert.True(t, held, "the namespace remains claimed")
			assert.Equal(t, got, shapePlugin, "the registrant remains the claimant")
		})

		t.Run("returns an error for the empty namespace", func(t *testing.T) {
			t.Parallel()

			assert.HasError(t, meta.NewRegistry().ClaimNamespace(""), "the claim fails")
		})

		t.Run("returns an error after the seal", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			r.Seal()
			assert.HasError(t, r.ClaimNamespace(shapeNamespace), "the claim fails")
		})
	})

	t.Run("Register", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a handle that reports the registered spelling", func(t *testing.T) {
			t.Parallel()

			role, err := meta.Register[string](claimed(t), meta.KeySpec{
				Name: "shape.role", Doc: "the classified role",
			})
			assert.NoError(t, err, "the key registers")
			assert.Equal(t, role.Name(), meta.KeyName("shape.role"), "the handle reports the spelling")
		})

		t.Run("returns a distinct id for each key", func(t *testing.T) {
			t.Parallel()

			r := claimed(t)
			role, err := meta.Register[string](r, meta.KeySpec{
				Name: "shape.role", Doc: "the classified role",
			})
			assert.NoError(t, err, "the first key registers")
			target, err := meta.Register[symbol.Identity](r, meta.KeySpec{
				Name: "shape.target", Doc: "the named twin",
			})
			assert.NoError(t, err, "the second key registers")
			assert.NotEqual(t, target.ID(), role.ID(), "the ids differ")
		})

		t.Run("registers through a second handle of the claiming registrant", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, r.For(shapePlugin).ClaimNamespace(shapeNamespace),
				"the plugin claims its namespace")
			_, err := meta.Register[string](r.For(shapePlugin), meta.KeySpec{
				Name: "shape.role", Doc: "the classified role",
			})
			assert.NoError(t, err, "the key registers")
		})

		t.Run("returns an error for a key in a namespace another plugin claimed", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, r.For(shapePlugin).ClaimNamespace(shapeNamespace),
				"the plugin claims its namespace")
			_, err := meta.Register[string](r.For(rivalPlugin), meta.KeySpec{
				Name: "shape.role", Doc: "a key under a foreign namespace",
			})
			assert.HasError(t, err, "the registration fails")
			assert.Contains(t, err.Error(), rivalPlugin, "the error names the registering plugin")
			assert.Contains(t, err.Error(), shapePlugin, "the error names the claiming plugin")
		})

		t.Run("returns an error for a plugin's key in the composition's namespace", func(t *testing.T) {
			t.Parallel()

			_, err := meta.Register[string](claimed(t).For(shapePlugin), meta.KeySpec{
				Name: "gen.role", Doc: "a key under the composition's namespace",
			})
			assert.HasError(t, err, "the registration fails")
			assert.Contains(t, err.Error(), compositionName, "the error names the composition")
		})

		t.Run("returns an error naming both specs for a name registered twice", func(t *testing.T) {
			t.Parallel()

			r := claimed(t)
			_, err := meta.Register[string](r, meta.KeySpec{
				Name: "shape.role", Doc: "the first claimant",
			})
			assert.NoError(t, err, "the first registration succeeds")

			_, err = meta.Register[int64](r, meta.KeySpec{
				Name: "shape.role", Doc: "the second claimant",
			})
			assert.HasError(t, err, "the second registration fails under any value type")
			assert.Contains(t, err.Error(), "the first claimant", "the error names the first spec")
			assert.Contains(t, err.Error(), "the second claimant", "the error names the second spec")
		})

		t.Run("returns the handle of the first registration for an equal spec registered again", func(t *testing.T) {
			t.Parallel()

			r := claimed(t)
			first, err := meta.Register[string](r, contractedRoleSpec(diag.SeverityError))
			assert.NoError(t, err, "the first registration succeeds")
			again, err := meta.Register[string](r, contractedRoleSpec(diag.SeverityError))
			assert.NoError(t, err, "the repeated registration succeeds")
			assert.Equal(t, again, first, "the repeated registration returns the first handle")
			assert.Length(t, slices.Collect(r.Keys()), 1, "the registry keeps one key")
			assert.Length(t, slices.Collect(r.Group("shape.writer")), 1, "the group keeps one member")
		})

		repeats := []struct {
			name   string
			first  meta.KeySpec
			again  func(r *meta.Registry) error
			marker string
		}{
			{
				name:  "returns an error naming both value types for a key registered again under another type",
				first: roleSpec(),
				again: func(r *meta.Registry) error {
					_, err := meta.Register[int64](r, roleSpec())
					return err
				},
				marker: "the value types string and int64",
			},
			{
				name:  "returns an error naming both kind lists for a key registered again with other kinds",
				first: roleSpec(),
				again: func(r *meta.Registry) error {
					spec := roleSpec()
					spec.Kinds = []symbol.Kind{symbol.KindStruct}
					_, err := meta.Register[string](r, spec)
					return err
				},
				marker: "the kinds [] and [" + symbol.KindStruct.String() + "]",
			},
			{
				name:  "returns an error naming both groups for a key registered again into another group",
				first: roleSpec(),
				again: func(r *meta.Registry) error {
					_, err := meta.Register[string](r, groupedRoleSpec())
					return err
				},
				marker: `the groups "" and "shape.writer"`,
			},
			{
				name:  "returns an error for a key registered again with a contract that the first lacks",
				first: groupedRoleSpec(),
				again: func(r *meta.Registry) error {
					_, err := meta.Register[string](r, contractedRoleSpec(diag.SeverityError))
					return err
				},
				marker: "two different completeness contracts",
			},
			{
				name:  "returns an error for a key registered again with a contract of another severity",
				first: contractedRoleSpec(diag.SeverityError),
				again: func(r *meta.Registry) error {
					_, err := meta.Register[string](r, contractedRoleSpec(diag.SeverityWarning))
					return err
				},
				marker: "two different completeness contracts",
			},
		}
		for _, tt := range repeats {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				r := claimed(t)
				_, err := meta.Register[string](r, tt.first)
				assert.NoError(t, err, "the first registration succeeds")
				err = tt.again(r)
				assert.HasError(t, err, "the repeated registration fails")
				assert.Contains(t, err.Error(), tt.marker, "the error contains the difference")
			})
		}

		t.Run("returns an error for a namespace nothing claimed", func(t *testing.T) {
			t.Parallel()

			_, err := meta.Register[string](claimed(t), meta.KeySpec{
				Name: "unclaimed.role", Doc: "an orphan",
			})
			assert.HasError(t, err, "the registration fails")
		})

		tests := []struct {
			name string
			give meta.KeyName
		}{
			{name: "returns an error for a name without a local part", give: "shape"},
			{name: "returns an error for a name ending in its separator", give: "shape."},
			{name: "returns an error for a name without a namespace", give: ".role"},
			{name: "returns an error for the empty name", give: ""},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := meta.Register[string](claimed(t), meta.KeySpec{Name: tt.give, Doc: "misspelled"})
				assert.HasError(t, err, "the registration fails")
			})
		}

		t.Run("returns an error for a spec without documentation", func(t *testing.T) {
			t.Parallel()

			_, err := meta.Register[string](claimed(t), meta.KeySpec{Name: "shape.role"})
			assert.HasError(t, err, "the registration fails")
		})

		t.Run("returns an error for a group spelled like a registered key", func(t *testing.T) {
			t.Parallel()

			r := claimed(t)
			_, err := meta.Register[bool](r, meta.KeySpec{Name: "shape.writer", Doc: "the writer flag"})
			assert.NoError(t, err, "the key registers")
			_, err = meta.Register[string](r, meta.KeySpec{
				Name: "shape.role", Group: "shape.writer", Doc: "the classified role",
			})
			assert.HasError(t, err, "the grouped key fails to register")
			assert.Contains(t, err.Error(), "shape.writer", "the error names the spelling")
		})

		t.Run("returns an error for a key spelled like a registered group", func(t *testing.T) {
			t.Parallel()

			r := claimed(t)
			_, err := meta.Register[string](r, meta.KeySpec{
				Name: "shape.role", Group: "shape.writer", Doc: "the classified role",
			})
			assert.NoError(t, err, "the grouped key registers")
			_, err = meta.Register[bool](r, meta.KeySpec{Name: "shape.writer", Doc: "the writer flag"})
			assert.HasError(t, err, "the key fails to register")
			assert.Contains(t, err.Error(), "shape.writer", "the error names the spelling")
		})

		t.Run("returns an error after the seal", func(t *testing.T) {
			t.Parallel()

			r := claimed(t)
			r.Seal()
			_, err := meta.Register[string](r, meta.KeySpec{Name: "shape.role", Doc: "late"})
			assert.HasError(t, err, "the registration fails")
		})
	})

	t.Run("Lookup", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the handle a spelling names", func(t *testing.T) {
			t.Parallel()

			r := claimed(t)
			registered, err := meta.Register[string](r, meta.KeySpec{Name: "shape.role", Doc: "a role"})
			assert.NoError(t, err, "the key registers")
			got, held := meta.Lookup[string](r, "shape.role")
			assert.True(t, held, "the spelling names the key")
			assert.Equal(t, got.ID(), registered.ID(), "the handle is the registered one")
		})

		t.Run("reports false for a spelling registered under another value type", func(t *testing.T) {
			t.Parallel()

			r := claimed(t)
			_, err := meta.Register[string](r, meta.KeySpec{Name: "shape.role", Doc: "a role"})
			assert.NoError(t, err, "the key registers")
			_, held := meta.Lookup[bool](r, "shape.role")
			assert.False(t, held, "the lookup reports false")
		})

		t.Run("reports false for a spelling nothing registered", func(t *testing.T) {
			t.Parallel()

			_, held := meta.Lookup[string](claimed(t), "shape.ghost")
			assert.False(t, held, "the lookup reports false")
		})
	})

	t.Run("Claimant", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the plugin that claimed a namespace", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, r.For(shapePlugin).ClaimNamespace(shapeNamespace), "the plugin claims its namespace")
			got, held := r.Claimant(shapeNamespace)
			assert.True(t, held, "the namespace is claimed")
			assert.Equal(t, got, shapePlugin, "the claimant is the plugin")
		})

		t.Run("returns the empty name for a namespace that the composition claimed", func(t *testing.T) {
			t.Parallel()

			got, held := claimed(t).Claimant(shapeNamespace)
			assert.True(t, held, "the namespace is claimed")
			assert.Equal(t, got, "", "the composition has the empty name")
		})

		t.Run("reports false for a namespace nothing claimed", func(t *testing.T) {
			t.Parallel()

			_, held := unclaimed(t).Claimant(shapeNamespace)
			assert.False(t, held, "nothing claimed the namespace")
		})
	})

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the id a spelling names", func(t *testing.T) {
			t.Parallel()

			r := claimed(t)
			role, err := meta.Register[string](r, meta.KeySpec{
				Name: "shape.role", Doc: "the classified role",
			})
			assert.NoError(t, err, "the key registers")

			id, known := r.Resolve("shape.role")
			assert.True(t, known, "the spelling resolves")
			assert.Equal(t, id, role.ID(), "the id is the handle's")
		})

		t.Run("reports false for a spelling nothing registered", func(t *testing.T) {
			t.Parallel()

			_, known := claimed(t).Resolve("shape.nonexistent")
			assert.False(t, known, "the spelling does not resolve")
		})
	})

	t.Run("Spec", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a registered key's spec", func(t *testing.T) {
			t.Parallel()

			r := claimed(t)
			role, err := meta.Register[string](r, meta.KeySpec{
				Name:  "shape.role",
				Kinds: []symbol.Kind{symbol.KindStruct},
				Doc:   "the classified role",
			})
			assert.NoError(t, err, "the key registers")

			spec, known := r.Spec(role.ID())
			assert.True(t, known, "the id is known")
			assert.Equal(t, spec.Name, meta.KeyName("shape.role"), "the spec has the spelling")
			assert.Equal(t, spec.Kinds, []symbol.Kind{symbol.KindStruct}, "the spec has the kinds")
		})

		tests := []struct {
			name string
			give meta.KeyID
		}{
			{name: "reports false for the zero id", give: 0},
			{name: "reports false for an id never assigned", give: 999},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, known := claimed(t).Spec(tt.give)
				assert.False(t, known, "the id is unknown")
			})
		}

		t.Run("returns the completeness contract as declared", func(t *testing.T) {
			t.Parallel()

			r := claimed(t)
			contract := &meta.Completeness{
				On:       []symbol.Kind{symbol.KindFunction},
				By:       diag.PhaseAnnotate,
				Severity: diag.SeverityError,
			}
			key, err := meta.Register[bool](r, meta.KeySpec{
				Name: "gen.exported", Contract: contract, Doc: "the export flag",
			})
			assert.NoError(t, err, "the key registers")

			spec, known := r.Spec(key.ID())
			assert.True(t, known, "the id is known")
			assert.Equal(t, spec.Contract, contract, "the spec has the declared contract")
		})
	})

	t.Run("Group", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the members in registration order", func(t *testing.T) {
			t.Parallel()

			r := claimed(t)
			role, err := meta.Register[string](r, meta.KeySpec{
				Name: "shape.role", Group: "shape.writer", Doc: "the classified role",
			})
			assert.NoError(t, err, "the first member registers")
			target, err := meta.Register[symbol.Identity](r, meta.KeySpec{
				Name: "shape.target", Group: "shape.writer", Doc: "the named twin",
			})
			assert.NoError(t, err, "the second member registers")
			_, err = meta.Register[bool](r, meta.KeySpec{
				Name: "shape.comparable", Doc: "outside the group",
			})
			assert.NoError(t, err, "a key outside the group registers")

			assert.Equal(t, slices.Collect(r.Group("shape.writer")),
				[]meta.KeyID{role.ID(), target.ID()},
				"the members are in registration order")
		})

		t.Run("returns nothing for a group nothing registered into", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, slices.Collect(claimed(t).Group("shape.nonexistent")),
				"the group has no members")
		})
	})

	t.Run("Keys", func(t *testing.T) {
		t.Parallel()

		t.Run("returns every spelling in registration order", func(t *testing.T) {
			t.Parallel()

			r := claimed(t)
			_, err := meta.Register[string](r, meta.KeySpec{
				Name: "shape.role", Doc: "the classified role",
			})
			assert.NoError(t, err, "the first key registers")
			_, err = meta.Register[bool](r, meta.KeySpec{
				Name: "gen.exported", Doc: "the export flag",
			})
			assert.NoError(t, err, "the second key registers")

			assert.Equal(t, slices.Collect(r.Keys()),
				[]meta.KeyName{"shape.role", "gen.exported"},
				"the spellings are in registration order")
		})

		t.Run("stops when the range stops", func(t *testing.T) {
			t.Parallel()

			r := claimed(t)
			_, err := meta.Register[string](r, meta.KeySpec{
				Name: "shape.role", Doc: "the classified role",
			})
			assert.NoError(t, err, "the key registers")
			seen := 0
			for range r.Keys() {
				seen++
				break
			}
			assert.Equal(t, seen, 1, "the iteration yields once")
		})
	})

	t.Run("Seal", func(t *testing.T) {
		t.Parallel()

		t.Run("keeps every registered key resolvable", func(t *testing.T) {
			t.Parallel()

			r, role := grouped(t)
			id, known := r.Resolve("shape.role")
			assert.True(t, known, "the sealed registry resolves the key")
			assert.Equal(t, id, role.ID(), "to the registered id")
		})
	})
}

// Building a registry and each registration allocate what the registry
// keeps, and the lookups a run makes allocate nothing, in the ordinary
// run, which runs no benchmark. Each counted registration takes a
// registry of its own, built outside the count, and each count keeps
// the first error of its calls, which cmp.Or returns without
// allocating. The check runs alone, because the count includes every
// goroutine's allocations.
func TestRegistryAllocs(t *testing.T) {
	var built *meta.Registry
	assert.MaxAllocs(t, func() { built = meta.NewRegistry() }, newRegistryAllocs,
		"NewRegistry allocates the handle, the registrations and their maps")
	assert.MaxAllocs(t, func() { built = built.For(shapePlugin) }, forAllocs, "For allocates the handle")

	var err error
	assert.MaxAllocsWithSetup(t, func() *meta.Registry { return unclaimed(t) }, func(r *meta.Registry) {
		err = cmp.Or(err, r.ClaimNamespace(shapeNamespace))
	}, firstNamespaceAllocs, "ClaimNamespace allocates the first entry of the namespace map")
	assert.NoError(t, err, "every first namespace is claimed")

	assert.MaxAllocsWithSetup(t, func() *meta.Registry { return claimedGen(t) }, func(r *meta.Registry) {
		err = cmp.Or(err, r.ClaimNamespace(shapeNamespace))
	}, 0, "ClaimNamespace allocates nothing for a second namespace")
	assert.NoError(t, err, "every second namespace is claimed")

	assert.MaxAllocsWithSetup(t, func() *meta.Registry { return claimed(t) }, func(r *meta.Registry) {
		err = cmp.Or(err, r.ClaimNamespace(shapeNamespace))
	}, 0, "ClaimNamespace allocates nothing for a namespace that its registrant claimed")
	assert.NoError(t, err, "every repeated claim succeeds")

	register := func(spec meta.KeySpec) func(*meta.Registry) {
		return func(r *meta.Registry) {
			_, rerr := meta.Register[string](r, spec)
			err = cmp.Or(err, rerr)
		}
	}
	assert.MaxAllocsWithSetup(t, func() *meta.Registry { return claimed(t) }, register(roleSpec()),
		firstKeyAllocs, "Register allocates the spec list, the type list and the name map's first entry")
	assert.NoError(t, err, "every key registers")
	assert.MaxAllocsWithSetup(t, func() *meta.Registry { return claimed(t) }, register(groupedRoleSpec()),
		firstGroupedKeyAllocs, "Register allocates the group's entry and its member list beside the key's")
	assert.NoError(t, err, "every grouped key registers")
	assert.MaxAllocsWithSetup(t, func() *meta.Registry { return roleRegistered(t) }, register(roleSpec()),
		0, "Register allocates nothing for an equal spec registered again")
	assert.NoError(t, err, "every repeated registration succeeds")

	assert.MaxAllocs(t, func() { built.Seal() }, 0, "Seal allocates nothing")

	r, role := grouped(t)
	var id meta.KeyID
	assert.MaxAllocs(t, func() { id, _ = r.Resolve("shape.role") }, 0, "Resolve allocates nothing")
	assert.Equal(t, id, role.ID(), "Resolve returns the key's id")
	claimant := true
	assert.MaxAllocs(t, func() { _, claimant = r.Claimant(shapeNamespace) }, 0, "Claimant allocates nothing")
	assert.True(t, claimant, "Claimant finds the claimed namespace")
	var handle meta.Key[string]
	assert.MaxAllocs(t, func() { handle, _ = meta.Lookup[string](r, "shape.role") }, 0, "Lookup allocates nothing")
	assert.Equal(t, handle.ID(), role.ID(), "Lookup returns the key's handle")
	var spec meta.KeySpec
	assert.MaxAllocs(t, func() { spec, _ = r.Spec(role.ID()) }, 0, "Spec allocates nothing")
	assert.Equal(t, spec.Name, role.Name(), "Spec returns the key's spec")
	members := 0
	assert.MaxAllocs(t, func() {
		members = 0
		for range r.Group("shape.writer") {
			members++
		}
	}, 0, "Group allocates nothing")
	assert.Equal(t, members, 1, "Group enumerates the group's one member")
	keys := 0
	assert.MaxAllocs(t, func() {
		keys = 0
		for range r.Keys() {
			keys++
		}
	}, 0, "Keys allocates nothing")
	assert.Equal(t, keys, 1, "Keys enumerates the one key")
}

// BenchmarkRegistry measures the registrations a composition makes once
// per build, each into a registry built outside the measurement, and
// the lookups a run makes against the sealed registry: every directive
// parameter naming a key resolves through one of them.
func BenchmarkRegistry(b *testing.B) {
	b.Run("NewRegistry", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(newRegistryAllocs)
		defer c.End()
		var r *meta.Registry
		for c.Loop() {
			r = meta.NewRegistry()
		}
		assert.Empty(b, slices.Collect(r.Keys()), "NewRegistry returns a registry without a key")
	})

	b.Run("For", func(b *testing.B) {
		r := meta.NewRegistry()
		c := bench.Start(b).MaxAllocs(forAllocs)
		defer c.End()
		var handle *meta.Registry
		for c.Loop() {
			handle = r.For(shapePlugin)
		}
		assert.NoError(b, handle.ClaimNamespace(shapeNamespace), "For returns a handle that claims")
	})

	claims := []struct {
		name   string
		fresh  func(assert.TB) *meta.Registry
		allocs uint64
	}{
		{name: "a first namespace", fresh: unclaimed, allocs: firstNamespaceAllocs},
		{name: "a second namespace", fresh: claimedGen, allocs: 0},
		{name: "a namespace that its registrant claimed", fresh: claimed, allocs: 0},
	}
	b.Run("ClaimNamespace", func(b *testing.B) {
		for _, tt := range claims {
			b.Run(tt.name, func(b *testing.B) {
				var r *meta.Registry
				fresh := func() { r = tt.fresh(b) }
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var err error
				for c.Loop() {
					c.Excluding(fresh)
					err = r.ClaimNamespace(shapeNamespace)
				}
				assert.NoError(b, err, "the namespace is claimed")
			})
		}
	})

	registrations := []struct {
		name   string
		fresh  func(assert.TB) *meta.Registry
		spec   meta.KeySpec
		allocs uint64
	}{
		{name: "a first key", fresh: claimed, spec: roleSpec(), allocs: firstKeyAllocs},
		{name: "a first key in a group", fresh: claimed, spec: groupedRoleSpec(), allocs: firstGroupedKeyAllocs},
		{name: "an equal spec registered again", fresh: roleRegistered, spec: roleSpec(), allocs: 0},
	}
	b.Run("Register", func(b *testing.B) {
		for _, tt := range registrations {
			b.Run(tt.name, func(b *testing.B) {
				var r *meta.Registry
				fresh := func() { r = tt.fresh(b) }
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var (
					key meta.Key[string]
					err error
				)
				for c.Loop() {
					c.Excluding(fresh)
					key, err = meta.Register[string](r, tt.spec)
				}
				assert.NoError(b, err, "the key registers")
				assert.False(b, key.IsZero(), "Register returns a handle that names the key")
			})
		}
	})

	b.Run("Seal", func(b *testing.B) {
		r := claimed(b)
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		for c.Loop() {
			r.Seal()
		}
		assert.HasError(b, r.ClaimNamespace("late"), "Seal ends registration")
	})

	r, role := grouped(b)

	b.Run("Claimant", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		held := false
		for c.Loop() {
			_, held = r.Claimant(shapeNamespace)
		}
		assert.True(b, held, "Claimant finds the claimed namespace")
	})

	b.Run("Resolve", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var id meta.KeyID
		for c.Loop() {
			id, _ = r.Resolve("shape.role")
		}
		assert.Equal(b, id, role.ID(), "Resolve returns the registered id")
	})

	b.Run("Lookup", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got meta.Key[string]
		for c.Loop() {
			got, _ = meta.Lookup[string](r, "shape.role")
		}
		assert.Equal(b, got.ID(), role.ID(), "Lookup returns the registered handle")
	})

	b.Run("Spec", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var spec meta.KeySpec
		for c.Loop() {
			spec, _ = r.Spec(role.ID())
		}
		assert.Equal(b, spec.Name, role.Name(), "Spec returns the registered spec")
	})

	b.Run("Group", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		members := 0
		for c.Loop() {
			members = 0
			for range r.Group("shape.writer") {
				members++
			}
		}
		assert.Equal(b, members, 1, "Group enumerates the group's member")
	})

	b.Run("Keys", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		keys := 0
		for c.Loop() {
			keys = 0
			for range r.Keys() {
				keys++
			}
		}
		assert.Equal(b, keys, 1, "Keys enumerates the registered key")
	})
}

// unclaimed returns the composition's handle on an empty registry, in
// which no namespace is claimed.
func unclaimed(tb assert.TB) *meta.Registry {
	tb.Helper()

	return meta.NewRegistry()
}

// claimed returns the composition's handle on a registry in which
// the composition claimed the shape and gen namespaces.
func claimed(tb assert.TB) *meta.Registry {
	tb.Helper()

	r := meta.NewRegistry()
	assert.NoError(tb, r.ClaimNamespace(shapeNamespace), "the shape namespace is claimed")
	assert.NoError(tb, r.ClaimNamespace(genNamespace), "the gen namespace is claimed")
	return r
}

// roleRegistered returns the composition's handle on a registry in
// which the composition claimed the shape and gen namespaces and
// registered the role key outside a group.
func roleRegistered(tb assert.TB) *meta.Registry {
	tb.Helper()

	r := claimed(tb)
	_, err := meta.Register[string](r, roleSpec())
	assert.NoError(tb, err, "the role key registers")
	return r
}

// claimedGen returns the composition's handle on a registry in which
// the composition claimed the gen namespace alone.
func claimedGen(tb assert.TB) *meta.Registry {
	tb.Helper()

	r := meta.NewRegistry()
	assert.NoError(tb, r.ClaimNamespace(genNamespace), "the gen namespace is claimed")
	return r
}

// grouped returns a sealed registry with the role key registered into
// the writer group, and the key's handle.
func grouped(tb assert.TB) (*meta.Registry, meta.Key[string]) {
	tb.Helper()

	r := claimed(tb)
	role, err := meta.Register[string](r, groupedRoleSpec())
	assert.NoError(tb, err, "the role key registers")
	r.Seal()
	return r, role
}

// roleSpec returns the spec of the role key outside a group.
func roleSpec() meta.KeySpec {
	return meta.KeySpec{Name: "shape.role", Doc: "the classified role"}
}

// groupedRoleSpec returns the spec of the role key in the writer group.
func groupedRoleSpec() meta.KeySpec {
	return meta.KeySpec{Name: "shape.role", Group: "shape.writer", Doc: "the classified role"}
}

// contractedRoleSpec returns the spec of the role key in the writer
// group, with a completeness contract of severity on every function by
// the end of the annotate phase. Each call allocates its own contract,
// so two specs compare by value.
func contractedRoleSpec(severity diag.Severity) meta.KeySpec {
	spec := groupedRoleSpec()
	spec.Contract = &meta.Completeness{
		On: []symbol.Kind{symbol.KindFunction}, By: diag.PhaseAnnotate, Severity: severity,
	}
	return spec
}
