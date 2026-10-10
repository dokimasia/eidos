// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/symbol"
)

// kernelAllocs is one registration of the kernel's keys into an empty
// registry: the first entry of the namespace map, the four kind lists,
// the first entry of the name map, and the spec and type lists growing
// to five entries, eight allocations.
const kernelAllocs = 14

// The kernel's own keys are the one registration every composition
// shares, so the contract is what a frontend stamps against.
func TestKernel(t *testing.T) {
	t.Parallel()

	t.Run("IsZero", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for the zero handles", func(t *testing.T) {
			t.Parallel()

			var keys meta.KernelKeys
			assert.True(t, keys.IsZero(), "the zero handles name nothing")
		})

		t.Run("reports false for the handles Kernel returns", func(t *testing.T) {
			t.Parallel()

			_, _, keys := kernelFixture(t)
			assert.False(t, keys.IsZero(), "the handles name keys")
		})
	})

	t.Run("Kernel", func(t *testing.T) {
		t.Parallel()

		t.Run("returns handles named by the kernel's key names", func(t *testing.T) {
			t.Parallel()

			_, _, keys := kernelFixture(t)
			assert.Equal(t, keys.Module.Name(), meta.ModuleKey, "the module handle has its name")
			assert.Equal(t, keys.ModuleRoot.Name(), meta.ModuleRootKey, "the root handle has its name")
			assert.Equal(t, keys.Sample.Name(), meta.SampleKey, "the sample handle has its name")
			assert.Equal(t, keys.Alternate.Name(), meta.AlternateKey, "the alternate handle has its name")
			assert.Equal(t, keys.Witness.Name(), meta.WitnessKey, "the witness handle has its name")
		})

		t.Run("registers the module key on packages", func(t *testing.T) {
			t.Parallel()

			_, facts, keys := kernelFixture(t)
			pkg := symbol.Identity{Lang: "fake", Package: "svc", Kind: symbol.KindPackage}
			claim := meta.Claim{Subject: pkg, Plugin: diag.Origin("fakefront")}
			assert.NoError(t, meta.Stamp(facts, keys.Module, "example.test/svc", claim),
				"a package stamps its module identity")
			got, held := meta.Get(facts, pkg, keys.Module)
			assert.True(t, held, "the identity reads back")
			assert.Equal(t, got, "example.test/svc", "the identity is the stamped one")
		})

		t.Run("registers the module keys for packages only", func(t *testing.T) {
			t.Parallel()

			_, facts, keys := kernelFixture(t)
			file := symbol.Identity{Lang: "fake", Package: "svc", Name: "a.zz", Kind: symbol.KindFile}
			err := meta.Stamp(facts, keys.ModuleRoot, ".", meta.Claim{Subject: file})
			assert.HasError(t, err, "a file does not stamp a module root")
		})

		t.Run("registers the sample keys on a field", func(t *testing.T) {
			t.Parallel()

			_, facts, keys := kernelFixture(t)
			field := symbol.Identity{
				Lang: "fake", Package: "svc", Owner: "Row", Name: "name", Kind: symbol.KindField,
			}
			assert.NoError(t, meta.Stamp(facts, keys.Sample, `"us-east"`, meta.Claim{Subject: field}),
				"a field stamps an authored sample")
			assert.NoError(t, meta.Stamp(facts, keys.Alternate, `"eu-west"`, meta.Claim{Subject: field}),
				"a field stamps an authored alternate")
		})

		t.Run("registers the witness key on a type parameter", func(t *testing.T) {
			t.Parallel()

			_, facts, keys := kernelFixture(t)
			param := symbol.Identity{
				Lang: "fake", Package: "svc", Owner: "Box", Name: "T", Kind: symbol.KindTypeParam,
			}
			want := symbol.Identity{Lang: "fake", Package: "time", Name: "Duration", Kind: symbol.KindAlias}
			assert.NoError(t, meta.Stamp(facts, keys.Witness, want, meta.Claim{Subject: param}),
				"a type parameter stamps an authored witness")
			got, held := meta.Get(facts, param, keys.Witness)
			assert.True(t, held, "the witness reads back")
			assert.Equal(t, got, want, "the witness is the stamped identity")
		})

		t.Run("registers the sample keys for declarations with a type only", func(t *testing.T) {
			t.Parallel()

			_, facts, keys := kernelFixture(t)
			pkg := symbol.Identity{Lang: "fake", Package: "svc", Kind: symbol.KindPackage}
			assert.HasError(t, meta.Stamp(facts, keys.Sample, "x", meta.Claim{Subject: pkg}),
				"a package does not stamp a sample")
		})

		t.Run("claims the kernel namespace for the kernel through any handle", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			_, err := meta.Kernel(r.For(shapePlugin))
			assert.NoError(t, err, "the kernel's keys register")
			_, err = meta.Register[string](r.For(shapePlugin), meta.KeySpec{
				Name: "gen.role", Doc: "a plugin's key under the kernel namespace",
			})
			assert.HasError(t, err, "the plugin's key fails to register")
			assert.Contains(t, err.Error(), meta.KernelOwner, "the error names the kernel")
		})

		t.Run("returns the handles of the first registration for a second registration", func(t *testing.T) {
			t.Parallel()

			assert.Deterministic(t, meta.Kernel, meta.NewRegistry(),
				"every registration after the first returns the handles of the first")
		})

		t.Run("returns an error naming the plugin that claimed the kernel namespace", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, r.For(rivalPlugin).ClaimNamespace(meta.KernelNamespace),
				"the plugin claims the kernel namespace")
			_, err := meta.Kernel(r)
			assert.HasError(t, err, "the kernel's registration fails")
			assert.Contains(t, err.Error(), rivalPlugin, "the error names the plugin")
		})
	})
}

// The kernel's registration allocates what its registry keeps, and its
// handles report their state without allocating, in the ordinary run,
// which runs no benchmark. Each counted registration takes an empty
// registry of its own, built outside the count. The check runs alone,
// because the count includes every goroutine's allocations.
func TestKernelAllocs(t *testing.T) {
	var (
		keys meta.KernelKeys
		err  error
	)
	assert.MaxAllocsWithSetup(t, meta.NewRegistry, func(r *meta.Registry) { keys, err = meta.Kernel(r) },
		kernelAllocs, "Kernel allocates the registrations of the kernel's keys")
	assert.NoError(t, err, "the kernel's keys register")

	zero := true
	assert.MaxAllocs(t, func() { zero = keys.IsZero() }, 0, "IsZero allocates nothing")
	assert.False(t, zero, "IsZero reports false for the handles Kernel returns")
}

// BenchmarkKernel measures the registration of the kernel's keys into an
// empty registry, which every composition makes once, and the check of
// the handles it returns.
func BenchmarkKernel(b *testing.B) {
	b.Run("Kernel", func(b *testing.B) {
		var r *meta.Registry
		fresh := func() { r = meta.NewRegistry() }
		c := bench.Start(b).MaxAllocs(kernelAllocs)
		defer c.End()
		var (
			keys meta.KernelKeys
			err  error
		)
		for c.Loop() {
			c.Excluding(fresh)
			keys, err = meta.Kernel(r)
		}
		assert.NoError(b, err, "the kernel's keys register")
		assert.False(b, keys.IsZero(), "Kernel returns handles that name keys")
	})

	b.Run("IsZero", func(b *testing.B) {
		_, _, keys := kernelFixture(b)
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		zero := true
		for c.Loop() {
			zero = keys.IsZero()
		}
		assert.False(b, zero, "IsZero reports false for the handles Kernel returns")
	})
}

// kernelFixture returns a registry with the kernel's keys registered,
// a fact store over it, and the handles.
func kernelFixture(tb assert.TB) (*meta.Registry, *meta.Facts, meta.KernelKeys) {
	tb.Helper()

	r := meta.NewRegistry()
	keys, err := meta.Kernel(r)
	assert.NoError(tb, err, "the kernel's keys register")
	return r, meta.NewFacts(r), keys
}
