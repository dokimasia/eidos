// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos_test

import (
	"fmt"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The worker counts the effect cases compare: sequential dispatch, the
// two workers the interleaved schedule needs, and the parallel dispatch
// the plugin suite runs.
const (
	oneWorker    = 1
	twoWorkers   = 2
	eightWorkers = 8
)

// The sizes of the effect fixtures: subjects in one package, the
// instances of one repeatable directive on one subject, the
// invocations the interleaved schedule places, and subjects enough
// that a parallel call collects their matches in more than one chunk.
const (
	manyStructs    = 64
	manyInstances  = 32
	scheduled      = 3
	chunkedStructs = 20_000
)

// schedule makes the effects of a rule's i-th invocation in canonical
// match order, at the point the schedule assigns them relative to the
// rule's other invocations.
type schedule func(i int, effects func())

// interleaved places three invocations of one rule on two workers so
// that the lanes' effects interleave by sequence. The first invocation
// waits until the second has made its effects, and the second waits
// until the third has made its effects. One lane therefore runs the
// first and the third invocation and the other lane runs the second,
// whatever order the lanes start in. The effects are made in the order
// second, first, third.
//
// A wait gives up after failureWait and records the miss: a dispatcher
// that runs the three invocations on one lane misses both waits.
type interleaved struct {
	second chan struct{}
	third  chan struct{}
	missed atomic.Bool
}

// interleave returns a schedule none of whose invocations has run.
func interleave() *interleaved {
	return &interleaved{second: make(chan struct{}), third: make(chan struct{})}
}

// run makes the i-th invocation's effects at its place in the schedule.
// It panics for an invocation outside the three the schedule places.
func (s *interleaved) run(i int, effects func()) {
	switch i {
	case 0:
		s.await(s.second)
		effects()
	case 1:
		effects()
		close(s.second)
		s.await(s.third)
	case 2:
		effects()
		close(s.third)
	default:
		panic(fmt.Sprintf("eidos_test: the interleaved schedule places three invocations, not invocation %d", i))
	}
}

// await blocks until ch closes, and records a miss after failureWait.
func (s *interleaved) await(ch <-chan struct{}) {
	select {
	case <-ch:
	case <-time.After(failureWait):
		s.missed.Store(true)
	}
}

// An invocation's placements, slot appends and findings are buffered
// with it and apply when its phase call's rules have run, in canonical
// match order, and its stamps rank by that order, so the output does
// not depend on the worker count or on the order the workers made the
// effects in.
func TestEffects(t *testing.T) {
	t.Parallel()

	t.Run("Generate", func(t *testing.T) {
		t.Parallel()

		// weave appends one field per gating instance into the origin's
		// emitted struct, through the Emitter, on the given workers, and
		// returns the struct. The schedule places each invocation's
		// append.
		weave := func(tb assert.TB, workers, instances int, place schedule) *emit.Struct {
			tb.Helper()

			g, alpha, _ := fixtureGraph(tb)
			_, facts := boolKey(tb)
			schema := stubSchema(gateName)
			schema.Repeatable = true
			ctx := on(genContext(tb, g, facts, repeated(alpha.ID, schema, instances)), workers)
			host := emitted(alpha)
			seed(tb, ctx, host)
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.Directive(schema,
					eidos.OnEmit(symbol.KindStruct, func(m *eidos.EmitMatch, e *eidos.Emitter) error {
						woven, held := m.Value.(*emit.Struct)
						assert.True(tb, held, "a struct rule receives structs")
						instance := m.Directive().Instance
						place(instance, func() {
							e.Slot(&woven.Fields).Append(&emit.Field{Name: fmt.Sprintf("f%02d", instance)})
						})
						return nil
					}))).
				Build()
			assert.NoError(tb, generatorOf(tb, p).Generate(ctx), "the phase call passes")
			return host
		}

		t.Run("applies slot appends in canonical match order on one worker", func(t *testing.T) {
			t.Parallel()

			got := fieldNames(weave(t, oneWorker, manyInstances, immediately))
			assert.Length(t, got, manyInstances, "every invocation appended once")
			assert.True(t, slices.IsSorted(got), "the appends follow the gating instances' order")
		})

		t.Run("applies slot appends in canonical match order when two workers interleave", func(t *testing.T) {
			t.Parallel()

			s := interleave()
			got := fieldNames(weave(t, twoWorkers, scheduled, s.run))
			assert.False(t, s.missed.Load(), "a second worker took a match while the first was busy")
			assert.Equal(t, got, fieldNames(weave(t, oneWorker, scheduled, immediately)),
				"the appends of the two lanes merge into the gating instances' order")
		})

		t.Run("applies slot appends in the order of one worker on eight", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, fieldNames(weave(t, eightWorkers, manyInstances, immediately)),
				fieldNames(weave(t, oneWorker, manyInstances, immediately)),
				"the slot's order does not depend on the worker count")
		})

		t.Run("applies a handler's slot appends after the phase call's rules ran", func(t *testing.T) {
			t.Parallel()

			g, alpha, _ := fixtureGraph(t)
			_, facts := boolKey(t)
			ctx := genContext(t, g, facts, nil)
			host := emitted(alpha)
			seed(t, ctx, host)
			var seen []int
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnEmit(symbol.KindStruct, func(m *eidos.EmitMatch, e *eidos.Emitter) error {
					s, held := m.Value.(*emit.Struct)
					assert.True(t, held, "a struct rule receives structs")
					e.Slot(&s.Fields).Append(&emit.Field{Name: "audited"})
					seen = append(seen, s.Fields.Len())
					return nil
				})).
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			assert.Equal(t, seen, []int{0}, "the handler does not see its own append")
			assert.Equal(t, fieldNames(host), []string{"audited"}, "the append applies when the call ends")
		})

		// warnEach reports one warning per subject of n on the given
		// workers and returns the findings. The schedule places each
		// invocation's warning.
		warnEach := func(tb assert.TB, workers, n int, place schedule) []string {
			tb.Helper()

			g, structs := manySubjects(tb, n)
			index := indexOf(structs)
			_, facts := boolKey(tb)
			ctx := on(genContext(tb, g, facts, nil), workers)
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					place(index[m.Struct.ID], func() { m.Warnf(testCode, "%s", m.Struct.Name) })
					return nil
				})).
				Build()
			assert.NoError(tb, generatorOf(tb, p).Generate(ctx), "the phase call passes")
			return messages(ctx.Sink)
		}

		t.Run("reports findings in canonical match order on one worker", func(t *testing.T) {
			t.Parallel()

			got := warnEach(t, oneWorker, manyStructs, immediately)
			assert.Length(t, got, manyStructs, "every subject reported once")
			assert.True(t, slices.IsSorted(got), "the findings follow the subjects' order")
		})

		t.Run("reports findings in canonical match order when two workers interleave", func(t *testing.T) {
			t.Parallel()

			s := interleave()
			got := warnEach(t, twoWorkers, scheduled, s.run)
			assert.False(t, s.missed.Load(), "a second worker took a match while the first was busy")
			assert.Equal(t, got, warnEach(t, oneWorker, scheduled, immediately),
				"the findings of the two lanes merge into the subjects' order")
		})

		t.Run("reports findings in the order of one worker on eight", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, warnEach(t, eightWorkers, manyStructs, immediately),
				warnEach(t, oneWorker, manyStructs, immediately),
				"the report does not depend on the worker count")
		})

		t.Run("reports findings in the order of one worker on eight across collected chunks", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, warnEach(t, eightWorkers, chunkedStructs, immediately),
				warnEach(t, oneWorker, chunkedStructs, immediately),
				"the sequence continues from one chunk to the next")
		})

		t.Run("reports the finding of a rule's one match on eight workers", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, warnEach(t, eightWorkers, 1, immediately), warnEach(t, oneWorker, 1, immediately),
				"a rule of one match runs on the calling goroutine")
		})

		t.Run("reports the absent-rules warning once per language when two workers interleave", func(t *testing.T) {
			t.Parallel()

			g, structs := manySubjects(t, scheduled)
			index := indexOf(structs)
			_, facts := boolKey(t)
			ctx := on(genContext(t, g, facts, nil), twoWorkers)
			s := interleave()
			p := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					s.run(index[m.Struct.ID], func() { m.Rules() })
					return nil
				})).
				Build()
			assert.NoError(t, generatorOf(t, p).Generate(ctx), "the phase call passes")
			assert.False(t, s.missed.Load(), "a second worker took a match while the first was busy")
			var warned []diag.Diag
			for d := range ctx.Sink.All() {
				if d.Code == rules.AbsentRules {
					warned = append(warned, d)
				}
			}
			assert.Length(t, warned, 1, "one warning for the language, which both lanes bound")
			assert.Equal(t, warned[0].Pos, structs[0].Pos, "at the first invocation in canonical match order")
		})
	})

	t.Run("Annotate", func(t *testing.T) {
		t.Parallel()

		// stampEach stamps the flag on every subject on the given workers
		// and returns the stamped subjects.
		stampEach := func(tb assert.TB, workers int) []symbol.Identity {
			tb.Helper()

			g, _ := manySubjects(tb, manyStructs)
			key, facts := boolKey(tb)
			ix, err := plugin.NewIndex(g, facts, nil, nil)
			assert.NoError(tb, err, "the routing surface builds")
			p, held := eidos.NewPlugin(contextPlugin).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error {
					eidos.Stamp(st, key, true)
					return nil
				})).
				Build().(plugin.Annotator)
			assert.True(tb, held, "stamper rules make the value an annotator")
			assert.NoError(tb, p.Annotate(&plugin.AnnotatorContext{
				Index: ix, Facts: facts, Sink: diag.NewSink(), Plugin: contextPlugin, Bucket: 1, Workers: workers,
			}), "the phase call passes")
			return slices.Collect(facts.ByKey(key.ID()))
		}

		t.Run("stamps on eight workers what one worker stamps", func(t *testing.T) {
			t.Parallel()

			got := stampEach(t, eightWorkers)
			assert.Length(t, got, manyStructs, "every subject is stamped")
			assert.Equal(t, got, stampEach(t, oneWorker), "the facts do not depend on the worker count")
		})

		t.Run("stamps the first invocation's value in canonical match order when two workers interleave",
			func(t *testing.T) {
				t.Parallel()

				g, alpha := gatedStruct(t, scheduled)
				key, facts := instanceKey(t)
				schema := stubSchema(gateName)
				schema.Repeatable = true
				ix, err := plugin.NewIndex(g, facts, repeated(alpha.ID, schema, scheduled), nil)
				assert.NoError(t, err, "the routing surface builds")
				s := interleave()
				p, held := eidos.NewPlugin(contextPlugin).
					Handle(eidos.Directive(schema,
						eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error {
							instance := m.Directive().Instance
							s.run(instance, func() { eidos.Stamp(st, key, int64(instance)) })
							return nil
						}))).
					Build().(plugin.Annotator)
				assert.True(t, held, "stamper rules make the value an annotator")
				sink := diag.NewSink()
				assert.NoError(t, p.Annotate(&plugin.AnnotatorContext{
					Index: ix, Facts: facts, Sink: sink, Plugin: contextPlugin, Bucket: 1, Workers: twoWorkers,
				}), "the phase call passes")
				assert.False(t, s.missed.Load(), "a second worker took a match while the first was busy")
				coretest.AssertCodes(t, sink)
				got, stamped := meta.Get(facts, alpha.ID, key)
				assert.True(t, stamped, "the subject is stamped")
				assert.Equal(t, got, int64(0), "the first instance's claim outranks the second's, which arrived first")
			})
	})
}

// immediately makes every invocation's effects as the invocation runs.
func immediately(_ int, effects func()) { effects() }

// manySubjects returns a frozen graph of n structs in the store
// package, named in identity order, each in a file of its own.
func manySubjects(tb assert.TB, n int) (*store.Graph, []*node.Struct) {
	tb.Helper()

	structs := make([]*node.Struct, n)
	decls := make([]symbol.Symbol, n)
	for i := range structs {
		s := coretest.Struct(coretest.StorePath, fmt.Sprintf("S%02d", i))
		s.Pos = position.Pos{File: fmt.Sprintf("s%02d.go", i), Line: 1, Col: 1}
		structs[i], decls[i] = s, s
	}
	g := store.New()
	assert.NoError(tb, g.AddPackage(coretest.Package(coretest.StorePath, decls...)),
		"the fixture package is admitted")
	g.Freeze()
	return g, structs
}

// instanceKey returns a registered int64 key and a fact store built
// over its registry.
func instanceKey(tb assert.TB) (meta.Key[int64], *meta.Facts) {
	tb.Helper()

	reg := meta.NewRegistry()
	assert.NoError(tb, reg.ClaimNamespace(fixtureNamespace), "the namespace is claimed")
	key, err := meta.Register[int64](reg, meta.KeySpec{
		Name: "t.instance", Doc: "names the gating instance that stamped a fixture subject",
	})
	assert.NoError(tb, err, "the key registers")
	return key, meta.NewFacts(reg)
}

// indexOf returns each struct's index into structs, keyed by identity,
// which is the struct's place in canonical match order.
func indexOf(structs []*node.Struct) map[symbol.Identity]int {
	index := make(map[symbol.Identity]int, len(structs))
	for i, s := range structs {
		index[s.ID] = i
	}
	return index
}

// gatedStruct returns a frozen graph of one struct with n raw instances
// of the gate directive.
func gatedStruct(tb assert.TB, n int) (*store.Graph, *node.Struct) {
	tb.Helper()

	alpha := coretest.Struct(coretest.StorePath, "Alpha")
	g := store.New()
	assert.NoError(tb, g.AddPackage(coretest.Package(coretest.StorePath, alpha)),
		"the fixture package is admitted")
	raws := make([]directive.Raw, n)
	for i := range raws {
		raws[i] = directive.Raw{Name: gateName}
	}
	assert.NoError(tb, g.AttachDirectives(alpha.ID, raws), "the gating instances attach")
	g.Freeze()
	return g, alpha
}

// repeated returns the validated table with n instances of a
// repeatable directive on one subject, in instance order.
func repeated(subject symbol.Identity, schema directive.Schema, n int) map[symbol.Identity][]directive.Directive {
	ds := make([]directive.Directive, n)
	for i := range ds {
		ds[i] = directive.Directive{Name: schema.Canonical(), Instance: i}
	}
	return map[symbol.Identity][]directive.Directive{subject: ds}
}

// on returns the generator context with its worker count set.
func on(ctx *plugin.GeneratorContext, workers int) *plugin.GeneratorContext {
	ctx.Workers = workers
	return ctx
}

// fieldNames returns the names of the fields a struct's slot contains,
// in slot order.
func fieldNames(s *emit.Struct) []string {
	names := make([]string, 0, s.Fields.Len())
	for _, f := range s.Fields.Items() {
		names = append(names, f.Name)
	}
	return names
}

// messages returns the messages of every finding in a sink, in its
// order.
func messages(sink *diag.Sink) []string {
	var out []string
	for d := range sink.All() {
		out = append(out, d.Msg)
	}
	return out
}
