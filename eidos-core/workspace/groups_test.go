// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// The files that the plans of the group cases write into the directory
// of the store package, the family of the mirror, the family of the echo
// and the family of the suites.
const (
	mirroredFile = "svc/store/gen.txt"
	echoedFile   = "svc/store/echo.txt"
	suiteFile    = "svc/store/suite.txt"
)

// The second mirror, the echo and the slotter of the group cases. The
// twin mirrors into the family file of the mirror. The emit-phase rule
// of the echo echoes each mirrored struct into a family of its own. The
// emit-phase rule of the slotter appends a field into each mirrored
// struct.
const (
	twinID    plugin.ID = "twin"
	echoID    plugin.ID = "echo"
	slotterID plugin.ID = "slotter"
)

// echoPriority places the echo after the generators of the default
// priority, so its emit-phase rule reads their units.
const echoPriority = 10

// peerName is the name of the type of the field of the referring
// mirror. No file declares it.
const peerName = "Peer"

// TestGroups tests the record of the groups. A recording run records
// each committed file of a plan, the group that generates it, and the
// names that it declares.
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

		t.Run("records the declaration edge of the package of each origin among a group's reads", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			sealedRun(t, sealing(t, mem, "plan"), workspace.Input{Tree: sealedTree()})
			assert.Contains(t, groupOf(t, mem, mirroredFile).Reads, state.DeclarationEdge(rowID.PackageIdentity()),
				"the layout reads the out directive on the package of the row")
		})

		t.Run("records no declaration edge for a declaration without an origin", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			free := generator(mirrorID, func(_ *eidos.StructMatch, e *eidos.Emitter) error {
				e.PackageFile().Append(&emit.Struct{Name: "Free"})
				return nil
			})
			sealedRun(t, built(t, sealingOf(t, mem, "plan", free)), workspace.Input{Tree: sealedTree()})
			assert.NotContains(t, groupOf(t, mem, mirroredFile).Reads, state.DeclarationEdge(symbol.Identity{}),
				"a declaration without an origin reads no declaration")
		})

		t.Run("records the declaration edge of the source file of each file among a group's reads", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			sourced := eidos.NewPlugin(mirrorID).
				Output(plugin.Output{Per: plugin.PerSource, Word: "gen"}).
				Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
					e.File().Append(&emit.Struct{Origin: m.Struct.Identity(), Name: "For" + m.Struct.Name})
					return nil
				})).Generator()
			sealedRun(t, built(t, sealingOf(t, mem, "plan", sourced)), workspace.Input{Tree: sealedTree()})
			source := symbol.Identity{
				Lang: rowID.Lang, Package: rowID.Package, Name: sealedSource, Kind: symbol.KindFile,
			}
			assert.Contains(t, groupOf(t, mem, mirroredFile).Reads, state.DeclarationEdge(source),
				"the backend writes the annotations of the source file into the file")
		})

		t.Run("records the declaration edge of each package whose declared name a file read among a group's reads",
			func(t *testing.T) {
				t.Parallel()

				mem := ledger.NewMem()
				w := built(t, clauseBuilder(mem, frontendtest.NewScripted(), func() (output.Sink, error) {
					return output.NewMem(), nil
				}))
				sealedRun(t, w, workspace.Input{Tree: clauseTree(cacheClause)})
				assert.Contains(t, groupOf(t, mem, mirroredFile).Reads, state.DeclarationEdge(cachePackage),
					"the backend writes the name that the cache package declares into the file")
			})

		t.Run("records the route of the unit of a file", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			sealedRun(t, sealing(t, mem, "plan"), workspace.Input{Tree: sealedTree()})
			g := groupOf(t, mem, mirroredFile)
			assert.Equal(t, g.Routes, []state.FileRoute{{File: mirroredFile, Unit: g.Key, Route: plugin.Route{
				Family: plugin.RouteHandler, Policy: plugin.RouteDefault,
				Dir: plugin.RouteSource, Name: plugin.RouteTarget,
			}}}, "the layout placed the unit of the mirror beside its source")
		})

		t.Run("records a route for each unit of a file that two units write", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			twin := generator(twinID, func(m *eidos.StructMatch, e *eidos.Emitter) error {
				e.PackageFile().Append(&emit.Struct{Origin: m.Struct.Identity(), Name: "Twin" + m.Struct.Name})
				return nil
			})
			w := built(t, sealingPlans(mem, workspace.Plan{
				Name: "plan", Generators: []plugin.Generator{twin, mirror(mirrorID)}, Backend: fixturePrinter(),
			}))
			sealedRun(t, w, workspace.Input{Tree: sealedTree()})
			g := groupOf(t, mem, mirroredFile)
			assert.Length(t, g.Routes, 2, "each unit of the file has a route")
			expect.Equal(t, g.Routes[0].Unit, g.Units[0], "the routes are in the order of the units")
			expect.Equal(t, g.Routes[1].Unit, g.Units[1], "the second route is the route of the second unit")
		})

		t.Run("records the name edge of each name that the settle looked up among a group's reads", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			w := built(t, sealingPlans(mem, workspace.Plan{
				Name: "plan", Generators: []plugin.Generator{referring()}, Backend: fixturePrinter(),
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

		t.Run("records no scope of the name of a record among a group's reads", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			sealedRun(t, built(t, sealingOf(t, mem, "plan", suites())), workspace.Input{Tree: roundsTree(rowLine, "")})
			assert.NotContains(t, groupOf(t, mem, suiteFile).Reads, state.ScopeEdge("plan", storePackage.Package, ""),
				"a record does not collide with the names of its scope")
		})

		t.Run("records two units that write one file in one group", func(t *testing.T) {
			t.Parallel()

			mem := ledger.NewMem()
			twin := generator(twinID, func(m *eidos.StructMatch, e *eidos.Emitter) error {
				e.PackageFile().Append(&emit.Struct{Origin: m.Struct.Identity(), Name: "Twin" + m.Struct.Name})
				return nil
			})
			w := built(t, sealingPlans(mem, workspace.Plan{
				Name: "plan", Generators: []plugin.Generator{mirror(mirrorID), twin}, Backend: fixturePrinter(),
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
					Backend: fixturePrinter(),
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
					Backend: fixturePrinter(),
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
// earlier turn into a family of its own. Its priority places it after a
// generator of the default priority.
func echo(tb assert.TB) plugin.Generator {
	tb.Helper()

	return eidos.NewPlugin(echoID).
		Priority(plugin.RoleGenerator, echoPriority).
		Output(plugin.Output{Per: plugin.PerPackage, Word: "echo"}).
		Handle(eidos.OnEmit(symbol.KindStruct, func(m *eidos.EmitMatch, e *eidos.Emitter) error {
			e.PackageFile().Append(&emit.Struct{Origin: m.Origin(), Name: "Echo" + m.Origin().Name})
			return nil
		})).Generator()
}

// slotter returns a generator whose emit-phase rule appends a field into
// each struct of an earlier turn. Its priority places it after a
// generator of the default priority.
func slotter(tb assert.TB) plugin.Generator {
	tb.Helper()

	return eidos.NewPlugin(slotterID).
		Priority(plugin.RoleGenerator, echoPriority).
		Handle(eidos.OnEmit(symbol.KindStruct, func(m *eidos.EmitMatch, e *eidos.Emitter) error {
			if s, is := m.Value.(*emit.Struct); is {
				e.Slot(&s.Fields).Append(&emit.Field{Name: "slotted", Type: &emit.TypeRef{Spelling: "int"}})
			}
			return nil
		})).Generator()
}

// referring returns a mirror whose struct has a field of the type
// peerName, so the settle looks the name up in the package of the
// struct.
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
