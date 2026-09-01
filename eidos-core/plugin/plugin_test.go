// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/plugin"
)

// named is the smallest plugin: a stable name and nothing else.
type named struct{ name plugin.ID }

func (p named) Name() plugin.ID { return p.name }

// A plugin's name is its diagnostic origin, its emit attribution and
// its arbitration rank, and a role is the role a priority attaches
// to, so both spellings are contract.
func TestPlugin(t *testing.T) {
	t.Parallel()

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		var p plugin.Plugin = named{name: "stubgen"}
		assert.Equal(t, p.Name(), "stubgen",
			"the name is the plugin's one identity everywhere it appears")
	})

	t.Run("Role/String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			role plugin.Role
			want string
		}{
			{name: "annotator", role: plugin.RoleAnnotator, want: "annotator"},
			{name: "generator", role: plugin.RoleGenerator, want: "generator"},
			{
				name: "names a role nothing declares by its number",
				role: plugin.RoleGenerator + 1,
				want: "Role(3)",
			},
			{
				name: "the zero role names no role",
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
