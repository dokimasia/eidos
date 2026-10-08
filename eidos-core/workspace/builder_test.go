// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"cmp"
	"strconv"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// The allocations of a builder's methods, which TestBuilderAllocs checks
// in the ordinary run and BenchmarkBuilder in a benchmark run.
const (
	// newBuilderAllocs is the builder.
	newBuilderAllocs = 1
	// listAllocs is the first call of a method that registers values on
	// a new builder: the list it appends to.
	listAllocs = 1
	// buildAllocs is the build of the composition of 64 annotators and 8
	// plans of 4 generators: 934 allocations for the registries, the
	// roster, the capability order, the compiled plans and the
	// fingerprint, whose fold spells every registered key. The 8 more are
	// for the runtime's type-assertion caches. An interface assertion that
	// misses its call site's cache builds a new cache about once in 1,024
	// misses, so a run of one iteration counts some of these builds: 12
	// fresh processes counted 0 to 3.
	buildAllocs = 934 + 8
)

// mirrorOptions is a valid options struct for the config cases.
type mirrorOptions struct {
	Depth int `opt:"depth" doc:"how deep the mirror walks"`
}

// setter is one method of a builder that registers or sets a value,
// called with a fixture value, and whether it appends to a list.
type setter struct {
	name string
	set  func(*workspace.Builder) *workspace.Builder
	list bool
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

// hiddenOptions are options that keep one field out of the canonical
// encoding.
type hiddenOptions struct {
	Tag    string `opt:"tag"    doc:"a field the encoding sees"`
	Secret string `opt:"secret" doc:"a field the encoding does not see" json:"-"`
}

// hiding is a versioned frontend whose options hide a field from the
// canonical encoding.
type hiding struct{ plugin.Frontend }

// Version returns a fixed version.
func (hiding) Version() string { return "1" }

// Options returns options that hide a field from the encoding.
func (hiding) Options() any { return &hiddenOptions{} }

// Build is the one gate every human-typed name passes: it runs
// every step and collects, so the composition's author reads every
// fault at once.
func TestBuilder(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a builder that declares no brand", func(t *testing.T) {
			t.Parallel()

			_, err := workspace.New().Build()
			assert.HasError(t, err, "an empty composition does not build")
			assert.Contains(t, err.Error(), "declares no brand", "the builder starts without a brand")
		})
	})

	for _, tt := range setters(t) {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			t.Run("returns its builder", func(t *testing.T) {
				t.Parallel()

				b := workspace.New()
				assert.Equal(t, tt.set(b), b, "a composition chains its calls on one builder", assert.ByIdentity())
			})
		})
	}

	t.Run("BrandName", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the brand that Brand declared", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, workspace.New().Brand(fixtureBrand).BrandName(), fixtureBrand, "the declared brand")
		})

		t.Run("returns the empty brand before Brand", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, workspace.New().BrandName(), output.Brand(""), "no brand is declared")
		})
	})

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
				name: "returns an error naming a negative memo limit",
				compose: func() *workspace.Builder {
					return valid().Memo(workspace.Memo{Limit: -1})
				},
				markers: []string{"memo limit -1", "negative"},
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
				name: "returns an error naming a required name nothing registers",
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
			{
				name: "returns an error naming a frontend whose options hide a field from the encoding",
				compose: func() *workspace.Builder {
					return valid().Frontends(hiding{frontendtest.NewScripted()})
				},
				markers: []string{strconv.Quote(string(frontendtest.ScriptedID)), "hiddenOptions.Secret"},
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

// Each method of a builder allocates within its ceiling in the ordinary
// run, which runs no benchmark. A registration and a build each take a
// builder composed outside the count, because a registration's first
// call allocates the list it appends to. The count of the builds keeps
// their first error, which cmp.Or returns without allocating. The check
// runs alone, because the count includes every goroutine's allocations.
func TestBuilderAllocs(t *testing.T) {
	var built *workspace.Builder
	assert.MaxAllocs(t, func() { built = workspace.New() }, newBuilderAllocs, "New allocates the builder")
	assert.NotNil(t, built, "New returns the builder")

	for _, tt := range setters(t) {
		if !tt.list {
			b := workspace.New()
			var got *workspace.Builder
			assert.MaxAllocs(t, func() { got = tt.set(b) }, 0, tt.name+" allocates nothing")
			assert.Equal(t, got, b, tt.name+" returns its builder", assert.ByIdentity())
			continue
		}
		assert.MaxAllocsWithSetup(t, workspace.New, func(b *workspace.Builder) { tt.set(b) }, listAllocs,
			tt.name+" allocates the list of a new builder")
	}

	branded := workspace.New().Brand(fixtureBrand)
	var brand output.Brand
	assert.MaxAllocs(t, func() { brand = branded.BrandName() }, 0, "BrandName allocates nothing")
	assert.Equal(t, brand, fixtureBrand, "BrandName returns the declared brand")

	var err error
	assert.MaxAllocsWithSetup(t, composition(), func(b *workspace.Builder) {
		_, berr := b.Build()
		err = cmp.Or(err, berr)
	}, buildAllocs, "Build allocates the registries, the plans and the fingerprint")
	assert.NoError(t, err, "every composition builds")
}

// BenchmarkBuilder measures a builder's construction, each registration
// on a new builder, each setter, and the steps over the composition of
// 64 annotators and 8 plans of 4 generators. The plugin values build
// once, and the loop measures the steps.
func BenchmarkBuilder(b *testing.B) {
	b.Run("New", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(newBuilderAllocs)
		defer c.End()
		var got *workspace.Builder
		for c.Loop() {
			got = workspace.New()
		}
		assert.NotNil(b, got, "New returns the builder")
	})

	for _, tt := range setters(b) {
		b.Run(tt.name, func(b *testing.B) {
			ceiling := uint64(0)
			if tt.list {
				ceiling = listAllocs
			}
			c := bench.Start(b).MaxAllocs(ceiling)
			defer c.End()
			builder := workspace.New()
			var got *workspace.Builder
			for c.Loop() {
				if tt.list {
					c.Excluding(func() { builder = workspace.New() })
				}
				got = tt.set(builder)
			}
			assert.Equal(b, got, builder, "the method returns its builder", assert.ByIdentity())
		})
	}

	b.Run("BrandName", func(b *testing.B) {
		branded := workspace.New().Brand(fixtureBrand)
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got output.Brand
		for c.Loop() {
			got = branded.BrandName()
		}
		assert.Equal(b, got, fixtureBrand, "BrandName returns the declared brand")
	})

	b.Run("Build", func(b *testing.B) {
		compose := composition()
		c := bench.Start(b).Warmup(1).MaxAllocs(buildAllocs)
		defer c.End()
		var (
			builder *workspace.Builder
			w       *workspace.Workspace
			err     error
		)
		for c.Loop() {
			c.Excluding(func() { builder = compose() })
			w, err = builder.Build()
		}
		assert.NoError(b, err, "the composition builds")
		assert.NotNil(b, w, "to its workspace")
	})
}

// setters returns each method of a builder that registers or sets a
// value, called with a fixture value built once.
func setters(tb assert.TB) []setter {
	tb.Helper()

	var (
		frontend  plugin.Frontend       = frontendtest.NewScripted()
		annotator                       = stamper("noter", quiet)
		plan                            = planTo("plan", "fixture", mirror("mirror"))
		check     plugin.WorkspaceCheck = &recordingCheck{name: "stubbed"}
		source    rules.SourceRules     = native{}
	)
	open := func() (output.Sink, error) { return output.NewMem(), nil }
	ledgerOpen := func() (ledger.Ledger, error) { return ledger.NewMem(), nil }
	register := func(*meta.Registry) error { return nil }
	return []setter{
		{name: "Brand", set: func(b *workspace.Builder) *workspace.Builder { return b.Brand(fixtureBrand) }},
		{name: "Frontends", list: true, set: func(b *workspace.Builder) *workspace.Builder {
			return b.Frontends(frontend)
		}},
		{name: "Annotators", list: true, set: func(b *workspace.Builder) *workspace.Builder {
			return b.Annotators(annotator)
		}},
		{name: "Plans", list: true, set: func(b *workspace.Builder) *workspace.Builder { return b.Plans(plan) }},
		{name: "Checks", list: true, set: func(b *workspace.Builder) *workspace.Builder { return b.Checks(check) }},
		{name: "Output", set: func(b *workspace.Builder) *workspace.Builder { return b.Output(open) }},
		{name: "Ledger", set: func(b *workspace.Builder) *workspace.Builder { return b.Ledger(ledgerOpen) }},
		{
			name: "Workspace",
			set:  func(b *workspace.Builder) *workspace.Builder { return b.Workspace(string(fixtureBrand)) },
		},
		{name: "Parallel", set: func(b *workspace.Builder) *workspace.Builder { return b.Parallel(4) }},
		{
			name: "Targets",
			list: true,
			set:  func(b *workspace.Builder) *workspace.Builder { return b.Targets("fixture") },
		},
		{name: "Keys", list: true, set: func(b *workspace.Builder) *workspace.Builder { return b.Keys(register) }},
		{name: "Rules", list: true, set: func(b *workspace.Builder) *workspace.Builder { return b.Rules(source) }},
		{name: "Config", set: func(b *workspace.Builder) *workspace.Builder { return b.Config(workspace.Config{}) }},
		{name: "Ignore", list: true, set: func(b *workspace.Builder) *workspace.Builder {
			return b.Ignore(directive.Name("legacy:"))
		}},
		{name: "Memo", set: func(b *workspace.Builder) *workspace.Builder {
			return b.Memo(workspace.Memo{Limit: memoLimit})
		}},
	}
}

// composition returns a function that composes 105 plugins on a new
// builder: 64 annotators forming one capability chain inside one
// priority, and 8 plans of 4 generators each behind their backends.
// Every builder it returns shares the plugin values, which build once.
func composition() func() *workspace.Builder {
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
	return func() *workspace.Builder {
		return workspace.New().Brand(fixtureBrand).Annotators(anns...).Targets("fixture").Plans(plans...)
	}
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
