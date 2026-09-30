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

// refPlugin is the plugin whose handler asks the reference fixture
// for a reference.
const refPlugin plugin.ID = "referrer"

// The reference fixture's template name and the payload it passes.
const (
	refTemplate = "method1.tpl"
	refPayload  = "payload"
)

// referenced returns the reference a graph handler of refPlugin asks
// its emitter for.
func referenced(t *testing.T) *emit.TemplateRef {
	t.Helper()

	g, _, _ := fixtureGraph(t)
	_, facts := boolKey(t)
	var got *emit.TemplateRef
	p := eidos.NewPlugin(refPlugin).
		Handle(eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
			got = e.Ref(refTemplate, refPayload)
			return nil
		})).
		Build()
	assert.NoError(t, generatorOf(t, p).Generate(genContext(t, g, facts, nil)), "the phase call passes")
	assert.NotNil(t, got, "the handler ran")
	return got
}

// The emitter is the handler's write surface: family misuse is a
// defect that panics, every family has its own handle, an empty
// append changes nothing, and the spellings a target decides arrive
// through it.
func TestEmitter(t *testing.T) {
	t.Parallel()

	t.Run("PlanFile", func(t *testing.T) {
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

		t.Run("returns a distinct handle per family in one invocation", func(t *testing.T) {
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
			assert.Length(t, byTag[""].Decls, 2, "the plan handle took only its own appends")
			assert.Length(t, byTag["aux"].Decls, 2, "the aux handle took only its own appends")
		})

		t.Run("returns distinct handles past the handle pool", func(t *testing.T) {
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
	})

	t.Run("Append", func(t *testing.T) {
		t.Parallel()

		t.Run("records nothing for no declarations", func(t *testing.T) {
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
			assert.Length(t, got.Decls, 0, "the unit has no declaration")
			assert.Length(t, got.Origins, 0, "an empty append fabricates no provenance")
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
			assert.Length(t, byKey, 2, "two subjects in two files assemble two units")
			assert.Equal(t, byKey[alpha.Pos.File].Per, plugin.PerSource, "each is keyed per source")
			assert.Length(t, byKey[alpha.Pos.File].Decls, 1,
				"the first file's unit took only its own subject")
			assert.Length(t, byKey[beta.Pos.File].Decls, 1,
				"the second file's unit took only its own subject")
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
				"a subject with no position keys the empty string and joins no other file's unit")
			assert.Equal(t, keys[placed.Pos.File], 1, "the positioned subject keeps its own file")
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
				assert.Length(t, u.Decls, 1, "each unit has its own language's subject alone")
				langs = append(langs, u.Pkg.Lang)
			}
			assert.Equal(t, langs, []symbol.Lang{coretest.Lang, otherLang},
				"two languages spelling one path assemble two units, in package order")
		})
	})

	t.Run("JoinName", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			word string
			base string
			want string
		}{
			{name: "joins the word after the base", word: "stub", base: "store", want: "storeStub"},
			{name: "keeps the case of the base", word: "stub", base: "Store", want: "StoreStub"},
			{name: "raises a first letter of several bytes", word: "éclair", base: "store", want: "storeÉclair"},
			{name: "returns the base for an empty word", word: "", base: "store", want: "store"},
			{name: "returns the word for an empty base", word: "stub", base: "", want: "stub"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, (&eidos.Emitter{}).JoinName(tt.word, tt.base), tt.want,
					"the join is the neutral spelling")
			})
		}
	})

	t.Run("Ref", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a reference whose owner is the plugin the phase call runs as", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, referenced(t).Owner, contextPlugin,
				"the owner is the identity the call attributes its units to")
		})

		t.Run("returns a reference to the named template", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, referenced(t).Name, refTemplate, "the name is the one given")
		})

		t.Run("returns a reference with the given payload", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, referenced(t).Data, any(refPayload), "the payload is the one given")
		})
	})
}
