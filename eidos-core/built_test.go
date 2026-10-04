// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos_test

import (
	"testing"
	"text/template"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/plugin"
)

// The declared-surface fixture's values: the plugin, its version,
// its priority and the capability labels the full declaration
// states.
const (
	plannerPlugin    = plugin.ID("planner")
	declaredVersion  = "1.2.0"
	declaredPriority = 7
	providedLabel    = plugin.Capability("registry")
	requiredLabel    = plugin.Capability("classified")
)

// registryOutput is the one family the full declaration states.
var registryOutput = plugin.Output{Per: plugin.PerPlan, Word: "registry"}

// plannerOptions is the options struct the full declaration states.
type plannerOptions struct{ Redact bool }

// A built plugin is the lowered declaration: what the providers
// return is exactly what was declared, so the composition reads
// truth.
func TestBuilt(t *testing.T) {
	t.Parallel()

	t.Run("Outputs", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declared families in declaration order", func(t *testing.T) {
			t.Parallel()

			outputs, held := declared(&plannerOptions{}).(plugin.OutputProvider)
			assert.True(t, held, "the declared outputs are returned")
			assert.Equal(t, outputs.Outputs(), []plugin.Output{registryOutput}, "the family is the declared one")
		})
	})

	t.Run("Priority", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declared priority of a role", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, capabilities(t).Priority(plugin.RoleGenerator), declaredPriority,
				"the declared role has its priority")
		})

		t.Run("returns zero for an undeclared role", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, capabilities(t).Priority(plugin.RoleAnnotator), 0,
				"an undeclared role has the zero priority")
		})
	})

	t.Run("Provides", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the provided labels", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, capabilities(t).Provides(), []plugin.Capability{providedLabel},
				"the provided label is returned")
		})
	})

	t.Run("Requires", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the required labels", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, capabilities(t).Requires(), []plugin.Capability{requiredLabel},
				"the required label is returned")
		})
	})

	t.Run("Version", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declared version", func(t *testing.T) {
			t.Parallel()

			versioned, held := declared(&plannerOptions{}).(plugin.Versioned)
			assert.True(t, held, "the version is returned")
			assert.Equal(t, versioned.Version(), declaredVersion, "the version is the declared one")
		})
	})

	t.Run("Options", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the pointer the plugin constructed", func(t *testing.T) {
			t.Parallel()

			opts := &plannerOptions{Redact: true}
			options, held := declared(opts).(plugin.OptionsProvider)
			assert.True(t, held, "the options struct is returned")
			assert.True(t, options.Options() == any(opts), "the defaults are intact")
		})
	})

	t.Run("Templates", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the plugin-level tree for any target", func(t *testing.T) {
			t.Parallel()

			tree, held := provider(t, eidos.NewPlugin(styledPlugin).Templates(stubTree())).Templates(otherTarget)
			assert.True(t, held, "the plugin-level tree serves every target")
			assert.Equal(t, readTree(t, tree), pluginText, "the tree is the declared one")
		})

		t.Run("reports false without a tree", func(t *testing.T) {
			t.Parallel()

			_, held := provider(t, eidos.NewPlugin(styledPlugin)).Templates(stubTarget)
			assert.False(t, held, "nothing serves the target")
		})

		t.Run("reports false for a target outside the declared ones", func(t *testing.T) {
			t.Parallel()

			tp := provider(t, eidos.NewPlugin(styledPlugin).For(stubTarget, eidos.Templates(stubTree())))
			_, held := tp.Templates(otherTarget)
			assert.False(t, held, "a tree for one target serves no other")
		})
	})

	t.Run("TemplateTargets", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the targets with trees of their own in order", func(t *testing.T) {
			t.Parallel()

			tp := provider(t, eidos.NewPlugin(styledPlugin).
				For(stubTarget, eidos.Templates(stubTree())).
				For(otherTarget, eidos.Templates(stubTree())))
			assert.Equal(t, tp.TemplateTargets(), []plugin.Target{otherTarget, stubTarget},
				"the targets are sorted")
		})

		t.Run("returns nothing for a plugin-level tree", func(t *testing.T) {
			t.Parallel()

			tp := provider(t, eidos.NewPlugin(styledPlugin).Templates(stubTree()))
			assert.Empty(t, tp.TemplateTargets(), "no target has a tree of its own")
		})

		t.Run("returns nothing for a target declaring helpers alone", func(t *testing.T) {
			t.Parallel()

			tp := provider(t, eidos.NewPlugin(styledPlugin).
				For(stubTarget, eidos.Funcs(template.FuncMap{toneHelper: shout})))
			assert.Empty(t, tp.TemplateTargets(), "helpers alone declare no tree")
		})
	})

	t.Run("TemplateFuncs", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the plugin-level helpers for a target For never named", func(t *testing.T) {
			t.Parallel()

			tp := provider(t, eidos.NewPlugin(styledPlugin).Funcs(template.FuncMap{toneHelper: shout}))
			assert.Equal(t, toneOf(t, tp.TemplateFuncs(otherTarget)), shout(toneInput),
				"the plugin-level helper serves the target")
		})

		t.Run("returns nothing without helpers", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, provider(t, eidos.NewPlugin(styledPlugin)).TemplateFuncs(stubTarget),
				"no helper is declared")
		})

		t.Run("returns nil for a target that declares a tree alone", func(t *testing.T) {
			t.Parallel()

			tp := provider(t, eidos.NewPlugin(styledPlugin).For(stubTarget, eidos.Templates(stubTree())))
			assert.Nil(t, tp.TemplateFuncs(stubTarget), "a layer without helpers returns nil")
		})
	})

	t.Run("Overrides", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nothing without overrides", func(t *testing.T) {
			t.Parallel()

			tp := provider(t, eidos.NewPlugin(styledPlugin).
				For(stubTarget, eidos.Funcs(template.FuncMap{toneHelper: shout})))
			assert.Empty(t, tp.Overrides(stubTarget), "no override is declared")
		})
	})
}

// declared builds a plugin stating every provider surface, with opts
// as its options struct.
func declared(opts *plannerOptions) plugin.Plugin {
	return eidos.NewPlugin(plannerPlugin).
		Version(declaredVersion).
		Output(registryOutput).
		Priority(plugin.RoleGenerator, declaredPriority).
		Provides(providedLabel).
		Requires(requiredLabel).
		Options(opts).
		Handle(emitNothing()).
		Build()
}

// capabilities returns the capability surface of the full
// declaration.
func capabilities(tb assert.TB) plugin.CapabilityProvider {
	tb.Helper()

	caps, held := declared(&plannerOptions{}).(plugin.CapabilityProvider)
	assert.True(tb, held, "the capabilities are returned")
	return caps
}
