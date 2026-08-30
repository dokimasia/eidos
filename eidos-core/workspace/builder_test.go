// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package workspace_test

import (
	"strconv"
	"testing"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// mirrorOptions is a lawful options struct for the config cases.
type mirrorOptions struct {
	Depth int `opt:"depth" doc:"how deep the mirror walks"`
}

// tuned answers a generator declaring cfg as its options struct.
func tuned(name plugin.ID, cfg any) plugin.Generator {
	p, held := eidos.NewPlugin(name).
		Options(cfg).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
		Handle(eidos.OnStruct(func(*eidos.StructMatch, *eidos.Emitter) error {
			return nil
		})).Build().(plugin.Generator)
	if !held {
		panic("workspace_test: an emitter rule lowers to the generator seat")
	}
	return p
}

// needy answers a generator whose directive schema requires a name
// nothing registers.
func needy() plugin.Generator {
	p, held := eidos.NewPlugin("needy").
		Output(plugin.Output{Per: plugin.PerPlan, Word: "gen"}).
		Handle(eidos.Directive(
			directive.Schema{
				Plugin: "needy", Name: "gate",
				Requires: []directive.Name{"ghost"},
				Doc:      "a fixture directive requiring a ghost",
			},
			eidos.OnEmit(symbol.KindStruct,
				func(*eidos.EmitMatch, *eidos.Emitter) error { return nil }),
		)).Build().(plugin.Generator)
	if !held {
		panic("workspace_test: an emitter rule lowers to the generator seat")
	}
	return p
}

// Build is the one gate every human-typed name passes: it climbs
// the ladder whole, collecting, so the composition's author reads
// every fault at once instead of an instalment plan.
func TestBuilder(t *testing.T) {
	t.Parallel()

	t.Run("a lawful composition climbs the ladder whole", func(t *testing.T) {
		t.Parallel()

		w, err := lawful().Build()
		assert.NoError(t, err, "no fault, no bill")
		assert.NotNil(t, w, "and the workspace is answered")
	})

	t.Run("every fault lands on the bill", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			compose func() *workspace.Builder
			markers []string
		}{
			{
				name: "two plugins answering one name",
				compose: func() *workspace.Builder {
					return lawful().Annotators(stamper("noter", quiet))
				},
				markers: []string{"two plugins", `"noter"`},
			},
			{
				name: "a nil annotator",
				compose: func() *workspace.Builder {
					return lawful().Annotators(nil)
				},
				markers: []string{"nil"},
			},
			{
				name: "a plugin named after a kernel phase",
				compose: func() *workspace.Builder {
					return lawful().Annotators(stamper("freeze", quiet))
				},
				markers: []string{"kernel phase", `"freeze"`},
			},
			{
				name: "a namespace claimed twice",
				compose: func() *workspace.Builder {
					claim := func(owner string) func(r *meta.Registry) error {
						return func(r *meta.Registry) error {
							return r.ClaimNamespace("shape", owner)
						}
					}
					return lawful().Keys(claim("one"), claim("another"))
				},
				markers: []string{"claimed twice", `"shape"`},
			},
			{
				name: "a nil key registration",
				compose: func() *workspace.Builder {
					return lawful().Keys(nil)
				},
				markers: []string{"key registration", "nil"},
			},
			{
				name: "a schema requiring a ghost",
				compose: func() *workspace.Builder {
					return lawful().Plans(planTo("second", "fixture", needy()))
				},
				markers: []string{"ghost"},
			},
			{
				name: "a capability provided twice",
				compose: func() *workspace.Builder {
					var calls []plugin.ID
					return lawful().Annotators(
						ordered("left", 1, caps("json"), nil, &calls),
						ordered("right", 1, caps("json"), nil, &calls),
					)
				},
				markers: []string{`"json"`, "provided", "left", "right"},
			},
			{
				name: "a required capability nothing provides",
				compose: func() *workspace.Builder {
					var calls []plugin.ID
					return lawful().Annotators(
						ordered("wanting", 1, nil, caps("missing"), &calls),
					)
				},
				markers: []string{`"missing"`, "requires", "wanting"},
			},
			{
				name: "a capability cycle",
				compose: func() *workspace.Builder {
					var calls []plugin.ID
					return lawful().Annotators(
						ordered("ouro", 1, caps("head"), caps("tail"), &calls),
						ordered("boros", 1, caps("tail"), caps("head"), &calls),
					)
				},
				markers: []string{"cycle", "ouro", "boros"},
			},
			{
				name: "an empty target name",
				compose: func() *workspace.Builder {
					return lawful().Targets("")
				},
				markers: []string{"target", "empty"},
			},
			{
				name: "a target declared twice",
				compose: func() *workspace.Builder {
					return lawful().Targets("fixture")
				},
				markers: []string{`"fixture"`, "twice"},
			},
			{
				name: "a malformed options struct",
				compose: func() *workspace.Builder {
					undocumented := &struct {
						Depth int `opt:"depth"`
					}{}
					return lawful().Plans(
						planTo("second", "fixture", tuned("tuned", undocumented)),
					)
				},
				markers: []string{"tuned", "doc"},
			},
			{
				name: "a config section for a stranger",
				compose: func() *workspace.Builder {
					return lawful().Config(workspace.Config{
						Options: map[string]map[string]any{"ghost": {"depth": 1}},
					})
				},
				markers: []string{`"ghost"`, "not hold"},
			},
			{
				name: "a config key nothing declares",
				compose: func() *workspace.Builder {
					return lawful().
						Plans(planTo("second", "fixture", tuned("tuned", &mirrorOptions{}))).
						Config(workspace.Config{
							Options: map[string]map[string]any{"tuned": {"nope": true}},
						})
				},
				markers: []string{`"nope"`, `"tuned"`},
			},
			{
				name: "a config value of the wrong type",
				compose: func() *workspace.Builder {
					return lawful().
						Plans(planTo("second", "fixture", tuned("tuned", &mirrorOptions{}))).
						Config(workspace.Config{
							Options: map[string]map[string]any{"tuned": {"depth": "deep"}},
						})
				},
				markers: []string{`"depth"`, "int", "string"},
			},
			{
				name: "a plan with no name",
				compose: func() *workspace.Builder {
					return lawful().Plans(planTo("", "fixture", mirror("second")))
				},
				markers: []string{"plan", "no name"},
			},
			{
				name: "two plans answering one name",
				compose: func() *workspace.Builder {
					return lawful().Plans(planTo("plan", "fixture", mirror("second")))
				},
				markers: []string{`"plan"`, "twice"},
			},
			{
				name: "a plan with no generators",
				compose: func() *workspace.Builder {
					return lawful().Plans(planTo("second", "fixture"))
				},
				markers: []string{`"second"`, "no generator"},
			},
			{
				name: "a plan with a nil generator",
				compose: func() *workspace.Builder {
					return lawful().Plans(planTo("second", "fixture", nil))
				},
				markers: []string{`"second"`, "nil generator"},
			},
			{
				name: "a plan listing one generator twice",
				compose: func() *workspace.Builder {
					m := mirror("second")
					return lawful().Plans(planTo("second", "fixture", m, m))
				},
				markers: []string{`"second"`, "twice"},
			},
			{
				name: "a plan with no backend",
				compose: func() *workspace.Builder {
					return lawful().Plans(workspace.Plan{
						Name:       "second",
						Generators: []plugin.Generator{mirror("second")},
					})
				},
				markers: []string{`"second"`, "no backend"},
			},
			{
				name: "an unregistered target",
				compose: func() *workspace.Builder {
					return lawful().Plans(planTo("second", "mars", mirror("second")))
				},
				markers: []string{`"second"`, `"mars"`},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := tt.compose().Build()
				assert.HasError(t, err, "the fault lands on the bill")
				for _, marker := range tt.markers {
					assert.Contains(t, err.Error(), marker, "the bill names it")
				}
			})
		}
	})

	t.Run("five faults across the ladder answer one bill", func(t *testing.T) {
		t.Parallel()

		var calls []plugin.ID
		_, err := workspace.New().
			Annotators(
				stamper("twin", quiet),
				stamper("twin", quiet),
				ordered("left", 1, caps("json"), nil, &calls),
				ordered("right", 1, caps("json"), nil, &calls),
				ordered("ouro", 2, caps("head"), caps("tail"), &calls),
				ordered("boros", 2, caps("tail"), caps("head"), &calls),
			).
			Targets("fixture").
			Plans(planTo("plan", "mars", mirror("mirror"))).
			Config(workspace.Config{
				Options: map[string]map[string]any{"ghost": {"depth": 1}},
			}).
			Build()
		assert.HasError(t, err, "Build collects rather than stopping")
		for _, marker := range []string{
			`"twin"`,  // the roster: two plugins, one name
			`"json"`,  // the registries: a capability provided twice
			"cycle",   // the lowering: a capability cycle
			`"ghost"`, // the options: a section for a stranger
			`"mars"`,  // the plans: an unregistered target
		} {
			assert.Contains(t, err.Error(), marker,
				"all five faults land on the one bill")
		}
	})
}

// BenchmarkBuild climbs the ladder over a composition of 105
// plugins: 64 annotators forming one capability chain inside one
// priority, and 8 plans of 4 generators each behind their
// backends. The plugin values build once; the ladder is what the
// loop prices.
func BenchmarkBuild(b *testing.B) {
	b.ReportAllocs()

	const seats, planned, width = 64, 8, 4
	var order []plugin.ID
	anns := make([]plugin.Annotator, 0, seats)
	for i := range seats {
		provides := caps(plugin.Capability("cap-" + strconv.Itoa(i)))
		var requires []plugin.Capability
		if i > 0 {
			requires = caps(plugin.Capability("cap-" + strconv.Itoa(i-1)))
		}
		anns = append(anns, ordered(
			plugin.ID("ann-"+strconv.Itoa(i)), 1, provides, requires, &order,
		))
	}
	plans := make([]workspace.Plan, 0, planned)
	for i := range planned {
		name := "plan-" + strconv.Itoa(i)
		gens := make([]plugin.Generator, 0, width)
		for j := range width {
			gens = append(gens, mirror(plugin.ID(name+"-gen-"+strconv.Itoa(j))))
		}
		plans = append(plans, planTo(name, "fixture", gens...))
	}

	for b.Loop() {
		w, err := workspace.New().
			Annotators(anns...).
			Targets("fixture").
			Plans(plans...).
			Build()
		if err != nil {
			b.Fatalf("Build: unexpected error: %v", err)
		}
		if w == nil {
			b.Fatal("Build must answer the workspace")
		}
	}
}
