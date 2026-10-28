// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos_test

import (
	"reflect"
	"strings"
	"testing"
	"text/template"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The full declaration states these values for the plugin, its version,
// its priority and its capabilities.
const (
	plannerPlugin    = plugin.ID("planner")
	declaredVersion  = "1.2.0"
	declaredPriority = 7
	providedLabel    = plugin.Capability("registry")
	requiredLabel    = plugin.Capability("classified")
	afterLabel       = plugin.Capability("indexed")
)

// The declaration with both roles provides stampedLabel in its annotator
// role alone, and emittedLabel in its generator role alone.
const (
	stampedLabel = plugin.Capability("stamped")
	emittedLabel = plugin.Capability("emitted")
)

// ghostKeyName is a key name that no fixture registers, in a namespace that
// a fixture claims. absentKeyName is a key name in a namespace that no
// fixture claims.
const (
	ghostKeyName  meta.KeyName = "t.ghost"
	absentKeyName meta.KeyName = "absent.marked"
)

// registryOutput is the one family that the full declaration states.
var registryOutput = plugin.Output{Per: plugin.PerPlan, Word: "registry"}

// plannerOptions is the options struct that the full declaration states.
type plannerOptions struct{ Redact bool }

// TestBuilt tests the lowered declaration. Each provider returns exactly
// what the plugin declares, so the composition reads the declaration.
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

		t.Run("returns the provided labels in the role of a generator", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, capabilities(t).Provides(plugin.RoleGenerator), []plugin.Capability{providedLabel},
				"the provided label is returned")
		})

		tests := []struct {
			name     string
			giveRole plugin.Role
			want     []plugin.Capability
		}{
			{
				name:     "returns the label of ProvidesIn in the annotator role after the label of Provides",
				giveRole: plugin.RoleAnnotator,
				want:     []plugin.Capability{providedLabel, stampedLabel},
			},
			{
				name:     "returns the label of ProvidesIn in the generator role after the label of Provides",
				giveRole: plugin.RoleGenerator,
				want:     []plugin.Capability{providedLabel, emittedLabel},
			},
			{
				name:     "returns no label in the zero role",
				giveRole: 0,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, halves(t).Provides(tt.giveRole), tt.want, "the role has the labels that it declares")
			})
		}
	})

	t.Run("Requires", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the required labels", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, capabilities(t).Requires(), []plugin.Capability{requiredLabel},
				"the required label is returned")
		})
	})

	t.Run("After", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the labels that the plugin runs after", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, capabilities(t).After(), []plugin.Capability{afterLabel},
				"the label that the plugin runs after is returned")
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
			assert.Equal(t, options.Options(), any(opts), "the plugin's own pointer is returned", assert.ByIdentity())
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

	t.Run("Removed", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the finding of the declared function", func(t *testing.T) {
			t.Parallel()

			r, held := eidos.NewPlugin(styledPlugin).Removed(lostLine).Handle(emitNothing()).Generator().(plugin.LineSetReader)
			assert.True(t, held, "the built generator reads removed lines")
			d, report := r.Removed(plugin.RemovedLine{Text: lostText})
			assert.True(t, report, "the function reports the line")
			assert.Equal(t, d.Msg, lostText, "the finding is the one of the function")
		})

		t.Run("reports false without a declared function", func(t *testing.T) {
			t.Parallel()

			r, held := eidos.NewPlugin(styledPlugin).Handle(emitNothing()).Generator().(plugin.LineSetReader)
			assert.True(t, held, "the built generator reads removed lines")
			_, report := r.Removed(plugin.RemovedLine{Text: lostText})
			assert.False(t, report, "no function reports nothing")
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

	t.Run("BindKeys", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a plugin whose gate on a named handle has the id of its name", func(t *testing.T) {
			t.Parallel()

			key, facts := boolKey(t)
			p := gatedPlugin(eidos.HasKey(meta.Named[bool](key.Name())))
			bound, err := binderOf(t, p).BindKeys(facts.Registry())
			assert.NoError(t, err, "the name binds")
			assert.Equal(t, subscribedKeys(t, bound), []meta.KeyID{key.ID()}, "the subscription has the id of the name")
		})

		t.Run("leaves the gate of the receiver unbound", func(t *testing.T) {
			t.Parallel()

			key, facts := boolKey(t)
			p := gatedPlugin(eidos.HasKey(meta.Named[bool](key.Name())))
			_, err := binderOf(t, p).BindKeys(facts.Registry())
			assert.NoError(t, err, "the name binds")
			assert.Equal(t, subscribedKeys(t, p), []meta.KeyID{0}, "the subscription of the receiver has no id")
		})

		t.Run("returns the receiver for a plugin without a named handle", func(t *testing.T) {
			t.Parallel()

			key, facts := boolKey(t)
			p := gatedPlugin(eidos.HasKey(key))
			bound, err := binderOf(t, p).BindKeys(facts.Registry())
			assert.NoError(t, err, "nothing binds")
			assert.Equal(t, bound, p, "the receiver is returned", assert.ByIdentity())
		})

		roles := []struct {
			name string
			give func(eidos.Pred) plugin.Plugin
		}{
			{
				name: "returns an annotator for an annotator",
				give: func(pred eidos.Pred) plugin.Plugin {
					return eidos.NewPlugin(plannerPlugin).
						Handle(eidos.Where(pred, eidos.OnStruct(nothing[*eidos.Stamper]))).
						Annotator()
				},
			},
			{name: "returns a generator for a generator", give: gatedPlugin},
			{
				name: "returns a dual plugin for a dual plugin",
				give: func(pred eidos.Pred) plugin.Plugin {
					return eidos.NewPlugin(plannerPlugin).
						Handle(eidos.Where(pred,
							eidos.OnStruct(nothing[*eidos.Stamper]),
							eidos.OnStruct(nothing[*eidos.Emitter]))).
						Dual()
				},
			},
		}
		for _, tt := range roles {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				key, facts := boolKey(t)
				p := tt.give(eidos.HasKey(meta.Named[bool](key.Name())))
				bound, err := binderOf(t, p).BindKeys(facts.Registry())
				assert.NoError(t, err, "the name binds")
				assert.Equal(t, reflect.TypeOf(bound), reflect.TypeOf(p),
					"the bound plugin has the type of the receiver")
			})
		}

		t.Run("returns an error naming the plugin and the key of a name that nothing registers", func(t *testing.T) {
			t.Parallel()

			_, facts := boolKey(t)
			_, err := binderOf(t, gatedPlugin(eidos.HasKey(meta.Named[bool](ghostKeyName)))).BindKeys(facts.Registry())
			assert.HasError(t, err, "the bind of the name fails")
			expect.That(t, err.Error()).
				Contains(string(plannerPlugin), "the error names the plugin").
				Contains(string(ghostKeyName), "the error names the key").
				Contains("nothing registers", "the error states the cause")
			expect.That(t, err).ErrorIsNot(plugin.ErrUnclaimedNamespace,
				"a registrant claims the namespace of the name")
		})

		t.Run("returns an error that wraps ErrUnclaimedNamespace for a name whose namespace nothing claims",
			func(t *testing.T) {
				t.Parallel()

				_, facts := boolKey(t)
				p := gatedPlugin(eidos.HasKey(meta.Named[bool](absentKeyName)))
				_, err := binderOf(t, p).BindKeys(facts.Registry())
				assert.ErrorIs(t, err, plugin.ErrUnclaimedNamespace, "no registrant claims the namespace of the name")
				expect.That(t, err.Error()).
					Contains(string(plannerPlugin), "the error names the plugin").
					Contains(string(absentKeyName), "the error names the key")
			})

		t.Run("leaves the gate on a name whose namespace nothing claims unbound", func(t *testing.T) {
			t.Parallel()

			_, facts := boolKey(t)
			p := gatedPlugin(eidos.HasKey(meta.Named[bool](absentKeyName)))
			bound, err := binderOf(t, p).BindKeys(facts.Registry())
			assert.HasError(t, err, "the bind of the name fails")
			assert.Equal(t, subscribedKeys(t, bound), []meta.KeyID{0}, "the subscription of the bound plugin has no id")
		})

		t.Run("returns an error for a name that the registry has under another value type", func(t *testing.T) {
			t.Parallel()

			key, facts := boolKey(t)
			_, err := binderOf(t, gatedPlugin(eidos.HasKey(meta.Named[string](key.Name())))).BindKeys(facts.Registry())
			assert.HasError(t, err, "the name has another value type")
			assert.Contains(t, err.Error(), "another value type than string", "the error states the type of the handle")
		})

		t.Run("returns one error for a name that two rules gate on", func(t *testing.T) {
			t.Parallel()

			_, facts := boolKey(t)
			p := eidos.NewPlugin(plannerPlugin).
				Handle(eidos.Where(eidos.HasKey(meta.Named[bool](ghostKeyName)),
					eidos.OnEmit(symbol.KindStruct, func(*eidos.EmitMatch, *eidos.Emitter) error { return nil }),
					eidos.OnEmit(symbol.KindMethod, func(*eidos.EmitMatch, *eidos.Emitter) error { return nil }),
				)).
				Generator()
			_, err := binderOf(t, p).BindKeys(facts.Registry())
			assert.HasError(t, err, "the bind of the name fails")
			assert.Equal(t, strings.Count(err.Error(), string(ghostKeyName)), 1, "the name is refused once")
		})
	})
}

// declared builds a plugin that declares every provider, with opts as its
// options struct.
func declared(opts *plannerOptions) plugin.Plugin {
	return eidos.NewPlugin(plannerPlugin).
		Version(declaredVersion).
		Output(registryOutput).
		Priority(plugin.RoleGenerator, declaredPriority).
		Provides(providedLabel).
		Requires(requiredLabel).
		After(afterLabel).
		Options(opts).
		Handle(emitNothing()).
		Generator()
}

// capabilities returns the full declaration as a
// [plugin.CapabilityProvider].
func capabilities(tb assert.TB) plugin.CapabilityProvider {
	tb.Helper()

	caps, held := declared(&plannerOptions{}).(plugin.CapabilityProvider)
	assert.True(tb, held, "the capabilities are returned")
	return caps
}

// halves returns a declaration with both roles as a
// [plugin.CapabilityProvider]. It provides providedLabel in both roles,
// stampedLabel in the annotator role and emittedLabel in the generator
// role.
func halves(tb assert.TB) plugin.CapabilityProvider {
	tb.Helper()

	p := eidos.NewPlugin(plannerPlugin).
		Provides(providedLabel).
		ProvidesIn(plugin.RoleAnnotator, stampedLabel).
		ProvidesIn(plugin.RoleGenerator, emittedLabel).
		Handle(eidos.OnStruct(nothing[*eidos.Stamper]), eidos.OnStruct(nothing[*eidos.Emitter])).
		Dual()
	caps, held := p.(plugin.CapabilityProvider)
	assert.True(tb, held, "the capabilities are returned")
	return caps
}

// gatedPlugin builds a generator with one emit rule under a gate on pred.
func gatedPlugin(pred eidos.Pred) plugin.Plugin {
	return eidos.NewPlugin(plannerPlugin).
		Handle(eidos.Where(pred,
			eidos.OnEmit(symbol.KindStruct, func(*eidos.EmitMatch, *eidos.Emitter) error { return nil }))).
		Generator()
}

// binderOf returns a built plugin as a [plugin.KeyBinder], and fails the
// test where the plugin does not implement it.
func binderOf(tb assert.TB, p plugin.Plugin) plugin.KeyBinder {
	tb.Helper()

	binder, binds := p.(plugin.KeyBinder)
	assert.True(tb, binds, "a built plugin binds its gates")
	return binder
}

// subscribedKeys returns the fact keys of the subscriptions of a built
// plugin, in subscription order.
func subscribedKeys(tb assert.TB, p plugin.Plugin) []meta.KeyID {
	tb.Helper()

	subscribed, declares := p.(plugin.Subscribed)
	assert.True(tb, declares, "a built plugin declares its gates")
	subscriptions := subscribed.Subscriptions()
	out := make([]meta.KeyID, 0, len(subscriptions))
	for _, s := range subscriptions {
		out = append(out, s.FactKey)
	}
	return out
}
