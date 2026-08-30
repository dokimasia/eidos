// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package eidos_test

import (
	"testing"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/plugin"
)

// A built plugin is the lowered declaration: what the providers
// answer is exactly what was declared, so the composition reads
// truth.
func TestBuilt(t *testing.T) {
	t.Parallel()

	t.Run("answers the declared providers", func(t *testing.T) {
		t.Parallel()

		out := plugin.Output{Per: plugin.PerPlan, Word: "registry"}
		opts := &struct{ Redact bool }{Redact: true}
		p := eidos.NewPlugin("planner").
			Version("1.2.0").
			Output(out).
			Priority(plugin.RoleGenerator, 7).
			Provides("registry").
			Requires("classified").
			Options(opts).
			Handle(emitNothing()).
			Build()

		outputs, ok := p.(plugin.OutputProvider)
		assert.True(t, ok, "declared outputs answer")
		assert.Equal(t, outputs.Outputs(), []plugin.Output{out},
			"as declared, in declaration order")

		caps, ok := p.(plugin.CapabilityProvider)
		assert.True(t, ok, "capabilities answer")
		assert.Equal(t, caps.Priority(plugin.RoleGenerator), 7,
			"the declared role carries its priority")
		assert.Equal(t, caps.Priority(plugin.RoleAnnotator), 0,
			"an undeclared role answers the zero priority")
		assert.Equal(t, caps.Provides(), []plugin.Capability{"registry"},
			"provided labels answer")
		assert.Equal(t, caps.Requires(), []plugin.Capability{"classified"},
			"required labels answer")

		versioned, ok := p.(plugin.Versioned)
		assert.True(t, ok, "the version answers")
		assert.Equal(t, versioned.Version(), "1.2.0", "as declared")

		options, ok := p.(plugin.OptionsProvider)
		assert.True(t, ok, "the options struct answers")
		assert.True(t, options.Options() == any(opts),
			"the same pointer the plugin constructed, defaults intact")
	})
}
