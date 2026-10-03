// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/backend"
	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/output"
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
// case can drop the group and not the key.
const flagGroup meta.GroupName = "shape.all"

// The plugin whose directive the negation cases write, and that
// directive's canonical spelling.
const (
	negatingPlugin plugin.ID      = "mirror"
	optOutName     directive.Name = "mirror:stub"
)

// rawMeta returns a positioned raw instance of the kernel meta
// directive dropping ref.
func rawMeta(ref string, line int) directive.Raw {
	return directive.Raw{
		Name: "meta",
		Args: []directive.RawArg{{Key: "drop", Value: directive.RawValue{Text: ref}}},
		Pos:  position.Pos{File: "alpha.go", Line: line, Col: 1},
	}
}

// rawBareMeta returns a positioned meta instance without a drop:
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
// without an origin: the defect the settle refuses, and the one way
// a plan fails after its schedule ran whole.
type dropping struct{ fakeBackend }

// Lower returns one declaration without an origin.
func (dropping) Lower(symbol.Symbol) ([]symbol.Symbol, error) {
	return []symbol.Symbol{&emit.Struct{Name: "made"}}, nil
}

// flagged returns a composition whose annotator registers and
// stamps shape.flag through its own key provider, and whose
// generator mirrors the subjects the flag reads present on. The
// key handle arrives when Build runs the steps. The handlers read
// it through their closures, which is the seam under test.
func flagged() (*workspace.Builder, *meta.Key[bool]) {
	var flag meta.Key[bool]
	shape, _ := eidos.NewPlugin("shape").
		Keys(func(r *meta.Registry) error {
			if err := r.ClaimNamespace("shape"); err != nil {
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
			return mirrored(m, e)
		})).Build().(plugin.Generator)
	b := workspace.New().
		Brand(fixtureBrand).
		Annotators(shape).
		Targets("fixture").
		Plans(workspace.Plan{
			Name:       "plan",
			Generators: []plugin.Generator{flagMirror},
			Backend:    fakeBackend{name: "printer", target: "fixture"},
		})
	return b, &flag
}

// keyedRun builds the flagged composition and runs it over the
// one-struct fixture after attach adds its directives.
func keyedRun(
	t *testing.T, attach func(*store.Graph, symbol.Identity),
) (*workspace.Report, symbol.Identity, meta.Key[bool], error) {
	t.Helper()

	b, flag := flagged()
	w, err := b.Build()
	assert.NoError(t, err, "the keyed composition composes")
	g, s := alpha(t)
	if attach != nil {
		attach(g, s.Identity())
	}
	report, err := w.Run(t.Context(), workspace.Input{Graph: g})
	return report, s.Identity(), *flag, err
}

// optingOut returns a composition whose mirror generator declares
// the stub directive, negatable or not, and mirrors every struct.
func optingOut(negatable bool) *workspace.Builder {
	mirrorer, _ := eidos.NewPlugin(negatingPlugin).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
		Handle(
			eidos.OnStruct(mirrored),
			eidos.Directive(directive.Schema{
				Plugin: string(negatingPlugin), Name: "stub", Negatable: negatable,
				Doc: "a fixture directive a subject negates to opt out",
			}, eidos.OnEmit(symbol.KindStruct,
				func(*eidos.EmitMatch, *eidos.Emitter) error { return nil })),
		).Build().(plugin.Generator)
	return workspace.New().
		Brand(fixtureBrand).
		Targets("fixture").
		Plans(planTo("plan", "fixture", mirrorer))
}

// pair returns an unfrozen one-package graph with two positioned
// structs, the second negating the mirror's stub directive.
func pair(tb assert.TB) (*store.Graph, *node.Struct, *node.Struct) {
	tb.Helper()

	first := coretest.Struct(coretest.StorePath, "Alpha")
	first.Pos = position.Pos{File: "alpha.go", Line: 3, Col: 1}
	second := coretest.Struct(coretest.StorePath, "Beta")
	second.Pos = position.Pos{File: "beta.go", Line: 7, Col: 1}
	g := store.New()
	assert.NoError(tb, g.AddPackage(coretest.Package(coretest.StorePath, first, second)),
		"the fixture package is admitted")
	assert.NoError(tb, g.AttachDirectives(second.Identity(), []directive.Raw{{
		Name: optOutName, Negated: true, Pos: position.Pos{File: "beta.go", Line: 6, Col: 1},
	}}), "the negated instance attaches before the seal")
	return g, first, second
}

// Run is the frame: seal, validate, drop, annotate, generate. What
// stops it, what merely fails it, and what it leaves behind are all
// contract.
func TestRun(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("runs a graph the load already sealed", func(t *testing.T) {
			t.Parallel()

			w, err := valid().Build()
			assert.NoError(t, err, "the fixture composition is valid")
			g, _ := alpha(t)
			g.Freeze()
			report, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.NoError(t, err, "the run takes the sealed graph")
			assert.NotNil(t, report, "the frame runs whole")
		})

		t.Run("returns an error for an input without a tree or a graph", func(t *testing.T) {
			t.Parallel()

			w, err := valid().Build()
			assert.NoError(t, err, "the fixture composition is valid")
			report, err := w.Run(t.Context(), workspace.Input{})
			assert.HasError(t, err, "there is nothing to run over")
			assert.Nil(t, report, "nothing ran")
		})

		t.Run("returns an error for an input with both a tree and a graph", func(t *testing.T) {
			t.Parallel()

			w, err := valid().Build()
			assert.NoError(t, err, "the fixture composition is valid")
			g, _ := alpha(t)
			report, err := w.Run(t.Context(), workspace.Input{Tree: fstest.MapFS{}, Graph: g})
			assert.HasError(t, err, "one input is read")
			assert.Nil(t, report, "nothing ran")
		})

		t.Run("stamps the key a plugin registers", func(t *testing.T) {
			t.Parallel()

			report, s, flag, err := keyedRun(t, nil)
			assert.NoError(t, err, "the run is clean")
			assert.False(t, report.Sink.Failed(), "nothing is reported")
			v, held := meta.Get(report.Facts, s, flag)
			assert.True(t, held && v, "the plugin's key is stamped")
		})

		t.Run("mirrors the stamped subject in the plan", func(t *testing.T) {
			t.Parallel()

			report, s, _, err := keyedRun(t, nil)
			assert.NoError(t, err, "the run is clean")
			got := units(report.Emits["plan"])
			assert.Length(t, got, 1, "the flagged subject is mirrored")
			assert.Equal(t, got[0].Plugin, plugin.ID("mirror"), "the unit names its plugin")
			assert.Equal(t, got[0].Origins, []symbol.Identity{s}, "the unit has its provenance")
		})

		t.Run("settles the plan's store before the report", func(t *testing.T) {
			t.Parallel()

			report, _, _, err := keyedRun(t, nil)
			assert.NoError(t, err, "the run is clean")
			assert.True(t, report.Emits["plan"].Settled(), "the store is settled")
		})

		t.Run("drops a fact the meta directive names", func(t *testing.T) {
			t.Parallel()

			report, s, flag, err := keyedRun(t, func(g *store.Graph, s symbol.Identity) {
				assert.NoError(t, g.AttachDirectives(s, []directive.Raw{rawMeta("shape.flag", 4)}),
					"the drop attaches before the seal")
			})
			assert.NoError(t, err, "a drop is authored intent, not a finding")
			assert.False(t, report.Sink.Failed(), "nothing is reported")
			_, held := meta.Get(report.Facts, s, flag)
			assert.False(t, held, "the drop outranks the stamp whenever it arrives")
			assert.Empty(t, units(report.Emits["plan"]), "the flag gates nothing")
		})

		t.Run("drops every member fact of a group the meta directive names", func(t *testing.T) {
			t.Parallel()

			report, s, flag, err := keyedRun(t, func(g *store.Graph, s symbol.Identity) {
				assert.NoError(t, g.AttachDirectives(s, []directive.Raw{rawMeta(string(flagGroup), 4)}),
					"the group drop attaches before the seal")
			})
			assert.NoError(t, err, "a drop is authored intent, not a finding")
			coretest.AssertCodes(t, report.Sink)
			_, held := meta.Get(report.Facts, s, flag)
			assert.False(t, held, "the tombstone covers the group")
			assert.Empty(t, units(report.Emits["plan"]), "the flag gates nothing")
		})

		t.Run("keeps the facts under directives that drop nothing", func(t *testing.T) {
			t.Parallel()

			report, s, flag, err := keyedRun(t, func(g *store.Graph, s symbol.Identity) {
				assert.NoError(t, g.AttachDirectives(s, []directive.Raw{
					rawDiag("tst-0007", 4),
					rawBareMeta(5),
				}), "the instances attach before the seal")
			})
			assert.NoError(t, err, "neither instance is a fault")
			coretest.AssertCodes(t, report.Sink)
			v, held := meta.Get(report.Facts, s, flag)
			assert.True(t, held && v, "the diag instance and the bare meta instance remove nothing")
			assert.Length(t, units(report.Emits["plan"]), 1, "the flag still gates")
		})

		t.Run("applies a load stamp at plugin authority", func(t *testing.T) {
			t.Parallel()

			var pkg symbol.Identity
			report, _, flag, err := keyedRun(t, func(g *store.Graph, s symbol.Identity) {
				pkg = symbol.Identity{Lang: s.Lang, Package: s.Package, Kind: symbol.KindPackage}
				assert.NoError(t, g.AttachStamps(pkg, []meta.RawStamp{{
					Key: "shape.flag", Value: true,
					Pos:    position.Pos{File: "alpha.go", Line: 1},
					Origin: "fakefront",
				}}), "the raw stamp attaches before the seal")
			})
			assert.NoError(t, err, "the run is clean")
			v, held := meta.Get(report.Facts, pkg, flag)
			assert.True(t, held && v, "the load's classification reads back through the typed handle")
		})

		strayStamp := func(g *store.Graph, s symbol.Identity) {
			assert.NoError(t, g.AttachStamps(s, []meta.RawStamp{{
				Key: "shape.ghost", Value: true,
				Pos:    position.Pos{File: "alpha.go", Line: 2},
				Origin: "fakefront",
			}}), "the stray stamp attaches")
		}

		t.Run("reports RefusedStamp for a stamp under an unregistered key", func(t *testing.T) {
			t.Parallel()

			report, _, _, err := keyedRun(t, strayStamp)
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the Error finding fails the run")
			coretest.AssertReports(t, report.Sink, meta.RefusedStamp)
		})

		t.Run("runs the plans after a refused stamp", func(t *testing.T) {
			t.Parallel()

			report, _, _, _ := keyedRun(t, strayStamp)
			assert.Length(t, units(report.Emits["plan"]), 1, "the frame runs whole")
		})

		unclaimed := func(g *store.Graph, s symbol.Identity) {
			assert.NoError(t, g.AttachDirectives(s, []directive.Raw{{
				Name: "ghost",
				Pos:  position.Pos{File: "alpha.go", Line: 5, Col: 1},
			}}), "the stray instance attaches")
		}

		t.Run("reports UnclaimedName for a directive nothing registered", func(t *testing.T) {
			t.Parallel()

			report, _, _, err := keyedRun(t, unclaimed)
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the Error finding fails the run")
			coretest.AssertReports(t, report.Sink, directive.UnclaimedName)
		})

		t.Run("runs the plans after an unclaimed directive", func(t *testing.T) {
			t.Parallel()

			report, _, _, _ := keyedRun(t, unclaimed)
			assert.Length(t, units(report.Emits["plan"]), 1, "the frame runs whole")
		})

		t.Run("reports nothing for an ignored foreign directive", func(t *testing.T) {
			t.Parallel()

			b, _ := flagged()
			w, err := b.Ignore("k8s:").Build()
			assert.NoError(t, err, "the composition opts out of a foreign tool's prefix")
			g, s := alpha(t)
			assert.NoError(t, g.AttachDirectives(s.Identity(), []directive.Raw{{
				Name: "k8s:deepcopy-gen",
				Pos:  position.Pos{File: "alpha.go", Line: 5, Col: 1},
			}}), "the foreign directive attaches like any other")
			report, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.NoError(t, err, "the run passes")
			coretest.AssertCodes(t, report.Sink)
			assert.Length(t, units(report.Emits["plan"]), 1, "the frame runs whole")
		})

		t.Run("reports DanglingSubject for a directive on a subject the graph does not contain", func(t *testing.T) {
			t.Parallel()

			report, _, _, err := keyedRun(t, func(g *store.Graph, _ symbol.Identity) {
				ghost := coretest.Struct("example.com/elsewhere", "Ghost")
				assert.NoError(t, g.AttachDirectives(ghost.Identity(), []directive.Raw{rawMeta("shape.flag", 9)}),
					"the dangling attachment arrives before the seal")
			})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "a dangling subject is an Error")
			coretest.AssertReports(t, report.Sink, directive.DanglingSubject)
		})

		t.Run("reports validation findings in position order", func(t *testing.T) {
			t.Parallel()

			const subjects = 64
			report, _, _, err := keyedRun(t, func(g *store.Graph, _ symbol.Identity) {
				for i := range subjects {
					ghost := coretest.Struct("example.com/elsewhere", fmt.Sprintf("Ghost%02d", i))
					// Identity order runs against position order, so a
					// report in subject or completion order is out of
					// position order.
					assert.NoError(t,
						g.AttachDirectives(ghost.Identity(), []directive.Raw{rawMeta("shape.flag", subjects-i)}),
						"the dangling attachment arrives before the seal")
				}
			})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "a dangling subject is an Error")
			var lines []int
			for d := range report.Sink.All() {
				if d.Code == directive.DanglingSubject {
					lines = append(lines, d.Pos.Line)
				}
			}
			assert.Length(t, lines, subjects, "each subject is one finding")
			assert.True(t, slices.IsSorted(lines), "the findings are in position order")
		})

		t.Run("validates a directive on a package", func(t *testing.T) {
			t.Parallel()

			report, _, _, err := keyedRun(t, func(g *store.Graph, _ symbol.Identity) {
				pkg := coretest.PackageID(coretest.StorePath)
				assert.NoError(t, g.AttachDirectives(pkg, []directive.Raw{rawMeta("shape.flag", 9)}),
					"a package-subject directive attaches before the seal")
			})
			assert.NoError(t, err, "a package is a subject the graph contains")
			coretest.AssertCodes(t, report.Sink)
		})

		t.Run("reports DanglingSubject for a stamp on a subject the graph does not contain", func(t *testing.T) {
			t.Parallel()

			report, _, _, err := keyedRun(t, func(g *store.Graph, _ symbol.Identity) {
				ghost := coretest.Struct("example.com/elsewhere", "Ghost")
				assert.NoError(t, g.AttachStamps(ghost.Identity(), []meta.RawStamp{{
					Key: "shape.ghostly", Value: true,
				}}), "the dangling stamp arrives before the seal")
			})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "a dangling stamp is an Error")
			coretest.AssertReports(t, report.Sink, directive.DanglingSubject)
		})

		t.Run("skips a subject that negates a plugin's directive in that plugin's bare rule", func(t *testing.T) {
			t.Parallel()

			w, err := optingOut(true).Build()
			assert.NoError(t, err, "the composition composes")
			g, first, _ := pair(t)
			report, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.NoError(t, err, "the negation is authored intent, not a finding")
			got := units(report.Emits["plan"])
			assert.Length(t, got, 1, "one unit is emitted")
			assert.Equal(t, got[0].Origins, []symbol.Identity{first.Identity()},
				"only the subject that did not negate is mirrored")
		})

		t.Run("reports NegationRefused for a negated directive whose schema is not negatable", func(t *testing.T) {
			t.Parallel()

			w, err := optingOut(false).Build()
			assert.NoError(t, err, "the composition composes")
			g, _, _ := pair(t)
			report, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the refused negation fails the run")
			coretest.AssertReports(t, report.Sink, directive.NegationRefused)
		})

		t.Run("mirrors a subject whose negation was refused", func(t *testing.T) {
			t.Parallel()

			w, err := optingOut(false).Build()
			assert.NoError(t, err, "the composition composes")
			g, first, second := pair(t)
			report, _ := w.Run(t.Context(), workspace.Input{Graph: g})
			got := units(report.Emits["plan"])
			assert.Length(t, got, 1, "one unit is emitted")
			assert.Equal(t, got[0].Origins, []symbol.Identity{first.Identity(), second.Identity()},
				"a refused instance opts nothing out")
		})

		t.Run("returns an annotator's error without running the plans", func(t *testing.T) {
			t.Parallel()

			angry := stamper("angry", func(*eidos.StructMatch, *eidos.Stamper) error {
				return errors.New("boom")
			})
			w, err := workspace.New().
				Brand(fixtureBrand).
				Annotators(angry).
				Targets("fixture").
				Plans(planTo("plan", "fixture", mirror("mirror"))).
				Build()
			assert.NoError(t, err, "the failing composition composes")
			g, _ := alpha(t)
			report, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.HasError(t, err, "the returned error stops the frame")
			assert.Contains(t, err.Error(), "angry", "the error names the role")
			assert.Contains(t, err.Error(), "boom", "the error has the cause")
			assert.ErrorIsNot(t, err, workspace.ErrRunFailed, "a handler error is a defect, not a finding")
			assert.Empty(t, report.Emits, "no plan ran")
		})

		t.Run("runs a plan whose sibling fails", func(t *testing.T) {
			t.Parallel()

			bad := generator("bad", func(*eidos.StructMatch, *eidos.Emitter) error {
				return errors.New("boom")
			})
			w, err := workspace.New().
				Brand(fixtureBrand).
				Targets("fixture").
				Plans(
					planTo("crashing", "fixture", bad),
					planTo("steady", "fixture", mirror("mirror")),
				).
				Build()
			assert.NoError(t, err, "the two-plan composition composes")
			g, _ := alpha(t)
			report, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.HasError(t, err, "the failing plan is reported")
			assert.Contains(t, err.Error(), `"crashing"`, "the error names the plan")
			assert.Length(t, units(report.Emits["steady"]), 1, "the sibling ran whole")
		})

		t.Run("returns a settle error for a lowering that drops the origin", func(t *testing.T) {
			t.Parallel()

			w, err := workspace.New().
				Brand(fixtureBrand).
				Targets("fixture").
				Plans(workspace.Plan{
					Name:       "plan",
					Generators: []plugin.Generator{mirror("mirror")},
					Backend:    dropping{fakeBackend{name: "printer", target: "fixture"}},
				}).
				Build()
			assert.NoError(t, err, "a backend declaring a lowering seam composes")
			g, _ := alpha(t)
			report, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.HasError(t, err, "the lowering fails the plan")
			assert.Contains(t, err.Error(), "settle", "the error names the stage")
			assert.Contains(t, err.Error(), "origin", "the error names the rule the backend broke")
			assert.ErrorIsNot(t, err, workspace.ErrRunFailed, "a backend defect is not a finding")

			got := units(report.Emits["plan"])
			assert.Length(t, got, 1, "the plan's store is in the report")
			emitted, held := got[0].Decls[0].(*emit.Struct)
			assert.True(t, held && emitted.Name == "ForAlpha",
				"the store has what the generator emitted, because a refused lowering replaces nothing")
		})

		t.Run("returns the context's error for a cancelled context", func(t *testing.T) {
			t.Parallel()

			w, err := valid().Build()
			assert.NoError(t, err, "the fixture composition is valid")
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			g, _ := alpha(t)
			report, err := w.Run(ctx, workspace.Input{Graph: g})
			assert.ErrorIs(t, err, context.Canceled, "the caller's cancellation is returned")
			assert.Equal(t, report.Plans, []workspace.PlanReport{{Name: "plan", Status: workspace.PlanCancelled}},
				"the report states the plan's outcome")
		})

		t.Run("reports UnreadableRecord for a record that does not read", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			place(t, root, ledger.ManifestPath(fixtureBrand)+"/ea.json", "not a record")
			report := cleanRun(t, built(t, onDisk(t, root, diskPlan(t, "plan", layout.Config{}))),
				routedIn(t, coretest.StorePath))
			unreadable := findings(report.Sink, workspace.UnreadableRecord)
			assert.Length(t, unreadable, 1, "one finding for the record")
			assert.Equal(t, unreadable[0].Severity, diag.SeverityInfo, "as information")
			assert.Equal(t, unreadable[0].Pos, position.Pos{File: ledger.ManifestPath(fixtureBrand)},
				"at the record's directory")
			assert.Equal(t, report.Plans[0].Status, workspace.PlanCommitted, "the run proceeds")
			assert.Equal(t, paths(recorded(t, root)), []string{storeGen}, "and its commit replaces the broken record")
		})

		t.Run("returns an error for a ledger that fails to open", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})).
				Ledger(func() (ledger.Ledger, error) { return nil, errNoDevice }))
			report, err := runOver(t, w, routedIn(t, coretest.StorePath))
			assert.ErrorIs(t, err, errNoDevice, "the open's error fails the run")
			assert.Equal(t, report.Plans[0].Status, workspace.PlanFailed, "no plan runs")
		})

		t.Run("returns an error for a ledger open function that returns (nil, nil)", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), diskPlan(t, "plan", layout.Config{})).
				Ledger(func() (ledger.Ledger, error) { return nil, nil }))
			_, err := runOver(t, w, routedIn(t, coretest.StorePath))
			assert.HasError(t, err, "the run fails")
			assert.Contains(t, err.Error(), "returned (nil, nil)", "the error names the fault")
		})

		t.Run("returns an error for a tree without a frontend to load it", func(t *testing.T) {
			t.Parallel()

			w, err := valid().Build()
			assert.NoError(t, err, "the fixture composition is valid")
			_, err = w.Run(t.Context(), workspace.Input{Tree: fstest.MapFS{}})
			assert.HasError(t, err, "the tree is not loaded")
			assert.Contains(t, err.Error(), "no frontend", "the error names the fault")
		})

		t.Run("returns the load's error for a store the load refuses", func(t *testing.T) {
			t.Parallel()

			w, err := valid().Frontends(frontendtest.NewScripted()).Build()
			assert.NoError(t, err, "the loading composition is valid")
			report, err := w.Run(t.Context(), workspace.Input{
				Tree: fstest.MapFS{}, Stores: map[string]fs.FS{"go:mod": fstest.MapFS{}},
			})
			assert.HasError(t, err, "the load fails")
			assert.Equal(t, report.Plans[0].Status, workspace.PlanFailed, "and no plan runs")
		})

		t.Run("stops before the next annotator after a cancellation", func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			later := false
			w, err := workspace.New().
				Brand(fixtureBrand).
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
			_, err = w.Run(ctx, workspace.Input{Graph: g})
			assert.ErrorIs(t, err, context.Canceled, "the cancellation is returned")
			assert.False(t, later, "the schedule stops at the next role")
		})

		t.Run("stops before the next generator after a cancellation", func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			later := false
			w, err := workspace.New().
				Brand(fixtureBrand).
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
			report, err := w.Run(ctx, workspace.Input{Graph: g})
			assert.ErrorIs(t, err, context.Canceled, "the plan returns the cancellation")
			assert.Equal(t, report.Plans[0].Status, workspace.PlanCancelled, "the report states the plan's outcome")
			assert.False(t, later, "the later bucket never ran")
		})

		grumpyRun := func(t *testing.T) (*workspace.Report, error) {
			t.Helper()

			grump := stamper("grump", func(m *eidos.StructMatch, st *eidos.Stamper) error {
				m.Errorf(runCode, "structs are refused here")
				return nil
			})
			w, err := workspace.New().
				Brand(fixtureBrand).
				Annotators(grump).
				Targets("fixture").
				Plans(planTo("plan", "fixture", mirror("mirror"))).
				Build()
			assert.NoError(t, err, "the grumpy composition composes")
			g, _ := alpha(t)
			return w.Run(t.Context(), workspace.Input{Graph: g})
		}

		t.Run("returns ErrRunFailed for an Error finding", func(t *testing.T) {
			t.Parallel()

			_, err := grumpyRun(t)
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the finding classifies the run")
		})

		t.Run("runs the plans after an Error finding", func(t *testing.T) {
			t.Parallel()

			report, _ := grumpyRun(t)
			assert.Length(t, units(report.Emits["plan"]), 1, "the frame runs to the end")
		})

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
				Brand(fixtureBrand).
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
			report, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.NoError(t, err, "the run is clean")
			assert.False(t, report.Sink.Failed(), "nothing is reported")
			assert.Equal(t, bound, s.Identity(), "the handler receives the bound identity")
			assert.Equal(t, seen.Subject, s.Identity(), "the rules are asked from the subject")
			assert.NotNil(t, seen.File, "the rules are asked in the subject's file")
		})

		t.Run("returns ErrRunFailed for a reference the rules bind to nothing", func(t *testing.T) {
			t.Parallel()

			var bound symbol.Identity
			w, err := pointing(&bound).Rules(native{}).Build()
			assert.NoError(t, err, "the composition composes")
			g, s := alpha(t)
			assert.NoError(t, g.AttachDirectives(s.Identity(), []directive.Raw{rawRef("Ghost", 2)}),
				"the directive attaches")
			_, err = w.Run(t.Context(), workspace.Input{Graph: g})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the run fails")
			assert.True(t, bound.IsZero(), "the rejected instance never gated the rule")
		})

		t.Run("reports UnresolvedReference for a language without rules", func(t *testing.T) {
			t.Parallel()

			var bound symbol.Identity
			w, err := pointing(&bound).Build()
			assert.NoError(t, err, "the composition composes without rules")
			g, s := alpha(t)
			assert.NoError(t, g.AttachDirectives(s.Identity(), []directive.Raw{rawRef("Alpha", 2)}),
				"the directive attaches")
			report, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the run fails")
			assert.True(t, slices.Contains(coretest.Codes(report.Sink), directive.UnresolvedReference),
				"UnresolvedReference is reported")
		})

		t.Run("binds the registered rules to every match", func(t *testing.T) {
			t.Parallel()

			var form symbol.TypeForm
			noter := stamper("noter", func(m *eidos.StructMatch, st *eidos.Stamper) error {
				form = m.Rules().TypeOf(&node.TypeRef{Spelling: "int"}).Form
				return nil
			})
			w, err := workspace.New().
				Brand(fixtureBrand).
				Annotators(noter).
				Rules(native{}).
				Targets("fixture").
				Plans(planTo("plan", "fixture", mirror("mirror"))).
				Build()
			assert.NoError(t, err, "the composition composes")
			g, _ := alpha(t)
			report, err := w.Run(t.Context(), workspace.Input{Graph: g})
			assert.NoError(t, err, "the run is clean")
			assert.Equal(t, form, symbol.FormScalar, "the annotator's match folds through the registered rules")
			assert.False(t, slices.Contains(coretest.Codes(report.Sink), rules.AbsentRules),
				"no language is unregistered")
		})

		t.Run("hands the composition's worker count to every phase call", func(t *testing.T) {
			t.Parallel()

			annotated, generated := &atomic.Int64{}, &atomic.Int64{}
			w := built(t, workspace.New().
				Brand(fixtureBrand).
				Annotators(annotateSeen{seen: annotated}).
				Targets("fixture").
				Parallel(eightWorkers).
				Plans(workspace.Plan{
					Name:       "plan",
					Generators: []plugin.Generator{generateSeen{seen: generated}},
					Backend:    fakeBackend{name: "printer", target: "fixture"},
				}))
			g, _ := alpha(t)
			cleanRun(t, w, g)
			assert.Equal(t, annotated.Load(), int64(eightWorkers), "the annotator's call runs on the workers")
			assert.Equal(t, generated.Load(), int64(eightWorkers), "and so does the generator's")
		})

		t.Run("writes on eight workers the bytes one worker writes for a package's accumulator", func(t *testing.T) {
			t.Parallel()

			registry := func() plugin.Generator { return generator("registry", mirrored) }
			serial := parallelFiles(t, 1, crowded(t, parallelSubjects, 0), registry())
			parallel := parallelFiles(t, eightWorkers, crowded(t, parallelSubjects, 0), registry())
			assert.Length(t, serial, 1, "the package's matches assemble one file")
			assert.Equal(t, parallel, serial, "the file does not depend on the worker count")
		})

		t.Run("writes on eight workers the bytes one worker writes for one slot many invocations append into",
			func(t *testing.T) {
				t.Parallel()

				serial := parallelFiles(t, 1, crowded(t, 1, parallelInstances), providing(), weaverOf())
				parallel := parallelFiles(t, eightWorkers, crowded(t, 1, parallelInstances), providing(), weaverOf())
				assert.Length(t, serial, 1, "the woven struct renders in one file")
				for _, body := range serial {
					assert.Contains(t, body, "audit00 audit01", "the fields follow the instances' order")
				}
				assert.Equal(t, parallel, serial, "the file does not depend on the worker count")
			})

		t.Run("names a weaver in the frame of the file it appends into", func(t *testing.T) {
			t.Parallel()

			mirror, weaver := providing(), weaverOf()
			files := parallelFiles(t, 1, crowded(t, 1, 1), mirror, weaver)
			assert.Length(t, files, 1, "the woven struct renders in one file")
			for _, body := range files {
				record, framed := output.Read([]byte(body))
				assert.True(t, framed, "the file is stamped")
				assert.Equal(t, record.Plugins, []plugin.ID{mirror.Name(), weaver.Name()},
					"the frame names the emitter and the weaver")
			}
		})
	})
}

// The sizes of the parallel run fixtures: structs in one package, the
// instances of the weaver's repeatable directive on one struct, and the
// worker count the parallel runs take.
const (
	parallelSubjects  = 64
	parallelInstances = 32
	eightWorkers      = 8
)

// mirroredCapability is what the providing mirror declares and the
// weaver requires, which puts the weaver in the later bucket.
const mirroredCapability plugin.Capability = "mirrored"

// annotateSeen is a hand-rolled annotator that records the worker
// count its phase call hands it.
type annotateSeen struct{ seen *atomic.Int64 }

// Name returns the annotator's name.
func (annotateSeen) Name() plugin.ID { return "annotate-seen" }

// Annotate records the call's worker count.
func (a annotateSeen) Annotate(ctx *plugin.AnnotatorContext) error {
	a.seen.Store(int64(ctx.Workers))
	return nil
}

// generateSeen is a hand-rolled generator that records the worker
// count its phase call hands it.
type generateSeen struct{ seen *atomic.Int64 }

// Name returns the generator's name.
func (generateSeen) Name() plugin.ID { return "generate-seen" }

// Generate records the call's worker count.
func (g generateSeen) Generate(ctx *plugin.GeneratorContext) error {
	g.seen.Store(int64(ctx.Workers))
	return nil
}

// auditSchema is the weaver's repeatable directive: each instance on a
// subject adds one field to the subject's mirrored struct.
var auditSchema = directive.Schema{
	Plugin: "weaver", Name: "audit", Repeatable: true,
	Doc: "adds one audited field to the mirrored struct",
}

// crowded returns an unfrozen graph declaring n structs in one file of
// the store package's own directory, and that many raw instances of the
// weaver's audit directive on the first struct.
func crowded(tb assert.TB, n, instances int) *store.Graph {
	tb.Helper()

	file := coretest.StorePath + "/crowd.go"
	decls := make([]symbol.Symbol, n)
	structs := make([]*node.Struct, n)
	for i := range structs {
		s := coretest.Struct(coretest.StorePath, fmt.Sprintf("S%02d", i))
		s.Pos = position.Pos{File: file, Line: i + 1, Col: 1}
		structs[i], decls[i] = s, s
	}
	p := coretest.Package(coretest.StorePath, decls...)
	p.Files[0].Path = file
	g := store.New()
	assert.NoError(tb, g.AddPackage(p), "the fixture package is admitted")
	raws := make([]directive.Raw, instances)
	for i := range raws {
		raws[i] = directive.Raw{Name: auditSchema.Canonical(), Pos: position.Pos{File: file, Line: i + 1, Col: 1}}
	}
	if instances > 0 {
		assert.NoError(tb, g.AttachDirectives(structs[0].Identity(), raws), "the instances attach before the seal")
	}
	return g
}

// providing returns the mirror generator under the capability the
// weaver requires.
func providing() plugin.Generator {
	p, held := eidos.NewPlugin("mirror").
		Provides(mirroredCapability).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
		Handle(eidos.OnStruct(mirrored)).Build().(plugin.Generator)
	if !held {
		panic("workspace_test: an emitter rule lowers to the generator role")
	}
	return p
}

// weaverOf returns the weaver: after the mirror, each instance of its
// audit directive on a subject appends one field into the subject's
// mirrored struct through the Emitter.
func weaverOf() plugin.Generator {
	p, held := eidos.NewPlugin("weaver").
		Requires(mirroredCapability).
		Handle(eidos.Directive(auditSchema,
			eidos.OnEmit(symbol.KindStruct, func(m *eidos.EmitMatch, e *eidos.Emitter) error {
				s, held := m.Value.(*emit.Struct)
				if !held {
					return nil
				}
				e.Slot(&s.Fields).Append(&emit.Field{
					Name: fmt.Sprintf("audit%02d", m.Directive().Instance),
					Type: &emit.TypeRef{Spelling: "int"},
				})
				return nil
			}))).Build().(plugin.Generator)
	if !held {
		panic("workspace_test: an emitter rule lowers to the generator role")
	}
	return p
}

// listing is a kit backend whose struct template spells the name of
// each field, so the order of a struct's field slot shows in the
// rendered bytes.
func listing(tb assert.TB) plugin.Backend {
	tb.Helper()

	return backend.New("lister", "fixture", plugin.CommentSyntax{Line: []string{"//"}}).
		KindTemplates(map[symbol.Kind]string{
			symbol.KindStruct: "type {{.Name}} struct { {{range .Fields.Items}}{{.Name}} {{end}}}\n",
		}).
		Naming(func(u plugin.Unit) string { return u.Word + ".txt" }).
		Scaffold(func(emit.Stmt, *render.ImportSet) ([]byte, error) {
			return nil, errors.New("the fixture spells no statements")
		}).
		Imports(func(*render.ImportSet) string { return "" }).
		Finalise(func(src []byte) ([]byte, error) { return src, nil }).
		Coverage(render.Coverage{Facts: totalCoverage()}).
		Build()
}

// parallelFiles runs one plan of the given generators toward the
// listing backend on the given workers over the graph, and returns the
// files the run committed, by path.
func parallelFiles(t *testing.T, workers int, g *store.Graph, gens ...plugin.Generator) map[string]string {
	t.Helper()

	var o opener
	w := built(t, workspace.New().
		Brand(fixtureBrand).
		Targets("fixture").
		Parallel(workers).
		Plans(workspace.Plan{Name: "plan", Generators: gens, Backend: listing(t)}).
		Output(o.open))
	cleanRun(t, w, g)
	out := map[string]string{}
	for _, sink := range o.opened() {
		for path, body := range sink.Files() {
			out[path] = string(body)
		}
	}
	return out
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
			report, err := w.Run(b.Context(), workspace.Input{Graph: g})
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
			if _, err := w.Run(b.Context(), workspace.Input{Graph: g}); err != nil {
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

// native is the scripted rules registered under the fixture's
// language, resolving a name as a struct in the fixture package and
// recording the scope it was asked from.
type native struct {
	scope *rules.Scope
}

// Lang returns the fixture's language.
func (native) Lang() symbol.Lang { return coretest.Lang }

// Members returns the scripted rules' member policy.
func (native) Members() rules.MemberPolicy { return rulestest.Scripted().Members() }

// ParamRole returns the scripted rules' role for a parameter.
func (native) ParamRole(p *node.Param, v rules.View) rules.ParamRole {
	return rulestest.Scripted().ParamRole(p, v)
}

// ReturnRoles returns the scripted rules' roles for the returns.
func (native) ReturnRoles(rs []*node.Return, v rules.View) ([]rules.ReturnRole, rules.ErrorModel) {
	return rulestest.Scripted().ReturnRoles(rs, v)
}

// Builtin returns the scripted rules' shape for a builtin.
func (native) Builtin(ref *node.TypeRef, v rules.View) rules.TypeShape {
	return rulestest.Scripted().Builtin(ref, v)
}

// Resolve returns the fixture package's struct named name, and
// records the scope it was asked from.
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

// SamplesOf returns the scripted rules' samples.
func (native) SamplesOf(ref *node.TypeRef, hint string, v rules.View) (rules.Sample, rules.Sample) {
	return rulestest.Scripted().SamplesOf(ref, hint, v)
}

// ZeroValue returns the scripted rules' zero value.
func (native) ZeroValue(ref *node.TypeRef, v rules.View) (emit.Value, bool) {
	return rulestest.Scripted().ZeroValue(ref, v)
}

// LiteralFor returns the scripted rules' literal for text.
func (native) LiteralFor(f *node.File, ref *node.TypeRef, text string, v rules.View) (emit.Value, bool) {
	return rulestest.Scripted().LiteralFor(f, ref, text, v)
}

// TypeName returns the scripted rules' type name.
func (native) TypeName(word, base string) string { return rulestest.Scripted().TypeName(word, base) }
