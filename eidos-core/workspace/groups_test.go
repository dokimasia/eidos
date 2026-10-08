// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// The files the group cases' plans write into the store package's
// directory: the mirror's family, and the echo's.
const (
	mirroredFile = "svc/store/gen.txt"
	echoedFile   = "svc/store/echo.txt"
)

// The second mirror, the echo and the slotter of the group cases: a
// generator that mirrors into the mirror's family file, a generator whose
// emit-phase rule echoes each mirrored struct into a family of its own,
// and a generator whose emit-phase rule appends a field into each
// mirrored struct.
const (
	twinID    plugin.ID = "twin"
	echoID    plugin.ID = "echo"
	slotterID plugin.ID = "slotter"
)

// echoPriority places the echo after the generators of the default
// priority, so its emit-phase rule reads their units.
const echoPriority = 10

// peerName is the type that the referring mirror's field names, which no
// file declares.
const peerName = "Peer"

// A recording run records each committed file of a plan, the group that
// generates it, and the names it declares.
func TestGroups(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("records the artifact of a committed file under the plan", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			report := sealedRun(t, sealing(t, mem, "plan"), workspace.Input{Tree: sealedTree()})
			got, held, err := phasesIn(t, mem).Artifact(mirroredFile)
			assert.NoError(t, err, "the artifacts table reads")
			assert.True(t, held, "the committed file is recorded")
			assert.Equal(t, got.Entry, report.Manifest.Files[0], "the artifact states the file's manifest entry")
		})

		t.Run("records the group of a file under the key its artifact names", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			sealedRun(t, sealing(t, mem, "plan"), workspace.Input{Tree: sealedTree()})
			g := groupOf(t, mem, mirroredFile)
			expect.Equal(t, g.Files, []string{mirroredFile}, "the group generates the file")
			expect.Length(t, g.Units, 1, "from the mirror's one unit")
			expect.Equal(t, g.Contributors, []plugin.MatchKey{{Plugin: mirrorID, Subject: rowID}},
				"which the mirror's invocation on the row placed into")
		})

		t.Run("lists the group among the readers of its unit's edge", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			sealedRun(t, sealing(t, mem, "plan"), workspace.Input{Tree: sealedTree()})
			g := groupOf(t, mem, mirroredFile)
			readers, err := phasesIn(t, mem).Readers(state.UnitEdge("plan", g.Key))
			assert.NoError(t, err, "the readers table reads")
			assert.Equal(t, readers, []state.RecordRef{state.GroupRef("plan", g.Key)}, "the group reads its unit")
		})

		t.Run("records the declaration edge of each origin among a group's reads", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			sealedRun(t, sealing(t, mem, "plan"), workspace.Input{Tree: sealedTree()})
			assert.Contains(t, groupOf(t, mem, mirroredFile).Reads, state.DeclarationEdge(rowID),
				"the layout reads the row's directives and position")
		})

		t.Run("records the name edge of each name that the settle looked up among a group's reads", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			w := built(t, sealingPlans(mem, workspace.Plan{
				Name: "plan", Generators: []plugin.Generator{referring()}, Backend: printer(t, "fixture"),
			}))
			sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			peer := plugin.NameKey{Package: storePackage.Package, Emitted: peerName}
			assert.Contains(t, groupOf(t, mem, mirroredFile).Reads, state.NameEdge("plan", peer),
				"the settle looks the field's type up in the store package")
		})

		t.Run("records the scope of each declared name among a group's reads", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			sealedRun(t, sealing(t, mem, "plan"), workspace.Input{Tree: sealedTree()})
			assert.Contains(t, groupOf(t, mem, mirroredFile).Reads, state.ScopeEdge("plan", storePackage.Package, ""),
				"the file declares a name in the store package")
		})

		t.Run("records two units that write one file in one group", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			w := built(t, sealingPlans(mem, workspace.Plan{
				Name: "plan", Generators: []plugin.Generator{mirror(mirrorID), mirror(twinID)},
				Backend: printer(t, "fixture"),
			}))
			sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			g := groupOf(t, mem, mirroredFile)
			expect.Length(t, g.Units, 2, "both generators placed a unit into the file")
			expect.Equal(t, g.Files, []string{mirroredFile}, "the group generates the one file")
		})

		t.Run("records the unit an emit-phase invocation matched in the group of the unit it placed into",
			func(t *testing.T) {
				t.Parallel()

				mem := ledger.NewMem()
				w := built(t, sealingPlans(mem, workspace.Plan{
					Name: "plan", Generators: []plugin.Generator{mirror(mirrorID), echo(t)},
					Backend: printer(t, "fixture"),
				}))
				sealedRun(t, w, workspace.Input{Tree: sealedTree()})
				mirrored, echoed := groupOf(t, mem, mirroredFile), groupOf(t, mem, echoedFile)
				expect.Equal(t, echoed.Key, mirrored.Key, "one group generates both files")
				expect.Equal(t, mirrored.Files, []string{echoedFile, mirroredFile}, "and lists both")
			})

		t.Run("lists the invocation that appended into a struct's slot among the contributors of its group",
			func(t *testing.T) {
				t.Parallel()

				mem := ledger.NewMem()
				w := built(t, sealingPlans(mem, workspace.Plan{
					Name: "plan", Generators: []plugin.Generator{mirror(mirrorID), slotter(t)},
					Backend: printer(t, "fixture"),
				}))
				sealedRun(t, w, workspace.Input{Tree: sealedTree()})
				g := groupOf(t, mem, mirroredFile)
				slotted := plugin.MatchKey{Plugin: slotterID, Subject: rowID, Host: plugin.EmitRef{Unit: g.Key}}
				assert.Contains(t, g.Contributors, slotted, "the slotter appended a field into the mirrored row")
			})

		t.Run("records the name that a file declares", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			sealedRun(t, sealing(t, mem, "plan"), workspace.Input{Tree: sealedTree()})
			got, held := phasesIn(t, mem).Names("plan", nil).InPackage(storePackage.Package, "ForRow")
			assert.True(t, held, "the mirrored struct's name is recorded")
			assert.Equal(t, got.File, mirroredFile, "under the file that declares it")
		})

		t.Run("records the export rows of a file of a plan that a check reads", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			flag := &meta.Key[bool]{}
			marked := sealedTree()
			marked[sealedSource].Data = append(marked[sealedSource].Data, []byte(markLine+"\n")...)
			sealedRun(t, built(t, phaseRecording(t, mem, flag)), workspace.Input{Tree: marked})
			got, held, err := phasesIn(t, mem).Artifact(mirroredFile)
			assert.NoError(t, err, "the artifacts table reads")
			assert.True(t, held, "the marked row's file is recorded")
			assert.Length(t, got.Export, 1, "the file exports the mirrored struct")
		})
	})
}

// echo returns a generator whose emit-phase rule echoes each struct of an
// earlier bucket into a family of its own. Its priority places it after a
// generator of the default priority.
func echo(tb assert.TB) plugin.Generator {
	tb.Helper()

	g, held := eidos.NewPlugin(echoID).
		Priority(plugin.RoleGenerator, echoPriority).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "echo"}).
		Handle(eidos.OnEmit(symbol.KindStruct, func(m *eidos.EmitMatch, e *eidos.Emitter) error {
			e.PackageFile().Append(&emit.Struct{Origin: m.Origin(), Name: "Echo" + m.Origin().Name})
			return nil
		})).Build().(plugin.Generator)
	assert.True(tb, held, "the echo lowers to the generator role")
	return g
}

// slotter returns a generator whose emit-phase rule appends a field into
// each struct of an earlier bucket. Its priority places it after a
// generator of the default priority.
func slotter(tb assert.TB) plugin.Generator {
	tb.Helper()

	g, held := eidos.NewPlugin(slotterID).
		Priority(plugin.RoleGenerator, echoPriority).
		Handle(eidos.OnEmit(symbol.KindStruct, func(m *eidos.EmitMatch, e *eidos.Emitter) error {
			if s, is := m.Value.(*emit.Struct); is {
				e.Slot(&s.Fields).Append(&emit.Field{Name: "slotted", Type: &emit.TypeRef{Spelling: "int"}})
			}
			return nil
		})).Build().(plugin.Generator)
	assert.True(tb, held, "the slotter lowers to the generator role")
	return g
}

// referring returns a mirror whose struct has a field of the type that
// peerName spells, so the settle looks the name up in the struct's
// package.
func referring() plugin.Generator {
	return generator(mirrorID, func(m *eidos.StructMatch, e *eidos.Emitter) error {
		s := &emit.Struct{Origin: m.Struct.Identity(), Name: "For" + m.Struct.Name}
		s.Fields.Append(&emit.Field{Name: "peer", Type: &emit.TypeRef{Spelling: peerName}})
		e.PackageFile().Append(s)
		return nil
	})
}

// groupOf returns the record of the group that generates a file of the
// plan named plan in a ledger's live generation.
func groupOf(t *testing.T, l ledger.Ledger, file string) state.Group {
	t.Helper()

	s := phasesIn(t, l)
	a, held, err := s.Artifact(file)
	assert.NoError(t, err, "the artifacts table reads")
	assert.True(t, held, "the file is recorded")
	g, held, err := s.Group("plan", a.Group)
	assert.NoError(t, err, "the groups table reads")
	assert.True(t, held, "the file's group is recorded")
	return g
}
