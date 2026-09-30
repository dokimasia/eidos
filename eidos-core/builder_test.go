// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos_test

import (
	"testing"
	"text/template"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// emitNothing returns a graph rule whose handler does nothing: the
// smallest rule a plugin can declare.
func emitNothing() eidos.Rule {
	return eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
		return nil
	})
}

// stubSchema returns a schema a plugin registers, for gate
// fixtures.
func stubSchema(name directive.Name) directive.Schema {
	return directive.Schema{
		Plugin: "stubgen", Name: name,
		Doc: "a fixture directive",
	}
}

// A plugin declaration is a value and Build freezes it: what it
// panics on, which roles the rules imply, and what the providers
// return are all contract.
func TestBuilder(t *testing.T) {
	t.Parallel()

	t.Run("Build", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a plugin named by the declaration", func(t *testing.T) {
			t.Parallel()

			p := eidos.NewPlugin("planner").Handle(emitNothing()).Build()
			assert.Equal(t, p.Name(), "planner", "the name is the declared one")
		})

		t.Run("returns a generator for an emitter rule", func(t *testing.T) {
			t.Parallel()

			_, generates := eidos.NewPlugin("planner").Handle(emitNothing()).Build().(plugin.Generator)
			assert.True(t, generates, "the plugin implements the generator role")
		})

		t.Run("returns no annotator without a stamper rule", func(t *testing.T) {
			t.Parallel()

			_, annotates := eidos.NewPlugin("planner").Handle(emitNothing()).Build().(plugin.Annotator)
			assert.False(t, annotates, "the plugin does not implement the annotator role")
		})

		t.Run("returns one schema for a wrapper that gates two rules", func(t *testing.T) {
			t.Parallel()

			s := stubSchema("stub")
			p := eidos.NewPlugin("stubgen").
				Handle(eidos.Directive(s,
					eidos.OnEmit(symbol.KindStruct,
						func(m *eidos.EmitMatch, e *eidos.Emitter) error { return nil }),
					eidos.OnEmit(symbol.KindMethod,
						func(m *eidos.EmitMatch, e *eidos.Emitter) error { return nil }),
				)).
				Build()

			provider, ok := p.(plugin.DirectiveProvider)
			assert.True(t, ok, "the plugin provides directives")
			assert.Equal(t, provider.Directives(), []directive.Schema{s}, "the schema is returned once")
		})

		t.Run("returns a key provider that runs the declared registrations in order", func(t *testing.T) {
			t.Parallel()

			p := eidos.NewPlugin("keyed").
				Keys(func(r *meta.Registry) error {
					return r.ClaimNamespace("keyed")
				}).
				Keys(func(r *meta.Registry) error {
					_, err := meta.Register[bool](r, meta.KeySpec{
						Name: "keyed.flag", Doc: "a fixture key",
					})
					return err
				}).
				Handle(emitNothing()).Build()

			kp, ok := p.(plugin.KeyProvider)
			assert.True(t, ok, "the plugin provides keys")
			reg := meta.NewRegistry()
			assert.NoError(t, kp.Keys(reg), "the registrations run against the registry")
			_, held := reg.Resolve("keyed.flag")
			assert.True(t, held, "the key is in the registry")
		})

		onEmit := func() eidos.Rule {
			return eidos.OnEmit(symbol.KindStruct,
				func(m *eidos.EmitMatch, e *eidos.Emitter) error { return nil })
		}
		tests := []struct {
			name  string
			build func()
		}{
			{
				name:  "panics on an empty name",
				build: func() { eidos.NewPlugin("").Handle(emitNothing()).Build() },
			},
			{
				name:  "panics on a declaration without rules",
				build: func() { eidos.NewPlugin("t").Build() },
			},
			{
				name:  "panics on the zero rule",
				build: func() { eidos.NewPlugin("t").Handle(eidos.Rule{}).Build() },
			},
			{
				name: "panics on a directive wrapper around no rules",
				build: func() {
					eidos.NewPlugin("stubgen").Handle(eidos.Directive(stubSchema("stub"))).Build()
				},
			},
			{
				name: "panics on a fact wrapper around no rules",
				build: func() {
					eidos.NewPlugin("t").Handle(eidos.Where(eidos.Pred{})).Build()
				},
			},
			{
				name: "panics on a kernel gate naming no directive",
				build: func() {
					eidos.NewPlugin("t").Handle(eidos.Gated("", onEmit())).Build()
				},
			},
			{
				name: "panics on a kernel gate naming a plugin's directive",
				build: func() {
					eidos.NewPlugin("t").Handle(eidos.Gated("stubgen:stub", onEmit())).Build()
				},
			},
			{
				name: "panics on a duplicate output tag",
				build: func() {
					eidos.NewPlugin("t").
						Output(plugin.Output{Per: plugin.PerPlan, Word: "a"}).
						Output(plugin.Output{Per: plugin.PerPlan, Word: "b"}).
						Handle(emitNothing()).Build()
				},
			},
			{
				name: "panics on an empty output word",
				build: func() {
					eidos.NewPlugin("t").
						Output(plugin.Output{Per: plugin.PerPlan}).
						Handle(emitNothing()).Build()
				},
			},
			{
				name: "panics on a zero output cardinality",
				build: func() {
					eidos.NewPlugin("t").
						Output(plugin.Output{Word: "a"}).
						Handle(emitNothing()).Build()
				},
			},
			{
				name: "panics on an empty capability label",
				build: func() {
					eidos.NewPlugin("t").Provides("").Handle(emitNothing()).Build()
				},
			},
			{
				name: "panics on a nil key registration",
				build: func() {
					eidos.NewPlugin("t").Keys(nil).Handle(emitNothing()).Build()
				},
			},
			{
				name: "panics on a nil plugin-level tree",
				build: func() {
					eidos.NewPlugin("t").Templates(nil).Handle(emitNothing()).Build()
				},
			},
			{
				name: "panics on a second plugin-level tree",
				build: func() {
					eidos.NewPlugin("t").
						Templates(stubTree()).Templates(stubTree()).
						Handle(emitNothing()).Build()
				},
			},
			{
				name: "panics on a nil helper map",
				build: func() {
					eidos.NewPlugin("t").Funcs(nil).Handle(emitNothing()).Build()
				},
			},
			{
				name: "panics on a helper name declared twice at plugin level",
				build: func() {
					eidos.NewPlugin("t").
						Funcs(template.FuncMap{toneHelper: shout}).
						Funcs(template.FuncMap{toneHelper: whisper}).
						Handle(emitNothing()).Build()
				},
			},
			{
				name: "panics on a helper text/template refuses",
				build: func() {
					eidos.NewPlugin("t").
						Funcs(template.FuncMap{toneHelper: 1}).
						Handle(emitNothing()).Build()
				},
			},
			{
				name: "panics on presentation for the zero target",
				build: func() {
					eidos.NewPlugin("t").
						For("", eidos.Templates(stubTree())).
						Handle(emitNothing()).Build()
				},
			},
			{
				name: "panics on one target in two For declarations",
				build: func() {
					eidos.NewPlugin("t").
						For(stubTarget, eidos.Templates(stubTree())).
						For(stubTarget, eidos.Funcs(template.FuncMap{toneHelper: shout})).
						Handle(emitNothing()).Build()
				},
			},
			{
				name: "panics on a For without options",
				build: func() {
					eidos.NewPlugin("t").For(stubTarget).Handle(emitNothing()).Build()
				},
			},
			{
				name: "panics on the zero target option",
				build: func() {
					eidos.NewPlugin("t").
						For(stubTarget, eidos.TargetOption{}).
						Handle(emitNothing()).Build()
				},
			},
			{
				name: "panics on a nil tree for a target",
				build: func() {
					eidos.NewPlugin("t").
						For(stubTarget, eidos.Templates(nil)).
						Handle(emitNothing()).Build()
				},
			},
			{
				name: "panics on two trees for one target",
				build: func() {
					eidos.NewPlugin("t").
						For(stubTarget, eidos.Templates(stubTree()), eidos.Templates(stubTree())).
						Handle(emitNothing()).Build()
				},
			},
			{
				name: "panics on a nil override map",
				build: func() {
					eidos.NewPlugin("t").
						For(stubTarget, eidos.Overrides(nil)).
						Handle(emitNothing()).Build()
				},
			},
			{
				name: "panics on one name as a helper of a target and its override",
				build: func() {
					eidos.NewPlugin("t").
						For(stubTarget,
							eidos.Funcs(template.FuncMap{toneHelper: shout}),
							eidos.Overrides(template.FuncMap{toneHelper: whisper})).
						Handle(emitNothing()).Build()
				},
			},
			{
				name: "panics on one directive name in two wrappers",
				build: func() {
					eidos.NewPlugin("stubgen").Handle(
						eidos.Directive(stubSchema("stub"), onEmit()),
						eidos.Directive(stubSchema("stub"), onEmit()),
					).Build()
				},
			},
			{
				name: "panics on a directive gate on a graph rule",
				build: func() {
					eidos.NewPlugin("stubgen").Handle(
						eidos.Directive(stubSchema("stub"), emitNothing()),
					).Build()
				},
			},
			{
				name: "panics on a fact gate on a graph rule",
				build: func() {
					eidos.NewPlugin("t").Handle(
						eidos.Where(eidos.Pred{}, emitNothing()),
					).Build()
				},
			},
			{
				name: "panics on a zero predicate",
				build: func() {
					eidos.NewPlugin("t").Handle(
						eidos.Where(eidos.Pred{}, onEmit()),
					).Build()
				},
			},
			{
				name: "panics on a gate on a zero key",
				build: func() {
					var unregistered meta.Key[bool]
					eidos.NewPlugin("t").Handle(
						eidos.Where(eidos.HasKey(unregistered), onEmit()),
					).Build()
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Panics(t, tt.build, "the declaration defect panics at Build")
			})
		}
	})
}
