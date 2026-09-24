// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos_test

import (
	"testing"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
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

	t.Run("File", func(t *testing.T) {
		t.Parallel()

		t.Run("keys one accumulator per subject's source file", func(t *testing.T) {
			t.Parallel()

			g, alpha, beta := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)

			p := eidos.NewPlugin("perfile").
				Output(plugin.Output{Per: plugin.PerSource, Word: "impl"}).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					e.File().Append(&emit.Struct{
						Origin: m.Struct.Identity(), Name: "Gen" + m.Struct.Name,
					})
					return nil
				})).
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")

			byKey := map[string]plugin.Unit{}
			for u := range ctx.Emit.Units() {
				byKey[u.Key] = u
			}
			assert.Length(t, byKey, 2,
				"two subjects in two files assemble two units")
			assert.Equal(t, byKey[alpha.Pos.File].Per, plugin.PerSource,
				"each is keyed per source")
			assert.Length(t, byKey[alpha.Pos.File].Decls, 1,
				"the first file's unit took only its own subject")
			assert.Length(t, byKey[beta.Pos.File].Decls, 1,
				"and the second file's only its own")
		})

		t.Run("keys the empty string for a subject with no position", func(t *testing.T) {
			t.Parallel()

			placed := coretest.Struct(coretest.StorePath, "Placed")
			placed.Pos = position.Pos{File: "placed.go", Line: 1, Col: 1}
			loose := coretest.Struct(coretest.StorePath, "Loose")
			g := coretest.Frozen(t,
				coretest.Package(coretest.StorePath, placed, loose))
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)

			p := eidos.NewPlugin("perfile").
				Output(plugin.Output{Per: plugin.PerSource, Word: "impl"}).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					e.File().Append(&emit.Struct{
						Origin: m.Struct.Identity(), Name: "Gen" + m.Struct.Name,
					})
					return nil
				})).
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")

			keys := map[string]int{}
			for u := range ctx.Emit.Units() {
				keys[u.Key] = len(u.Decls)
			}
			assert.Equal(t, keys[""], 1,
				"a subject carrying no position keys the empty string "+
					"rather than joining another file's unit")
			assert.Equal(t, keys[placed.Pos.File], 1,
				"and the positioned subject keeps its own file")
		})
	})

	t.Run("PackageFile", func(t *testing.T) {
		t.Parallel()

		t.Run("keys one accumulator per language and package path", func(t *testing.T) {
			t.Parallel()

			const otherLang symbol.Lang = "other"
			native := coretest.Struct(coretest.StorePath, "Native")
			foreign := coretest.Struct(coretest.StorePath, "Foreign")
			foreign.ID.Lang = otherLang
			samePath := coretest.Package(coretest.StorePath, foreign)
			samePath.ID.Lang = otherLang
			samePath.Files[0].ID.Lang = otherLang
			g := coretest.Frozen(t, coretest.Package(coretest.StorePath, native), samePath)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)

			p := eidos.NewPlugin("perpkg").
				Output(plugin.Output{Per: plugin.PerPackage, Word: "audit"}).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					e.PackageFile().Append(&emit.Struct{
						Origin: m.Struct.Identity(), Name: "For" + m.Struct.Name,
					})
					return nil
				})).
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")

			var langs []symbol.Lang
			for u := range ctx.Emit.Units() {
				assert.Equal(t, u.Key, coretest.StorePath, "every unit keys the one package path")
				assert.Length(t, u.Decls, 1, "and holds its own language's subject alone")
				langs = append(langs, u.Pkg.Lang)
			}
			assert.Equal(t, langs, []symbol.Lang{coretest.Lang, otherLang},
				"two languages spelling one path assemble two units, in package order")
		})
	})
}
