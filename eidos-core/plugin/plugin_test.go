// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/plugin"
)

// named is the smallest plugin: a stable name and nothing else.
type named struct{ name plugin.ID }

// Name returns the fixture's name.
func (p named) Name() plugin.ID { return p.name }

// A plugin's name is its diagnostic origin, its emit attribution and
// its arbitration rank, and a role is the role a priority attaches
// to, so both spellings are contract.
func TestPlugin(t *testing.T) {
	t.Parallel()

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the plugin's declared name", func(t *testing.T) {
			t.Parallel()

			var p plugin.Plugin = named{name: "stubgen"}
			assert.Equal(t, p.Name(), "stubgen",
				"the name is the plugin's one identity everywhere it appears")
		})
	})

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			role plugin.Role
			want string
		}{
			{name: "returns annotator for RoleAnnotator", role: plugin.RoleAnnotator, want: "annotator"},
			{name: "returns generator for RoleGenerator", role: plugin.RoleGenerator, want: "generator"},
			{
				name: "returns the number of a role nothing declares",
				role: plugin.RoleGenerator + 1,
				want: "Role(3)",
			},
			{
				name: "returns the number of the zero role",
				role: 0,
				want: "Role(0)",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.role.String(), tt.want,
					"a fault names the role it places, so the spelling is API")
			})
		}
	})
}

// A declared role spells without allocating in the ordinary run, which
// runs no benchmark.
func TestPluginZeroAlloc(t *testing.T) {
	role := plugin.RoleGenerator
	var got string
	assert.MaxAllocs(t, func() { got = role.String() }, 0, "String allocates nothing for a declared role")
	assert.Equal(t, got, "generator", "String spells RoleGenerator")
}

// BenchmarkPlugin measures the spelling of a declared role, which a
// fault and a stats record name.
func BenchmarkPlugin(b *testing.B) {
	b.Run("String", func(b *testing.B) {
		role := plugin.RoleGenerator
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = role.String()
		}
		assert.Equal(b, got, "generator", "String spells RoleGenerator")
	})
}
