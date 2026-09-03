// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

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
	"go.dokimi.dev/eidos/core/rules/rulestest"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// runCode is a code for the run fixtures' findings.
var runCode = diag.Code{Prefix: "tst", Number: 7}

// flagGroup is the fact group the fixture key registers into, so a
// case can drop the group rather than the key.
const flagGroup meta.GroupName = "shape.all"

// rawMeta returns a positioned raw instance of the kernel meta
// directive dropping ref.
func rawMeta(ref string, line int) directive.Raw {
	return directive.Raw{
		Name: "meta",
		Args: []directive.RawArg{{Key: "drop", Value: directive.RawValue{Text: ref}}},
		Pos:  position.Pos{File: "alpha.go", Line: line, Col: 1},
	}
}

// rawBareMeta returns a positioned meta instance carrying no drop:
// the schema admits one, and the drop pass has to pass over it.
func rawBareMeta(line int) directive.Raw {
	return directive.Raw{
		Name: "meta",
		Pos:  position.Pos{File: "alpha.go", Line: line, Col: 1},
	}
}

// rawDiag returns a positioned kernel diag instance, which is a
// validated directive the drop pass is not about.
func rawDiag(code string, line int) directive.Raw {
	return directive.Raw{
		Name: "diag",
		Args: []directive.RawArg{{Key: "off", Value: directive.RawValue{Text: code}}},
		Pos:  position.Pos{File: "alpha.go", Line: line, Col: 1},
	}
}

// generatorAt returns a facade-built generator placed by priority,
// running h once per struct in scope: two of them in one plan run
// in the order their priorities fix.
func generatorAt(
	name plugin.ID, pri int, h func(*eidos.StructMatch, *eidos.Emitter) error,
) plugin.Generator {
	p, held := eidos.NewPlugin(name).
		Priority(plugin.RoleGenerator, pri).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
		Handle(eidos.OnStruct(h)).Build().(plugin.Generator)
	if !held {
		panic("workspace_test: an emitter rule lowers to the generator role")
	}
	return p
}

// dropping is a backend whose lowering hook returns a declaration
// carrying no origin: the defect the settle refuses, and the one
// way a plan fails after its schedule ran whole.
type dropping struct{ fakeBackend }

func (dropping) Lower(symbol.Symbol) ([]symbol.Symbol, error) {
	return []symbol.Symbol{&emit.Struct{Name: "made"}}, nil
}

// flagged returns a composition whose annotator registers and
// stamps shape.flag through its own key provider, and whose
// generator mirrors the subjects the flag reads present on. The
// key handle arrives when Build runs the steps; the handlers read
// it through their closures, which is the seam under test.
func flagged() (*workspace.Builder, *meta.Key[bool]) {
	var flag meta.Key[bool]
	shape, _ := eidos.NewPlugin("shape").
		Keys(func(r *meta.Registry) error {
			if err := r.ClaimNamespace("shape", "shape"); err != nil {
				return err
			}
			k, err := meta.Register[bool](r, meta.KeySpec{
				Name: "shape.flag", Group: flagGroup, Doc: "marks a fixture subject",
			})
			flag = k
			return err
		}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, st *eidos.Stamper) error {
			eidos.Stamp(st, flag, true)
			return nil
		})).Build().(plugin.Annotator)
	flagMirror, _ := eidos.NewPlugin("mirror").
		Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			if _, held := eidos.Fact(m, flag); !held {
				return nil
			}
			e.PackageFile().Append(&emit.Struct{
				Origin: m.Struct.Identity(),
				Name:   "For" + m.Struct.Name,
			})
			return nil
		})).Build().(plugin.Generator)
	b := workspace.New().
		Annotators(shape).
		Targets("fixture").
		Plans(workspace.Plan{
			Name:       "plan",
			Generators: []plugin.Generator{flagMirror},
			Backend:    fakeBackend{name: "printer", target: "fixture"},
		})
	return b, &flag
}

// Run is the frame: seal, validate, drop, annotate, generate. What
// stops it, what merely fails it, and what it leaves behind are all
// contract.
func TestRun(t *testing.T) {
	t.Parallel()

	t.Run("takes a graph the load already sealed", func(t *testing.T) {
		t.Parallel()

		w, err := valid().Build()
		assert.NoError(t, err, "the fixture composition is valid")
		g, _ := alpha(t)
		g.Freeze()
		report, err := w.Run(t.Context(), g)
		assert.NoError(t, err,
			"a sealed graph is what the load driver hands over, so the run takes it")
		assert.NotNil(t, report, "and the frame ran whole")
	})

	t.Run("refuses a missing graph", func(t *testing.T) {
		t.Parallel()

		w, err := valid().Build()
		assert.NoError(t, err, "the fixture composition is valid")
		report, err := w.Run(t.Context(), nil)
		assert.HasError(t, err, "there is nothing to run over")
		assert.Nil(t, report, "nothing ran")
	})

	t.Run("the keyed frame runs whole", func(t *testing.T) {
		t.Parallel()

		b, flag := flagged()
		w, err := b.Build()
		assert.NoError(t, err, "the keyed composition composes")
		g, s := alpha(t)
		report, err := w.Run(t.Context(), g)
		assert.NoError(t, err, "the run is clean")
		assert.False(t, report.Sink.Failed(), "no findings")
		v, held := meta.Get(report.Facts, s.Identity(), *flag)
		assert.True(t, held && v, "the composition-registered key was stamped")
		got := units(report.Emits["plan"])
		assert.Length(t, got, 1, "the flagged subject was mirrored")
		assert.Equal(t, got[0].Plugin, plugin.ID("mirror"),
			"the unit names its plugin")
		assert.Equal(t, got[0].Origins, []symbol.Identity{s.Identity()},
			"and carries its provenance")
		assert.True(t, report.Emits["plan"].Settled(),
			"the plan's store settles before the report carries it")
	})

	t.Run("a meta drop wins over the stamp", func(t *testing.T) {
		t.Parallel()

		b, flag := flagged()
		w, err := b.Build()
		assert.NoError(t, err, "the keyed composition composes")
		g, s := alpha(t)
		assert.NoError(t,
			g.AttachDirectives(s.Identity(), []directive.Raw{rawMeta("shape.flag", 4)}),
			"the drop attaches before the seal")
		report, err := w.Run(t.Context(), g)
		assert.NoError(t, err, "a drop is authored intent, not a finding")
		assert.False(t, report.Sink.Failed(), "and reports nothing")
		_, held := meta.Get(report.Facts, s.Identity(), *flag)
		assert.False(t, held, "the drop outranks the stamp whenever it arrives")
		assert.Empty(t, units(report.Emits["plan"]), "so the flag gates nothing")
	})

	t.Run("a group drop removes every member fact", func(t *testing.T) {
		t.Parallel()

		b, flag := flagged()
		w, err := b.Build()
		assert.NoError(t, err, "the keyed composition composes")
		g, s := alpha(t)
		assert.NoError(t,
			g.AttachDirectives(s.Identity(),
				[]directive.Raw{rawMeta(string(flagGroup), 4)}),
			"the group drop attaches before the seal")
		report, err := w.Run(t.Context(), g)
		assert.NoError(t, err, "a drop is authored intent, not a finding")
		coretest.AssertCodes(t, report.Sink)
		_, held := meta.Get(report.Facts, s.Identity(), *flag)
		assert.False(t, held,
			"the tombstone covers the group, so every member reads absent")
		assert.Empty(t, units(report.Emits["plan"]), "and the flag gates nothing")
	})

	t.Run("a directive that is not a drop leaves the facts alone", func(t *testing.T) {
		t.Parallel()

		b, flag := flagged()
		w, err := b.Build()
		assert.NoError(t, err, "the keyed composition composes")
		g, s := alpha(t)
		assert.NoError(t,
			g.AttachDirectives(s.Identity(), []directive.Raw{
				rawDiag("tst-0007", 4),
				rawBareMeta(5),
			}),
			"the instances attach before the seal")
		report, err := w.Run(t.Context(), g)
		assert.NoError(t, err, "neither instance is a fault")
		coretest.AssertCodes(t, report.Sink)
		v, held := meta.Get(report.Facts, s.Identity(), *flag)
		assert.True(t, held && v,
			"a diag instance and a meta instance carrying no drop both pass "+
				"the drop step without removing anything")
		assert.Length(t, units(report.Emits["plan"]), 1, "so the flag still gates")
	})

	t.Run("a load stamp applies at plugin authority", func(t *testing.T) {
		t.Parallel()

		b, flag := flagged()
		w, err := b.Build()
		assert.NoError(t, err, "the keyed composition composes")
		g, s := alpha(t)
		pkg := symbol.Identity{
			Lang: s.Identity().Lang, Package: s.Identity().Package, Kind: symbol.KindPackage,
		}
		assert.NoError(t, g.AttachStamps(pkg, []meta.RawStamp{{
			Key: "shape.flag", Value: true,
			Pos:    position.Pos{File: "alpha.go", Line: 1},
			Origin: "fakefront",
		}}), "the raw stamp attaches before the seal")

		report, err := w.Run(t.Context(), g)
		assert.NoError(t, err, "the run is clean")
		v, held := meta.Get(report.Facts, pkg, *flag)
		assert.True(t, held && v,
			"the load's classification reads back through the typed handle")
	})

	t.Run("a refused stamp reports and the frame continues", func(t *testing.T) {
		t.Parallel()

		b, _ := flagged()
		w, err := b.Build()
		assert.NoError(t, err, "the keyed composition composes")
		g, s := alpha(t)
		assert.NoError(t, g.AttachStamps(s.Identity(), []meta.RawStamp{{
			Key: "shape.ghost", Value: true,
			Pos:    position.Pos{File: "alpha.go", Line: 2},
			Origin: "fakefront",
		}}), "the stray stamp attaches")

		report, err := w.Run(t.Context(), g)
		assert.ErrorIs(t, err, workspace.ErrRunFailed, "an Error finding fails the run")
		coretest.AssertReports(t, report.Sink, meta.RefusedStamp)
		assert.Length(t, units(report.Emits["plan"]), 1, "and the frame still ran whole")
	})

	t.Run("a rejected instance never gates and fails the run", func(t *testing.T) {
		t.Parallel()

		b, _ := flagged()
		w, err := b.Build()
		assert.NoError(t, err, "the keyed composition composes")
		g, s := alpha(t)
		ghost := directive.Raw{
			Name: "ghost",
			Pos:  position.Pos{File: "alpha.go", Line: 5, Col: 1},
		}
		assert.NoError(t, g.AttachDirectives(s.Identity(), []directive.Raw{ghost}),
			"the stray instance attaches")
		report, err := w.Run(t.Context(), g)
		assert.ErrorIs(t, err, workspace.ErrRunFailed,
			"an Error finding fails the run")
		coretest.AssertReports(t, report.Sink, directive.UnclaimedName)
		assert.Length(t, units(report.Emits["plan"]), 1,
			"and the frame still ran whole")
	})

	t.Run("an ignored foreign directive never reaches validation", func(t *testing.T) {
		t.Parallel()

		b, _ := flagged()
		w, err := b.Ignore("k8s:").Build()
		assert.NoError(t, err, "the composition opts out of a foreign tool's prefix")
		g, s := alpha(t)
		foreign := directive.Raw{
			Name: "k8s:deepcopy-gen",
			Pos:  position.Pos{File: "alpha.go", Line: 5, Col: 1},
		}
		assert.NoError(t, g.AttachDirectives(s.Identity(), []directive.Raw{foreign}),
			"the foreign carrier attaches like any other")
		report, err := w.Run(t.Context(), g)
		assert.NoError(t, err, "and the run passes")
		coretest.AssertCodes(t, report.Sink)
		assert.Length(t, units(report.Emits["plan"]), 1, "the frame ran whole")
	})

	t.Run("a dangling subject reports its code", func(t *testing.T) {
		t.Parallel()

		b, _ := flagged()
		w, err := b.Build()
		assert.NoError(t, err, "the keyed composition composes")
		g, _ := alpha(t)
		ghost := coretest.Struct("example.com/elsewhere", "Ghost")
		assert.NoError(t,
			g.AttachDirectives(ghost.Identity(), []directive.Raw{rawMeta("shape.flag", 9)}),
			"the dangling attachment arrives before the seal")
		report, err := w.Run(t.Context(), g)
		assert.ErrorIs(t, err, workspace.ErrRunFailed,
			"a dangling subject is an Error")
		coretest.AssertReports(t, report.Sink, directive.DanglingSubject)
	})

	t.Run("validates a record on the package itself", func(t *testing.T) {
		t.Parallel()

		b, _ := flagged()
		w, err := b.Build()
		assert.NoError(t, err, "the keyed composition composes")
		g, _ := alpha(t)
		pkgID := coretest.PackageID(coretest.StorePath)
		assert.NoError(t,
			g.AttachDirectives(pkgID, []directive.Raw{rawMeta("shape.flag", 9)}),
			"a package-subject directive attaches before the seal")
		report, err := w.Run(t.Context(), g)
		assert.NoError(t, err,
			"a package is a subject the graph holds, not a dangling one")
		coretest.AssertCodes(t, report.Sink)
	})

	t.Run("a dangling stamp subject reports the same way", func(t *testing.T) {
		t.Parallel()

		b, _ := flagged()
		w, err := b.Build()
		assert.NoError(t, err, "the keyed composition composes")
		g, _ := alpha(t)
		ghost := coretest.Struct("example.com/elsewhere", "Ghost")
		assert.NoError(t,
			g.AttachStamps(ghost.Identity(), []meta.RawStamp{{
				Key: "shape.ghostly", Value: true,
			}}),
			"the dangling stamp arrives before the seal")
		report, err := w.Run(t.Context(), g)
		assert.ErrorIs(t, err, workspace.ErrRunFailed,
			"a ghost fact would enumerate under a subject no reader reaches")
		coretest.AssertReports(t, report.Sink, directive.DanglingSubject)
	})

	t.Run("an annotator error stops the frame", func(t *testing.T) {
		t.Parallel()

		angry := stamper("angry", func(*eidos.StructMatch, *eidos.Stamper) error {
			return errors.New("boom")
		})
		w, err := workspace.New().
			Annotators(angry).
			Targets("fixture").
			Plans(planTo("plan", "fixture", mirror("mirror"))).
			Build()
		assert.NoError(t, err, "the failing composition still composes")
		g, _ := alpha(t)
		report, err := w.Run(t.Context(), g)
		assert.HasError(t, err, "a returned error is fatal to the frame")
		assert.Contains(t, err.Error(), "angry", "the error names the role")
		assert.Contains(t, err.Error(), "boom", "and carries the cause")
		assert.ErrorIsNot(t, err, workspace.ErrRunFailed,
			"a handler error is a defect, not a finding")
		assert.Empty(t, report.Emits, "no plan ran")
	})

	t.Run("a failing plan does not stop its sibling", func(t *testing.T) {
		t.Parallel()

		bad := generator("bad", func(*eidos.StructMatch, *eidos.Emitter) error {
			return errors.New("boom")
		})
		w, err := workspace.New().
			Targets("fixture").
			Plans(
				planTo("crashing", "fixture", bad),
				planTo("steady", "fixture", mirror("mirror")),
			).
			Build()
		assert.NoError(t, err, "the two-plan composition composes")
		g, _ := alpha(t)
		report, err := w.Run(t.Context(), g)
		assert.HasError(t, err, "the failing plan is reported")
		assert.Contains(t, err.Error(), `"crashing"`, "by name")
		assert.Length(t, units(report.Emits["steady"]), 1,
			"while the sibling ran whole")
	})

	t.Run("a settle refusal fails its plan", func(t *testing.T) {
		t.Parallel()

		w, err := workspace.New().
			Targets("fixture").
			Plans(workspace.Plan{
				Name:       "plan",
				Generators: []plugin.Generator{mirror("mirror")},
				Backend:    dropping{fakeBackend{name: "printer", target: "fixture"}},
			}).
			Build()
		assert.NoError(t, err, "a backend declaring a lowering seam still composes")
		g, _ := alpha(t)
		report, err := w.Run(t.Context(), g)
		assert.HasError(t, err, "a lowering dropping the origin fails the plan")
		assert.Contains(t, err.Error(), "settle", "the failure names the stage")
		assert.Contains(t, err.Error(), "origin", "and the rule the backend broke")
		assert.ErrorIsNot(t, err, workspace.ErrRunFailed,
			"a backend defect is not a finding")

		got := units(report.Emits["plan"])
		assert.Length(t, got, 1, "the plan's store still arrives in the report")
		emitted, held := got[0].Decls[0].(*emit.Struct)
		assert.True(t, held && emitted.Name == "ForAlpha",
			"carrying what the generator emitted, because a refused lowering "+
				"replaces nothing")
	})

	t.Run("a cancelled context stops the frame", func(t *testing.T) {
		t.Parallel()

		w, err := valid().Build()
		assert.NoError(t, err, "the fixture composition is valid")
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		g, _ := alpha(t)
		_, err = w.Run(ctx, g)
		assert.ErrorIs(t, err, context.Canceled,
			"the caller's cancellation returns")
	})

	t.Run("a cancellation mid-schedule stops the next annotator", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		later := false
		w, err := workspace.New().
			Annotators(
				stamperAt("first", 1, nil, nil,
					func(*eidos.StructMatch, *eidos.Stamper) error {
						cancel()
						return nil
					}),
				stamperAt("second", 2, nil, nil,
					func(*eidos.StructMatch, *eidos.Stamper) error {
						later = true
						return nil
					}),
			).
			Targets("fixture").
			Plans(planTo("plan", "fixture", mirror("mirror"))).
			Build()
		assert.NoError(t, err, "the two-annotator composition composes")
		g, _ := alpha(t)
		_, err = w.Run(ctx, g)
		assert.ErrorIs(t, err, context.Canceled,
			"the cancellation returns rather than being swallowed")
		assert.False(t, later,
			"the schedule stops at the next role, not at the end of the bucket list")
	})

	t.Run("a cancellation mid-plan stops the next generator", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		later := false
		w, err := workspace.New().
			Targets("fixture").
			Plans(planTo("plan", "fixture",
				generatorAt("first", 1, func(*eidos.StructMatch, *eidos.Emitter) error {
					cancel()
					return nil
				}),
				generatorAt("second", 2, func(*eidos.StructMatch, *eidos.Emitter) error {
					later = true
					return nil
				}),
			)).
			Build()
		assert.NoError(t, err, "the two-generator plan composes")
		g, _ := alpha(t)
		_, err = w.Run(ctx, g)
		assert.ErrorIs(t, err, context.Canceled,
			"the plan reports the cancellation under its own name")
		assert.Contains(t, err.Error(), `"plan"`, "naming the plan")
		assert.False(t, later, "and the later bucket never ran")
	})

	t.Run("an Error finding fails the run without stopping it", func(t *testing.T) {
		t.Parallel()

		grump := stamper("grump", func(m *eidos.StructMatch, st *eidos.Stamper) error {
			m.Errorf(runCode, "structs are refused here")
			return nil
		})
		w, err := workspace.New().
			Annotators(grump).
			Targets("fixture").
			Plans(planTo("plan", "fixture", mirror("mirror"))).
			Build()
		assert.NoError(t, err, "the grumpy composition composes")
		g, _ := alpha(t)
		report, err := w.Run(t.Context(), g)
		assert.ErrorIs(t, err, workspace.ErrRunFailed,
			"the finding classifies the run")
		assert.Length(t, units(report.Emits["plan"]), 1,
			"and the frame ran to the end regardless")
	})

	t.Run("reference resolution", func(t *testing.T) {
		t.Parallel()

		refSchema := directive.Schema{
			Plugin: "refy", Name: "ref",
			Params: []directive.ParamSpec{{
				Key: "to", Type: directive.TypeReference, Resolution: directive.ResolveCallableInScope,
				Doc: "the struct the directive points at",
			}},
			Doc: "points at a sibling",
		}
		// pointing returns a composition whose one generator records
		// what the ref directive's param bound to.
		pointing := func(bound *symbol.Identity) *workspace.Builder {
			gen, _ := eidos.NewPlugin("refy").
				Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
				Handle(eidos.Directive(refSchema, eidos.OnStruct(
					func(m *eidos.StructMatch, e *eidos.Emitter) error {
						v, _ := m.Directive().Param("to")
						*bound = v.Target
						return nil
					},
				))).Build().(plugin.Generator)
			return workspace.New().
				Targets("fixture").
				Plans(workspace.Plan{
					Name: "plan", Generators: []plugin.Generator{gen},
					Backend: fakeBackend{name: "printer", target: "fixture"},
				})
		}

		t.Run("binds a reference param through the registered rules", func(t *testing.T) {
			t.Parallel()

			var bound symbol.Identity
			var seen rules.Scope
			w, err := pointing(&bound).Rules(native{scope: &seen}).Build()
			assert.NoError(t, err, "the composition composes")
			s := coretest.Struct(coretest.StorePath, "Alpha")
			s.Pos = position.Pos{File: "alpha.go", Line: 3, Col: 1}
			pkg := coretest.Package(coretest.StorePath, s)
			pkg.Files[0].Pos = position.Pos{File: "alpha.go", Line: 1, Col: 1}
			g := store.New()
			assert.NoError(t, g.AddPackage(pkg), "the fixture package is admitted")
			assert.NoError(t, g.AttachDirectives(s.Identity(), []directive.Raw{rawRef("Alpha", 2)}),
				"the directive attaches")
			report, err := w.Run(t.Context(), g)
			assert.NoError(t, err, "the run is clean")
			assert.False(t, report.Sink.Failed(), "without a finding")
			assert.Equal(t, bound, s.Identity(), "the handler receives the bound identity")
			assert.Equal(t, seen.Subject, s.Identity(), "the rules were asked from the subject")
			assert.NotNil(t, seen.File, "in the file the subject sits in")
		})

		t.Run("refuses a reference the rules bind to nothing", func(t *testing.T) {
			t.Parallel()

			var bound symbol.Identity
			w, err := pointing(&bound).Rules(native{}).Build()
			assert.NoError(t, err, "the composition composes")
			g, s := alpha(t)
			assert.NoError(t, g.AttachDirectives(s.Identity(), []directive.Raw{rawRef("Ghost", 2)}),
				"the directive attaches")
			_, err = w.Run(t.Context(), g)
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the run fails")
			assert.True(t, bound.IsZero(), "and the rejected instance never gated the rule")
		})

		t.Run("refuses a reference for a language without rules", func(t *testing.T) {
			t.Parallel()

			var bound symbol.Identity
			w, err := pointing(&bound).Build()
			assert.NoError(t, err, "the composition composes without rules")
			g, s := alpha(t)
			assert.NoError(t, g.AttachDirectives(s.Identity(), []directive.Raw{rawRef("Alpha", 2)}),
				"the directive attaches")
			report, err := w.Run(t.Context(), g)
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the run fails")
			assert.True(t, slices.Contains(coretest.Codes(report.Sink), directive.UnresolvedReference),
				"under the reference's code")
		})
	})

	t.Run("binds the registered rules to every match", func(t *testing.T) {
		t.Parallel()

		var form symbol.TypeForm
		noter := stamper("noter", func(m *eidos.StructMatch, st *eidos.Stamper) error {
			form = m.Rules().TypeOf(&node.TypeRef{Spelling: "int"}).Form
			return nil
		})
		w, err := workspace.New().
			Annotators(noter).
			Rules(native{}).
			Targets("fixture").
			Plans(planTo("plan", "fixture", mirror("mirror"))).
			Build()
		assert.NoError(t, err, "the composition composes")
		g, _ := alpha(t)
		report, err := w.Run(t.Context(), g)
		assert.NoError(t, err, "the run is clean")
		assert.Equal(t, form, symbol.FormScalar, "the annotator's match folded through the registered rules")
		assert.False(t, slices.Contains(coretest.Codes(report.Sink), rules.AbsentRules),
			"and no language went unregistered")
	})
}

// BenchmarkRun takes the frame over the canonical workspace: 1000
// packages of 10 files of 20 declarations, one annotator stamping
// every struct through its own key provider, one plan mirroring
// the flagged subjects. Each iteration loads a fresh graph,
// because the seal is Run's own, so the number is a cold run with
// the load included.
func BenchmarkRun(b *testing.B) {
	const packages, files, decls = 1_000, 10, 20
	pkgs := coretest.Workspace(packages, files, decls)
	builder, _ := flagged()
	w, err := builder.Build()
	if err != nil {
		b.Fatalf("Build: unexpected error: %v", err)
	}

	b.Run("cold run over 200k declarations", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			g := store.New()
			for _, p := range pkgs {
				if err := g.AddPackage(p); err != nil {
					b.Fatalf("AddPackage: unexpected error: %v", err)
				}
			}
			report, err := w.Run(b.Context(), g)
			if err != nil {
				b.Fatalf("Run: unexpected error: %v", err)
			}
			if len(report.Emits) != 1 {
				b.Fatal("the plan store must arrive")
			}
		}
	})

	b.Run("frame overhead over one declaration", func(b *testing.B) {
		b.ReportAllocs()
		one := coretest.Package(coretest.StorePath, coretest.Struct(coretest.StorePath, "Alpha"))
		for b.Loop() {
			g := store.New()
			if err := g.AddPackage(one); err != nil {
				b.Fatalf("AddPackage: unexpected error: %v", err)
			}
			if _, err := w.Run(b.Context(), g); err != nil {
				b.Fatalf("Run: unexpected error: %v", err)
			}
		}
	})
}

// rawRef returns a positioned raw instance of the fixture ref
// directive pointing at name.
func rawRef(name string, line int) directive.Raw {
	return directive.Raw{
		Name: "refy:ref",
		Args: []directive.RawArg{{Key: "to", Value: directive.RawValue{Text: name}}},
		Pos:  position.Pos{File: "alpha.go", Line: line, Col: 1},
	}
}

// native is the scripted rules speaking the fixture's language,
// resolving a name as a struct in the fixture package and recording
// the scope it was asked from.
type native struct {
	scope *rules.Scope
}

func (native) Lang() symbol.Lang { return coretest.Lang }

func (native) Members() rules.MemberPolicy { return rulestest.Scripted().Members() }

func (native) ParamRole(p *node.Param, v rules.View) rules.ParamRole {
	return rulestest.Scripted().ParamRole(p, v)
}

func (native) ReturnRoles(rs []*node.Return, v rules.View) ([]rules.ReturnRole, rules.ErrorModel) {
	return rulestest.Scripted().ReturnRoles(rs, v)
}

func (native) Builtin(ref *node.TypeRef, v rules.View) rules.TypeShape {
	return rulestest.Scripted().Builtin(ref, v)
}

func (n native) Resolve(
	scope rules.Scope, name string, _ directive.ResolutionKind, v rules.View,
) (symbol.Symbol, error) {
	if n.scope != nil {
		*n.scope = scope
	}
	if sym, held := v.Lookup(coretest.ID(coretest.StorePath, name, symbol.KindStruct)); held {
		return sym, nil
	}
	return nil, fmt.Errorf("nothing in %s is named %s", coretest.StorePath, name)
}

func (native) SamplesOf(ref *node.TypeRef, hint string, v rules.View) (rules.Sample, rules.Sample) {
	return rulestest.Scripted().SamplesOf(ref, hint, v)
}

func (native) ZeroValue(ref *node.TypeRef, v rules.View) (emit.Value, bool) {
	return rulestest.Scripted().ZeroValue(ref, v)
}

func (native) LiteralFor(f *node.File, ref *node.TypeRef, text string, v rules.View) (emit.Value, bool) {
	return rulestest.Scripted().LiteralFor(f, ref, text, v)
}

func (native) TypeName(word, base string) string { return rulestest.Scripted().TypeName(word, base) }
