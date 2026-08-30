// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package plugin_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
)

// declared is a fixture plugin declaring every provider surface.
type declared struct {
	named
}

func (declared) Directives() []directive.Schema {
	return []directive.Schema{{Plugin: "stubgen", Name: "stub", Doc: "a fixture"}}
}

func (declared) Keys(*meta.Registry) error { return nil }

func (declared) Outputs() []plugin.Output {
	return []plugin.Output{{Per: plugin.PerPlan, Word: "registry"}}
}
func (declared) Priority(r plugin.Role) int    { return int(r) }
func (declared) Provides() []plugin.Capability { return []plugin.Capability{"a"} }
func (declared) Requires() []plugin.Capability { return []plugin.Capability{"b"} }
func (declared) Options() any                  { return nil }
func (declared) Version() string               { return "1.0.0" }

// The providers are how the composition learns what a plugin
// declared, so their shapes are contract: each answers data, and a
// plugin declares by satisfying the interface.
func TestProviders(t *testing.T) {
	t.Parallel()

	t.Run("a declaring plugin satisfies every surface", func(t *testing.T) {
		t.Parallel()

		var p plugin.Plugin = declared{name: "stubgen"}
		_, directives := p.(plugin.DirectiveProvider)
		assert.True(t, directives, "the directive surface asserts")
		_, keys := p.(plugin.KeyProvider)
		assert.True(t, keys, "the key surface asserts")
		_, outputs := p.(plugin.OutputProvider)
		assert.True(t, outputs, "the output surface asserts")
		_, options := p.(plugin.OptionsProvider)
		assert.True(t, options, "the options surface asserts")

		caps, ok := p.(plugin.CapabilityProvider)
		assert.True(t, ok, "the capability surface asserts")
		assert.Equal(t, caps.Priority(plugin.RoleGenerator), int(plugin.RoleGenerator),
			"the priority answers per role seat")

		versioned, ok := p.(plugin.Versioned)
		assert.True(t, ok, "the version surface asserts")
		assert.Equal(t, versioned.Version(), "1.0.0", "answering what was declared")
	})

	t.Run("a bare plugin satisfies none of them", func(t *testing.T) {
		t.Parallel()

		var p plugin.Plugin = named{name: "bare"}
		_, directives := p.(plugin.DirectiveProvider)
		assert.False(t, directives, "a plugin that declares nothing asserts nothing")
		_, keys := p.(plugin.KeyProvider)
		assert.False(t, keys, "the key surface is opt-in")
		_, versioned := p.(plugin.Versioned)
		assert.False(t, versioned, "the version surface is opt-in")
	})
}
