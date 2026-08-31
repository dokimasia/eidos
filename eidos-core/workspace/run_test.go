// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package workspace_test

import (
	"context"
	"errors"
	"testing"

	"go.dokimi.dev/assert"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// runCode is a code for the run fixtures' findings.
var runCode = diag.Code{Prefix: "tst", Number: 7}

// rawMeta returns a positioned raw instance of the kernel meta
// directive dropping ref.
func rawMeta(ref string, line int) directive.Raw {
	return directive.Raw{
		Name: "meta",
		Args: []directive.RawArg{{Key: "drop", Value: directive.RawValue{Text: ref}}},
		Pos:  position.Pos{File: "alpha.go", Line: line, Col: 1},
	}
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
				Name: "shape.flag", Doc: "marks a fixture subject",
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

// codesOf collects every finding's code.
func codesOf(sink *diag.Sink) []diag.Code {
	var out []diag.Code
	for d := range sink.All() {
		out = append(out, d.Code)
	}
	return out
}

// Run is the frame: seal, validate, drop, annotate, generate. What
// stops it, what merely fails it, and what it leaves behind are all
// contract.
func TestRun(t *testing.T) {
	t.Parallel()

	t.Run("refuses a graph already frozen", func(t *testing.T) {
		t.Parallel()

		w, err := valid().Build()
		assert.NoError(t, err, "the fixture composition is valid")
		g, _ := alpha(t)
		g.Freeze()
		report, err := w.Run(t.Context(), g)
		assert.HasError(t, err, "the seal is Run's own")
		assert.Contains(t, err.Error(), "frozen", "the refusal says why")
		assert.Nil(t, report, "nothing ran")
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
		assert.Contains(t, codesOf(report.Sink), directive.UnclaimedName,
			"under the validator's registered code")
		assert.Length(t, units(report.Emits["plan"]), 1,
			"and the frame still ran whole")
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
		assert.Contains(t, codesOf(report.Sink), directive.DanglingSubject,
			"under its reserved code")
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
