// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// memOutput opens a fresh in-memory sink for every run.
func memOutput() (output.Sink, error) { return output.NewMem(), nil }

// framing is a backend stating a comment syntax and rendering
// nothing: what a writing composition refuses at Build.
type framing struct{ fakeBackend }

// Syntax returns the line comment form the fixture frames through.
func (framing) Syntax() plugin.CommentSyntax {
	return plugin.CommentSyntax{Line: []string{"//"}}
}

// runOrdering composes the annotators over the one-struct fixture
// and runs, so each role's handler runs exactly once, in schedule
// order.
func runOrdering(t *testing.T, anns ...plugin.Annotator) {
	t.Helper()

	w, err := workspace.New().
		Brand(fixtureBrand).
		Annotators(anns...).
		Targets("fixture").
		Plans(planTo("plan", "fixture", mirror("mirror"))).
		Build()
	assert.NoError(t, err, "the ordering fixture composes")
	g, _ := alpha(t)
	_, err = w.Run(t.Context(), workspace.Input{Graph: g})
	assert.NoError(t, err, "the ordering fixture runs")
}

// nameless is a hand-rolled annotator returning no name. The facade
// cannot spell one, and the roster still refuses it, because
// findings, units and claims all key on the name.
type nameless struct{}

// Name returns the empty name.
func (nameless) Name() plugin.ID { return "" }

// Annotate stamps nothing.
func (nameless) Annotate(*plugin.AnnotatorContext) error { return nil }

// handmade is a hand-rolled annotator passed as a struct value with
// a slice field, so its type is not comparable. The facade builds
// pointers, and the composition still reads a plugin that arrives
// through the service provider interface in any shape the interface
// admits.
type handmade struct {
	name   plugin.ID
	labels []string
}

// Name returns the declared name.
func (h handmade) Name() plugin.ID { return h.name }

// Annotate stamps nothing.
func (handmade) Annotate(*plugin.AnnotatorContext) error { return nil }

// declaring is a hand-rolled annotator returning its capability
// lists verbatim, duplicates and empty labels included. The facade
// refuses an empty label at its own Build, and the composition
// still checks one that arrives through the service provider
// interface.
type declaring struct {
	name     plugin.ID
	provides []plugin.Capability
	requires []plugin.Capability
}

// Name returns the declared name.
func (d *declaring) Name() plugin.ID { return d.name }

// Annotate stamps nothing.
func (*declaring) Annotate(*plugin.AnnotatorContext) error { return nil }

// Priority places every role at priority one.
func (*declaring) Priority(plugin.Role) int { return 1 }

// Provides returns the provided labels verbatim.
func (d *declaring) Provides() []plugin.Capability { return d.provides }

// Requires returns the required labels verbatim.
func (d *declaring) Requires() []plugin.Capability { return d.requires }

// capable spells one hand-rolled annotator's capability lists
// inline.
func capable(name plugin.ID, provides, requires []plugin.Capability) plugin.Annotator {
	return &declaring{name: name, provides: provides, requires: requires}
}

// keyed returns an annotator whose key provider runs register.
func keyed(name plugin.ID, register func(*meta.Registry) error) plugin.Annotator {
	p, held := eidos.NewPlugin(name).
		Keys(register).
		Handle(eidos.OnStruct(quiet)).Build().(plugin.Annotator)
	if !held {
		panic("workspace_test: a stamper rule lowers to the annotator role")
	}
	return p
}

// schemad returns a generator declaring s, so the registration step
// meets a plugin's own schema.
func schemad(name plugin.ID, s directive.Schema) plugin.Generator {
	p, held := eidos.NewPlugin(name).
		Output(plugin.Output{Per: plugin.PerSource, Word: "gen"}).
		Handle(eidos.Directive(s, eidos.OnEmit(symbol.KindStruct,
			func(*eidos.EmitMatch, *eidos.Emitter) error { return nil }))).
		Build().(plugin.Generator)
	if !held {
		panic("workspace_test: an emitter rule lowers to the generator role")
	}
	return p
}

// slotted returns a one-template tree whose template places the
// slots, the smallest tree the template rules admit.
func slotted() fstest.MapFS {
	return fstest.MapFS{"method1.tpl": &fstest.MapFile{Data: []byte("{{slots}}")}}
}

// styled returns the generator a presentation declaration builds,
// with one family and a rule that emits nothing.
func styled(b *eidos.Builder) plugin.Generator {
	p, held := b.
		Output(plugin.Output{Per: plugin.PerSource, Word: "gen"}).
		Handle(eidos.OnStruct(func(*eidos.StructMatch, *eidos.Emitter) error { return nil })).
		Build().(plugin.Generator)
	if !held {
		panic("workspace_test: an emitter rule lowers to the generator role")
	}
	return p
}

// sectioned returns a composition with one config section for one
// plugin's options struct.
func sectioned(name string, cfg any, section map[string]any) *workspace.Builder {
	return valid().
		Plans(planTo("second", "fixture", tuned(plugin.ID(name), cfg))).
		Config(workspace.Config{Options: map[string]map[string]any{name: section}})
}

// The steps' output is the schedule, and the schedule is
// observable twice over: the order handlers run in, and the bucket
// number every claim records.
func TestSteps(t *testing.T) {
	t.Parallel()

	t.Run("assemble", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error naming a plugin without a name", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Annotators(nameless{}).Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), "empty name", "the error names the fault")
		})

		t.Run("returns no error for a plugin whose type is not comparable", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Annotators(handmade{name: "handmade", labels: []string{"a"}}).Build()
			assert.NoError(t, err, "the composition reads the plugin through its name")
		})

		t.Run("returns no error for one provider listed twice", func(t *testing.T) {
			t.Parallel()

			one := &handmade{name: "listed"}
			_, err := valid().Annotators(one, one).Build()
			assert.NoError(t, err, "the repeated pointer is the same provider")
		})

		t.Run("returns an error naming two providers under one name", func(t *testing.T) {
			t.Parallel()

			_, err := valid().
				Annotators(&handmade{name: "twinned"}, &handmade{name: "twinned"}).
				Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), "twinned", "the error names the contended name")
		})
	})

	t.Run("register", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a plugin's key registration error", func(t *testing.T) {
			t.Parallel()

			_, err := valid().
				Annotators(keyed("keyless", func(*meta.Registry) error {
					return errors.New("the shape namespace is taken")
				})).
				Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), "namespace is taken", "the error has the provider's cause")
		})

		t.Run("returns an error naming a plugin's key in another plugin's namespace", func(t *testing.T) {
			t.Parallel()

			_, err := valid().
				Annotators(
					keyed("shape", func(r *meta.Registry) error { return r.ClaimNamespace("shape") }),
					keyed("rival", func(r *meta.Registry) error {
						_, err := meta.Register[string](r, meta.KeySpec{
							Name: "shape.role", Doc: "a key under a foreign namespace",
						})
						return err
					}),
				).
				Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), `"rival"`, "the error names the registering plugin")
			assert.Contains(t, err.Error(), `"shape"`, "the error names the claiming plugin")
		})

		t.Run("returns a plugin's schema registration error", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Plans(planTo("second", "fixture",
				schemad("picky", directive.Schema{Plugin: "picky", Name: "gate"}))).
				Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), "states no semantics", "the error names the fault")
			assert.Contains(t, err.Error(), "gate", "the error names the schema")
		})

		t.Run("returns an error naming a plugin's schema under another plugin's name", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Plans(planTo("second", "fixture",
				schemad("impostor", directive.Schema{Plugin: "picky", Name: "gate", Doc: "a borrowed name"}))).
				Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), "impostor", "the error names the declaring plugin")
			assert.Contains(t, err.Error(), `"picky"`, "the error names the borrowed name")
		})

		sealedRegistry := func(t *testing.T) *meta.Registry {
			t.Helper()

			w, err := valid().Build()
			assert.NoError(t, err, "the composition builds")
			g, _ := alpha(t)
			report, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.NoError(t, err, "the composition runs")
			return report.Facts.Registry()
		}

		t.Run("seals the key registry against a namespace claim", func(t *testing.T) {
			t.Parallel()

			assert.HasError(t, sealedRegistry(t).ClaimNamespace("late"), "the claim fails")
		})

		t.Run("seals the key registry against a key registration", func(t *testing.T) {
			t.Parallel()

			_, err := meta.Register[string](sealedRegistry(t), meta.KeySpec{
				Name: "late.role", Doc: "registered after the seal",
			})
			assert.HasError(t, err, "the registration fails")
		})
	})

	t.Run("capabilities", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error naming a plugin that provides an empty label", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Annotators(capable("hollow", caps(""), nil)).Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), "provides an empty capability label", "the error names the side")
			assert.Contains(t, err.Error(), "hollow", "the error names the plugin")
		})

		t.Run("returns an error naming a plugin that requires an empty label", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Annotators(capable("wanting", nil, caps(""))).Build()
			assert.HasError(t, err, "the composition fails")
			assert.Contains(t, err.Error(), "requires an empty capability label", "the error names the side")
			assert.Contains(t, err.Error(), "wanting", "the error names the plugin")
		})

		t.Run("counts a label one plugin provides twice as one provider", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Annotators(capable("twice", caps("json", "json"), nil)).Build()
			assert.NoError(t, err, "the label does not collide with itself")
		})

		t.Run("reports a label one plugin requires twice once", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Annotators(capable("asking", nil, caps("missing", "missing"))).Build()
			assert.HasError(t, err, "nothing provides the label")
			assert.Equal(t, strings.Count(err.Error(), `requires capability "missing"`), 1,
				"the gap is reported once")
		})
	})

	t.Run("configure", func(t *testing.T) {
		t.Parallel()

		t.Run("replaces a constructed default with the configured value", func(t *testing.T) {
			t.Parallel()

			opts := &mirrorOptions{Depth: 1}
			w, err := sectioned("tuned", opts, map[string]any{"depth": 3}).Build()
			assert.NoError(t, err, "the section meets the tag contract")
			assert.NotNil(t, w, "the workspace composes")
			assert.Equal(t, opts.Depth, 3, "the plugin's struct has the configured value")
		})

		t.Run("skips populating options whose declaration fails", func(t *testing.T) {
			t.Parallel()

			opts := &struct {
				Depth int `opt:"depth"`
			}{}
			_, err := sectioned("tuned", opts, map[string]any{"depth": 3}).Build()
			assert.HasError(t, err, "the options struct is undocumented")
			assert.Contains(t, err.Error(), "doc", "the error names the fault")
			assert.Equal(t, opts.Depth, 0, "the struct keeps its constructed value")
		})

		t.Run("returns an error naming an options field the encoding hides", func(t *testing.T) {
			t.Parallel()

			opts := &struct {
				Depth  int  `opt:"depth"  doc:"how deep the mirror walks"`
				Strict bool `opt:"strict" doc:"whether the walk refuses a gap" json:"-"`
			}{}
			_, err := sectioned("tuned", opts, map[string]any{"depth": 3}).Build()
			assert.HasError(t, err, "the fingerprint folds every option")
			assert.Contains(t, err.Error(), "Strict", "the error names the hidden field")
		})

		t.Run("returns an error naming a plugin whose options the encoding rejects", func(t *testing.T) {
			t.Parallel()

			opts := &struct {
				Hook func() `opt:"hook" doc:"what runs after the walk"`
			}{}
			_, err := valid().Plans(planTo("second", "fixture", tuned("tuned", opts))).Build()
			assert.HasError(t, err, "options without an encoding have no fingerprint")
			assert.Contains(t, err.Error(), "tuned", "the error names the plugin")
		})

		t.Run("returns an error for a section of a plugin without options", func(t *testing.T) {
			t.Parallel()

			_, err := valid().Config(workspace.Config{
				Options: map[string]map[string]any{"mirror": {"depth": 1}},
			}).Build()
			assert.HasError(t, err, "a section nothing reads is a typo")
			assert.Contains(t, err.Error(), "declares no options", "the error names the fault")
			assert.Contains(t, err.Error(), `"mirror"`, "the error names the plugin")
		})
	})

	t.Run("stampable", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error naming a backend that does not render", func(t *testing.T) {
			t.Parallel()

			_, err := workspace.New().
				Brand(fixtureBrand).
				Annotators(stamper("noter", quiet)).
				Targets("fixture").
				Plans(workspace.Plan{
					Name:       "plan",
					Generators: []plugin.Generator{mirror("mirror")},
					Backend:    framing{fakeBackend{name: "framer", target: "fixture"}},
				}).
				Output(memOutput).
				Build()
			assert.HasError(t, err, "a writing composition needs every backend to render")
			assert.Contains(t, err.Error(), "does not render", "the error names the cause")
			assert.Contains(t, err.Error(), "framer", "the error names the backend")
		})

		t.Run("returns an error naming a backend that spells no filenames", func(t *testing.T) {
			t.Parallel()

			_, err := workspace.New().
				Brand(fixtureBrand).
				Annotators(stamper("noter", quiet)).
				Targets("fixture").
				Plans(workspace.Plan{
					Name:       "plan",
					Generators: []plugin.Generator{mirror("mirror")},
					Backend:    framing{fakeBackend{name: "framer", target: "fixture"}},
				}).
				Output(memOutput).
				Build()
			assert.HasError(t, err, "a writing composition needs every backend to name its files")
			assert.Contains(t, err.Error(), "spells no filenames", "the error names the cause")
		})
	})

	t.Run("lower", func(t *testing.T) {
		t.Parallel()

		t.Run("runs the lower priority first", func(t *testing.T) {
			t.Parallel()

			var calls []plugin.ID
			runOrdering(t,
				ordered("late", 5, nil, nil, &calls),
				ordered("early", 1, nil, nil, &calls),
			)
			assert.Equal(t, calls, []plugin.ID{"early", "late"}, "the declaration order does not decide")
		})

		t.Run("runs a provider before its requirer within one priority", func(t *testing.T) {
			t.Parallel()

			var calls []plugin.ID
			runOrdering(t,
				ordered("needs", 1, nil, caps("cap"), &calls),
				ordered("gives", 1, caps("cap"), nil, &calls),
			)
			assert.Equal(t, calls, []plugin.ID{"gives", "needs"}, "the names do not decide")
		})

		t.Run("orders by name what priorities and capabilities leave open", func(t *testing.T) {
			t.Parallel()

			var calls []plugin.ID
			runOrdering(t,
				ordered("beta", 1, nil, nil, &calls),
				ordered("alph", 1, nil, nil, &calls),
			)
			assert.Equal(t, calls, []plugin.ID{"alph", "beta"}, "the names break the tie")
		})

		t.Run("numbers each claim's bucket by its schedule position", func(t *testing.T) {
			t.Parallel()

			var rank meta.Key[string]
			register := func(r *meta.Registry) error {
				if err := r.ClaimNamespace("order"); err != nil {
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
				Brand(fixtureBrand).
				Keys(register).
				Annotators(stampRank("second", 2), stampRank("first", 1)).
				Targets("fixture").
				Plans(planTo("plan", "fixture", mirror("mirror"))).
				Build()
			assert.NoError(t, err, "the bucket fixture composes")
			g, s := alpha(t)
			report, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.NoError(t, err, "the bucket fixture runs")

			got, held := meta.Get(report.Facts, s.Identity(), rank)
			assert.True(t, held, "the fact is stamped")
			assert.Equal(t, got, "first", "the earlier bucket outranks the later")
			buckets := map[plugin.ID]int{}
			for view := range report.Facts.Claims(s.Identity(), rank.ID()) {
				buckets[view.Claim.Plugin] = view.Claim.Bucket
			}
			assert.Equal(t, buckets, map[plugin.ID]int{"first": 1, "second": 2},
				"each claim has its plugin's schedule position")
		})
	})

	t.Run("compilePlans", func(t *testing.T) {
		t.Parallel()

		t.Run("runs a plan's generators in bucket order", func(t *testing.T) {
			t.Parallel()

			var calls []plugin.ID
			placed := func(name plugin.ID, pri int) plugin.Generator {
				return generatorAt(name, pri, func(*eidos.StructMatch, *eidos.Emitter) error {
					calls = append(calls, name)
					return nil
				})
			}
			w, err := workspace.New().
				Brand(fixtureBrand).
				Targets("fixture").
				Plans(workspace.Plan{
					Name:       "plan",
					Generators: []plugin.Generator{placed("late", 9), placed("early", 3)},
					Backend:    fakeBackend{name: "printer", target: "fixture"},
				}).
				Build()
			assert.NoError(t, err, "the plan fixture composes")
			g, _ := alpha(t)
			_, err = w.Run(t.Context(), workspace.Input{Graph: g})
			assert.NoError(t, err, "the plan fixture runs")
			assert.Equal(t, calls, []plugin.ID{"early", "late"}, "the list order does not decide")
		})

		t.Run("returns an error naming a generator whose trees serve other targets alone", func(t *testing.T) {
			t.Parallel()

			_, err := valid().
				Plans(planTo("second", "fixture", styled(eidos.NewPlugin("styled").
					For("other", eidos.Templates(slotted()))))).
				Build()
			assert.HasError(t, err, "the plan's target has no tree to resolve references in")
			assert.Contains(t, err.Error(), "styled", "the error names the generator")
			assert.Contains(t, err.Error(), `"other"`, "the error names the declared target")
		})

		t.Run("returns no error for a generator whose plugin-level tree serves every target", func(t *testing.T) {
			t.Parallel()

			_, err := valid().
				Plans(planTo("second", "fixture", styled(eidos.NewPlugin("styled").
					Templates(slotted()).
					For("other", eidos.Templates(slotted()))))).
				Build()
			assert.NoError(t, err, "the plugin-level tree serves the plan's target")
		})
	})
}
