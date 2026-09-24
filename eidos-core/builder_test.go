// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos_test

import (
	"testing"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// emitNothing returns a graph rule whose handler does nothing: the
// smallest rule a declaration can carry.
func emitNothing() eidos.Rule {
	return eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
		return nil
	})
}

// stubSchema returns a plugin-owned schema for gate fixtures.
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

		t.Run("implies the roles the rules carry", func(t *testing.T) {
			t.Parallel()

			p := eidos.NewPlugin("planner").Handle(emitNothing()).Build()
			assert.Equal(t, p.Name(), "planner", "the name is the identity")
			_, generates := p.(plugin.Generator)
			assert.True(t, generates, "an emitter rule makes a generator")
			_, annotates := p.(plugin.Annotator)
			assert.False(t, annotates,
				"no stamper rule was declared, so the annotator role does not exist")
		})

		t.Run("collects the schemas the wrappers carry", func(t *testing.T) {
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

			owned, ok := p.(plugin.DirectiveProvider)
			assert.True(t, ok, "carried schemas are returned for registration")
			assert.Equal(t, owned.Directives(), []directive.Schema{s},
				"one wrapper gating two rules registers one schema")
		})

		t.Run("carries the key registration through", func(t *testing.T) {
			t.Parallel()

			p := eidos.NewPlugin("keyed").
				Keys(func(r *meta.Registry) error {
					return r.ClaimNamespace("keyed", "the fixture")
				}).
				Keys(func(r *meta.Registry) error {
					_, err := meta.Register[bool](r, meta.KeySpec{
						Name: "keyed.flag", Doc: "a fixture key",
					})
					return err
				}).
				Handle(emitNothing()).Build()

			kp, ok := p.(plugin.KeyProvider)
			assert.True(t, ok, "the declaration returns through the provider")
			reg := meta.NewRegistry()
			assert.NoError(t, kp.Keys(reg),
				"the declared registrations run in order against the registry")
			_, held := reg.Resolve("keyed.flag")
			assert.True(t, held, "the key arrived in the registry it was handed")
		})

		t.Run("panics on a declaration defect", func(t *testing.T) {
			t.Parallel()

			onEmit := func() eidos.Rule {
				return eidos.OnEmit(symbol.KindStruct,
					func(m *eidos.EmitMatch, e *eidos.Emitter) error { return nil })
			}
			tests := []struct {
				name  string
				build func()
			}{
				{
					name:  "an empty name",
					build: func() { eidos.NewPlugin("").Handle(emitNothing()).Build() },
				},
				{
					name:  "no rules",
					build: func() { eidos.NewPlugin("t").Build() },
				},
				{
					name:  "the zero rule",
					build: func() { eidos.NewPlugin("t").Handle(eidos.Rule{}).Build() },
				},
				{
					name: "a directive wrapper around no rules",
					build: func() {
						eidos.NewPlugin("stubgen").Handle(eidos.Directive(stubSchema("stub"))).Build()
					},
				},
				{
					name: "a fact wrapper around no rules",
					build: func() {
						eidos.NewPlugin("t").Handle(eidos.Where(eidos.Pred{})).Build()
					},
				},
				{
					name: "a kernel gate naming no directive",
					build: func() {
						eidos.NewPlugin("t").Handle(eidos.Gated("", onEmit())).Build()
					},
				},
				{
					name: "a kernel gate naming a plugin's directive",
					build: func() {
						eidos.NewPlugin("t").Handle(eidos.Gated("stubgen:stub", onEmit())).Build()
					},
				},
				{
					name: "a duplicate output tag",
					build: func() {
						eidos.NewPlugin("t").
							Output(plugin.Output{Per: plugin.PerPlan, Word: "a"}).
							Output(plugin.Output{Per: plugin.PerPlan, Word: "b"}).
							Handle(emitNothing()).Build()
					},
				},
				{
					name: "an empty output word",
					build: func() {
						eidos.NewPlugin("t").
							Output(plugin.Output{Per: plugin.PerPlan}).
							Handle(emitNothing()).Build()
					},
				},
				{
					name: "a zero output cardinality",
					build: func() {
						eidos.NewPlugin("t").
							Output(plugin.Output{Word: "a"}).
							Handle(emitNothing()).Build()
					},
				},
				{
					name: "an empty capability label",
					build: func() {
						eidos.NewPlugin("t").Provides("").Handle(emitNothing()).Build()
					},
				},
				{
					name: "a nil key registration",
					build: func() {
						eidos.NewPlugin("t").Keys(nil).Handle(emitNothing()).Build()
					},
				},
				{
					name: "a nil template tree",
					build: func() {
						eidos.NewPlugin("t").
							Templates("stub", nil).Handle(emitNothing()).Build()
					},
				},
				{
					name: "a zero template target",
					build: func() {
						eidos.NewPlugin("t").
							Templates("", stubTree()).Handle(emitNothing()).Build()
					},
				},
				{
					name: "one template target declared twice",
					build: func() {
						eidos.NewPlugin("t").
							Templates("stub", stubTree()).
							Templates("stub", stubTree()).
							Handle(emitNothing()).Build()
					},
				},
				{
					name: "one directive name in two wrappers",
					build: func() {
						eidos.NewPlugin("stubgen").Handle(
							eidos.Directive(stubSchema("stub"), onEmit()),
							eidos.Directive(stubSchema("stub"), onEmit()),
						).Build()
					},
				},
				{
					name: "a directive gate on a graph rule",
					build: func() {
						eidos.NewPlugin("stubgen").Handle(
							eidos.Directive(stubSchema("stub"), emitNothing()),
						).Build()
					},
				},
				{
					name: "a fact gate on a graph rule",
					build: func() {
						eidos.NewPlugin("t").Handle(
							eidos.Where(eidos.Pred{}, emitNothing()),
						).Build()
					},
				},
				{
					name: "a zero predicate",
					build: func() {
						eidos.NewPlugin("t").Handle(
							eidos.Where(eidos.Pred{}, onEmit()),
						).Build()
					},
				},
				{
					name: "a gate on a zero key",
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
					assert.Panics(t, tt.build,
						"a wrong declaration panics on the first Build in any test")
				})
			}
		})
	})
}
