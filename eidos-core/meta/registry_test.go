// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package meta_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/symbol"
)

// claimed returns a registry with the fixture namespaces claimed.
func claimed(tb assert.TB) *meta.Registry {
	tb.Helper()

	r := meta.NewRegistry()
	assert.NoError(tb, r.ClaimNamespace("shape", "eidos-plugin-shape"),
		"the shape namespace claims")
	assert.NoError(tb, r.ClaimNamespace("gen", "eidos-core"),
		"the gen namespace claims")
	return r
}

func TestRegistry(t *testing.T) {
	t.Parallel()

	t.Run("ClaimNamespace", func(t *testing.T) {
		t.Parallel()

		t.Run("refuses a namespace claimed twice", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, r.ClaimNamespace("shape", "eidos-plugin-shape"),
				"the first claim registers")

			err := r.ClaimNamespace("shape", "another-module")
			assert.HasError(t, err, "a namespace claimed twice is refused")
			assert.Contains(t, err.Error(), "eidos-plugin-shape",
				"naming the owner already holding it")
			assert.Contains(t, err.Error(), "another-module",
				"and the claimant that tried")
			assert.HasPrefix(t, err.Error(), "meta: ", "under the package prefix")
		})

		t.Run("refuses an empty namespace and an empty owner", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.HasError(t, r.ClaimNamespace("", "eidos-core"),
				"an empty namespace owns nothing")
			assert.HasError(t, r.ClaimNamespace("gen", ""),
				"an unnamed owner cannot be reported in a collision")
		})
	})

	t.Run("Register", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the typed handle", func(t *testing.T) {
			t.Parallel()

			r := claimed(t)
			role, err := meta.Register[string](r, meta.KeySpec{
				Name: "shape.role", Doc: "the classified role",
			})
			assert.NoError(t, err, "a key in a claimed namespace registers")
			assert.Equal(t, role.Name(), meta.KeyName("shape.role"),
				"the handle carries the spelling")

			target, err := meta.Register[symbol.Identity](r, meta.KeySpec{
				Name: "shape.target", Doc: "the named twin",
			})
			assert.NoError(t, err, "an identity-valued key registers")
			assert.NotEqual(t, target.ID(), role.ID(),
				"every key gets its own id")
		})

		t.Run("refuses a name registered twice", func(t *testing.T) {
			t.Parallel()

			r := claimed(t)
			_, err := meta.Register[string](r, meta.KeySpec{
				Name: "shape.role", Doc: "the first claimant",
			})
			assert.NoError(t, err, "the first registration arrives")

			_, err = meta.Register[int64](r, meta.KeySpec{
				Name: "shape.role", Doc: "the second claimant",
			})
			assert.HasError(t, err, "a name registered twice is refused, whatever its type")
			assert.Contains(t, err.Error(), "the first claimant",
				"naming the claimant already holding it")
			assert.Contains(t, err.Error(), "the second claimant",
				"and the one that tried")
		})

		t.Run("refuses a name without a claimed namespace", func(t *testing.T) {
			t.Parallel()

			r := claimed(t)
			_, err := meta.Register[string](r, meta.KeySpec{
				Name: "unclaimed.role", Doc: "an orphan",
			})
			assert.HasError(t, err,
				"a typo in the namespace fails at registration, not as a new namespace")
		})

		t.Run("refuses a name without a local part", func(t *testing.T) {
			t.Parallel()

			r := claimed(t)
			for _, name := range []meta.KeyName{"shape", "shape.", ".role", ""} {
				_, err := meta.Register[string](r, meta.KeySpec{Name: name, Doc: "misspelled"})
				assert.HasError(t, err,
					"a key name is a namespace and a local part, both non-empty")
			}
		})

		t.Run("refuses a spec without documentation", func(t *testing.T) {
			t.Parallel()

			r := claimed(t)
			_, err := meta.Register[string](r, meta.KeySpec{Name: "shape.role"})
			assert.HasError(t, err, "the parity matrix tabulates the doc, so it exists")
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
			assert.True(t, known, "a registered spelling resolves")
			assert.Equal(t, id, role.ID(), "to the handle's id")

			_, known = r.Resolve("shape.nonexistent")
			assert.False(t, known, "a spelling nothing registered does not")
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
			assert.True(t, known, "a registered id returns")
			assert.Equal(t, spec.Name, meta.KeyName("shape.role"), "its spelling")
			assert.Equal(t, spec.Kinds, []symbol.Kind{symbol.KindStruct}, "its kinds")

			_, known = r.Spec(meta.KeyID(0))
			assert.False(t, known, "the zero id names no key")
			_, known = r.Spec(meta.KeyID(999))
			assert.False(t, known, "and neither does an id never assigned")
		})

		t.Run("carries a completeness contract as declared", func(t *testing.T) {
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
			assert.NoError(t, err, "a key promising coverage registers")

			spec, known := r.Spec(key.ID())
			assert.True(t, known, "and returns")
			assert.Equal(t, spec.Contract, contract, "with the promise held, unchecked")
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
			assert.NoError(t, err, "and the second")

			assert.Equal(t, slices.Collect(r.Keys()),
				[]meta.KeyName{"shape.role", "gen.exported"},
				"a candidate-naming refusal enumerates what registered, in order")
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
			assert.Equal(t, seen, 1, "the iteration stops when the range stops")
		})
	})

	t.Run("Group", func(t *testing.T) {
		t.Parallel()

		t.Run("returns members in registration order", func(t *testing.T) {
			t.Parallel()

			r := claimed(t)
			role, err := meta.Register[string](r, meta.KeySpec{
				Name: "shape.role", Group: "shape.writer", Doc: "the classified role",
			})
			assert.NoError(t, err, "the first member registers")
			target, err := meta.Register[symbol.Identity](r, meta.KeySpec{
				Name: "shape.target", Group: "shape.writer", Doc: "the named twin",
			})
			assert.NoError(t, err, "and the second")
			_, err = meta.Register[bool](r, meta.KeySpec{
				Name: "shape.comparable", Doc: "outside the group",
			})
			assert.NoError(t, err, "and one outside the group")

			assert.Equal(t, slices.Collect(r.Group("shape.writer")),
				[]meta.KeyID{role.ID(), target.ID()},
				"the group holds its members in registration order")
			assert.Empty(t, slices.Collect(r.Group("shape.nonexistent")),
				"a group nothing registered into holds nothing")
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
