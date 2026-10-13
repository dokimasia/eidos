// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"math"
	"testing"
	"testing/fstest"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// The files that the statistics cases add to the tree of the sealed
// cases, a second package that the scripted frontend claims and a file
// that no frontend claims.
const (
	apiSource = "svc/api/user.zz"
	readme    = "README.md"
)

// planMirrorID is the name of the generator that mirrors every struct
// into one plan file.
const planMirrorID plugin.ID = "plan-mirror"

// planDir is the output directory of the plan file of planMirrorID.
const planDir = "gen"

// The counts that the cases expect of the tree.
const (
	// treeFiles is the number of files that the gate stats.
	treeFiles = 3
	// claimedFiles is the number of claimed files that a cold gate hashes.
	claimedFiles = 2
	// treeStructs is the number of structs of the files, Row and User.
	treeStructs = 2
	// treePackages is the number of packages of the files, svc/store and
	// svc/api. The lowering entry of the target journals one invocation
	// for each.
	treePackages = 2
)

// The files of the coupled tree, two directories whose files declare one
// package.
const (
	firstShared  = "a/one.zz"
	secondShared = "b/two.zz"
)

// editTime is the modification time of an edited file, as the write of
// an editor sets it.
var editTime = time.Unix(1, 0)

// TestStats tests the statistics of a run. Each count is the number of
// things that the run did, and nothing else.
func TestStats(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("counts every unit of a cold run as parsed", func(t *testing.T) {
			t.Parallel()

			report := sealedRun(t, sealing(t, ledger.NewMem(), "plan"), workspace.Input{Tree: statsTree()})
			assert.Equal(t, report.Stats.Parsed, len(report.Load.Units), "the run parses every unit")
			assert.Equal(t, report.Stats.Kept+report.Stats.Restored, 0, "and keeps or restores none")
		})

		t.Run("counts the files the gate statted", func(t *testing.T) {
			t.Parallel()

			report := sealedRun(t, sealing(t, ledger.NewMem(), "plan"), workspace.Input{Tree: statsTree()})
			assert.Equal(t, report.Stats.Statted, treeFiles, "the gate stats every file of the tree")
		})

		t.Run("counts the claimed files a cold run hashed", func(t *testing.T) {
			t.Parallel()

			report := sealedRun(t, sealing(t, ledger.NewMem(), "plan"), workspace.Input{Tree: statsTree()})
			assert.Equal(t, report.Stats.Hashed, claimedFiles, "the gate reads every claimed file")
		})

		t.Run("counts no hash for a file the record proves unchanged", func(t *testing.T) {
			t.Parallel()

			w := sealing(t, ledger.NewMem(), "plan")
			sealedRun(t, w, workspace.Input{Tree: statsTree()})
			report := sealedRun(t, w, workspace.Input{Tree: statsTree()})
			assert.Equal(t, report.Stats.Hashed, 0, "every stat matches its record")
		})

		t.Run("counts every unit of an unchanged second run as kept", func(t *testing.T) {
			t.Parallel()

			w := sealing(t, ledger.NewMem(), "plan")
			sealedRun(t, w, workspace.Input{Tree: statsTree()})
			report := sealedRun(t, w, workspace.Input{Tree: statsTree()})
			assert.Equal(t, report.Stats.Kept, len(report.Load.Units), "the generation keeps every unit")
			assert.Equal(t, report.Stats.Parsed, 0, "and nothing parses")
		})

		t.Run("counts a unit an edit couples to as reparsed", func(t *testing.T) {
			t.Parallel()

			w := sealing(t, ledger.NewMem(), "plan")
			sealedRun(t, w, workspace.Input{Tree: coupledTree("type Other int\n", time.Time{})})
			report := sealedRun(t, w, workspace.Input{Tree: coupledTree("type Other int\ntype Twin int\n", editTime)})
			assert.Equal(t, report.Stats.Reparsed, 1, "the unit that declares Twin parses again")
		})

		t.Run("counts the recorded region of the unit that an edit changed", func(t *testing.T) {
			t.Parallel()

			w := sealing(t, ledger.NewMem(), "plan")
			sealedRun(t, w, workspace.Input{Tree: statsTree()})
			report := sealedRun(t, w, workspace.Input{Tree: editedStatsTree()})
			assert.Equal(t, report.Stats.Decoded, 1,
				"the load compares the api unit with its recorded region, and the plan reads no kept unit")
		})

		t.Run("counts the region of a kept unit that the run reads", func(t *testing.T) {
			t.Parallel()

			w := built(t, planMirroring(t, ledger.NewMem()))
			sealedRun(t, w, workspace.Input{Tree: statsTree()})
			report := sealedRun(t, w, workspace.Input{Tree: editedStatsTree()})
			assert.Equal(t, report.Stats.Decoded, 2,
				"the recorded region of the api unit, and the kept store unit, whose row the plan's one group reads")
		})

		t.Run("counts no region for a run over an unchanged tree", func(t *testing.T) {
			t.Parallel()

			w := built(t, planMirroring(t, ledger.NewMem()))
			sealedRun(t, w, workspace.Input{Tree: statsTree()})
			report := sealedRun(t, w, workspace.Input{Tree: statsTree()})
			assert.Equal(t, report.Stats.Decoded, 0, "no phase reads a kept unit")
		})

		t.Run("counts the subjects whose directives the run validated", func(t *testing.T) {
			t.Parallel()

			report, _, _, err := keyedRun(t, func(g *store.Graph, s symbol.Identity) {
				assert.NoError(t, g.AttachDirectives(s, []directive.Raw{rawDiag(suppressible, 4)}),
					"the struct's instance attaches before the seal")
				assert.NoError(t, g.AttachDirectives(coretest.PackageID(coretest.StorePath),
					[]directive.Raw{rawMeta("shape.flag", 9)}), "the package's instance attaches before the seal")
			})
			assert.NoError(t, err, "the run is clean")
			assert.Equal(t, report.Stats.Validated, 2, "the struct and its package")
		})

		t.Run("counts no subject the graph does not contain", func(t *testing.T) {
			t.Parallel()

			report, _, _, err := keyedRun(t, func(g *store.Graph, s symbol.Identity) {
				assert.NoError(t, g.AttachDirectives(s, []directive.Raw{rawDiag(suppressible, 4)}),
					"the struct's instance attaches before the seal")
				ghost := coretest.Struct("example.com/elsewhere", "Ghost")
				assert.NoError(t, g.AttachDirectives(ghost.Identity(), []directive.Raw{rawMeta("shape.flag", 9)}),
					"the dangling instance attaches before the seal")
			})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the dangling subject fails the run")
			assert.Equal(t, report.Stats.Validated, 1, "the struct alone")
		})

		t.Run("counts the invocations of each phase call", func(t *testing.T) {
			t.Parallel()

			report := sealedRun(t, sealing(t, ledger.NewMem(), "plan"), workspace.Input{Tree: statsTree()})
			assert.Equal(t, report.Stats.Invoked, []workspace.Invoked{
				{Plugin: fixtureLowering, Phase: plugin.PhaseAnnotate, Count: treePackages},
				{Plugin: noterID, Phase: plugin.PhaseAnnotate, Count: treeStructs},
				{Plan: "plan", Plugin: mirrorID, Phase: plugin.PhaseGenerate, Count: treeStructs},
			}, "the lowering entry runs once for each package, and the annotator and the generator once for each struct")
		})

		t.Run("counts the plans' generator calls after the annotators in composition order", func(t *testing.T) {
			t.Parallel()

			second := sealedPlan("second", mirror("second-mirror"))
			first := sealedPlan("first", mirror("first-mirror"))
			report := sealedRun(t, built(t, sealingPlans(ledger.NewMem(), second, first)),
				workspace.Input{Tree: statsTree()})
			calls := make([]plugin.ID, 0, len(report.Stats.Invoked))
			for _, c := range report.Stats.Invoked {
				calls = append(calls, c.Plugin)
			}
			assert.Equal(t, calls,
				[]plugin.ID{fixtureLowering, noterID, second.Generators[0].Name(), first.Generators[0].Name()},
				"the lowering entry and the annotator, then the plans as the composition lists them")
		})

		t.Run("counts the files the plans rendered", func(t *testing.T) {
			t.Parallel()

			report := sealedRun(t, sealing(t, ledger.NewMem(), "plan"), workspace.Input{Tree: statsTree()})
			assert.NotEmpty(t, report.Manifest.Files, "the plan renders files")
			assert.Equal(t, report.Stats.Rendered, len(report.Manifest.Files),
				"a clean cold run records every file it rendered")
		})

		t.Run("counts the workspace checks the run called", func(t *testing.T) {
			t.Parallel()

			w := built(t, sealingBuilder(t, ledger.NewMem(), "plan").Checks(
				&recordingCheck{name: "first", reads: []string{"plan"}},
				&recordingCheck{name: "second", reads: []string{"plan"}},
			))
			report := sealedRun(t, w, workspace.Input{Tree: statsTree()})
			assert.Equal(t, report.Stats.Checked, 2, "both checks run")
		})

		t.Run("counts no check that reads a failed plan", func(t *testing.T) {
			t.Parallel()

			w := built(t, onDisk(t, t.TempDir(), failing(t, "plan"), diskPlan(t, "fine", centralised("fine"))).Checks(
				&recordingCheck{name: "blocked", reads: []string{"plan"}},
				&recordingCheck{name: "called", reads: []string{"fine"}},
			))
			report, err := runOver(t, w, routedIn(t, coretest.StorePath))
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the failing plan fails the run")
			assert.Equal(t, report.Stats.Checked, 1, "the check over the clean plan alone")
		})

		t.Run("counts the bytes the commit wrote", func(t *testing.T) {
			t.Parallel()

			report := sealedRun(t, sealing(t, ledger.NewMem(), "plan"), workspace.Input{Tree: statsTree()})
			expect.InRange(t, report.Stats.Written, 1, math.Inf(1), "the commit wrote the generation")
			expect.InRange(t, report.Stats.Size, 1, math.Inf(1), "the live state has its bytes")
		})

		t.Run("reports no generation for a commit that fails", func(t *testing.T) {
			t.Parallel()

			w := sealing(t, crashing{Mem: ledger.NewMem()}, "plan")
			report, err := w.Run(t.Context(), workspace.Input{Tree: statsTree()})
			assert.ErrorIs(t, err, errRecord, "the commit fails")
			assert.False(t, report.Stats.Generation, "and makes no generation live")
		})

		t.Run("reports a run over a graph as cold", func(t *testing.T) {
			t.Parallel()

			report := sealedRun(t, sealing(t, ledger.NewMem(), "plan"),
				workspace.Input{Graph: routedIn(t, coretest.StorePath)})
			assert.True(t, report.Stats.Cold, "the run reads no generation")
			assert.Equal(t, report.Stats.Statted, 0, "and stats nothing")
		})
	})
}

// coupledTree returns one package whose files are in two directories,
// so the scripted language makes two units. The first declares Twin,
// and the second has the body second, written at the time written.
func coupledTree(second string, written time.Time) fstest.MapFS {
	return fstest.MapFS{
		firstShared:  {Data: []byte("package shared\ntype Twin string\n")},
		secondShared: {Data: []byte("package shared\n" + second), ModTime: written},
	}
}

// statsTree returns the tree that the statistics cases load.
func statsTree() fstest.MapFS {
	tree := sealedTree()
	tree[apiSource] = &fstest.MapFile{Data: []byte("package svc/api\ntype User string\n")}
	tree[readme] = &fstest.MapFile{Data: []byte("# svc\n")}
	return tree
}

// editedStatsTree returns the tree of the statistics cases after an edit
// that adds a struct to the api package.
func editedStatsTree() fstest.MapFS {
	tree := statsTree()
	tree[apiSource] = &fstest.MapFile{
		Data: []byte("package svc/api\ntype User string\ntype Account int\n"), ModTime: editTime,
	}
	return tree
}

// planMirroring returns the builder of a composition over the scripted
// frontend that records into a ledger, whose one plan mirrors every
// struct into one file under the output directory of the plan.
func planMirroring(tb assert.TB, l ledger.Ledger) *workspace.Builder {
	tb.Helper()

	return sealingPlans(l, workspace.Plan{
		Name: "plan", Generators: []plugin.Generator{planMirror()}, Backend: fixturePrinter(),
		Layout: layout.Config{Dir: planDir},
	})
}

// planMirror returns a generator that mirrors every struct into one plan
// file, so one group contains the mirror of every struct.
func planMirror() plugin.Generator {
	return eidos.NewPlugin(planMirrorID).
		Output(plugin.Output{Per: plugin.PerPlan, Word: "all"}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			e.PlanFile().Append(&emit.Struct{Origin: m.Struct.Identity(), Name: "For" + m.Struct.Name})
			return nil
		})).Generator()
}
