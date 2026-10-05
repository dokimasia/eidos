// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
)

// modulesAllocs is one read of the modules table of two modules: the run
// read from the ledger, its index, one list of its entries, the merged
// entries, the map of counts, and for each module the parts its key
// splits into and the strings of its language, root and path.
const modulesAllocs = 13

// The modules the module cases count: one at the workspace's root, and
// one in a nested directory.
var (
	rootModule   = plugin.Module{Lang: frontendtest.ScriptedLang, Path: "example.test/svc", Root: "."}
	nestedModule = plugin.Module{Lang: frontendtest.ScriptedLang, Path: "example.test/tools", Root: "tools"}
)

// Each module's package count records under its language, its root and
// its path, and reads back whole.
func TestModules(t *testing.T) {
	t.Parallel()

	t.Run("PhaseState", func(t *testing.T) {
		t.Parallel()

		t.Run("Modules", func(t *testing.T) {
			t.Parallel()

			t.Run("returns how many packages name each module", func(t *testing.T) {
				t.Parallel()

				counts := map[plugin.Module]int{rootModule: 3, nestedModule: 1}
				g := recordedModules(t, ledger.NewMem(), counts)
				got, err := g.Phases(t.Context()).Modules()
				assert.NoError(t, err, "the modules table reads")
				assert.Equal(t, got, counts, "each module with its count")
			})

			t.Run("deletes a module no package names any more", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				recordedModules(t, l, map[plugin.Module]int{rootModule: 3, nestedModule: 1})
				g := recordedModules(t, l, map[plugin.Module]int{rootModule: 4})
				got, err := g.Phases(t.Context()).Modules()
				assert.NoError(t, err, "the modules table reads")
				assert.Equal(t, got, map[plugin.Module]int{rootModule: 4}, "the nested module is gone")
			})

			tests := []struct {
				name string
				row  state.Row
			}{
				{
					name: "returns ErrDamaged for a key of two parts",
					row:  state.Row{Key: []byte("scripted\x00."), Value: []byte{1}},
				},
				{
					name: "returns ErrDamaged for a count followed by more bytes",
					row:  state.Row{Key: []byte("scripted\x00.\x00example.test/svc"), Value: []byte{1, 2}},
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					g := putRows(t, state.TableModules, tt.row)
					_, err := g.Phases(t.Context()).Modules()
					assert.ErrorIs(t, err, state.ErrDamaged, "the row does not spell a count")
				})
			}

			t.Run("returns ErrDamaged for a table that does not read whole", func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				g := recordedModules(t, l, map[plugin.Module]int{rootModule: 3})
				damageRun(t, l, func(int) int { return 3 })
				_, err := g.Phases(t.Context()).Modules()
				assert.ErrorIs(t, err, state.ErrDamaged, "the damaged block is found")
			})
		})
	})
}

// A read of the modules' counts allocates within its ceiling in the
// ordinary run, which runs no benchmark. The check runs alone, because
// AllocsPerRun counts every goroutine's allocations and refuses to run
// beside parallel tests.
func TestModulesAllocs(t *testing.T) {
	s := recordedModules(t, ledger.NewMem(), map[plugin.Module]int{rootModule: 3, nestedModule: 1}).Phases(t.Context())
	assert.MaxAllocs(t, func() {
		if got, err := s.Modules(); err != nil || len(got) != 2 {
			t.Fatalf("Modules: %d modules, error %v", len(got), err)
		}
	}, modulesAllocs, "Modules allocates the table it reads and the counts")
}

// BenchmarkModules measures a read of the counts of a generation that
// records two modules.
func BenchmarkModules(b *testing.B) {
	s := recordedModules(b, ledger.NewMem(), map[plugin.Module]int{rootModule: 3, nestedModule: 1}).Phases(b.Context())

	b.Run("PhaseState", func(b *testing.B) {
		b.Run("Modules", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(modulesAllocs)
			defer c.End()
			var (
				got map[plugin.Module]int
				err error
			)
			for c.Loop() {
				got, err = s.Modules()
			}
			assert.NoError(b, err, "the modules table reads")
			assert.Length(b, got, 2, "Modules returns both modules")
		})
	})
}

// recordedModules records a run of module counts over the ledger's live
// generation where it has one, and returns the generation the commit made
// live.
func recordedModules(tb testing.TB, l ledger.Ledger, counts map[plugin.Module]int) *state.Generation {
	tb.Helper()

	parent, err := state.Open(tb.Context(), l)
	if err != nil {
		parent = nil
	}
	p, err := state.RecordPhases(tb.Context(), parent, &state.Recorder{}, state.PhaseRun{
		Facts: meta.NewFacts(meta.NewRegistry()), Modules: counts,
	})
	assert.NoError(tb, err, "the phases record")
	c := state.NewCommit(parent, nil)
	p.Commit(c, nil)
	_, err = c.Write(tb.Context(), l, header(past), manifest.Manifest{Version: manifest.Version})
	assert.NoError(tb, err, "the commit writes")
	g, err := state.Open(tb.Context(), l)
	assert.NoError(tb, err, "and its generation opens")
	return g
}
