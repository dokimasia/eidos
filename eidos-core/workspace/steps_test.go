// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"errors"
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// framing is a backend stating a comment syntax and rendering
// nothing: what a writing composition refuses at Build.
type framing struct{ fakeBackend }

func (framing) Syntax() plugin.CommentSyntax {
	return plugin.CommentSyntax{Line: []string{"//"}}
}

// runOrdering composes the annotators over the one-struct fixture
// and runs, so each role's handler runs exactly once, in schedule
// order.
func runOrdering(t *testing.T, anns ...plugin.Annotator) {
	t.Helper()

	w, err := workspace.New().
		Annotators(anns...).
		Targets("fixture").
		Plans(planTo("plan", "fixture", mirror("mirror"))).
		Build()
	assert.NoError(t, err, "the ordering fixture composes")
	g, _ := alpha(t)
	_, err = w.Run(t.Context(), g)
	assert.NoError(t, err, "the ordering fixture runs")
}

// nameless is a hand-rolled annotator returning no name. The facade
// cannot spell one, and the roster still has to refuse it, because
// findings, units and claims all key on the name.
type nameless struct{}

func (nameless) Name() plugin.ID                         { return "" }
func (nameless) Annotate(*plugin.AnnotatorContext) error { return nil }

// handmade is a hand-rolled annotator passed as a struct value
// carrying a slice, so its type is no map key. The facade builds
// pointers, and the composition still answers for a plugin arriving
// through the service provider interface in any shape the interface
// admits.
type handmade struct {
	name   plugin.ID
	labels []string
}

func (h handmade) Name() plugin.ID                       { return h.name }
func (handmade) Annotate(*plugin.AnnotatorContext) error { return nil }

// declaring is a hand-rolled annotator returning its capability
// lists verbatim, duplicates and empty labels included. The facade
// refuses an empty label at its own Build, and the composition
// still answers for one arriving through the service provider
// interface.
type declaring struct {
	name     plugin.ID
	provides []plugin.Capability
	requires []plugin.Capability
}

func (d *declaring) Name() plugin.ID                       { return d.name }
func (*declaring) Annotate(*plugin.AnnotatorContext) error { return nil }
func (*declaring) Priority(plugin.Role) int                { return 1 }
func (d *declaring) Provides() []plugin.Capability         { return d.provides }
func (d *declaring) Requires() []plugin.Capability         { return d.requires }

// capable spells one hand-rolled annotator's capability lists
// inline.
func capable(name plugin.ID, provides, requires []plugin.Capability) plugin.Annotator {
	return &declaring{name: name, provides: provides, requires: requires}
}

// keyless returns an annotator whose key registration refuses.
func keyless(name plugin.ID, because string) plugin.Annotator {
	p, held := eidos.NewPlugin(name).
		Keys(func(*meta.Registry) error { return errors.New(because) }).
		Handle(eidos.OnStruct(quiet)).Build().(plugin.Annotator)
	if !held {
		panic("workspace_test: a stamper rule lowers to the annotator role")
	}
	return p
}

// schemad returns a generator owning s, so the registration step
// meets a plugin's own schema.
func schemad(name plugin.ID, s directive.Schema) plugin.Generator {
	p, held := eidos.NewPlugin(name).
		Output(plugin.Output{Per: plugin.PerPlan, Word: "gen"}).
		Handle(eidos.Directive(s, eidos.OnEmit(symbol.KindStruct,
			func(*eidos.EmitMatch, *eidos.Emitter) error { return nil }))).
		Build().(plugin.Generator)
	if !held {
		panic("workspace_test: an emitter rule lowers to the generator role")
	}
	return p
}

// sectioned returns a composition carrying one config section for
// one plugin's options struct.
func sectioned(name string, cfg any, section map[string]any) *workspace.Builder {
	return valid().
		Plans(planTo("second", "fixture", tuned(plugin.ID(name), cfg))).
		Config(workspace.Config{Options: map[string]map[string]any{name: section}})
}

// The steps' output is the schedule, and the schedule is
// observable twice over: the order handlers run in, and the bucket
// number every claim carries.
func TestSteps(t *testing.T) {
	t.Parallel()

	t.Run("assemble", func(t *testing.T) {
		t.Parallel()

		t.Run("refuses a plugin returning no name", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Annotators(nameless{}).Build()
			assert.HasError(t, err, "everything durable keys on the name")
			assert.Contains(t, err.Error(), "empty name", "and the fault says so")
		})

		t.Run("seats a plugin whose type is no map key", func(t *testing.T) {
			t.Parallel()

			one := handmade{name: "handmade", labels: []string{"a"}}
			_, err := valid().Annotators(one).Build()
			assert.NoError(t, err,
				"composition reads a plugin through its name and never "+
					"hashes the value, so a slice field is no obstacle")
		})

		t.Run("seats one provider listed twice once", func(t *testing.T) {
			t.Parallel()

			// One pointer repeated is the same provider named again,
			// not two plugins under one name.
			one := &handmade{name: "listed"}
			_, err := valid().Annotators(one, one).Build()
			assert.NoError(t, err,
				"one provider listed twice is seated once, not a collision")
		})

		t.Run("refuses two providers under one name", func(t *testing.T) {
			t.Parallel()

			_, err := valid().
				Annotators(&handmade{name: "twinned"}, &handmade{name: "twinned"}).
				Build()
			assert.HasError(t, err, "two plugins cannot share a name")
			assert.Contains(t, err.Error(), "twinned",
				"and the fault names the name they contend for")
		})
	})

	t.Run("register", func(t *testing.T) {
		t.Parallel()

		t.Run("collects a plugin's own key registration fault", func(t *testing.T) {
			t.Parallel()

			_, err := valid().
				Annotators(keyless("keyless", "the shape namespace is spoken for")).
				Build()
			assert.HasError(t, err, "a key provider's refusal is the composition's fault")
			assert.Contains(t, err.Error(), "spoken for", "carrying its cause")
		})

		t.Run("collects a plugin's own schema fault", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Plans(planTo("second", "fixture",
				schemad("picky", directive.Schema{Plugin: "picky", Name: "gate"}))).
				Build()
			assert.HasError(t, err, "a schema stating no semantics registers nowhere")
			assert.Contains(t, err.Error(), "states no semantics", "and the fault says so")
			assert.Contains(t, err.Error(), "gate", "naming the schema")
		})

		t.Run("seals the key registry before a run stamps", func(t *testing.T) {
			t.Parallel()

			w, err := valid().Build()
			assert.NoError(t, err, "the composition builds")
			g, _ := alpha(t)
			report, err := w.Run(t.Context(), g)
			assert.NoError(t, err, "and runs")

			assert.HasError(t, report.Facts.Registry().ClaimNamespace("late", "latecomer"),
				"a namespace claimed during a run refuses")
			_, err = meta.Register[string](report.Facts.Registry(), meta.KeySpec{
				Name: "late.role", Doc: "registered after the seal",
			})
			assert.HasError(t, err, "and so does a key")
		})
	})

	t.Run("capabilities", func(t *testing.T) {
		t.Parallel()

		t.Run("refuses an empty provided label", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Annotators(capable("hollow", caps(""), nil)).Build()
			assert.HasError(t, err, "a label nothing spells orders nothing")
			assert.Contains(t, err.Error(), "provides an empty capability label",
				"and the fault says which side it came from")
			assert.Contains(t, err.Error(), "hollow", "naming the plugin")
		})

		t.Run("refuses an empty required label", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Annotators(capable("wanting", nil, caps(""))).Build()
			assert.HasError(t, err, "a label nothing spells orders nothing")
			assert.Contains(t, err.Error(), "requires an empty capability label",
				"and the fault says which side it came from")
			assert.Contains(t, err.Error(), "wanting", "naming the plugin")
		})

		t.Run("one plugin naming a label twice is one provider", func(t *testing.T) {
			t.Parallel()

			_, err := valid().
				Annotators(capable("twice", caps("json", "json"), nil)).
				Build()
			assert.NoError(t, err,
				"a label listed twice by one plugin is not two providers, "+
					"so it does not collide with itself")
		})

		t.Run("one plugin asking twice is asked about once", func(t *testing.T) {
			t.Parallel()

			_, err := valid().
				Annotators(capable("asking", nil, caps("missing", "missing"))).
				Build()
			assert.HasError(t, err, "nothing provides the label")
			assert.Equal(t,
				strings.Count(err.Error(), `requires capability "missing"`), 1,
				"and the composition's author reads the gap once, not once per mention")
		})
	})

	t.Run("configure", func(t *testing.T) {
		t.Parallel()

		t.Run("the config replaces a constructed default", func(t *testing.T) {
			t.Parallel()

			opts := &mirrorOptions{Depth: 1}
			w, err := sectioned("tuned", opts, map[string]any{"depth": 3}).Build()
			assert.NoError(t, err, "the section holds the tag contract")
			assert.NotNil(t, w, "and the workspace composes")
			assert.Equal(t, opts.Depth, 3,
				"the plugin reads the configured value off the struct it declared")
		})

		t.Run("a plugin whose schema failed is not populated", func(t *testing.T) {
			t.Parallel()

			opts := &struct {
				Depth int `opt:"depth"`
			}{}
			_, err := sectioned("tuned", opts, map[string]any{"depth": 3}).Build()
			assert.HasError(t, err, "the options struct is undocumented")
			assert.Contains(t, err.Error(), "doc", "which is the fault reported")
			assert.Equal(t, opts.Depth, 0,
				"and population is skipped, so one broken struct is one fault")
		})

		t.Run("refuses options with a field the encoding cannot see", func(t *testing.T) {
			t.Parallel()

			opts := &struct {
				Depth  int  `opt:"depth"  doc:"how deep the mirror walks"`
				Strict bool `opt:"strict" doc:"whether the walk refuses a gap" json:"-"`
			}{}
			_, err := sectioned("tuned", opts, map[string]any{"depth": 3}).Build()
			assert.HasError(t, err, "the fingerprint folds every option, so a hidden field refuses")
			assert.Contains(t, err.Error(), "Strict", "naming the hidden field")
		})

		t.Run("refuses options the encoding cannot encode", func(t *testing.T) {
			t.Parallel()

			opts := &struct {
				Hook func() `opt:"hook" doc:"what runs after the walk"`
			}{}
			_, err := valid().Plans(planTo("second", "fixture", tuned("tuned", opts))).Build()
			assert.HasError(t, err, "an options struct the encoder refuses has no fingerprint")
			assert.Contains(t, err.Error(), "tuned", "naming the plugin")
		})

		t.Run("refuses a section for a plugin declaring no options", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Config(workspace.Config{
				Options: map[string]map[string]any{"mirror": {"depth": 1}},
			}).Build()
			assert.HasError(t, err, "a section nothing reads is a typo, not a default")
			assert.Contains(t, err.Error(), "declares no options", "and the fault says so")
			assert.Contains(t, err.Error(), `"mirror"`, "naming the plugin")
		})
	})

	t.Run("stampable", func(t *testing.T) {
		t.Parallel()

		t.Run("refuses writing through a backend that does not render", func(t *testing.T) {
			t.Parallel()

			_, err := workspace.New().
				Annotators(stamper("noter", quiet)).
				Targets("fixture").
				Plans(workspace.Plan{
					Name:       "plan",
					Generators: []plugin.Generator{mirror("mirror")},
					Backend:    framing{fakeBackend{name: "framer", target: "fixture"}},
				}).
				Output(output.NewMem(), "eidos").
				Build()
			assert.HasError(t, err, "a writing composition needs every backend to render")
			assert.Contains(t, err.Error(), "does not render", "and the fault names the cause")
			assert.Contains(t, err.Error(), "framer", "naming the backend")
		})
	})

	t.Run("priority places the roles", func(t *testing.T) {
		t.Parallel()

		var calls []plugin.ID
		runOrdering(t,
			ordered("late", 5, nil, nil, &calls),
			ordered("early", 1, nil, nil, &calls),
		)
		assert.Equal(t, calls, []plugin.ID{"early", "late"},
			"the lower priority runs first, whatever the declaration order")
	})

	t.Run("capabilities order inside one priority", func(t *testing.T) {
		t.Parallel()

		var calls []plugin.ID
		runOrdering(t,
			ordered("needs", 1, nil, caps("cap"), &calls),
			ordered("gives", 1, caps("cap"), nil, &calls),
		)
		assert.Equal(t, calls, []plugin.ID{"gives", "needs"},
			"the provider runs before its requirer, whatever the names say")
	})

	t.Run("the name breaks the tie", func(t *testing.T) {
		t.Parallel()

		var calls []plugin.ID
		runOrdering(t,
			ordered("beta", 1, nil, nil, &calls),
			ordered("alph", 1, nil, nil, &calls),
		)
		assert.Equal(t, calls, []plugin.ID{"alph", "beta"},
			"names order what priorities and capabilities leave open")
	})

	t.Run("the bucket number is the schedule position", func(t *testing.T) {
		t.Parallel()

		var rank meta.Key[string]
		register := func(r *meta.Registry) error {
			if err := r.ClaimNamespace("order", "the test"); err != nil {
				return err
			}
			k, err := meta.Register[string](r, meta.KeySpec{
				Name: "order.rank", Doc: "which role stamped first",
			})
			rank = k
			return err
		}
		stampRank := func(name plugin.ID, pri int) plugin.Annotator {
			return stamperAt(name, pri, nil, nil,
				func(m *eidos.StructMatch, st *eidos.Stamper) error {
					eidos.Stamp(st, rank, string(name))
					return nil
				})
		}
		w, err := workspace.New().
			Keys(register).
			Annotators(stampRank("second", 2), stampRank("first", 1)).
			Targets("fixture").
			Plans(planTo("plan", "fixture", mirror("mirror"))).
			Build()
		assert.NoError(t, err, "the bucket fixture composes")
		g, s := alpha(t)
		report, err := w.Run(t.Context(), g)
		assert.NoError(t, err, "the bucket fixture runs")

		got, held := meta.Get(report.Facts, s.Identity(), rank)
		assert.True(t, held, "the fact is stamped")
		assert.Equal(t, got, "first", "the earlier bucket wins the rank")
		buckets := map[plugin.ID]int{}
		for view := range report.Facts.Claims(s.Identity(), rank.ID()) {
			buckets[view.Claim.Plugin] = view.Claim.Bucket
		}
		assert.Equal(t, buckets, map[plugin.ID]int{"first": 1, "second": 2},
			"each claim carries its plugin's schedule position")
	})

	t.Run("generators order within the plan", func(t *testing.T) {
		t.Parallel()

		var calls []plugin.ID
		placed := func(name plugin.ID, pri int) plugin.Generator {
			p, held := eidos.NewPlugin(name).
				Priority(plugin.RoleGenerator, pri).
				Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
				Handle(eidos.OnStruct(func(*eidos.StructMatch, *eidos.Emitter) error {
					calls = append(calls, name)
					return nil
				})).Build().(plugin.Generator)
			assert.True(t, held, "an emitter rule lowers to the generator role")
			return p
		}
		w, err := workspace.New().
			Targets("fixture").
			Plans(workspace.Plan{
				Name:       "plan",
				Generators: []plugin.Generator{placed("late", 9), placed("early", 3)},
				Backend:    fakeBackend{name: "printer", target: "fixture"},
			}).
			Build()
		assert.NoError(t, err, "the plan fixture composes")
		g, _ := alpha(t)
		_, err = w.Run(t.Context(), g)
		assert.NoError(t, err, "the plan fixture runs")
		assert.Equal(t, calls, []plugin.ID{"early", "late"},
			"the plan's generators run in bucket order, not list order")
	})
}
