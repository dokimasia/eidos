// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package directive_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/directive"
)

// kernelAllocs is one call of Kernel: the list of six schemas, the
// param list of each of the five schemas that declare params, and the
// witness's open spec.
const kernelAllocs = 7

// The kernel schemas are the spellings consumers write in source,
// so their shape is API: names, params and repeatability all pin
// here.
func TestKernel(t *testing.T) {
	t.Parallel()

	t.Run("Kernel", func(t *testing.T) {
		t.Parallel()

		t.Run("returns six schemas that seal without a fault", func(t *testing.T) {
			t.Parallel()

			r := directive.NewRegistry()
			schemas := directive.Kernel()
			assert.Length(t, schemas, 6, "meta, out, diag, skip, sample and witness")
			for _, s := range schemas {
				assert.NoError(t, r.Register(s), "every kernel schema passes its own registry")
				assert.Equal(t, s.Plugin, "", "and belongs to the kernel")
			}
			assert.Empty(t, r.Seal(), "and the set seals without faults")
		})

		t.Run("returns a repeatable meta schema", func(t *testing.T) {
			t.Parallel()

			assert.True(t, kernelSchema(t, directive.KernelMeta).Repeatable, "several facts drop on one subject")
		})

		t.Run("returns a meta schema whose drop resolves against the metadata registry", func(t *testing.T) {
			t.Parallel()

			drop := kernelSchema(t, directive.KernelMeta).Params[0]
			assert.Equal(t, drop.Key, directive.MetaDrop, "under the drop key")
			assert.Equal(t, drop.Resolution, directive.ResolveMetadataKey,
				"resolved against the metadata registry, so a typo names candidates")
		})

		t.Run("returns an out schema that declares the reserved routing keys", func(t *testing.T) {
			t.Parallel()

			params := kernelSchema(t, directive.KernelOut).Params
			keys := make([]directive.ParamKey, 0, len(params))
			for _, spec := range params {
				keys = append(keys, spec.Key)
			}
			expect.That(t, keys).
				Contains(directive.OutPath, "the redirect target").
				Contains(directive.OutTag,
					"and the companion selector: the kernel defines the reserved keys, so its own schema may declare one")
		})

		t.Run("returns a repeatable diag schema", func(t *testing.T) {
			t.Parallel()

			assert.True(t, kernelSchema(t, directive.KernelDiag).Repeatable, "several codes suppress on one subject")
		})

		t.Run("returns a diag schema that requires the code it suppresses", func(t *testing.T) {
			t.Parallel()

			off := kernelSchema(t, directive.KernelDiag).Params[0]
			assert.Equal(t, off.Key, directive.DiagOff, "under the off key")
			assert.True(t, off.Required, "a suppression without a code suppresses nothing")
		})

		t.Run("returns a diag schema whose code resolves against the registered codes", func(t *testing.T) {
			t.Parallel()

			off := kernelSchema(t, directive.KernelDiag).Params[0]
			assert.Equal(t, off.Resolution, directive.ResolveDiagnosticCode,
				"resolved against the registered codes, so a typo is a validation Error")
		})

		t.Run("returns a skip schema of one instance per subject", func(t *testing.T) {
			t.Parallel()

			assert.False(t, kernelSchema(t, directive.KernelSkip).Repeatable, "one exclusion per subject")
		})

		t.Run("returns a skip schema whose plugin param is optional", func(t *testing.T) {
			t.Parallel()

			plugin := kernelSchema(t, directive.KernelSkip).Params[0]
			assert.Equal(t, plugin.Key, directive.SkipPlugin, "under the plugin key")
			assert.False(t, plugin.Required, "a bare skip excludes from everything")
		})

		t.Run("returns a sample schema that requires its value", func(t *testing.T) {
			t.Parallel()

			sample := kernelSchema(t, directive.KernelSample)
			assert.Length(t, sample.Params, 2, "value and alternate")
			assert.Equal(t, sample.Params[0].Key, directive.SampleValue, "the first is the value")
			assert.True(t, sample.Params[0].Required, "which every instance states")
		})

		t.Run("returns a sample schema whose alternate is optional", func(t *testing.T) {
			t.Parallel()

			alternate := kernelSchema(t, directive.KernelSample).Params[1]
			assert.Equal(t, alternate.Key, directive.SampleAlternate, "the second is the alternate")
			assert.False(t, alternate.Required, "which may be derived")
		})

		t.Run("returns a sample schema of one instance per subject", func(t *testing.T) {
			t.Parallel()

			assert.False(t, kernelSchema(t, directive.KernelSample).Repeatable, "one sample per subject")
		})

		t.Run("returns an open witness schema over types in scope", func(t *testing.T) {
			t.Parallel()

			witness := kernelSchema(t, directive.KernelWitness)
			assert.Empty(t, witness.Params, "no key is declared: the type parameters are")
			assert.NotNil(t, witness.Open, "so the schema is open")
			assert.Equal(t, witness.Open.Type, directive.TypeReference, "every key names a type")
			assert.Equal(t, witness.Open.Resolution, directive.ResolveTypeInScope, "resolved in scope")
		})
	})
}

// The kernel's schemas allocate within their ceiling in the ordinary
// run, which runs no benchmark.
func TestKernelAllocs(t *testing.T) {
	var got []directive.Schema
	assert.MaxAllocs(t, func() { got = directive.Kernel() }, kernelAllocs,
		"Kernel allocates the schemas and their params")
	assert.Length(t, got, 6, "Kernel returns the six kernel schemas")
}

// BenchmarkKernel measures one construction of the kernel's schemas,
// which a workspace registers before any plugin's.
func BenchmarkKernel(b *testing.B) {
	b.Run("Kernel", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(kernelAllocs)
		defer c.End()
		var got []directive.Schema
		for c.Loop() {
			got = directive.Kernel()
		}
		assert.Length(b, got, 6, "Kernel returns the six kernel schemas")
	})
}

// kernelSchema returns the kernel schema of a name, and fails the
// test where none does.
func kernelSchema(tb assert.TB, name directive.Name) directive.Schema {
	tb.Helper()

	for _, s := range directive.Kernel() {
		if s.Name == name {
			return s
		}
	}
	assert.True(tb, false, "every kernel name returns a schema")
	return directive.Schema{}
}
