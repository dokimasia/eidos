// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos_test

import (
	"testing"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The emitter owns the accumulator bookkeeping: family misuse is a
// defect that panics, and an empty append changes nothing.
func TestEmitter(t *testing.T) {
	t.Parallel()

	t.Run("Out", func(t *testing.T) {
		t.Parallel()

		t.Run("panics on more than one tag", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			p := eidos.NewPlugin("t").
				Output(plugin.Output{Per: plugin.PerPlan, Word: "a"}).
				Output(plugin.Output{Tag: "x", Per: plugin.PerPlan, Word: "b"}).
				Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
					e.PlanFile("x", "x")
					return nil
				})).
				Build()
			assert.Panics(t, func() {
				_ = generatorOf(t, p).Generate(genContext(t, g, facts, nil))
			}, "two tags address nothing")
		})

		t.Run("panics on the wrong cardinality", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			p := eidos.NewPlugin("t").
				Output(plugin.Output{Per: plugin.PerSource, Word: "stub"}).
				Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
					e.PlanFile()
					return nil
				})).
				Build()
			assert.Panics(t, func() {
				_ = generatorOf(t, p).Generate(genContext(t, g, facts, nil))
			}, "a per-source family is not a plan file")
		})

		t.Run("two live handles in one invocation stay distinct", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)

			p := eidos.NewPlugin("splitter").
				Output(plugin.Output{Per: plugin.PerPlan, Word: "audit"}).
				Output(plugin.Output{Tag: "aux", Per: plugin.PerPackage, Word: "aux"}).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					plan := e.PlanFile()
					aux := e.PackageFile("aux")
					plan.Append(&emit.Struct{
						Origin: m.Struct.Identity(), Name: "Plan" + m.Struct.Name,
					})
					aux.Append(&emit.Struct{
						Origin: m.Struct.Identity(), Name: "Aux" + m.Struct.Name,
					})
					return nil
				})).
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")

			byTag := map[string]plugin.Unit{}
			for u := range ctx.Emit.Units() {
				byTag[u.Tag] = u
			}
			assert.Length(t, byTag, 2, "each family assembled its own unit")
			assert.Length(t, byTag[""].Decls, 2,
				"the plan handle took only its own appends")
			assert.Length(t, byTag["aux"].Decls, 2, "and the aux handle its own")
		})

		t.Run("handles past the pool still arrive apart", func(t *testing.T) {
			t.Parallel()

			g, _, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)

			tags := []eidos.Tag{"", "b", "c", "d", "e", "f"}
			b := eidos.NewPlugin("fanout")
			for _, tag := range tags {
				b.Output(plugin.Output{
					Tag: string(tag), Per: plugin.PerPlan, Word: "w" + string(tag),
				})
			}
			p := b.Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
				handles := make([]*eidos.Out, 0, len(tags))
				for _, tag := range tags {
					handles = append(handles, e.PlanFile(tag))
				}
				for i, h := range handles {
					h.Append(&emit.Struct{
						Origin: m.Struct.Identity(),
						Name:   string(tags[i]) + m.Struct.Name,
					})
				}
				return nil
			})).Build()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")

			units := 0
			for u := range ctx.Emit.Units() {
				units++
				assert.Length(t, u.Decls, 2, "every family took its two appends")
				first, held := u.Decls[0].(*emit.Struct)
				assert.True(t, held, "the fixture emits structs")
				assert.HasPrefix(t, first.Name, u.Tag,
					"each append arrived under the handle that made it")
			}
			assert.Equal(t, units, len(tags), "one unit per family")
		})

		t.Run("an empty append records nothing", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			seed(t, ctx, emitted(alpha))

			p := eidos.NewPlugin("weaver").
				Output(plugin.Output{Per: plugin.PerPlan, Word: "audit"}).
				Handle(eidos.OnEmit(symbol.KindStruct,
					func(m *eidos.EmitMatch, e *eidos.Emitter) error {
						e.PlanFile().Append()
						return nil
					})).
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")

			var got plugin.Unit
			found := false
			for u := range ctx.Emit.Units() {
				if u.Plugin == "weaver" {
					got, found = u, true
				}
			}
			assert.True(t, found, "the touched accumulator still flushes")
			assert.Length(t, got.Decls, 0, "holding nothing")
			assert.Length(t, got.Origins, 0,
				"an empty append fabricates no provenance")
		})
	})
}
