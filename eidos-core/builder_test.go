// Copyright Dokimasia B.V. 2026
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

// These constants are the allocation counts of the methods of a
// declaration. TestBuilderAllocs checks them in the ordinary run, and
// BenchmarkBuilder in a benchmark run. A method without a constant
// allocates nothing.
const (
	// newPluginAllocs counts the allocations of a declaration, which are
	// the builder and its map of priorities.
	newPluginAllocs = 2
	// declareAllocs counts the allocation of the first call of a method
	// that appends to a list of a new declaration, which is the list.
	declareAllocs = 1
	// priorityAllocs counts the allocation of the first priority of a
	// declaration, which is the first group of its map.
	priorityAllocs = 1
	// forAllocs counts the allocations of the first target of a
	// declaration, which are its list of targets and the copy of the
	// options of the target.
	forAllocs = 2
	// buildAllocs counts the allocations of one typed build of the bench
	// declaration. They are the table of output families and its entry,
	// the flattened rules as they grow to three, the predicates that a
	// wrapper passes to its rules, the built plugin with its copies of the
	// outputs and the priorities, the subscriptions as they grow to three,
	// and the role wrapper.
	buildAllocs = 13
)

// declaring is one method of a declaration, called with a fixture value.
// It has the name of its benchmark and the allocation count of its call on
// a new declaration. A method that sets one value takes the same
// declaration on every call.
type declaring struct {
	name   string
	allocs uint64
	set    func(*eidos.Builder) *eidos.Builder
}

// TestBuilder tests the declaration of a plugin and its typed builds. The
// panics of a build, the role of the built plugin and what its providers
// return are a contract.
func TestBuilder(t *testing.T) {
	t.Parallel()

	t.Run("NewPlugin", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a declaration under the name", func(t *testing.T) {
			t.Parallel()

			p := eidos.NewPlugin("planner").Handle(emitNothing()).Generator()
			assert.Equal(t, p.Name(), "planner", "the plugin is named as declared")
		})
	})

	for _, tt := range declarations(t) {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			t.Run("returns its builder", func(t *testing.T) {
				t.Parallel()

				b := eidos.NewPlugin("planner")
				assert.Equal(t, tt.set(b), b, "a declaration chains its calls on one builder", assert.ByIdentity())
			})
		})
	}

	t.Run("Annotator", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a plugin named by the declaration", func(t *testing.T) {
			t.Parallel()

			p := eidos.NewPlugin("planner").Handle(eidos.OnStruct(nothing[*eidos.Stamper])).Annotator()
			assert.Equal(t, p.Name(), "planner", "the name is the declared one")
		})

		t.Run("returns a plugin without the generator role", func(t *testing.T) {
			t.Parallel()

			p := eidos.NewPlugin("planner").Handle(eidos.OnStruct(nothing[*eidos.Stamper])).Annotator()
			_, generates := p.(plugin.Generator)
			assert.False(t, generates, "the plugin does not implement the generator role")
		})

		t.Run("panics on a rule that emits", func(t *testing.T) {
			t.Parallel()

			got := assert.Panics(t, func() {
				eidos.NewPlugin("planner").
					Handle(eidos.OnStruct(nothing[*eidos.Stamper]), eidos.OnStruct(nothing[*eidos.Emitter])).
					Annotator()
			}, "an annotator with a rule that emits panics")
			assert.Equal(t, got, any("eidos: planner declares a rule that emits, so build it with Generator or Dual"),
				"the panic names the plugin and the typed builds that fit it")
		})
	})

	t.Run("Dual", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a plugin named by the declaration", func(t *testing.T) {
			t.Parallel()

			p := eidos.NewPlugin("planner").
				Handle(eidos.OnStruct(nothing[*eidos.Stamper]), eidos.OnStruct(nothing[*eidos.Emitter])).
				Dual()
			assert.Equal(t, p.Name(), "planner", "the name is the declared one")
		})

		t.Run("panics without a rule that stamps facts", func(t *testing.T) {
			t.Parallel()

			got := assert.Panics(t, func() {
				eidos.NewPlugin("planner").Handle(eidos.OnStruct(nothing[*eidos.Emitter])).Dual()
			}, "a dual plugin without a rule that stamps facts panics")
			assert.Equal(t, got, any("eidos: planner declares no rule that stamps facts, so build it with Generator"),
				"the panic names the plugin and the typed build that fits it")
		})

		t.Run("panics without a rule that emits", func(t *testing.T) {
			t.Parallel()

			got := assert.Panics(t, func() {
				eidos.NewPlugin("planner").Handle(eidos.OnStruct(nothing[*eidos.Stamper])).Dual()
			}, "a dual plugin without a rule that emits panics")
			assert.Equal(t, got, any("eidos: planner declares no rule that emits, so build it with Annotator"),
				"the panic names the plugin and the typed build that fits it")
		})
	})

	t.Run("Generator", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a plugin named by the declaration", func(t *testing.T) {
			t.Parallel()

			p := eidos.NewPlugin("planner").Handle(emitNothing()).Generator()
			assert.Equal(t, p.Name(), "planner", "the name is the declared one")
		})

		t.Run("returns a plugin without the annotator role", func(t *testing.T) {
			t.Parallel()

			_, annotates := eidos.NewPlugin("planner").Handle(emitNothing()).Generator().(plugin.Annotator)
			assert.False(t, annotates, "the plugin does not implement the annotator role")
		})

		t.Run("panics on a rule that stamps facts", func(t *testing.T) {
			t.Parallel()

			got := assert.Panics(t, func() {
				eidos.NewPlugin("planner").
					Handle(eidos.OnStruct(nothing[*eidos.Stamper]), eidos.OnStruct(nothing[*eidos.Emitter])).
					Generator()
			}, "a generator with a rule that stamps facts panics")
			want := any("eidos: planner declares a rule that stamps facts, so build it with Annotator or Dual")
			assert.Equal(t, got, want, "the panic names the plugin and the typed builds that fit it")
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
				Generator()

			provider, ok := p.(plugin.DirectiveProvider)
			assert.True(t, ok, "the plugin provides directives")
			assert.Equal(t, provider.Directives(), []directive.Schema{s}, "the schema is returned once")
		})

		t.Run("returns a plugin whose gate on a named handle waits for the workspace to bind it", func(t *testing.T) {
			t.Parallel()

			p := eidos.NewPlugin("t").
				Handle(eidos.Where(eidos.HasKey(meta.Named[bool]("t.flag")),
					eidos.OnEmit(symbol.KindStruct, func(*eidos.EmitMatch, *eidos.Emitter) error { return nil }))).
				Generator()
			assert.Equal(t, subscribedKeys(t, p), []meta.KeyID{0}, "the gate has no id before the binding")
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
				Handle(emitNothing()).Generator()

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
				build: func() { eidos.NewPlugin("").Handle(emitNothing()).Generator() },
			},
			{
				name:  "panics on a declaration without rules",
				build: func() { eidos.NewPlugin("t").Generator() },
			},
			{
				name:  "panics on the zero rule",
				build: func() { eidos.NewPlugin("t").Handle(eidos.Rule{}).Generator() },
			},
			{
				name: "panics on a directive wrapper around no rules",
				build: func() {
					eidos.NewPlugin("stubgen").Handle(eidos.Directive(stubSchema("stub"))).Generator()
				},
			},
			{
				name: "panics on a fact wrapper around no rules",
				build: func() {
					eidos.NewPlugin("t").Handle(eidos.Where(eidos.Pred{})).Generator()
				},
			},
			{
				name: "panics on a kernel gate naming no directive",
				build: func() {
					eidos.NewPlugin("t").Handle(eidos.Gated("", onEmit())).Generator()
				},
			},
			{
				name: "panics on a kernel gate naming a plugin's directive",
				build: func() {
					eidos.NewPlugin("t").Handle(eidos.Gated("stubgen:stub", onEmit())).Generator()
				},
			},
			{
				name: "panics on a duplicate output tag",
				build: func() {
					eidos.NewPlugin("t").
						Output(plugin.Output{Per: plugin.PerPlan, Word: "a"}).
						Output(plugin.Output{Per: plugin.PerPlan, Word: "b"}).
						Handle(emitNothing()).Generator()
				},
			},
			{
				name: "panics on an empty output word",
				build: func() {
					eidos.NewPlugin("t").
						Output(plugin.Output{Per: plugin.PerPlan}).
						Handle(emitNothing()).Generator()
				},
			},
			{
				name: "panics on a zero output cardinality",
				build: func() {
					eidos.NewPlugin("t").
						Output(plugin.Output{Word: "a"}).
						Handle(emitNothing()).Generator()
				},
			},
			{
				name: "panics on a family template that is no path inside a tree",
				build: func() {
					eidos.NewPlugin("t").
						Output(plugin.Output{Per: plugin.PerPlan, Word: "a", Template: "../test.tmpl"}).
						Handle(emitNothing()).Generator()
				},
			},
			{
				name: "panics on an empty capability label",
				build: func() {
					eidos.NewPlugin("t").Provides("").Handle(emitNothing()).Generator()
				},
			},
			{
				name: "panics on an empty required capability label",
				build: func() {
					eidos.NewPlugin("t").Requires("").Handle(emitNothing()).Generator()
				},
			},
			{
				name: "panics on an empty capability label that the plugin runs after",
				build: func() {
					eidos.NewPlugin("t").After("").Handle(emitNothing()).Generator()
				},
			},
			{
				name: "panics on a nil key registration",
				build: func() {
					eidos.NewPlugin("t").Keys(nil).Handle(emitNothing()).Generator()
				},
			},
			{
				name: "panics on a nil plugin-level tree",
				build: func() {
					eidos.NewPlugin("t").Templates(nil).Handle(emitNothing()).Generator()
				},
			},
			{
				name: "panics on a second plugin-level tree",
				build: func() {
					eidos.NewPlugin("t").
						Templates(stubTree()).Templates(stubTree()).
						Handle(emitNothing()).Generator()
				},
			},
			{
				name: "panics on a nil helper map",
				build: func() {
					eidos.NewPlugin("t").Funcs(nil).Handle(emitNothing()).Generator()
				},
			},
			{
				name: "panics on a helper name declared twice at plugin level",
				build: func() {
					eidos.NewPlugin("t").
						Funcs(template.FuncMap{toneHelper: shout}).
						Funcs(template.FuncMap{toneHelper: whisper}).
						Handle(emitNothing()).Generator()
				},
			},
			{
				name: "panics on a helper that is no function",
				build: func() {
					eidos.NewPlugin("t").
						Funcs(template.FuncMap{toneHelper: 1}).
						Handle(emitNothing()).Generator()
				},
			},
			{
				name: "panics on presentation for the zero target",
				build: func() {
					eidos.NewPlugin("t").
						For("", eidos.Templates(stubTree())).
						Handle(emitNothing()).Generator()
				},
			},
			{
				name: "panics on one target in two For declarations",
				build: func() {
					eidos.NewPlugin("t").
						For(stubTarget, eidos.Templates(stubTree())).
						For(stubTarget, eidos.Funcs(template.FuncMap{toneHelper: shout})).
						Handle(emitNothing()).Generator()
				},
			},
			{
				name: "panics on a For without options",
				build: func() {
					eidos.NewPlugin("t").For(stubTarget).Handle(emitNothing()).Generator()
				},
			},
			{
				name: "panics on the zero target option",
				build: func() {
					eidos.NewPlugin("t").
						For(stubTarget, eidos.TargetOption{}).
						Handle(emitNothing()).Generator()
				},
			},
			{
				name: "panics on a nil tree for a target",
				build: func() {
					eidos.NewPlugin("t").
						For(stubTarget, eidos.Templates(nil)).
						Handle(emitNothing()).Generator()
				},
			},
			{
				name: "panics on two trees for one target",
				build: func() {
					eidos.NewPlugin("t").
						For(stubTarget, eidos.Templates(stubTree()), eidos.Templates(stubTree())).
						Handle(emitNothing()).Generator()
				},
			},
			{
				name: "panics on a nil override map",
				build: func() {
					eidos.NewPlugin("t").
						For(stubTarget, eidos.Overrides(nil)).
						Handle(emitNothing()).Generator()
				},
			},
			{
				name: "panics on a target's helper named like its override",
				build: func() {
					eidos.NewPlugin("t").
						For(stubTarget,
							eidos.Funcs(template.FuncMap{toneHelper: shout}),
							eidos.Overrides(template.FuncMap{toneHelper: whisper})).
						Handle(emitNothing()).Generator()
				},
			},
			{
				name: "panics on one directive name in two wrappers",
				build: func() {
					eidos.NewPlugin("stubgen").Handle(
						eidos.Directive(stubSchema("stub"), onEmit()),
						eidos.Directive(stubSchema("stub"), onEmit()),
					).Generator()
				},
			},
			{
				name: "panics on a directive gate on a graph rule",
				build: func() {
					eidos.NewPlugin("stubgen").Handle(
						eidos.Directive(stubSchema("stub"), emitNothing()),
					).Generator()
				},
			},
			{
				name: "panics on a fact gate on a graph rule",
				build: func() {
					eidos.NewPlugin("t").Handle(
						eidos.Where(eidos.Pred{}, emitNothing()),
					).Generator()
				},
			},
			{
				name: "panics on a zero predicate",
				build: func() {
					eidos.NewPlugin("t").Handle(
						eidos.Where(eidos.Pred{}, onEmit()),
					).Generator()
				},
			},
			{
				name: "panics on a gate on a zero key",
				build: func() {
					var unregistered meta.Key[bool]
					eidos.NewPlugin("t").Handle(
						eidos.Where(eidos.HasKey(unregistered), onEmit()),
					).Generator()
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Panics(t, tt.build, "the declaration defect panics at the typed build")
			})
		}

		key, _ := boolKey(t)
		onFunction := eidos.OnFunction(func(*eidos.FunctionMatch, *eidos.Emitter) error { return nil })
		onMethod := eidos.OnMethod(func(*eidos.MethodMatch, *eidos.Emitter) error { return nil })
		onStruct := eidos.OnStruct(func(*eidos.StructMatch, *eidos.Emitter) error { return nil })
		onEmitMethod := eidos.OnEmit(symbol.KindMethod, func(*eidos.EmitMatch, *eidos.Emitter) error { return nil })
		overriding := stubSchema("shape")
		overriding.Overrides = true
		explicit := stubSchema("audit")
		explicit.Kinds = []symbol.Kind{symbol.KindInterface, symbol.KindStruct}
		explicitOverriding := overriding
		explicitOverriding.Kinds = []symbol.Kind{symbol.KindFunction}
		derivations := []struct {
			name  string
			rules []eidos.Rule
			want  []symbol.Kind
		}{
			{
				name:  "returns the kinds of the rules under a wrapper in the order of symbol.Kind",
				rules: []eidos.Rule{eidos.Directive(stubSchema("mock"), onStruct, onFunction)},
				want:  []symbol.Kind{symbol.KindFunction, symbol.KindStruct},
			},
			{
				name:  "returns one kind for two rules of one kind under a wrapper",
				rules: []eidos.Rule{eidos.Directive(stubSchema("mock"), onStruct, onStruct)},
				want:  []symbol.Kind{symbol.KindStruct},
			},
			{
				name:  "returns no kinds for a wrapper around an OnEmit rule",
				rules: []eidos.Rule{eidos.Directive(stubSchema("mock"), onStruct, onEmit())},
			},
			{
				name:  "returns the kinds of the bare rules of the plugin for an override schema",
				rules: []eidos.Rule{eidos.Directive(overriding, onFunction), onMethod},
				want:  []symbol.Kind{symbol.KindFunction, symbol.KindMethod},
			},
			{
				name: "returns the kinds of the fact-gated rules of the plugin for an override schema",
				rules: []eidos.Rule{
					eidos.Directive(overriding, onFunction),
					eidos.Where(eidos.HasKey(key), onStruct),
				},
				want: []symbol.Kind{symbol.KindFunction, symbol.KindStruct},
			},
			{
				name: "returns no kind of a rule under another wrapper for an override schema",
				rules: []eidos.Rule{
					eidos.Directive(overriding, onFunction),
					eidos.Directive(stubSchema("mock"), onStruct),
				},
				want: []symbol.Kind{symbol.KindFunction},
			},
			{
				name: "returns no kind of a rule under a kernel gate for an override schema",
				rules: []eidos.Rule{
					eidos.Directive(overriding, onFunction),
					eidos.Gated(directive.KernelSkip, onStruct),
				},
				want: []symbol.Kind{symbol.KindFunction},
			},
			{
				name:  "returns no kind of a graph rule for an override schema",
				rules: []eidos.Rule{eidos.Directive(overriding, onFunction), emitNothing()},
				want:  []symbol.Kind{symbol.KindFunction},
			},
			{
				name:  "returns no kinds for an override schema beside a bare OnEmit rule",
				rules: []eidos.Rule{eidos.Directive(overriding, onFunction), onEmit()},
			},
			{
				name:  "returns no kind of a bare rule for a schema that does not override",
				rules: []eidos.Rule{eidos.Directive(stubSchema("mock"), onFunction), onMethod},
				want:  []symbol.Kind{symbol.KindFunction},
			},
			{
				name:  "returns the explicit kinds in their declared order",
				rules: []eidos.Rule{eidos.Directive(explicit, onStruct)},
				want:  explicit.Kinds,
			},
			{
				name:  "returns the explicit kinds beside an OnEmit rule of another kind under the wrapper",
				rules: []eidos.Rule{eidos.Directive(explicit, onEmitMethod)},
				want:  explicit.Kinds,
			},
			{
				name:  "returns the explicit kinds of an override schema beside a bare rule of another kind",
				rules: []eidos.Rule{eidos.Directive(explicitOverriding, onFunction), onMethod},
				want:  explicitOverriding.Kinds,
			},
		}
		for _, tt := range derivations {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				schemas := schemasOf(t, eidos.NewPlugin("stubgen").Handle(tt.rules...).Generator())
				assert.NotEmpty(t, schemas, "the plugin provides the schema of its first wrapper")
				assert.Equal(t, schemas[0].Kinds, tt.want, "the kinds are the kinds that an instance can affect")
			})
		}

		t.Run("panics on explicit kinds that leave out the kind of a rule under the wrapper", func(t *testing.T) {
			t.Parallel()

			misfit := stubSchema("audit")
			misfit.Kinds = []symbol.Kind{symbol.KindInterface}
			got := assert.Panics(t, func() {
				eidos.NewPlugin("stubgen").Handle(eidos.Directive(misfit, onStruct)).Generator()
			}, "the declaration defect panics at the typed build")
			assert.Equal(t, got, any("eidos: stubgen handles Struct under directive stubgen:audit, "+
				"and the kinds of the directive leave it out"),
				"the panic states the plugin, the kind and the directive")
		})
	})
}

// TestBuilderAllocs checks in the ordinary run, which runs no benchmark,
// that each method of a declaration allocates within its ceiling. Each
// counted call takes a declaration built outside the count, because the
// first call of a method allocates its list and a declaration builds once.
// The check runs alone, because the count includes the allocations of
// every goroutine.
func TestBuilderAllocs(t *testing.T) {
	var declared *eidos.Builder
	assert.MaxAllocs(t, func() { declared = eidos.NewPlugin("bench") }, newPluginAllocs,
		"NewPlugin allocates the builder and its map of priorities")
	assert.NotNil(t, declared, "NewPlugin returns the declaration")

	fresh := func() *eidos.Builder { return eidos.NewPlugin("bench") }
	for _, tt := range declarations(t) {
		var got *eidos.Builder
		msg := tt.name + " allocates within its ceiling on a new declaration"
		assert.MaxAllocsWithSetup(t, fresh, func(b *eidos.Builder) { got = tt.set(b) }, tt.allocs, msg)
		assert.NotNil(t, got, tt.name+" returns its builder")
	}

	var p plugin.Plugin
	assert.MaxAllocsWithSetup(t, benchDeclaration(t), func(b *eidos.Builder) { p = b.Generator() }, buildAllocs,
		"Generator allocates the lowered rules, the subscriptions and the built plugin")
	assert.Equal(t, p.Name(), "bench", "Generator returns the declared plugin")
}

// BenchmarkBuilder measures the construction of a declaration, each method
// on a new declaration, and the typed build of a declaration of a graph
// rule and two emit rules under a fact gate. Each call takes a new
// declaration built outside the measurement, because the first call of a
// method allocates its list and a Builder builds once.
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
			assert.Equal(b, got, declared, "the method returns its builder", assert.ByIdentity())
		})
	}

	b.Run("Generator", func(b *testing.B) {
		declare := benchDeclaration(b)
		var declared *eidos.Builder
		c := bench.Start(b).MaxAllocs(buildAllocs)
		defer c.End()
		var p plugin.Plugin
		for c.Loop() {
			c.Excluding(func() { declared = declare() })
			p = declared.Generator()
		}
		assert.Equal(b, p.Name(), "bench", "Generator returns the declared plugin")
	})
}

// emitNothing returns a graph rule whose handler does nothing, which is
// the smallest rule that a plugin can declare.
func emitNothing() eidos.Rule {
	return eidos.OnGraph(func(m *eidos.GraphMatch, e *eidos.Emitter) error {
		return nil
	})
}

// schemasOf returns the schemas that a built plugin provides.
func schemasOf(tb assert.TB, p plugin.Plugin) []directive.Schema {
	tb.Helper()

	provider, ok := p.(plugin.DirectiveProvider)
	assert.True(tb, ok, "the plugin provides directives")
	return provider.Directives()
}

// stubSchema returns a schema that a plugin registers, for the fixtures of
// gates.
func stubSchema(name directive.Name) directive.Schema {
	return directive.Schema{
		Plugin: "stubgen", Name: name,
		Doc: "a fixture directive",
	}
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
		{name: "After", allocs: declareAllocs, set: func(b *eidos.Builder) *eidos.Builder {
			return b.After(afterLabel)
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

// benchDeclaration returns a function that declares a new bench plugin.
// The plugin has one output family, a graph rule, and two emit rules under
// one fact gate.
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
