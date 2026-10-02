// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"strconv"
	"testing"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// mirrorOptions is a valid options struct for the config cases.
type mirrorOptions struct {
	Depth int `opt:"depth" doc:"how deep the mirror walks"`
}

// tuned returns a generator declaring cfg as its options struct.
func tuned(name plugin.ID, cfg any) plugin.Generator {
	p, held := eidos.NewPlugin(name).
		Options(cfg).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
		Handle(eidos.OnStruct(func(*eidos.StructMatch, *eidos.Emitter) error {
			return nil
		})).Build().(plugin.Generator)
	if !held {
		panic("workspace_test: an emitter rule lowers to the generator role")
	}
	return p
}

// needy returns a generator whose directive schema requires a name
// nothing registers.
func needy() plugin.Generator {
	p, held := eidos.NewPlugin("needy").
		Output(plugin.Output{Per: plugin.PerSource, Word: "gen"}).
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
		panic("workspace_test: an emitter rule lowers to the generator role")
	}
	return p
}

// unversioned is a frontend that declares no version: the embedded
// interface promotes the frontend's methods and not its version.
type unversioned struct{ plugin.Frontend }

// renamed is a versioned frontend under another name.
type renamed struct {
	plugin.Frontend
	name plugin.ID
}

// Name returns the frontend's new name.
func (r renamed) Name() plugin.ID { return r.name }

// Version returns a fixed version.
func (renamed) Version() string { return "1" }

// Build is the one gate every human-typed name passes: it runs
// every step and collects, so the composition's author reads every
// fault at once.
func TestBuilder(t *testing.T) {
	t.Parallel()

	t.Run("Build", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the workspace for a valid composition", func(t *testing.T) {
			t.Parallel()

			w, err := valid().Build()
			assert.NoError(t, err, "the composition builds")
			assert.NotNil(t, w, "the workspace is returned")
		})

		t.Run("returns the workspace for a frontend named after its language's backend", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Frontends(renamed{frontendtest.NewScripted(), "plan-printer"}).Build()
			assert.NoError(t, err, "a language's frontend and backend share the language's name")
		})

		t.Run("returns the workspace for zero workers", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Parallel(0).Build()
			assert.NoError(t, err, "zero workers dispatch sequentially")
		})

		tests := []struct {
			name    string
			compose func() *workspace.Builder
			markers []string
		}{
			{
				name: "returns an error for a composition without a brand",
				compose: func() *workspace.Builder {
					return valid().Brand("")
				},
				markers: []string{"declares no brand"},
			},
			{
				name: "returns an error naming a brand outside the spelling",
				compose: func() *workspace.Builder {
					return valid().Brand("Acme")
				},
				markers: []string{`"Acme"`, "not a brand"},
			},
			{
				name: "returns an error naming a negative worker count",
				compose: func() *workspace.Builder {
					return valid().Parallel(-1)
				},
				markers: []string{"Parallel(-1)", "negative"},
			},
			{
				name: "returns an error naming two plugins with one name",
				compose: func() *workspace.Builder {
					return valid().Annotators(stamper("noter", quiet))
				},
				markers: []string{"two plugins", `"noter"`},
			},
			{
				name: "returns an error for a nil annotator",
				compose: func() *workspace.Builder {
					return valid().Annotators(nil)
				},
				markers: []string{"nil"},
			},
			{
				name: "returns an error naming a plugin named after a kernel phase",
				compose: func() *workspace.Builder {
					return valid().Annotators(stamper("freeze", quiet))
				},
				markers: []string{"kernel phase", `"freeze"`},
			},
			{
				name: "returns an error naming a plugin named after the layout phase",
				compose: func() *workspace.Builder {
					return valid().Annotators(stamper("layout", quiet))
				},
				markers: []string{"kernel phase", `"layout"`},
			},
			{
				name: "returns an error naming a layout refinement of a generator outside the plan",
				compose: func() *workspace.Builder {
					p := planTo("second", "fixture", mirror("second-mirror"))
					p.Layout = layout.Config{Plugins: map[plugin.ID]layout.Refinement{"ghost": {}}}
					return valid().Plans(p)
				},
				markers: []string{`plan "second"`, "ghost", "no generator of the plan"},
			},
			{
				name: "returns an error naming a per-plan family without a directory",
				compose: func() *workspace.Builder {
					indexer, held := eidos.NewPlugin("indexer").
						Output(plugin.Output{Per: plugin.PerPlan, Word: "index"}).
						Handle(eidos.OnStruct(mirrored)).
						Build().(plugin.Generator)
					if !held {
						panic("workspace_test: an emitter rule lowers to the generator role")
					}
					return valid().Plans(planTo("second", "fixture", indexer))
				},
				markers: []string{`plan "second"`, "indexer", "states none"},
			},
			{
				name: "returns an error naming a namespace the composition claims twice",
				compose: func() *workspace.Builder {
					claim := func(r *meta.Registry) error { return r.ClaimNamespace("shape") }
					return valid().Keys(claim).Keys(claim)
				},
				markers: []string{"claimed twice", `"shape"`},
			},
			{
				name: "returns an error for a nil key registration",
				compose: func() *workspace.Builder {
					return valid().Keys(nil)
				},
				markers: []string{"key registration", "nil"},
			},
			{
				name: "returns an error naming a name a schema requires and nothing registers",
				compose: func() *workspace.Builder {
					return valid().Plans(planTo("second", "fixture", needy()))
				},
				markers: []string{"ghost"},
			},
			{
				name: "returns an error naming a capability provided twice",
				compose: func() *workspace.Builder {
					var calls []plugin.ID
					return valid().Annotators(
						ordered("left", 1, caps("json"), nil, &calls),
						ordered("right", 1, caps("json"), nil, &calls),
					)
				},
				markers: []string{`"json"`, "provided", "left", "right"},
			},
			{
				name: "returns an error naming a required capability nothing provides",
				compose: func() *workspace.Builder {
					var calls []plugin.ID
					return valid().Annotators(
						ordered("wanting", 1, nil, caps("missing"), &calls),
					)
				},
				markers: []string{`"missing"`, "requires", "wanting"},
			},
			{
				name: "returns an error naming the plugins of a capability cycle",
				compose: func() *workspace.Builder {
					var calls []plugin.ID
					return valid().Annotators(
						ordered("ouro", 1, caps("head"), caps("tail"), &calls),
						ordered("boros", 1, caps("tail"), caps("head"), &calls),
					)
				},
				markers: []string{"cycle", "ouro", "boros"},
			},
			{
				name: "returns an error for an empty target name",
				compose: func() *workspace.Builder {
					return valid().Targets("")
				},
				markers: []string{"target", "empty"},
			},
			{
				name: "returns an error naming a target declared twice",
				compose: func() *workspace.Builder {
					return valid().Targets("fixture")
				},
				markers: []string{`"fixture"`, "twice"},
			},
			{
				name: "returns an error naming a plugin with a malformed options struct",
				compose: func() *workspace.Builder {
					undocumented := &struct {
						Depth int `opt:"depth"`
					}{}
					return valid().Plans(
						planTo("second", "fixture", tuned("tuned", undocumented)),
					)
				},
				markers: []string{"tuned", "doc"},
			},
			{
				name: "returns an error naming a config section for a plugin the composition does not contain",
				compose: func() *workspace.Builder {
					return valid().Config(workspace.Config{
						Options: map[string]map[string]any{"ghost": {"depth": 1}},
					})
				},
				markers: []string{`"ghost"`, "does not contain"},
			},
			{
				name: "returns an error naming a config key nothing declares",
				compose: func() *workspace.Builder {
					return valid().
						Plans(planTo("second", "fixture", tuned("tuned", &mirrorOptions{}))).
						Config(workspace.Config{
							Options: map[string]map[string]any{"tuned": {"nope": true}},
						})
				},
				markers: []string{`"nope"`, `"tuned"`},
			},
			{
				name: "returns an error naming a config value of the wrong type",
				compose: func() *workspace.Builder {
					return valid().
						Plans(planTo("second", "fixture", tuned("tuned", &mirrorOptions{}))).
						Config(workspace.Config{
							Options: map[string]map[string]any{"tuned": {"depth": "deep"}},
						})
				},
				markers: []string{`"depth"`, "int", "string"},
			},
			{
				name: "returns an error for a plan without a name",
				compose: func() *workspace.Builder {
					return valid().Plans(planTo("", "fixture", mirror("second")))
				},
				markers: []string{"plan", "no name"},
			},
			{
				name: "returns an error naming two plans with one name",
				compose: func() *workspace.Builder {
					return valid().Plans(planTo("plan", "fixture", mirror("second")))
				},
				markers: []string{`"plan"`, "twice"},
			},
			{
				name: "returns an error naming a plan without generators",
				compose: func() *workspace.Builder {
					return valid().Plans(planTo("second", "fixture"))
				},
				markers: []string{`"second"`, "no generator"},
			},
			{
				name: "returns an error naming a plan with a nil generator",
				compose: func() *workspace.Builder {
					return valid().Plans(planTo("second", "fixture", nil))
				},
				markers: []string{`"second"`, "nil generator"},
			},
			{
				name: "returns an error naming a plan that lists one generator twice",
				compose: func() *workspace.Builder {
					m := mirror("second")
					return valid().Plans(planTo("second", "fixture", m, m))
				},
				markers: []string{`"second"`, "twice"},
			},
			{
				name: "returns an error naming a plan without a backend",
				compose: func() *workspace.Builder {
					return valid().Plans(workspace.Plan{
						Name:       "second",
						Generators: []plugin.Generator{mirror("second")},
					})
				},
				markers: []string{`"second"`, "no backend"},
			},
			{
				name: "returns an error naming a plan's unregistered target",
				compose: func() *workspace.Builder {
					return valid().Plans(planTo("second", "mars", mirror("second")))
				},
				markers: []string{`"second"`, `"mars"`},
			},
			{
				name: "returns an error naming an ignore that covers a kernel name",
				compose: func() *workspace.Builder {
					return valid().Ignore(directive.KernelSkip)
				},
				markers: []string{"skip", "kernel"},
			},
			{
				name: "returns an error naming a language registered twice",
				compose: func() *workspace.Builder {
					return valid().Rules(native{}, native{})
				},
				markers: []string{string(coretest.Lang)},
			},
			{
				name: "returns an error for nil rules",
				compose: func() *workspace.Builder {
					return valid().Rules(nil)
				},
				markers: []string{"nil"},
			},
			{
				name: "returns an error for a nil frontend",
				compose: func() *workspace.Builder {
					return valid().Frontends(nil)
				},
				markers: []string{"frontend 1 of 1 is nil"},
			},
			{
				name: "returns an error naming a frontend without a version",
				compose: func() *workspace.Builder {
					return valid().Frontends(unversioned{frontendtest.NewScripted()})
				},
				markers: []string{"declares no version", strconv.Quote(string(frontendtest.ScriptedID))},
			},
			{
				name: "returns an error naming two frontends with one name",
				compose: func() *workspace.Builder {
					return valid().Frontends(frontendtest.NewScripted(), frontendtest.NewScripted())
				},
				markers: []string{"two frontends", strconv.Quote(string(frontendtest.ScriptedID))},
			},
			{
				name: "returns an error naming a frontend named after a kernel phase",
				compose: func() *workspace.Builder {
					return valid().Frontends(renamed{frontendtest.NewScripted(), "load"})
				},
				markers: []string{"kernel phase", `"load"`},
			},
			{
				name: "returns an error for a frontend with an empty name",
				compose: func() *workspace.Builder {
					return valid().Frontends(renamed{frontendtest.NewScripted(), ""})
				},
				markers: []string{"empty name"},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := tt.compose().Build()
				assert.HasError(t, err, "the composition fails")
				for _, marker := range tt.markers {
					assert.Contains(t, err.Error(), marker, "the error names the fault")
				}
			})
		}

		t.Run("returns one error joining the faults of every step", func(t *testing.T) {
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
			assert.HasError(t, err, "Build collects and does not stop")
			for _, marker := range []string{
				"no brand", // the brand: none declared
				`"twin"`,   // the roster: two plugins, one name
				`"json"`,   // the registries: a capability provided twice
				"cycle",    // the lowering: a capability cycle
				`"ghost"`,  // the options: a section for another plugin
				`"mars"`,   // the plans: an unregistered target
			} {
				assert.Contains(t, err.Error(), marker, "the error names each step's fault")
			}
		})
	})
}

// BenchmarkBuild runs the steps over a composition of 105
// plugins: 64 annotators forming one capability chain inside one
// priority, and 8 plans of 4 generators each behind their
// backends. The plugin values build once, and the loop measures
// the steps.
func BenchmarkBuild(b *testing.B) {
	b.ReportAllocs()

	const roles, planned, width = 64, 8, 4
	var order []plugin.ID
	anns := make([]plugin.Annotator, 0, roles)
	for i := range roles {
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
			Brand(fixtureBrand).
			Annotators(anns...).
			Targets("fixture").
			Plans(plans...).
			Build()
		if err != nil {
			b.Fatalf("Build: unexpected error: %v", err)
		}
		if w == nil {
			b.Fatal("Build must return the workspace")
		}
	}
}
