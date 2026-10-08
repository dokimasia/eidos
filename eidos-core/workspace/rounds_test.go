// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/backend"
	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// The files the mirror generates in the store package and in the api
// package of the rounds tree.
const (
	storeGenerated = "svc/store/gen.txt"
	apiGenerated   = "svc/api/gen.txt"
)

// The lines of the rounds tree besides the warm tree's: the api package's
// user before and after an edit that widens it, and a struct the failing
// mirror refuses. The mirror spells the user's mirror as the first line of
// the api package's generated file, and an edit by hand replaces it with
// the second.
const (
	userLine     = "type User int\n"
	widerUser    = "type User int bool\n"
	badLine      = "type Bad int\n"
	mirroredUser = "type ForUser struct{}"
	editedUser   = "type ForUser struct{ Edited bool }"
)

// The plans of the export cases: the producer, and the two dependents
// whose seers read the producer's export.
const (
	producerPlan                 = "first"
	dependentPlan                = "second"
	otherDependentPlan           = "third"
	seerID             plugin.ID = "seer"
)

// idleID names the generator whose one rule matches no declaration of the
// rounds tree.
const idleID plugin.ID = "idle"

// columnMirrorID names the generator that mirrors the column alone into a
// family of its own.
const columnMirrorID plugin.ID = "column-mirror"

// The generator that mirrors each struct into two families, and the tag
// of its second family.
const (
	twofoldID plugin.ID = "twofold"
	stubTag   eidos.Tag = "stub"
)

// caseSource is a file of a package whose directory differs from the
// store package's directory only in case.
const caseSource = "svc/Store/col.zz"

// The codes the rounds' generators report under: the warning of the
// warning mirror on the user, and the error of the failing mirror on the
// struct it refuses.
var (
	roundsWarning = diag.Code{Prefix: "tst", Number: 10}
	roundsError   = diag.Code{Prefix: "tst", Number: 11}
)

// A warm run executes again the groups of a plan that an edit makes dirty
// and keeps the plan's other files. It leaves what a cold run over the
// edited tree leaves.
func TestRounds(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		before := roundsTree(rowLine, userLine)
		edited := roundsTree(widerRow, userLine)

		t.Run("renders only the file of the group that an edit makes dirty", func(t *testing.T) {
			t.Parallel()

			report, err := warmAfter(t, sealing(t, ledger.NewMem(), "plan"), before, edited)
			assert.NoError(t, err, "the run is clean")
			assert.Equal(t, report.Stats.Rendered, 1, "the store package's file renders again")
		})

		t.Run("records the entries that a cold run over the edited tree records", func(t *testing.T) {
			t.Parallel()

			warm, err := warmAfter(t, sealing(t, ledger.NewMem(), "plan"), before, edited)
			assert.NoError(t, err, "the warm run is clean")
			cold := sealedRun(t, sealing(t, ledger.NewMem(), "plan"), workspace.Input{Tree: edited})
			assert.Equal(t, warm.Manifest.Files, cold.Manifest.Files, "the warm run keeps the api package's entry")
		})

		t.Run("records the entries that a cold run records after an edit deletes a package", func(t *testing.T) {
			t.Parallel()

			after := roundsTree(rowLine, userLine)
			delete(after, apiSource)
			warm, err := warmAfter(t, sealing(t, ledger.NewMem(), "plan"), before, after)
			assert.NoError(t, err, "the warm run is clean")
			cold := sealedRun(t, sealing(t, ledger.NewMem(), "plan"), workspace.Input{Tree: after})
			assert.Equal(t, warm.Manifest.Files, cold.Manifest.Files, "the warm run drops the api package's entry")
		})

		t.Run("renders no file of a plan whose sources an edit does not reach", func(t *testing.T) {
			t.Parallel()

			w := built(t, roundsBuilder(t, ledger.NewMem(), workspace.Sources{Packages: []string{"svc/api"}},
				mirror(mirrorID)))
			report, err := warmAfter(t, w, before, edited)
			assert.NoError(t, err, "the run is clean")
			expect.Equal(t, report.Stats.Rendered, 0, "the plan's file is kept")
			expect.Equal(t, generatedBy(report, mirrorID), 0, "the mirror runs no invocation")
		})

		t.Run("renders the file of a plan whose sources an edit reaches", func(t *testing.T) {
			t.Parallel()

			w := built(t, roundsBuilder(t, ledger.NewMem(), workspace.Sources{Packages: []string{"svc/store"}},
				mirror(mirrorID)))
			report, err := warmAfter(t, w, before, edited)
			assert.NoError(t, err, "the run is clean")
			assert.Equal(t, report.Stats.Rendered, 1, "the scope admits the edited store package")
		})

		t.Run(
			"renders only the dirty group's file of a plan with a generator that matched nothing",
			func(t *testing.T) {
				t.Parallel()

				w := built(t, roundsBuilder(t, ledger.NewMem(), workspace.Sources{}, mirror(mirrorID), idle()))
				report, err := warmAfter(t, w, before, edited)
				assert.NoError(t, err, "the run is clean")
				assert.Equal(t, report.Stats.Rendered, 1, "the store package's file renders again")
			},
		)

		t.Run("renders the file of a generator that implements its role directly after an edit", func(t *testing.T) {
			t.Parallel()

			w := built(t, roundsBuilder(t, ledger.NewMem(), workspace.Sources{}, mirror(mirrorID), direct{}))
			report, err := warmAfter(t, w, before, roundsTree(rowLine, widerUser))
			assert.NoError(t, err, "the run is clean")
			assert.Equal(t, report.Stats.Rendered, 2, "the api package's file and the direct generator's file render")
		})

		t.Run("reports the finding of a generator invocation that the run keeps", func(t *testing.T) {
			t.Parallel()

			w := built(t, roundsBuilder(t, ledger.NewMem(), workspace.Sources{}, warningMirror()))
			report, err := warmAfter(t, w, before, edited)
			assert.NoError(t, err, "a warning fails no run")
			assert.Length(t, findings(report.Sink, roundsWarning), 1, "the run reports the warning on the user again")
		})

		t.Run("runs whole a plan that did not commit in the run before", func(t *testing.T) {
			t.Parallel()

			w := built(t, roundsBuilder(t, ledger.NewMem(), workspace.Sources{}, failingMirror()))
			report, err := warmAfter(t, w, roundsTree(rowLine+badLine, userLine), before)
			assert.NoError(t, err, "the run without the refused struct is clean")
			assert.Equal(t, report.Stats.Rendered, 2, "both packages' files render")
		})

		t.Run("renders no file of a dependent plan whose producer's export is unchanged", func(t *testing.T) {
			t.Parallel()

			w := built(t, sealingPlans(ledger.NewMem(), sealedPlan(t, producerPlan, mirror(mirrorID)),
				seeing(t, dependentPlan)))
			narrow, wide := roundsTree(rowLine+colLine, userLine), roundsTree(widerRow+colLine, userLine)
			report, err := warmAfter(t, w, narrow, wide)
			assert.NoError(t, err, "the run is clean")
			assert.Equal(t, report.Stats.Rendered, 1, "only the producer's store file renders")
		})

		t.Run("renders the file of a dependent plan whose producer's export changed", func(t *testing.T) {
			t.Parallel()

			w := built(t, sealingPlans(ledger.NewMem(), sealedPlan(t, producerPlan, mirror(mirrorID)),
				seeing(t, dependentPlan)))
			report, err := warmAfter(t, w, roundsTree(rowLine+colLine, userLine),
				roundsTree(rowLine+colLine+readerLine, userLine))
			assert.NoError(t, err, "the run is clean")
			assert.Equal(t, report.Stats.Rendered, 2, "the producer's store file and the dependent's file render")
		})

		t.Run("renders the file of each dependent plan whose producer's export changed", func(t *testing.T) {
			t.Parallel()

			w := built(t, sealingPlans(ledger.NewMem(), sealedPlan(t, producerPlan, mirror(mirrorID)),
				seeing(t, dependentPlan), seeing(t, otherDependentPlan)))
			report, err := warmAfter(t, w, roundsTree(rowLine+colLine, userLine),
				roundsTree(rowLine+colLine+readerLine, userLine))
			assert.NoError(t, err, "the run is clean")
			assert.Equal(t, report.Stats.Rendered, 3, "the producer's store file and each dependent's file render")
		})

		t.Run("renders the file of a group that declares a name in the scope of a new name", func(t *testing.T) {
			t.Parallel()

			w := built(t, roundsBuilder(t, ledger.NewMem(), workspace.Sources{}, mirror(mirrorID), columnMirror()))
			report, err := warmAfter(t, w, roundsTree(rowLine+colLine, userLine),
				roundsTree(rowLine+colLine+readerLine, userLine))
			assert.NoError(t, err, "the run is clean")
			assert.Equal(t, report.Stats.Rendered, 2, "the column's file renders again beside the mirror's file")
		})

		t.Run("renders only the dirty group's file of a plan whose mirrors declare fields", func(t *testing.T) {
			t.Parallel()

			w := built(t, roundsBuilder(t, ledger.NewMem(), workspace.Sources{}, referring(), columnMirror()))
			report, err := warmAfter(t, w, roundsTree(rowLine+colLine, userLine),
				roundsTree(widerRow+colLine, userLine))
			assert.NoError(t, err, "the run is clean")
			assert.Equal(t, report.Stats.Rendered, 1, "a field declares no name in the scope of the column's file")
		})

		t.Run("reports the PathCollision of a new path that differs from a kept path only in case", func(t *testing.T) {
			t.Parallel()

			after := roundsTree(rowLine, userLine)
			after[caseSource] = &fstest.MapFile{Data: []byte("package svc/Store\n" + colLine)}
			warm, err := warmAfter(t, sealing(t, ledger.NewMem(), "plan"), before, after)
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the collision fails the plan")
			assert.Length(t, findings(warm.Sink, layout.PathCollision), 1, "the warm run reports the collision")
			cold, _ := sealing(t, ledger.NewMem(), "plan").Run(t.Context(), workspace.Input{Tree: after})
			assert.Equal(t, slices.SortedStableFunc(warm.Sink.All(), diag.Diag.Compare),
				slices.SortedStableFunc(cold.Sink.All(), diag.Diag.Compare),
				"the warm run reports the same findings as the cold run")
		})

		t.Run("reports the render finding of a group that the run keeps", func(t *testing.T) {
			t.Parallel()

			w := built(t, sealingPlans(ledger.NewMem(), workspace.Plan{
				Name: "plan", Generators: []plugin.Generator{commentingMirror()}, Backend: uncommented(t),
			}))
			report, err := warmAfter(t, w, before, edited)
			assert.NoError(t, err, "a warning fails no run")
			assert.Length(t, findings(report.Sink, render.RefusedFact), 1,
				"the run reports the warning on the user's mirror again")
		})

		t.Run("reports DriftedOutput for a generated file that changed on disk", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			assert.NoError(t, os.CopyFS(root, before), "the tree copies into the run's directory")
			w := built(t, sealingOnDisk(t, root))
			sealedRun(t, w, workspace.Input{Tree: os.DirFS(root)})
			replaceIn(t, filepath.Join(root, filepath.FromSlash(apiGenerated)), mirroredUser, editedUser)
			report, err := w.Run(t.Context(), workspace.Input{Tree: os.DirFS(root)})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the drifted file fails the plan")
			assert.False(t, report.Stats.Cold, "the run reads the sealed state")
			assert.Length(t, findings(report.Sink, workspace.DriftedOutput), 1, "the run finds the edit")
		})

		t.Run("renders no file of a group whose generated file a run only touched", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			assert.NoError(t, os.CopyFS(root, before), "the tree copies into the run's directory")
			w := built(t, sealingOnDisk(t, root))
			sealedRun(t, w, workspace.Input{Tree: os.DirFS(root)})
			path := filepath.Join(root, filepath.FromSlash(apiGenerated))
			assert.NoError(t, os.Chtimes(path, warmEdit, warmEdit), "the generated file's times move")
			report := sealedRun(t, w, workspace.Input{Tree: os.DirFS(root)})
			assert.Equal(t, report.Stats.Rendered, 0, "the file's bytes equal its record")
		})

		t.Run("renders no file of a clean group that a contributor of a dirty group places into", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			assert.NoError(t, os.CopyFS(root, before), "the tree copies into the run's directory")
			w := built(t, sealingOf(t, nil, "plan", twofold()).
				Output(func() (output.Sink, error) { return output.NewDisk(root, fixtureBrand) }).
				Ledger(func() (ledger.Ledger, error) { return ledger.OpenDir(root, fixtureBrand) }))
			sealedRun(t, w, workspace.Input{Tree: os.DirFS(root)})
			replaceIn(t, filepath.Join(root, filepath.FromSlash(apiGenerated)), mirroredUser, editedUser)
			report, err := w.Run(t.Context(), workspace.Input{Tree: os.DirFS(root)})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the drifted file fails the plan")
			assert.Equal(t, report.Stats.Rendered, 1, "the run renders the drifted file and keeps the stub file")
		})
	})
}

// roundsTree returns a tree of the store package's row file and the api
// package's user file, each with its lines.
func roundsTree(store, api string) fstest.MapFS {
	return fstest.MapFS{
		sealedSource: {Data: []byte("package svc/store\n" + store)},
		apiSource:    {Data: []byte("package svc/api\n" + api)},
	}
}

// roundsBuilder returns the builder of a composition over the scripted
// frontend that records into a ledger and renders one plan into memory:
// the plan named plan, over the sources, whose generators are gens.
func roundsBuilder(
	tb assert.TB, l ledger.Ledger, sources workspace.Sources, gens ...plugin.Generator,
) *workspace.Builder {
	tb.Helper()

	return workspace.New().
		Brand(fixtureBrand).
		Frontends(frontendtest.NewScripted()).
		Targets("fixture").
		Plans(workspace.Plan{
			Name: "plan", Sources: sources, Generators: gens, Backend: printer(tb, "fixture"),
		}).
		Output(func() (output.Sink, error) { return output.NewMem(), nil }).
		Ledger(func() (ledger.Ledger, error) { return l, nil })
}

// sealingOnDisk returns the builder of the sealing composition over the
// directory root: its output and its ledger are on disk under root.
func sealingOnDisk(tb assert.TB, root string) *workspace.Builder {
	tb.Helper()

	return sealingBuilder(tb, nil, "plan").
		Output(func() (output.Sink, error) { return output.NewDisk(root, fixtureBrand) }).
		Ledger(func() (ledger.Ledger, error) { return ledger.OpenDir(root, fixtureBrand) })
}

// idle returns a generator whose one rule runs on enums, which the rounds
// tree does not declare, so each of its calls runs no invocation.
func idle() plugin.Generator {
	p, held := eidos.NewPlugin(idleID).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "idle"}).
		Handle(eidos.OnEnum(func(*eidos.EnumMatch, *eidos.Emitter) error { return nil })).Build().(plugin.Generator)
	if !held {
		panic("workspace_test: an emitter rule lowers to the generator role")
	}
	return p
}

// columnMirror returns a generator that mirrors the column alone into a
// family of its own: a file in the store package's scope and outside the
// mirror's group.
func columnMirror() plugin.Generator {
	p, held := eidos.NewPlugin(columnMirrorID).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "col"}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			if m.Struct.Name == colID.Name {
				e.PackageFile().Append(&emit.Struct{Origin: m.Struct.Identity(), Name: "Only" + m.Struct.Name})
			}
			return nil
		})).Build().(plugin.Generator)
	if !held {
		panic("workspace_test: an emitter rule lowers to the generator role")
	}
	return p
}

// twofold returns a generator that mirrors each struct into its primary
// family and into the stub family. One invocation contributes to two
// groups of a package, because no file contains the units of both
// families.
func twofold() plugin.Generator {
	p, held := eidos.NewPlugin(twofoldID).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "gen"}).
		Output(plugin.Output{Tag: string(stubTag), Per: plugin.PerPackage, Word: "stub"}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			e.PackageFile().Append(&emit.Struct{Origin: m.Struct.Identity(), Name: "For" + m.Struct.Name})
			e.PackageFile(stubTag).Append(&emit.Struct{Origin: m.Struct.Identity(), Name: "Stub" + m.Struct.Name})
			return nil
		})).Build().(plugin.Generator)
	if !held {
		panic("workspace_test: an emitter rule lowers to the generator role")
	}
	return p
}

// commentingMirror returns a mirror that states a comment on the user's
// mirror.
func commentingMirror() plugin.Generator {
	return generator(mirrorID, func(m *eidos.StructMatch, e *eidos.Emitter) error {
		s := &emit.Struct{Origin: m.Struct.Identity(), Name: "For" + m.Struct.Name}
		if m.Struct.Name == "User" {
			s.Comment = "the user's mirror"
		}
		e.PackageFile().Append(s)
		return nil
	})
}

// uncommented returns the printer under a coverage that refuses comments,
// so the render warns about each struct that states one.
func uncommented(tb assert.TB) plugin.Backend {
	tb.Helper()

	facts := totalCoverage()
	facts[symbol.FactComment] = render.Refuses
	return backend.New("uncommented", "fixture", plugin.CommentSyntax{Line: []string{"//"}}).
		KindTemplates(map[symbol.Kind]string{symbol.KindStruct: "type {{.Name}} struct{}\n"}).
		Naming(func(u plugin.Unit) string { return u.Word + ".txt" }).
		Scaffold(func(emit.Stmt, *render.ImportSet) ([]byte, error) {
			return nil, errors.New("the fixture spells no statements")
		}).
		Imports(func(*render.ImportSet) string { return "" }).
		Finalise(func(src []byte) ([]byte, error) { return src, nil }).
		Coverage(render.Coverage{Facts: facts}).
		Build()
}

// warningMirror returns a mirror that warns on the user.
func warningMirror() plugin.Generator {
	return generator(mirrorID, func(m *eidos.StructMatch, e *eidos.Emitter) error {
		if m.Struct.Name == "User" {
			m.Warnf(roundsWarning, "the mirror warns on the user")
		}
		return mirrored(m, e)
	})
}

// failingMirror returns a mirror that reports an Error on the struct named
// Bad.
func failingMirror() plugin.Generator {
	return generator(mirrorID, func(m *eidos.StructMatch, e *eidos.Emitter) error {
		if m.Struct.Name == "Bad" {
			m.Errorf(roundsError, "the mirror refuses Bad")
		}
		return mirrored(m, e)
	})
}

// seeing returns a dependent plan of a name, which depends on the
// producer. Its seer, named after the plan, counts the declarations of the
// producer's export on the column, and renders the count into a file named
// after the plan.
func seeing(tb assert.TB, name string) workspace.Plan {
	tb.Helper()

	seer := generator(plugin.ID(name)+"-"+seerID, func(m *eidos.StructMatch, e *eidos.Emitter) error {
		if m.Struct.Name != colID.Name {
			return nil
		}
		doc, _ := m.Export(producerPlan)
		e.PackageFile().Append(&emit.Struct{Origin: m.Struct.Identity(), Name: fmt.Sprintf("Seen%d", len(doc.Symbols))})
		return nil
	})
	return workspace.Plan{
		Name: name, DependsOn: []string{producerPlan}, Generators: []plugin.Generator{seer},
		Backend: printerAs(tb, plugin.ID(name)+"-printer", "fixture", name+".txt"),
	}
}

// replaceIn replaces the text from with to in the file at path, the way a
// person edits the file.
func replaceIn(t *testing.T, path, from, to string) {
	t.Helper()

	b, err := os.ReadFile(path)
	assert.NoError(t, err, "the file reads")
	assert.Contains(t, string(b), from, "the file contains the text the edit replaces")
	assert.NoError(t, os.WriteFile(path, []byte(strings.Replace(string(b), from, to, 1)), 0o644),
		"the edited file writes")
}

// generatedBy returns the number of invocations that a run's statistics
// list for a generator, and zero for a generator that they do not list.
func generatedBy(report *workspace.Report, p plugin.ID) int {
	n := 0
	for _, c := range report.Stats.Invoked {
		if c.Plugin == p && c.Phase == plugin.PhaseGenerate {
			n += c.Count
		}
	}
	return n
}
