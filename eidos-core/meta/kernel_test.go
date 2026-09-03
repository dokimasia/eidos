// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/symbol"
)

// The kernel's own keys are the one registration every composition
// shares, so the contract is what a frontend stamps against.
func TestKernel(t *testing.T) {
	t.Parallel()

	t.Run("Kernel", func(t *testing.T) {
		t.Parallel()

		t.Run("registers the module keys on packages", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			keys, err := meta.Kernel(r)
			assert.NoError(t, err, "the kernel keys register into an empty registry")
			assert.Equal(t, keys.Module.Name(), meta.ModuleKey, "the module handle is typed")
			assert.Equal(t, keys.ModuleRoot.Name(), meta.ModuleRootKey, "and the root's too")
			assert.False(t, keys.Module.IsZero(), "a registered handle names its key")

			facts := meta.NewFacts(r)
			pkg := symbol.Identity{Lang: "fake", Package: "svc", Kind: symbol.KindPackage}
			claim := meta.Claim{Subject: pkg, Plugin: diag.Origin("fakefront")}
			assert.NoError(t, meta.Stamp(facts, keys.Module, "example.test/svc", claim),
				"a package carries its module identity")
			got, held := meta.Get(facts, pkg, keys.Module)
			assert.True(t, held, "and reads it back")
			assert.Equal(t, got, "example.test/svc", "as stamped")

			file := symbol.Identity{Lang: "fake", Package: "svc", Name: "a.zz", Kind: symbol.KindFile}
			err = meta.Stamp(facts, keys.ModuleRoot, ".", meta.Claim{Subject: file})
			assert.HasError(t, err, "a file is not a subject the module keys admit")
		})

		t.Run("registers the authored-value keys on what carries a type", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			keys, err := meta.Kernel(r)
			assert.NoError(t, err, "the kernel keys register")
			assert.False(t, keys.IsZero(), "and the handles name their keys")
			facts := meta.NewFacts(r)
			field := symbol.Identity{
				Lang: "fake", Package: "svc", Owner: "Row", Name: "name", Kind: symbol.KindField,
			}
			assert.NoError(t, meta.Stamp(facts, keys.Sample, `"us-east"`, meta.Claim{Subject: field}),
				"a field carries an authored sample")
			assert.NoError(t, meta.Stamp(facts, keys.Alternate, `"eu-west"`, meta.Claim{Subject: field}),
				"and its alternate")
			param := symbol.Identity{
				Lang: "fake", Package: "svc", Owner: "Box", Name: "T", Kind: symbol.KindTypeParam,
			}
			want := symbol.Identity{Lang: "fake", Package: "time", Name: "Duration", Kind: symbol.KindAlias}
			assert.NoError(t, meta.Stamp(facts, keys.Witness, want, meta.Claim{Subject: param}),
				"a type parameter carries an authored witness, as an identity")
			got, held := meta.Get(facts, param, keys.Witness)
			assert.True(t, held, "which reads back")
			assert.Equal(t, got, want, "whole")
			pkg := symbol.Identity{Lang: "fake", Package: "svc", Kind: symbol.KindPackage}
			assert.HasError(t, meta.Stamp(facts, keys.Sample, "x", meta.Claim{Subject: pkg}),
				"a package carries no type, so it carries no sample")
		})

		t.Run("refuses a registry the kernel already claimed", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			_, err := meta.Kernel(r)
			assert.NoError(t, err, "the first registration holds")
			_, err = meta.Kernel(r)
			assert.HasError(t, err, "a second registration is a duplicate claim")
			assert.Contains(t, err.Error(), meta.KernelOwner, "naming the owner")
		})

		t.Run("refuses a namespace a plugin claimed first", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, r.ClaimNamespace(meta.KernelNamespace, "impostor"),
				"the registry alone cannot tell an impostor apart")
			_, err := meta.Kernel(r)
			assert.HasError(t, err, "which the kernel's own registration reports")
			assert.Contains(t, err.Error(), "impostor", "naming the claimant")
		})
	})
}
