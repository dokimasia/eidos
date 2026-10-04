// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package eidos_test

import (
	"testing"
	"text/template"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The allocations of a declaration's methods, which TestBuilderAllocs
// checks in the ordinary run and BenchmarkBuilder in a benchmark run. A
// method without a constant allocates nothing.
const (
	// newPluginAllocs is a declaration: the builder and its map of
	// priorities.
	newPluginAllocs = 2
	// declareAllocs is the first call of a method that appends to a list
	// of a new declaration: the list.
	declareAllocs = 1
	// priorityAllocs is a declaration's first priority: its map's first
	// group.
	priorityAllocs = 1
	// forAllocs is a declaration's first target: its list of targets and
	// the copy of the target's options.
	forAllocs = 2
	// buildAllocs is one Build of the bench declaration: the table of
	// output families and its entry, the flattened rules growing to three
	// and the predicates a wrapper hands its rules, the built value with
	// its copies of the outputs and the priorities, the subscriptions
	// growing to three, and the role wrapper.
	buildAllocs = 13
)

// declaring is one method of a declaration called with a fixture value,
// named as its benchmark is, and what its call on a new declaration
// allocates. A method that sets one value takes the same declaration
// on every call.
type declaring struct {
	name   string
	allocs uint64
	set    func(*eidos.Builder) *eidos.Builder
}

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

	t.Run("NewPlugin", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a declaration under the name", func(t *testing.T) {
			t.Parallel()

			p := eidos.NewPlugin("planner").Handle(emitNothing()).Build()
			assert.Equal(t, p.Name(), "planner", "the plugin is named as declared")
		})
	})

	for _, tt := range declarations(t) {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			t.Run("returns its builder", func(t *testing.T) {
				t.Parallel()

				b := eidos.NewPlugin("planner")
				assert.True(t, tt.set(b) == b, "a declaration chains its calls on one builder")
			})
		})
	}

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
				name: "panics on a helper that is no function",
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
				name: "panics on a target's helper named like its override",
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

// Each method of a declaration allocates within its ceiling in the
// ordinary run, which runs no benchmark. A method that appends takes a
// declaration built before the count, because its first call allocates
// the list, and so does Build, because a declaration builds once. The
// check runs alone, because AllocsPerRun counts every goroutine's
// allocations and refuses to run beside parallel tests.
func TestBuilderAllocs(t *testing.T) {
	var declared *eidos.Builder
	assert.MaxAllocs(t, func() { declared = eidos.NewPlugin("bench") }, newPluginAllocs,
		"NewPlugin allocates the builder and its map of priorities")
	assert.NotNil(t, declared, "NewPlugin returns the declaration")

	for _, tt := range declarations(t) {
		fresh, at := newDeclarations(allocRuns), 0
		msg := tt.name + " allocates within its ceiling on a new declaration"
		assert.MaxAllocs(t, func() {
			tt.set(fresh[at])
			at++
		}, tt.allocs, msg)
	}

	declare := benchDeclaration(t)
	fresh, at := make([]*eidos.Builder, allocRuns), 0
	for i := range fresh {
		fresh[i] = declare()
	}
	var p plugin.Plugin
	assert.MaxAllocs(t, func() {
		p = fresh[at].Build()
		at++
	}, buildAllocs, "Build allocates the lowered rules, the subscriptions and the built value")
	assert.Equal(t, p.Name(), "bench", "Build returns the declared plugin")
}

// BenchmarkBuilder measures a declaration's construction, each method
// on a new declaration, and freezing a declaration of a graph rule and
// two fact-gated emit rules. Each call takes a declaration built afresh
// outside the measurement, because a method's first call allocates its
// list and a Builder freezes once.
func BenchmarkBuilder(b *testing.B) {
	b.Run("NewPlugin", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(newPluginAllocs)
		defer c.End()
		var declared *eidos.Builder
		for c.Loop() {
			declared = eidos.NewPlugin("bench")
		}
		assert.NotNil(b, declared, "NewPlugin returns the declaration")
	})

	for _, tt := range declarations(b) {
		b.Run(tt.name, func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(tt.allocs)
			defer c.End()
			declared := eidos.NewPlugin("bench")
			var got *eidos.Builder
			for c.Loop() {
				if tt.allocs > 0 {
					c.Excluding(func() { declared = eidos.NewPlugin("bench") })
				}
				got = tt.set(declared)
			}
			assert.True(b, got == declared, "the method returns its builder")
		})
	}

	b.Run("Build", func(b *testing.B) {
		declare := benchDeclaration(b)
		var declared *eidos.Builder
		c := bench.Start(b).MaxAllocs(buildAllocs)
		defer c.End()
		var p plugin.Plugin
		for c.Loop() {
			c.Excluding(func() { declared = declare() })
			p = declared.Build()
		}
		assert.Equal(b, p.Name(), "bench", "Build returns the declared plugin")
	})
}

// declarations returns each method of a declaration, called with a
// fixture value built once.
func declarations(tb assert.TB) []declaring {
	tb.Helper()

	var options any = &plannerOptions{}
	family := plugin.Output{Per: plugin.PerPlan, Word: "registry"}
	register := func(*meta.Registry) error { return nil }
	tree, helpers := stubTree(), template.FuncMap{toneHelper: shout}
	layer := eidos.Templates(stubTree())
	rule := emitNothing()
	return []declaring{
		{name: "Version", set: func(b *eidos.Builder) *eidos.Builder { return b.Version(declaredVersion) }},
		{name: "Output", allocs: declareAllocs, set: func(b *eidos.Builder) *eidos.Builder { return b.Output(family) }},
		{name: "Priority", allocs: priorityAllocs, set: func(b *eidos.Builder) *eidos.Builder {
			return b.Priority(plugin.RoleAnnotator, declaredPriority)
		}},
		{name: "Provides", allocs: declareAllocs, set: func(b *eidos.Builder) *eidos.Builder {
			return b.Provides(providedLabel)
		}},
		{name: "Requires", allocs: declareAllocs, set: func(b *eidos.Builder) *eidos.Builder {
			return b.Requires(requiredLabel)
		}},
		{name: "Options", set: func(b *eidos.Builder) *eidos.Builder { return b.Options(options) }},
		{name: "Keys", allocs: declareAllocs, set: func(b *eidos.Builder) *eidos.Builder { return b.Keys(register) }},
		{
			name:   "Templates",
			allocs: declareAllocs,
			set:    func(b *eidos.Builder) *eidos.Builder { return b.Templates(tree) },
		},
		{name: "Funcs", allocs: declareAllocs, set: func(b *eidos.Builder) *eidos.Builder { return b.Funcs(helpers) }},
		{
			name:   "For",
			allocs: forAllocs,
			set:    func(b *eidos.Builder) *eidos.Builder { return b.For(stubTarget, layer) },
		},
		{name: "Handle", allocs: declareAllocs, set: func(b *eidos.Builder) *eidos.Builder { return b.Handle(rule) }},
	}
}

// newDeclarations returns n new declarations, built before a count, so
// each counted call takes one of its own.
func newDeclarations(n int) []*eidos.Builder {
	out := make([]*eidos.Builder, n)
	for i := range out {
		out[i] = eidos.NewPlugin("bench")
	}
	return out
}

// benchDeclaration returns a function that declares the bench plugin
// afresh: one output family, a graph rule, and two emit rules under one
// fact gate.
func benchDeclaration(tb assert.TB) func() *eidos.Builder {
	tb.Helper()

	key, _ := boolKey(tb)
	return func() *eidos.Builder {
		return eidos.NewPlugin("bench").
			Output(plugin.Output{Per: plugin.PerPlan, Word: "registry"}).
			Handle(
				eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
					return nil
				}),
				eidos.Where(eidos.HasKey(key),
					eidos.OnEmit(symbol.KindStruct,
						func(m *eidos.EmitMatch, e *eidos.Emitter) error {
							return nil
						}),
					eidos.OnEmit(symbol.KindMethod,
						func(m *eidos.EmitMatch, e *eidos.Emitter) error {
							return nil
						}),
				),
			)
	}
}
