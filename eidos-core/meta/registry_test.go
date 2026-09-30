// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

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

// claimed returns the composition's handle on a registry in which
// the composition claimed the shape and gen namespaces.
func claimed(tb assert.TB) *meta.Registry {
	tb.Helper()

	r := meta.NewRegistry()
	assert.NoError(tb, r.ClaimNamespace(shapeNamespace), "the shape namespace is claimed")
	assert.NoError(tb, r.ClaimNamespace(genNamespace), "the gen namespace is claimed")
	return r
}

func TestRegistry(t *testing.T) {
	t.Parallel()

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
}

// Resolution is the boundary's lookup: every directive parameter
// naming a key goes through it.
func BenchmarkRegistry(b *testing.B) {
	b.Run("Resolve", func(b *testing.B) {
		b.ReportAllocs()

		r := claimed(b)
		if _, err := meta.Register[string](r, meta.KeySpec{
			Name: "shape.role", Doc: "the classified role",
		}); err != nil {
			b.Fatalf("Register: unexpected error: %v", err)
		}

		for b.Loop() {
			if _, known := r.Resolve("shape.role"); !known {
				b.Fatal("a registered spelling did not resolve")
			}
		}
	})
}
