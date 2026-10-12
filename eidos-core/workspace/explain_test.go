// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"encoding/binary"
	"math"
	"slices"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// The files of the explained trees beside the source of the store. They
// are the source of a second package, a file that the scripted frontend
// refuses because it declares a struct before its package line, a path
// that no plan generates, and the source of the package whose last path
// element contains a dot.
const (
	otherSource    = "svc/other/other.zz"
	refusedSource  = "svc/bad/bad.zz"
	refusedContent = "type Bad int\n"
	ungenerated    = "svc/store/none.txt"
	dottedSource   = dottedPath + "/row.zz"
)

// flagName is the key that the flagger of the recording composition
// stamps.
const flagName meta.KeyName = "shape.flag"

// The plan that the recording composition declares, and a plan that
// depends on it.
const (
	planName     = "plan"
	bindingsPlan = "bindings"
)

// The plugins of the explanation cases. also writes a second declaration
// into the file of a plan, binder is the generator of a dependent plan
// that reads the export of the plan, and lister is a check that
// enumerates.
const (
	alsoID   plugin.ID = "also"
	binderID plugin.ID = "binder"
	listerID plugin.ID = "lister"
)

// bogusID is the ID of a record that no run records.
const bogusID = 1

// The positions that the explanation cases name, the declaration of the
// row, the mark on the row and the first line of the refused file.
var (
	rowAt     = position.Pos{File: sealedSource, Line: 2}
	markAt    = position.Pos{File: sealedSource, Line: 3}
	refusedAt = position.Pos{File: refusedSource, Line: 1}
)

// explainedRows are the rows of the explained run that the damage cases
// break. They are the path of the file of the marker, the key of the row
// of its group, the key of the row of its contributor and the key of the
// readers row of the unit of the contributor.
type explainedRows struct {
	path        string
	group       []byte
	contributor []byte
	unit        []byte
}

// damageCase is the row of a table that a case breaks after the explained
// run, and the target whose explanation meets the damage.
type damageCase struct {
	name   string
	table  state.Table
	key    func(r explainedRows) []byte
	target func(r explainedRows) workspace.Target
}

// TestExplain tests the explanation of a target. Explain reads what the
// last generation records about a target, and runs nothing.
func TestExplain(t *testing.T) {
	t.Parallel()

	t.Run("ExecutionKind", func(t *testing.T) {
		t.Parallel()

		t.Run("String", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give workspace.ExecutionKind
				want string
			}{
				{
					name: "returns validation for ExecutionValidation",
					give: workspace.ExecutionValidation,
					want: "validation",
				},
				{
					name: "returns invocation for ExecutionInvocation",
					give: workspace.ExecutionInvocation,
					want: "invocation",
				},
				{name: "returns check for ExecutionCheck", give: workspace.ExecutionCheck, want: "check"},
				{name: "returns group for ExecutionGroup", give: workspace.ExecutionGroup, want: "group"},
				{name: "returns audit for ExecutionAudit", give: workspace.ExecutionAudit, want: "audit"},
				{name: "returns region for ExecutionRegion", give: workspace.ExecutionRegion, want: "region"},
				{
					name: "returns the number of an undeclared kind",
					give: workspace.ExecutionKind(7),
					want: "ExecutionKind(7)",
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, tt.give.String(), tt.want, "the spelling is pinned")
				})
			}
		})
	})

	t.Run("ReadGrain", func(t *testing.T) {
		t.Parallel()

		t.Run("String", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give workspace.ReadGrain
				want string
			}{
				{name: "returns declaration for ReadDeclaration", give: workspace.ReadDeclaration, want: "declaration"},
				{name: "returns package for ReadPackage", give: workspace.ReadPackage, want: "package"},
				{name: "returns fact for ReadFact", give: workspace.ReadFact, want: "fact"},
				{name: "returns kind for ReadKind", give: workspace.ReadKind, want: "kind"},
				{name: "returns directive for ReadDirective", give: workspace.ReadDirective, want: "directive"},
				{name: "returns export for ReadExport", give: workspace.ReadExport, want: "export"},
				{name: "returns the number of an undeclared grain", give: workspace.ReadGrain(7), want: "ReadGrain(7)"},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, tt.give.String(), tt.want, "the spelling is pinned")
				})
			}
		})
	})

	t.Run("Explain", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the entry of a generated file", func(t *testing.T) {
			t.Parallel()

			w, mem, path := explained(t)
			got, err := w.Explain(t.Context(), workspace.Target{Path: path})
			assert.NoError(t, err, "the generation explains the file")
			assert.Length(t, got.Files, 1, "the explanation lists the file")
			assert.Equal(t, got.Files[0].Entry, recordIn(t, mem).Files[0], "the entry is the recorded one")
		})

		t.Run("returns the contributor of a generated file's group", func(t *testing.T) {
			t.Parallel()

			w, _, path := explained(t)
			got, err := w.Explain(t.Context(), workspace.Target{Path: path})
			assert.NoError(t, err, "the generation explains the file")
			assert.Length(t, got.Files, 1, "the explanation lists the file")
			assert.Length(t, got.Files[0].Contributors, 1, "one invocation placed the mirror")
			expect.Equal(t, got.Files[0].Contributors[0].Plugin, markerID, "the marker placed it")
			expect.Equal(t, got.Files[0].Contributors[0].Subject, rowID, "on the row")
		})

		t.Run("returns the group and the contributor of a generated file", func(t *testing.T) {
			t.Parallel()

			w, _, path := explained(t)
			got, err := w.Explain(t.Context(), workspace.Target{Path: path})
			assert.NoError(t, err, "the generation explains the file")
			want := []workspace.ExecutionKind{workspace.ExecutionGroup, workspace.ExecutionInvocation}
			assert.Equal(t, kindsOf(got), want, "the group comes before its contributor")
			expect.Equal(t, got.Executions[0].Plan, planName, "the group is the plan's")
			expect.Equal(t, got.Executions[1].Match.Plugin, markerID, "the contributor is the marker's invocation")
		})

		t.Run("identifies the declaration read of a contributor", func(t *testing.T) {
			t.Parallel()

			w, _, path := explained(t)
			got, err := w.Explain(t.Context(), workspace.Target{Path: path})
			assert.NoError(t, err, "the generation explains the file")
			assert.Length(t, got.Executions, 2, "the group and the contributor")
			row := workspace.Read{Grain: workspace.ReadDeclaration, Subject: rowID}
			assert.Contains(t, got.Executions[1].Reads, row, "the marker read the row")
		})

		t.Run("identifies the fact read of a contributor", func(t *testing.T) {
			t.Parallel()

			w, _, path := explained(t)
			got, err := w.Explain(t.Context(), workspace.Target{Path: path})
			assert.NoError(t, err, "the generation explains the file")
			assert.Length(t, got.Executions, 2, "the group and the contributor")
			assert.Contains(t, got.Executions[1].Reads,
				workspace.Read{Grain: workspace.ReadFact, Subject: rowID, Key: flagName}, "the marker read the flag")
		})

		t.Run("counts the reads of a group that it cannot identify", func(t *testing.T) {
			t.Parallel()

			w, _, path := explained(t)
			got, err := w.Explain(t.Context(), workspace.Target{Path: path})
			assert.NoError(t, err, "the generation explains the file")
			assert.Length(t, got.Executions, 2, "the group and the contributor")
			assert.InRange(t, got.Executions[0].Unidentified, 1, math.Inf(1), "the group read its unit's edge")
		})

		t.Run("identifies the kind read of a check", func(t *testing.T) {
			t.Parallel()

			got := listed(t)
			at := slices.IndexFunc(
				got.Executions,
				func(r workspace.ExplainedExecution) bool { return r.Check == listerID },
			)
			assert.InRange(t, at, 0, math.Inf(1), "the lister read the row")
			assert.Contains(t, got.Executions[at].Reads,
				workspace.Read{Grain: workspace.ReadKind, Kind: symbol.KindStruct}, "the lister enumerated the structs")
		})

		t.Run("identifies the directive read of a check", func(t *testing.T) {
			t.Parallel()

			got := listed(t)
			at := slices.IndexFunc(
				got.Executions,
				func(r workspace.ExplainedExecution) bool { return r.Check == listerID },
			)
			assert.InRange(t, at, 0, math.Inf(1), "the lister read the row")
			assert.Contains(t, got.Executions[at].Reads,
				workspace.Read{Grain: workspace.ReadDirective, Directive: markSchema.Canonical()},
				"the lister enumerated the carriers of the mark")
		})

		t.Run("identifies the export read of the invocation of a dependent plan", func(t *testing.T) {
			t.Parallel()

			binder := generator(binderID, func(m *eidos.StructMatch, e *eidos.Emitter) error {
				_, _ = m.Export(planName)
				e.PackageFile().Append(&emit.Struct{Origin: m.Struct.Identity(), Name: "Bound" + m.Struct.Name})
				return nil
			})
			bindings := sealedPlan(bindingsPlan, binder)
			bindings.DependsOn = []string{planName}
			w := built(t, sealingPlans(ledger.NewMem(), sealedPlan(planName, mirror(mirrorID)), bindings))
			sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			got, err := w.Explain(t.Context(), workspace.Target{Identity: rowID})
			assert.NoError(t, err, "the generation explains the row")
			bound := func(r workspace.ExplainedExecution) bool { return r.Match.Plugin == binderID }
			at := slices.IndexFunc(got.Executions, bound)
			assert.InRange(t, at, 0, math.Inf(1), "the binder ran on the row")
			assert.Contains(t, got.Executions[at].Reads, workspace.Read{Grain: workspace.ReadExport, Plan: planName},
				"the binder read the plan's export")
		})

		t.Run("returns the entry alone of a file whose group the generation does not record", func(t *testing.T) {
			t.Parallel()

			w, mem, path := explained(t)
			deleteRow(t, mem, state.TableGroups, rowsOf(t, mem, path).group)
			got, err := w.Explain(t.Context(), workspace.Target{Path: path})
			assert.NoError(t, err, "the generation explains the file")
			assert.Length(t, got.Files, 1, "the explanation lists the file")
			expect.Empty(t, got.Files[0].Contributors, "the file has no recorded contributor")
			expect.Empty(t, got.Executions, "the explanation has no group")
		})

		t.Run("returns no file for a path that no plan generated", func(t *testing.T) {
			t.Parallel()

			w, _, _ := explained(t)
			got, err := w.Explain(t.Context(), workspace.Target{Path: ungenerated})
			assert.NoError(t, err, "the generation explains the path")
			expect.NotEmpty(t, got.Generation, "the explanation names the generation")
			expect.Empty(t, got.Files, "no plan generated the path")
			expect.Empty(t, got.Executions, "no execution produced the path")
		})

		t.Run("returns the claim on the fact of a declaration", func(t *testing.T) {
			t.Parallel()

			w, _, _ := explained(t)
			got, err := w.Explain(t.Context(), workspace.Target{Identity: rowID})
			assert.NoError(t, err, "the generation explains the row")
			at := slices.IndexFunc(got.Claims, func(c workspace.ExplainedClaim) bool { return c.Key == flagName })
			assert.InRange(t, at, 0, math.Inf(1), "the flagger stamped the flag on the row")
			expect.Equal(t, got.Claims[at].Claim.Plugin, flaggerID, "the flagger claims the flag")
			expect.Equal(t, got.Claims[at].Value, any(true), "the claim has the value of the flagger")
			expect.True(t, got.Claims[at].Winner, "the claim ranks first")
		})

		t.Run("returns the claim of the lowering entry on the name of a declaration in the target", func(t *testing.T) {
			t.Parallel()

			w, _, _ := explained(t)
			got, err := w.Explain(t.Context(), workspace.Target{Identity: rowID})
			assert.NoError(t, err, "the generation explains the row")
			name := plugin.Target("fixture").NameKey()
			at := slices.IndexFunc(got.Claims, func(c workspace.ExplainedClaim) bool { return c.Key == name })
			assert.InRange(t, at, 0, math.Inf(1), "the lowering entry stamped the name of the row in the target")
			expect.Equal(t, got.Claims[at].Claim.Plugin, fixtureLowering, "the lowering entry claims the name")
			expect.Equal(t, got.Claims[at].Value, any(rowID.Name), "the backend keeps the name of the row")
		})

		t.Run("returns the executions that read a declaration", func(t *testing.T) {
			t.Parallel()

			w, _, _ := explained(t)
			got, err := w.Explain(t.Context(), workspace.Target{Identity: rowID})
			assert.NoError(t, err, "the generation explains the row")
			assert.Equal(t, kindsOf(got), []workspace.ExecutionKind{
				workspace.ExecutionValidation,
				workspace.ExecutionInvocation,
				workspace.ExecutionInvocation,
				workspace.ExecutionCheck,
				workspace.ExecutionGroup,
			}, "the mark's validation, the flagger, the marker, the lookup and the mirror's group read the row")
		})

		t.Run("returns the files that the plans generated from a declaration", func(t *testing.T) {
			t.Parallel()

			w, _, path := explained(t)
			got, err := w.Explain(t.Context(), workspace.Target{Identity: rowID})
			assert.NoError(t, err, "the generation explains the row")
			assert.Length(t, got.Files, 1, "the marker generated one file from the row")
			assert.Equal(t, got.Files[0].Entry.Path, path, "the marker's file")
		})

		t.Run("returns once a file that two invocations on a declaration generated", func(t *testing.T) {
			t.Parallel()

			also := generator(alsoID, func(m *eidos.StructMatch, e *eidos.Emitter) error {
				e.PackageFile().Append(&emit.Struct{Origin: m.Struct.Identity(), Name: "Also" + m.Struct.Name})
				return nil
			})
			w := built(t, sealingPlans(ledger.NewMem(), workspace.Plan{
				Name:       planName,
				Generators: []plugin.Generator{mirror(mirrorID), also},
				Backend:    fixturePrinter(),
			}))
			sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			got, err := w.Explain(t.Context(), workspace.Target{Identity: rowID})
			assert.NoError(t, err, "the generation explains the row")
			assert.Length(t, got.Files, 1, "both generators wrote into the plan's one file")
		})

		t.Run("leaves out a file whose artifact the generation does not record", func(t *testing.T) {
			t.Parallel()

			w, mem, path := explained(t)
			deleteRow(t, mem, state.TableArtifacts, []byte(path))
			got, err := w.Explain(t.Context(), workspace.Target{Identity: rowID})
			assert.NoError(t, err, "the generation explains the row")
			assert.Empty(t, got.Files, "the group's file has no artifact")
		})

		t.Run("returns the explanation of the declaration that an identity without a kind names", func(t *testing.T) {
			t.Parallel()

			w, _, _ := explained(t)
			want, err := w.Explain(t.Context(), workspace.Target{Identity: rowID})
			assert.NoError(t, err, "the generation explains the row")
			got, err := w.Explain(t.Context(), workspace.Target{Identity: kindlessRow()})
			assert.NoError(t, err, "the generation explains the identity without a kind")
			assert.Equal(t, got, want, "the identity names the row")
		})

		t.Run("returns the explanation of the identity for the spelling of a dotted package", func(t *testing.T) {
			t.Parallel()

			var flag meta.Key[bool]
			w := built(t, phaseRecording(t, ledger.NewMem(), &flag))
			sealedRun(t, w, workspace.Input{Tree: fstest.MapFS{
				dottedSource: {Data: []byte("package " + dottedPath + "\ntype Row int string\n" + markLine + "\n")},
			}})
			row := rowID
			row.Package = dottedPath
			want, err := w.Explain(t.Context(), workspace.Target{Identity: row})
			assert.NoError(t, err, "the generation explains the row")
			assert.NotEmpty(t, want.Executions, "executions read the row")
			target, err := w.ParseTarget(row.String())
			assert.NoError(t, err, "the spelling of the row parses")
			got, err := w.Explain(t.Context(), target)
			assert.NoError(t, err, "the generation explains the spelling")
			assert.Equal(t, got, want, "the spelling names the row of the dotted package")
		})

		t.Run("returns the claim on the fact of a key at a position", func(t *testing.T) {
			t.Parallel()

			w, _, _ := explained(t)
			got, err := w.Explain(t.Context(), workspace.Target{Key: flagName, At: rowAt})
			assert.NoError(t, err, "the generation explains the fact")
			assert.Length(t, got.Claims, 1, "one claim on the row's flag")
			assert.Equal(t, got.Claims[0].Claim.Subject, rowID, "the claim is on the row")
		})

		t.Run("returns the execution that read the fact of a key at a position", func(t *testing.T) {
			t.Parallel()

			w, _, _ := explained(t)
			got, err := w.Explain(t.Context(), workspace.Target{Key: flagName, At: rowAt})
			assert.NoError(t, err, "the generation explains the fact")
			assert.Equal(t, kindsOf(got), []workspace.ExecutionKind{workspace.ExecutionInvocation},
				"one execution read the flag")
			assert.Equal(t, got.Executions[0].Match.Plugin, markerID, "the marker read it")
		})

		t.Run("returns the validation that reported a code at a position", func(t *testing.T) {
			t.Parallel()

			var flag meta.Key[bool]
			w := built(t, phaseRecording(t, ledger.NewMem(), &flag))
			_, err := w.Run(t.Context(), workspace.Input{Tree: markedTree(" bogus=1")})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the unknown key fails the run")
			got, err := w.Explain(t.Context(), workspace.Target{Code: directive.UnknownKey, At: markAt})
			assert.NoError(t, err, "the generation explains the finding")
			assert.Equal(t, kindsOf(got), []workspace.ExecutionKind{workspace.ExecutionValidation},
				"the validation reported it")
			assert.Equal(t, got.Executions[0].Subject, rowID, "on the row's mark")
		})

		t.Run("returns the invocation that reported a code at a position", func(t *testing.T) {
			t.Parallel()

			w := built(t, warmBuilder(t, ledger.NewMem(), &warmKeys{}, warner()))
			sealedRun(t, w, workspace.Input{Tree: warmTree(rowLine, colLine)})
			got, err := w.Explain(t.Context(), workspace.Target{Code: warmWarning, At: rowAt})
			assert.NoError(t, err, "the generation explains the finding")
			assert.Equal(t, kindsOf(got), []workspace.ExecutionKind{workspace.ExecutionInvocation},
				"an invocation reported it")
			assert.Equal(t, got.Executions[0].Match.Plugin, warnerID, "the warner's invocation")
		})

		t.Run("returns the audit finding of a code at a position", func(t *testing.T) {
			t.Parallel()

			w := built(t, sealingBuilder(t, ledger.NewMem(), planName).Keys(contracted))
			sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			got, err := w.Explain(t.Context(), workspace.Target{Code: workspace.UnmetContract, At: rowAt})
			assert.NoError(t, err, "the generation explains the finding")
			assert.Equal(t, kindsOf(got), []workspace.ExecutionKind{workspace.ExecutionAudit}, "the audit reported it")
			assert.Equal(t, got.Executions[0].Subject, rowID, "on the row")
		})

		t.Run("returns the region that reported a code at a position", func(t *testing.T) {
			t.Parallel()

			var flag meta.Key[bool]
			w := built(t, phaseRecording(t, ledger.NewMem(), &flag))
			tree := markedTree("")
			tree[refusedSource] = &fstest.MapFile{Data: []byte(refusedContent)}
			_, err := w.Run(t.Context(), workspace.Input{Tree: tree})
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the refused file fails the run")
			got, err := w.Explain(t.Context(), workspace.Target{Code: frontendtest.ScriptedBadFile, At: refusedAt})
			assert.NoError(t, err, "the generation explains the finding")
			assert.Equal(t, kindsOf(got), []workspace.ExecutionKind{workspace.ExecutionRegion},
				"the load's region reported it")
			assert.Length(t, got.Executions[0].Findings, 1, "with the refused file's finding")
		})

		t.Run("returns no execution for a code at another column", func(t *testing.T) {
			t.Parallel()

			w := built(t, warmBuilder(t, ledger.NewMem(), &warmKeys{}, warner()))
			sealedRun(t, w, workspace.Input{Tree: warmTree(rowLine, colLine)})
			elsewhere := rowAt
			elsewhere.Col = 9
			got, err := w.Explain(t.Context(), workspace.Target{Code: warmWarning, At: elsewhere})
			assert.NoError(t, err, "the generation explains the position")
			assert.Empty(t, got.Executions, "the warning is at the line's start")
		})

		t.Run("takes the lock for workspace.Explain as the caller", func(t *testing.T) {
			t.Parallel()

			l := &recordingLock{Mem: ledger.NewMem()}
			var flag meta.Key[bool]
			w := built(t, phaseRecording(t, l, &flag))
			sealedRun(t, w, workspace.Input{Tree: markedTree("")})
			_, err := w.Explain(t.Context(), workspace.Target{Path: ungenerated})
			assert.NoError(t, err, "the generation explains the path")
			assert.Equal(t, l.held.Caller, "workspace.Explain", "the record names the kernel's entry point")
		})

		t.Run("returns a LockedError while another holder has the lock", func(t *testing.T) {
			t.Parallel()

			w, mem, path := explained(t)
			release, err := mem.Lock(t.Context(), otherHolder)
			assert.NoError(t, err, "another holder takes the lock")
			t.Cleanup(func() { assert.NoError(t, release(), "the other holder releases the lock") })
			_, err = w.Explain(t.Context(), workspace.Target{Path: path})
			locked := assert.ErrorAs[*ledger.LockedError](t, err, "the lock is taken")
			assert.Equal(t, locked.Holder, otherHolder, "the error names the holder")
		})

		t.Run("returns the release's error after the explanation", func(t *testing.T) {
			t.Parallel()

			var flag meta.Key[bool]
			w := built(t, phaseRecording(t, unlocking{ledger.NewMem()}, &flag))
			_, err := w.Run(t.Context(), workspace.Input{Tree: markedTree("")})
			assert.ErrorIs(t, err, errUnlock, "the run's release fails")
			_, err = w.Explain(t.Context(), workspace.Target{Path: ungenerated})
			assert.ErrorIs(t, err, errUnlock, "the explanation's release fails")
		})

		t.Run("returns ErrNoGeneration for a ledger without a generation", func(t *testing.T) {
			t.Parallel()

			var flag meta.Key[bool]
			w := built(t, phaseRecording(t, ledger.NewMem(), &flag))
			_, err := w.Explain(t.Context(), workspace.Target{Path: ungenerated})
			assert.ErrorIs(t, err, workspace.ErrNoGeneration, "no run wrote a generation")
		})

		t.Run("returns ErrNoGeneration for a composition without a ledger", func(t *testing.T) {
			t.Parallel()

			_, err := built(t, valid()).Explain(t.Context(), workspace.Target{Path: ungenerated})
			assert.ErrorIs(t, err, workspace.ErrNoGeneration, "the composition keeps no record")
		})

		t.Run("returns an error for a generation that does not open", func(t *testing.T) {
			t.Parallel()

			w, mem, path := explained(t)
			assert.NoError(t, mem.Write(t.Context(), currentBlob, []byte(undecodableRow)), "CURRENT names nothing")
			_, err := w.Explain(t.Context(), workspace.Target{Path: path})
			assert.ErrorIs(t, err, state.ErrDamaged, "the generation does not open")
		})

		t.Run("returns an error for a claims row that does not decode", func(t *testing.T) {
			t.Parallel()

			w, mem, _ := explained(t)
			forgeTable(t, mem, state.TableClaims)
			_, err := w.Explain(t.Context(), workspace.Target{Identity: rowID})
			assert.ErrorIs(t, err, state.ErrDamaged, "the row's claims do not restore")
		})

		regions := []struct {
			name string
			give workspace.Target
		}{
			{
				name: "returns an error for the region of a position that does not decode",
				give: workspace.Target{Key: flagName, At: rowAt},
			},
			{
				name: "returns an error for the region of an identity without a kind that does not decode",
				give: workspace.Target{Identity: kindlessRow()},
			},
		}
		for _, tt := range regions {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				w, mem, _ := explained(t)
				truncate(t, mem, func(live int) bool { return live > 0 })
				_, err := w.Explain(t.Context(), tt.give)
				assert.ErrorIs(t, err, state.ErrDamaged, "the region does not decode")
			})
		}

		t.Run("returns an error for the groups row of a unit's group that does not decode", func(t *testing.T) {
			t.Parallel()

			w, mem, path := explained(t)
			// The unit's readers row lists one group under an ID that no record
			// has: the count, the group kind's byte and the ID, big-endian.
			putRow(t, mem, state.TableReaders, rowsOf(t, mem, path).unit,
				binary.BigEndian.AppendUint64([]byte{1, byte(state.RecordGroup)}, bogusID))
			forgeRow(t, mem, state.TableGroups, binary.BigEndian.AppendUint64(nil, bogusID))
			_, err := w.Explain(t.Context(), workspace.Target{Identity: rowID})
			assert.ErrorIs(t, err, state.ErrDamaged, "the unit's group does not decode")
		})

		refused := []struct {
			name string
			give workspace.Target
		}{
			{name: "returns an error for a target without a form", give: workspace.Target{}},
			{
				name: "returns an error for a target of two forms",
				give: workspace.Target{Path: ungenerated, Identity: rowID},
			},
			{name: "returns an error for a key without a position", give: workspace.Target{Key: flagName}},
			{
				name: "returns an error for a code at a position without a line",
				give: workspace.Target{Code: warmWarning, At: position.Pos{File: sealedSource}},
			},
			{name: "returns an error for a path at a position", give: workspace.Target{Path: ungenerated, At: rowAt}},
			{
				name: "returns an error for a key that the composition does not register",
				give: workspace.Target{Key: ghostKey, At: rowAt},
			},
		}
		for _, tt := range refused {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				w, _, _ := explained(t)
				_, err := w.Explain(t.Context(), tt.give)
				assert.HasError(t, err, "the target is refused")
				expect.That(t, err).ErrorIsNot(workspace.ErrNoGeneration, "before the ledger is read")
			})
		}

		for _, tt := range damageCases() {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				w, mem, path := explained(t)
				rows := rowsOf(t, mem, path)
				forgeRow(t, mem, tt.table, tt.key(rows))
				_, err := w.Explain(t.Context(), tt.target(rows))
				assert.ErrorIs(t, err, state.ErrDamaged, "the explanation meets the damage")
			})
		}
	})
}

// damageCases returns the row of each table that an explanation reads, and
// the target whose explanation meets the row when it does not decode.
func damageCases() []damageCase {
	byPath := func(r explainedRows) workspace.Target { return workspace.Target{Path: r.path} }
	byIdentity := func(explainedRows) workspace.Target { return workspace.Target{Identity: rowID} }
	byFact := func(explainedRows) workspace.Target { return workspace.Target{Key: flagName, At: rowAt} }
	byCode := func(explainedRows) workspace.Target {
		return workspace.Target{Code: workspace.UnmetContract, At: rowAt}
	}
	return []damageCase{
		{
			name:   "returns an error for the artifacts row of a path that does not decode",
			table:  state.TableArtifacts,
			key:    func(r explainedRows) []byte { return []byte(r.path) },
			target: byPath,
		},
		{
			name:   "returns an error for the groups row of a path that does not decode",
			table:  state.TableGroups,
			key:    func(r explainedRows) []byte { return r.group },
			target: byPath,
		},
		{
			name:   "returns an error for the invocations row of a contributor that does not decode",
			table:  state.TableInvocations,
			key:    func(r explainedRows) []byte { return r.contributor },
			target: byPath,
		},
		{
			name:   "returns an error for a units row that does not decode",
			table:  state.TableUnits,
			key:    func(explainedRows) []byte { return []byte(sealedSource) },
			target: byPath,
		},
		{
			name:  "returns an error for the readers row of a declaration that does not decode",
			table: state.TableReaders,
			key: func(explainedRows) []byte {
				return binary.BigEndian.AppendUint64(nil, uint64(state.DeclarationEdge(rowID)))
			},
			target: byIdentity,
		},
		{
			name:  "returns an error for the validations row of a declaration that does not decode",
			table: state.TableValidations,
			key: func(explainedRows) []byte {
				return binary.BigEndian.AppendUint64(nil, state.ValidationRef(rowID).ID)
			},
			target: byIdentity,
		},
		{
			name:   "returns an error for the checks row of a check that does not decode",
			table:  state.TableChecks,
			key:    func(explainedRows) []byte { return binary.BigEndian.AppendUint64(nil, state.CheckRef(lookupID).ID) },
			target: byIdentity,
		},
		{
			name:   "returns an error for the groups row of a group that read a declaration",
			table:  state.TableGroups,
			key:    func(r explainedRows) []byte { return r.group },
			target: byIdentity,
		},
		{
			name:   "returns an error for the readers row of a unit that does not decode",
			table:  state.TableReaders,
			key:    func(r explainedRows) []byte { return r.unit },
			target: byIdentity,
		},
		{
			name:   "returns an error for the artifacts row of a group's file that does not decode",
			table:  state.TableArtifacts,
			key:    func(r explainedRows) []byte { return []byte(r.path) },
			target: byIdentity,
		},
		{
			name:  "returns an error for the readers row of a fact that does not decode",
			table: state.TableReaders,
			key: func(explainedRows) []byte {
				return binary.BigEndian.AppendUint64(nil, uint64(state.FactEdge(rowID, flagName)))
			},
			target: byFact,
		},
		{
			name:   "returns an error for the readers row of the findings that does not decode",
			table:  state.TableReaders,
			key:    func(explainedRows) []byte { return binary.BigEndian.AppendUint64(nil, uint64(state.FindingsEdge)) },
			target: byCode,
		},
		{
			name:   "returns an error for an audit row that does not decode",
			table:  state.TableAudit,
			key:    func(explainedRows) []byte { return []byte(contractKey) },
			target: byCode,
		},
	}
}

// explained runs the recording composition over the tree of the marked
// row, which the marker mirrors, and returns the workspace, its ledger and
// the path of the file of the marker.
func explained(t *testing.T) (*workspace.Workspace, *ledger.Mem, string) {
	t.Helper()

	mem := ledger.NewMem()
	var flag meta.Key[bool]
	w := built(t, phaseRecording(t, mem, &flag))
	report := sealedRun(t, w, workspace.Input{Tree: markedTree("")})
	assert.Length(t, report.Manifest.Files, 1, "the marker writes one file")
	return w, mem, report.Manifest.Files[0].Path
}

// markedTree returns the tree of the store package whose row has the
// mark, with args after the name of the mark, beside a package of one
// unmarked struct.
func markedTree(args string) fstest.MapFS {
	return fstest.MapFS{
		sealedSource: {Data: []byte("package svc/store\ntype Row int string\n" + markLine + args + "\n")},
		otherSource:  {Data: []byte("package svc/other\ntype Other int\n")},
	}
}

// kindlessRow returns the identity of the row without its kind, as
// symbol.Parse reads the spelling of the row.
func kindlessRow() symbol.Identity {
	id := rowID
	id.Kind = symbol.KindInvalid
	return id
}

// listed runs the recording composition with a check that looks the row
// up and enumerates the structs and the carriers of the mark, and returns
// the explanation of the row.
func listed(t *testing.T) *workspace.Explanation {
	t.Helper()

	lister := &recordingCheck{name: listerID, reads: []string{planName}, script: func(ctx *plugin.CheckContext) {
		ctx.Reader.Lookup(rowID)
		for range ctx.Reader.ByKind(symbol.KindStruct) {
		}
		for range ctx.Reader.ByDirective(markSchema.Canonical()) {
		}
	}}
	var flag meta.Key[bool]
	w := built(t, phaseRecording(t, ledger.NewMem(), &flag).Checks(lister))
	sealedRun(t, w, workspace.Input{Tree: markedTree("")})
	got, err := w.Explain(t.Context(), workspace.Target{Identity: rowID})
	assert.NoError(t, err, "the generation explains the row")
	return got
}

// kindsOf returns the kind of each execution of an explanation, in its
// order.
func kindsOf(e *workspace.Explanation) []workspace.ExecutionKind {
	kinds := make([]workspace.ExecutionKind, 0, len(e.Executions))
	for _, r := range e.Executions {
		kinds = append(kinds, r.Kind)
	}
	return kinds
}

// rowsOf returns the rows of the explained run that the damage cases
// break, as the live generation of a ledger records them for the file at
// path.
func rowsOf(t *testing.T, l ledger.Ledger, path string) explainedRows {
	t.Helper()

	s := phasesIn(t, l)
	a, _, err := s.Artifact(path)
	assert.NoError(t, err, "the artifact reads")
	g, _, err := s.Group(planName, a.Group)
	assert.NoError(t, err, "the group reads")
	assert.Length(t, g.Contributors, 1, "one invocation contributed")
	inv, _, err := s.Invocation(planName, g.Contributors[0])
	assert.NoError(t, err, "the contributor's record reads")
	assert.Length(t, inv.Units, 1, "the contributor placed one unit")
	return explainedRows{
		path:        path,
		group:       binary.BigEndian.AppendUint64(nil, state.GroupRef(planName, a.Group).ID),
		contributor: binary.BigEndian.AppendUint64(nil, state.InvocationRef(planName, g.Contributors[0]).ID),
		unit:        binary.BigEndian.AppendUint64(nil, uint64(state.UnitEdge(planName, inv.Units[0]))),
	}
}

// forgeTable makes live a generation over the live generation of a ledger,
// in which no row of a table decodes.
func forgeTable(t *testing.T, l ledger.Ledger, table state.Table) {
	t.Helper()

	g, err := state.Open(t.Context(), l)
	assert.NoError(t, err, "the live generation opens")
	rows, err := g.All(t.Context(), table)
	assert.NoError(t, err, "the table reads")
	assert.NotEmpty(t, rows, "the table has rows to forge")
	m, digests, err := state.ReadManifest(t.Context(), l)
	assert.NoError(t, err, "the record reads")
	c := state.NewCommit(g, digests)
	for _, r := range rows {
		c.Put(table, r.Key, []byte(undecodableRow))
	}
	_, err = c.Write(t.Context(), l, g.Header, m)
	assert.NoError(t, err, "the forged generation commits")
}

// putRow makes live a generation over the live generation of a ledger, in
// which the row of a key in a table is row.
func putRow(t *testing.T, l ledger.Ledger, table state.Table, key, row []byte) {
	t.Helper()

	g, err := state.Open(t.Context(), l)
	assert.NoError(t, err, "the live generation opens")
	m, digests, err := state.ReadManifest(t.Context(), l)
	assert.NoError(t, err, "the record reads")
	c := state.NewCommit(g, digests)
	c.Put(table, key, row)
	_, err = c.Write(t.Context(), l, g.Header, m)
	assert.NoError(t, err, "the generation with the row commits")
}

// deleteRow makes live a generation over the live generation of a ledger,
// in which the row of a key in a table is deleted.
func deleteRow(t *testing.T, l ledger.Ledger, table state.Table, key []byte) {
	t.Helper()

	g, err := state.Open(t.Context(), l)
	assert.NoError(t, err, "the live generation opens")
	m, digests, err := state.ReadManifest(t.Context(), l)
	assert.NoError(t, err, "the record reads")
	c := state.NewCommit(g, digests)
	c.Delete(table, key)
	_, err = c.Write(t.Context(), l, g.Header, m)
	assert.NoError(t, err, "the generation without the row commits")
}
