// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package directive_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/directive"
)

// A schema is a value with no behaviour, so its twin covers the
// vocabulary's contracts: the zero values, the distinctness of the
// closed sets, and the two methods.
func TestSchema(t *testing.T) {
	t.Parallel()

	t.Run("Canonical", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the prefixed spelling of a plugin's schema", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, pluginSchema().Canonical(), directive.Name("mockgen:stub"), "the spelling has the prefix")
		})

		t.Run("returns the bare spelling of a kernel schema", func(t *testing.T) {
			t.Parallel()

			s := directive.Schema{Name: directive.KernelMeta}
			assert.Equal(t, s.Canonical(), directive.KernelMeta, "the spelling is bare")
		})
	})

	t.Run("ParamType", func(t *testing.T) {
		t.Parallel()

		t.Run("types nothing at the zero value", func(t *testing.T) {
			t.Parallel()

			var got directive.ParamType
			for _, declared := range []directive.ParamType{
				directive.TypeString, directive.TypeInt, directive.TypeBool,
				directive.TypeList, directive.TypeReference,
			} {
				assert.NotEqual(t, got, declared, "the zero value is no declared type")
			}
		})

		t.Run("declares five distinct values", func(t *testing.T) {
			t.Parallel()

			types := []directive.ParamType{
				directive.TypeString, directive.TypeInt, directive.TypeBool,
				directive.TypeList, directive.TypeReference,
			}
			assert.NoDuplicates(t, func() ([]directive.ParamType, error) { return types, nil },
				"each type has its own value")
		})
	})

	t.Run("ResolutionKind", func(t *testing.T) {
		t.Parallel()

		t.Run("resolves nothing at the zero value", func(t *testing.T) {
			t.Parallel()

			var got directive.ResolutionKind
			assert.Equal(t, got, directive.ResolveNone, "the zero value is ResolveNone")
		})

		t.Run("declares eight distinct values", func(t *testing.T) {
			t.Parallel()

			kinds := []directive.ResolutionKind{
				directive.ResolveNone, directive.ResolveCallableInScope,
				directive.ResolvePackageVar, directive.ResolveValueField,
				directive.ResolveHostParam, directive.ResolveMemberOnHandle,
				directive.ResolveMetadataKey, directive.ResolveTypeInScope,
			}
			assert.NoDuplicates(t, func() ([]directive.ResolutionKind, error) { return kinds, nil },
				"each kind has its own value")
		})
	})

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give directive.ResolutionKind
			want string
		}{
			{name: "returns the spelling of ResolveNone", give: directive.ResolveNone, want: "no resolution"},
			{
				name: "returns the spelling of ResolveCallableInScope",
				give: directive.ResolveCallableInScope, want: "a callable in scope",
			},
			{
				name: "returns the spelling of ResolvePackageVar",
				give: directive.ResolvePackageVar, want: "a package variable",
			},
			{
				name: "returns the spelling of ResolveValueField",
				give: directive.ResolveValueField, want: "a field on the subject's type",
			},
			{
				name: "returns the spelling of ResolveHostParam",
				give: directive.ResolveHostParam, want: "a parameter of the host callable",
			},
			{
				name: "returns the spelling of ResolveMemberOnHandle",
				give: directive.ResolveMemberOnHandle, want: "a member on a handle",
			},
			{
				name: "returns the spelling of ResolveMetadataKey",
				give: directive.ResolveMetadataKey, want: "a metadata key or group",
			},
			{
				name: "returns the spelling of ResolveTypeInScope",
				give: directive.ResolveTypeInScope, want: "a type in scope",
			},
			{
				name: "returns the number of an undeclared kind",
				give: directive.ResolutionKind(99), want: "resolution kind 99",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.String(), tt.want, "the spelling is pinned")
			})
		}
	})

	t.Run("ParamKey", func(t *testing.T) {
		t.Parallel()

		t.Run("reserves two distinct routing keys", func(t *testing.T) {
			t.Parallel()

			assert.NotEqual(t, directive.ReservedOut, directive.ReservedTag, "the keys differ")
		})
	})
}

// A declared kind spells without allocating, a plugin schema's canonical
// spelling allocates the prefixed name, and a kernel schema's allocates
// nothing, in the ordinary run, which runs no benchmark.
func TestSchemaAllocs(t *testing.T) {
	kind := directive.ResolveTypeInScope
	var spelled string
	assert.MaxAllocs(t, func() { spelled = kind.String() }, 0, "String allocates nothing for a declared kind")
	assert.Equal(t, spelled, "a type in scope", "String spells ResolveTypeInScope")
	plugin, kernel := pluginSchema(), directive.Schema{Name: directive.KernelMeta}
	var name directive.Name
	assert.MaxAllocs(t, func() { name = plugin.Canonical() }, 1,
		"Canonical allocates a plugin schema's prefixed spelling")
	assert.Equal(t, name, directive.Name("mockgen:stub"), "Canonical prefixes the plugin")
	assert.MaxAllocs(t, func() { name = kernel.Canonical() }, 0, "Canonical allocates nothing for a kernel schema")
	assert.Equal(t, name, directive.KernelMeta, "Canonical leaves a kernel name bare")
}

// BenchmarkSchema measures a resolution kind's spelling and a schema's
// canonical spelling.
func BenchmarkSchema(b *testing.B) {
	b.Run("ResolutionKind.String", func(b *testing.B) {
		kind := directive.ResolveTypeInScope
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = kind.String()
		}
		assert.Equal(b, got, "a type in scope", "String spells ResolveTypeInScope")
	})

	b.Run("Canonical", func(b *testing.B) {
		b.Run("a plugin schema", func(b *testing.B) {
			s := pluginSchema()
			c := bench.Start(b).MaxAllocs(1)
			defer c.End()
			var got directive.Name
			for c.Loop() {
				got = s.Canonical()
			}
			assert.Equal(b, got, directive.Name("mockgen:stub"), "Canonical prefixes the plugin")
		})

		b.Run("a kernel schema", func(b *testing.B) {
			s := directive.Schema{Name: directive.KernelMeta}
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var got directive.Name
			for c.Loop() {
				got = s.Canonical()
			}
			assert.Equal(b, got, directive.KernelMeta, "Canonical leaves a kernel name bare")
		})
	})
}

// pluginSchema returns the stub schema of the mockgen plugin, whose
// canonical spelling is prefixed.
func pluginSchema() directive.Schema {
	return directive.Schema{Plugin: "mockgen", Name: "stub"}
}
