// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package eidos_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/plugin"
)

// stubTree returns a one-template tree for template declarations.
func stubTree() fstest.MapFS {
	return fstest.MapFS{
		"body.tmpl": &fstest.MapFile{Data: []byte("ok")},
	}
}

// A built plugin is the lowered declaration: what the providers
// answer is exactly what was declared, so the composition reads
// truth.
func TestBuilt(t *testing.T) {
	t.Parallel()

	t.Run("returns the declared providers", func(t *testing.T) {
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
		assert.True(t, ok, "the declared outputs are returned")
		assert.Equal(t, outputs.Outputs(), []plugin.Output{out},
			"as declared, in declaration order")

		caps, ok := p.(plugin.CapabilityProvider)
		assert.True(t, ok, "the capabilities are returned")
		assert.Equal(t, caps.Priority(plugin.RoleGenerator), 7,
			"the declared role carries its priority")
		assert.Equal(t, caps.Priority(plugin.RoleAnnotator), 0,
			"an undeclared role returns the zero priority")
		assert.Equal(t, caps.Provides(), []plugin.Capability{"registry"},
			"the provided labels are returned")
		assert.Equal(t, caps.Requires(), []plugin.Capability{"classified"},
			"the required labels are returned")

		versioned, ok := p.(plugin.Versioned)
		assert.True(t, ok, "the version returns")
		assert.Equal(t, versioned.Version(), "1.2.0", "as declared")

		options, ok := p.(plugin.OptionsProvider)
		assert.True(t, ok, "the options struct returns")
		assert.True(t, options.Options() == any(opts),
			"the same pointer the plugin constructed, defaults intact")
	})

	t.Run("returns the declared template tree", func(t *testing.T) {
		t.Parallel()

		p := eidos.NewPlugin("planner").
			Templates("stub", stubTree()).
			Handle(emitNothing()).
			Build()

		tp, ok := p.(plugin.TemplateProvider)
		assert.True(t, ok, "the built value returns the provider")
		tree, held := tp.Templates("stub")
		assert.True(t, held, "the declared target returns its tree")
		src, err := fs.ReadFile(tree, "body.tmpl")
		assert.NoError(t, err, "the returned tree is readable")
		assert.Equal(t, string(src), "ok", "same tree, same bytes")

		_, held = tp.Templates("other")
		assert.False(t, held, "an undeclared target returns absent")
		assert.True(t, tp.TemplateFuncs("stub") == nil,
			"the facade carries no helper declaration")
		assert.True(t, tp.Overrides() == nil,
			"the facade declares no overrides")
	})
}
