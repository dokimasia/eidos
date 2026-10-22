// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
)

// declared is a fixture plugin that implements every provider.
type declared struct {
	named
}

// Directives returns one fixture schema.
func (declared) Directives() []directive.Schema {
	return []directive.Schema{{Plugin: "stubgen", Name: "stub", Doc: "a fixture"}}
}

// Keys registers nothing.
func (declared) Keys(*meta.Registry) error { return nil }

// BindKeys returns the receiver, which has no named handle.
func (d declared) BindKeys(*meta.Registry) (plugin.Plugin, error) { return d, nil }

// Outputs returns one family for each plan.
func (declared) Outputs() []plugin.Output {
	return []plugin.Output{{Per: plugin.PerPlan, Word: "registry"}}
}

// Priority returns the number of the role r as its priority.
func (declared) Priority(r plugin.Role) int { return int(r) }

// Provides returns the capability a in every role.
func (declared) Provides(plugin.Role) []plugin.Capability { return []plugin.Capability{"a"} }

// Requires returns the capability b.
func (declared) Requires() []plugin.Capability { return []plugin.Capability{"b"} }

// After returns the capability c.
func (declared) After() []plugin.Capability { return []plugin.Capability{"c"} }

// Options returns no options struct.
func (declared) Options() any { return nil }

// Version returns the version 1.0.0.
func (declared) Version() string { return "1.0.0" }

// Policies returns the fixture specs.
func (declared) Policies() []plugin.PolicySpec { return policySpecs() }

// TestProviders tests the provider interfaces, through which the
// composition reads what a plugin declares. Each returns data, and a
// plugin declares by implementing the interface.
func TestProviders(t *testing.T) {
	t.Parallel()

	t.Run("a declaring plugin satisfies every surface", func(t *testing.T) {
		t.Parallel()

		var p plugin.Plugin = declared{name: "stubgen"}
		_, directives := p.(plugin.DirectiveProvider)
		assert.True(t, directives, "the directive surface asserts")
		_, keys := p.(plugin.KeyProvider)
		assert.True(t, keys, "the key surface asserts")
		_, binds := p.(plugin.KeyBinder)
		assert.True(t, binds, "the binding surface asserts")
		_, outputs := p.(plugin.OutputProvider)
		assert.True(t, outputs, "the output surface asserts")
		_, options := p.(plugin.OptionsProvider)
		assert.True(t, options, "the options surface asserts")

		caps, ok := p.(plugin.CapabilityProvider)
		assert.True(t, ok, "the capability surface asserts")
		assert.Equal(t, caps.Priority(plugin.RoleGenerator), int(plugin.RoleGenerator),
			"the priority is read per role")

		versioned, ok := p.(plugin.Versioned)
		assert.True(t, ok, "the version surface asserts")
		assert.Equal(t, versioned.Version(), "1.0.0", "returning what was declared")

		policies, ok := p.(plugin.PolicyProvider)
		assert.True(t, ok, "the policy surface asserts")
		assert.Equal(t, policies.Policies(), policySpecs(), "returning the declared specs")
	})

	t.Run("a bare plugin satisfies none of them", func(t *testing.T) {
		t.Parallel()

		var p plugin.Plugin = named{name: "bare"}
		_, directives := p.(plugin.DirectiveProvider)
		assert.False(t, directives, "a plugin that declares nothing asserts nothing")
		_, keys := p.(plugin.KeyProvider)
		assert.False(t, keys, "the key surface is opt-in")
		_, binds := p.(plugin.KeyBinder)
		assert.False(t, binds, "the binding surface is opt-in")
		_, versioned := p.(plugin.Versioned)
		assert.False(t, versioned, "the version surface is opt-in")
		_, policies := p.(plugin.PolicyProvider)
		assert.False(t, policies, "the policy surface is opt-in")
	})
}
