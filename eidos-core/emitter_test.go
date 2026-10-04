// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos_test

import (
	"slices"
	"strconv"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

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

// The tags of the invocation fixture's families beside its primary
// per-source family.
const (
	packageTag eidos.Tag = "pkg"
	planTag    eidos.Tag = "plan"
)

// The allocations of a phase call over the invocation fixture whose
// handler calls one method of the emitter, its handle or its slot view
// once per invocation. A phase call's state is pooled, so an accessor,
// an append and a slot view allocate nothing per invocation, and a call
// allocates what its effects leave in the store.
const (
	// unitAllocs is a call whose invocations touch one accumulator: the
	// store's acceptance of the one unit, which has no declarations, so
	// the first group of the store's set of keys, the key, and its list
	// of units.
	unitAllocs = 3
	// appendAllocs is a call whose invocations append one declaration
	// each to one accumulator: the unit, its list of declarations and
	// its list of origins. The declaration has no origin, so the store
	// indexes nothing.
	appendAllocs = unitAllocs + 2
	// refAllocs is a call whose invocations take one template reference
	// each, which a body keeps.
	refAllocs = invocationStructs
	// orderAllocs is an emit-phase call: the order of the store's units,
	// which the rule's enumeration sorts once.
	orderAllocs = 1
	// slotAppendAllocs is an emit-phase call whose invocations append one
	// value each into a slot of a value the store contains: each host's
	// slot, the store's map from its declarations to their units, which
	// allocates four times at its final size, and the unit's list of
	// contributors.
	slotAppendAllocs = orderAllocs + invocationStructs + 4 + 1
	// joinNameAllocs is the joined name of a word and a base.
	joinNameAllocs = 1
)

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

		t.Run("places one origin's declarations in gating instance order across rules", func(t *testing.T) {
			t.Parallel()

			const instances = 2
			g, alpha := gatedStruct(t, instances)
			_, facts := boolKey(t)
			schema := stubSchema(gateName)
			schema.Repeatable = true
			ctx := genContext(t, g, facts, repeated(alpha.ID, schema, instances))
			placing := func(rule string) emitHandler {
				return func(m *eidos.StructMatch, e *eidos.Emitter) error {
					e.File().Append(&emit.Struct{
						Origin: m.Struct.Identity(),
						Name:   rule + strconv.Itoa(m.Directive().Instance),
					})
					return nil
				}
			}
			p := eidos.NewPlugin(contextPlugin).
				Output(plugin.Output{Per: plugin.PerSource, Word: "impl"}).
				Handle(eidos.Directive(schema, eidos.OnStruct(placing("A")), eidos.OnStruct(placing("B")))).
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")

			var names []string
			for u := range ctx.Emit.Units() {
				for _, d := range u.Decls {
					s, held := d.(*emit.Struct)
					assert.True(t, held, "the fixture emits structs")
					names = append(names, s.Name)
				}
			}
			assert.Equal(t, names, []string{"A0", "B0", "A1", "B1"},
				"each instance's declarations precede the next instance's, in rule order")
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

		t.Run("keys one accumulator per package path of each language", func(t *testing.T) {
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

	t.Run("Slot", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a view whose appends apply to the slot", func(t *testing.T) {
			t.Parallel()

			host, _ := woven(t, 2)
			assert.Equal(t, fieldNames(host), []string{"audited", "audited"}, "both appends apply")
		})

		t.Run("returns a view that names the plugin among the host unit's contributors", func(t *testing.T) {
			t.Parallel()

			_, unit := woven(t, 1)
			assert.Equal(t, unit.Contributors, []plugin.ID{contextPlugin},
				"the weaver is attributed in the unit it appended into")
		})

		t.Run("panics on a nil slot", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			seed(t, ctx, emitted(alpha))
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnEmit(symbol.KindStruct, func(m *eidos.EmitMatch, e *eidos.Emitter) error {
					e.Slot[*emit.Field](nil)
					return nil
				})).
				Build()
			assert.Panics(t, func() { _ = generatorOf(t, p).Generate(ctx) },
				"a view of no slot would append nowhere")
		})
	})

	t.Run("SlotView.Append", func(t *testing.T) {
		t.Parallel()

		t.Run("records no contributor for no values", func(t *testing.T) {
			t.Parallel()

			_, unit := woven(t, 0)
			assert.Length(t, unit.Contributors, 0, "an empty append contributes nothing")
		})

		t.Run("panics on the zero view", func(t *testing.T) {
			t.Parallel()

			var view eidos.SlotView[*emit.Field]
			assert.Panics(t, func() { view.Append(&emit.Field{Name: "lost"}) },
				"the zero view has no invocation to buffer with")
		})
	})
}

// Each method of the emitter, its handle and its slot view allocates
// nothing per invocation in the ordinary run, which runs no benchmark,
// and a phase call allocates what its effects leave in the store. A
// name join allocates the name. The check runs alone, because
// AllocsPerRun counts every goroutine's allocations and refuses to run
// beside parallel tests.
func TestEmitterAllocs(t *testing.T) {
	checkPhaseAllocs(t, emitterCases(t))

	e := &eidos.Emitter{}
	var joined string
	assert.MaxAllocs(t, func() { joined = e.JoinName("stub", "store") }, joinNameAllocs,
		"JoinName allocates the joined name")
	assert.Equal(t, joined, "storeStub", "JoinName joins the word after the base")
	assert.MaxAllocs(t, func() { joined = e.JoinName("", "store") }, 0,
		"JoinName allocates nothing for an empty word")
	assert.Equal(t, joined, "store", "JoinName returns the base for an empty word")
}

// BenchmarkEmitter measures a phase call over the invocation fixture
// for each method of the emitter, its handle and its slot view, each
// called once per invocation, and the join of a name.
func BenchmarkEmitter(b *testing.B) {
	benchPhases(b, emitterCases(b))

	e := &eidos.Emitter{}
	b.Run("JoinName", func(b *testing.B) {
		b.Run("a word and a base", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(joinNameAllocs)
			defer c.End()
			var got string
			for c.Loop() {
				got = e.JoinName("stub", "store")
			}
			assert.Equal(b, got, "storeStub", "JoinName joins the word after the base")
		})

		b.Run("an empty word", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var got string
			for c.Loop() {
				got = e.JoinName("", "store")
			}
			assert.Equal(b, got, "store", "JoinName returns the base for an empty word")
		})
	})
}

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

// woven runs a weaver whose handler appends the given number of fields
// through the view into the seeded value's slot, then appends nothing,
// and returns the seeded value and the seeded unit after the phase
// call.
func woven(tb assert.TB, fields int) (*emit.Struct, plugin.Unit) {
	tb.Helper()

	g, alpha, _ := fixtureGraph(tb)
	_, facts := boolKey(tb)
	ctx := genContext(tb, g, facts, nil)
	host := emitted(alpha)
	seed(tb, ctx, host)
	p := eidos.NewPlugin(contextPlugin).
		Handle(eidos.OnEmit(symbol.KindStruct, func(m *eidos.EmitMatch, e *eidos.Emitter) error {
			s, held := m.Value.(*emit.Struct)
			assert.True(tb, held, "a struct rule receives structs")
			view := e.Slot(&s.Fields)
			for range fields {
				view.Append(&emit.Field{Name: "audited"})
			}
			view.Append()
			return nil
		})).
		Build()
	assert.NoError(tb, generatorOf(tb, p).Generate(ctx), "the phase call passes")
	var earlier plugin.Unit
	for u := range ctx.Emit.Units() {
		if u.Plugin == "earlier" {
			earlier = u
		}
	}
	return host, earlier
}

// emitterCases returns a phase call over the invocation fixture for each
// method of the emitter, its handle and its slot view, each called once
// per invocation. The plugin declares a family at each cardinality.
func emitterCases(tb assert.TB) []phaseCase {
	tb.Helper()

	fresh, _, _ := invocationContexts(tb, false)
	seeded, _, _ := invocationContexts(tb, true)
	families := func(h func(*eidos.StructMatch, *eidos.Emitter)) plugin.Generator {
		return generatorOf(tb, eidos.NewPlugin(contextPlugin).
			Output(plugin.Output{Per: plugin.PerSource, Word: "impl"}).
			Output(plugin.Output{Tag: string(packageTag), Per: plugin.PerPackage, Word: "pkg"}).
			Output(plugin.Output{Tag: string(planTag), Per: plugin.PerPlan, Word: "plan"}).
			Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
				h(m, e)
				return nil
			})).
			Build())
	}
	weaver := func(h func(*emit.Struct, *eidos.Emitter)) plugin.Generator {
		return generatorOf(tb, eidos.NewPlugin(contextPlugin).
			Handle(eidos.OnEmit(symbol.KindStruct, func(m *eidos.EmitMatch, e *eidos.Emitter) error {
				s, held := m.Value.(*emit.Struct)
				if !held {
					tb.Fatalf("the struct rule received a %T", m.Value)
				}
				h(s, e)
				return nil
			})).
			Build())
	}
	oneUnit := func(per plugin.Cardinality, decls int) func(assert.TB, *plugin.GeneratorContext) {
		return func(tb assert.TB, ctx *plugin.GeneratorContext) {
			units := slices.Collect(ctx.Emit.Units())
			assert.Length(tb, units, 1, "the call flushes one unit")
			assert.Equal(tb, units[0].Per, per, "at the family's cardinality")
			assert.Length(tb, units[0].Decls, decls, "with the call's appends")
		}
	}
	unchanged := func(tb assert.TB, ctx *plugin.GeneratorContext) {
		assert.Length(tb, slices.Collect(ctx.Emit.Units()), 1, "the seeded unit alone remains")
	}

	declaration := &emit.Struct{Name: "Appended"}
	var ref *emit.TemplateRef
	field := &emit.Field{Name: "audited"}
	return []phaseCase{
		{
			name: "File", allocs: unitAllocs, fresh: fresh, check: oneUnit(plugin.PerSource, 0),
			gen: families(func(_ *eidos.StructMatch, e *eidos.Emitter) { e.File() }),
		},
		{
			name: "PackageFile", allocs: unitAllocs, fresh: fresh, check: oneUnit(plugin.PerPackage, 0),
			gen: families(func(_ *eidos.StructMatch, e *eidos.Emitter) { e.PackageFile(packageTag) }),
		},
		{
			name: "PlanFile", allocs: unitAllocs, fresh: fresh, check: oneUnit(plugin.PerPlan, 0),
			gen: families(func(_ *eidos.StructMatch, e *eidos.Emitter) { e.PlanFile(planTag) }),
		},
		{
			name: "Out.Append", allocs: appendAllocs, fresh: fresh,
			check: oneUnit(plugin.PerSource, invocationStructs),
			gen:   families(func(_ *eidos.StructMatch, e *eidos.Emitter) { e.File().Append(declaration) }),
		},
		{
			name: "Ref", allocs: refAllocs, fresh: fresh,
			gen: families(func(_ *eidos.StructMatch, e *eidos.Emitter) { ref = e.Ref(refTemplate, refPayload) }),
			check: func(tb assert.TB, ctx *plugin.GeneratorContext) {
				assert.Equal(tb, ref.Owner, ctx.Plugin, "the reference names the plugin the call runs as")
			},
		},
		{
			name: "Slot", allocs: orderAllocs, fresh: seeded, check: unchanged,
			gen: weaver(func(s *emit.Struct, e *eidos.Emitter) { e.Slot(&s.Fields) }),
		},
		{
			name: "SlotView.Append", allocs: slotAppendAllocs, fresh: seeded,
			gen: weaver(func(s *emit.Struct, e *eidos.Emitter) { e.Slot(&s.Fields).Append(field) }),
			check: func(tb assert.TB, ctx *plugin.GeneratorContext) {
				for u := range ctx.Emit.Units() {
					assert.Equal(
						tb,
						u.Contributors,
						[]plugin.ID{ctx.Plugin},
						"the weaver contributes to the seeded unit",
					)
					for _, d := range u.Decls {
						s, held := d.(*emit.Struct)
						assert.True(tb, held, "the seeded unit has structs")
						assert.Equal(tb, fieldNames(s), []string{field.Name}, "each host has the appended field")
					}
				}
			},
		},
	}
}
