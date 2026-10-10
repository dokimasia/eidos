// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
)

// A table is its runs, newest deciding, so a row written later always
// reads over a row written before it.
func TestTable(t *testing.T) {
	t.Parallel()

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give state.Table
			want string
		}{
			{name: "spells the first table", give: state.TableFiles, want: "files"},
			{name: "spells the last table", give: state.TableChecks, want: "checks"},
			{name: "spells a table outside the set by its number", give: state.Table(40), want: "Table(40)"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.String(), tt.want, "the table's spelling")
			})
		}
	})

	t.Run("get", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the newest run's row of a key", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			first := committed(t, l, past, "alpha")
			c := state.NewCommit(first, nil)
			c.Put(state.TableChecks, []byte("alpha"), []byte("newer"))
			_, err := c.Write(t.Context(), l, header(past), manifest.Manifest{Version: manifest.Version})
			assert.NoError(t, err, "the second commit writes")
			g, err := state.Open(t.Context(), l)
			assert.NoError(t, err, "and opens")
			row, held, err := g.Get(t.Context(), state.TableChecks, []byte("alpha"))
			assert.NoError(t, err, "the row reads")
			assert.True(t, held, "the key has a row")
			assert.Equal(t, string(row), "newer", "the newer run decides")
		})
	})

	t.Run("all", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the newest row of each key once", func(t *testing.T) {
			t.Parallel()

			l := ledger.NewMem()
			first := committed(t, l, past, "alpha", "beta")
			c := state.NewCommit(first, nil)
			c.Put(state.TableChecks, []byte("beta"), []byte("newer"))
			_, err := c.Write(t.Context(), l, header(past), manifest.Manifest{Version: manifest.Version})
			assert.NoError(t, err, "the second commit writes")
			g, err := state.Open(t.Context(), l)
			assert.NoError(t, err, "and opens")
			rows, err := g.All(t.Context(), state.TableChecks)
			assert.NoError(t, err, "the table reads")
			assert.Equal(t, rows, []state.Row{
				{Key: []byte("alpha"), Value: []byte("row of alpha")},
				{Key: []byte("beta"), Value: []byte("newer")},
			}, "each key once, the newer run deciding")
		})
	})
}

// A declared table spells without allocating in the ordinary run, which
// runs no benchmark.
func TestTableAllocs(t *testing.T) {
	table := state.TableChecks
	var got string
	assert.MaxAllocs(t, func() { got = table.String() }, 0, "String allocates nothing for a declared table")
	assert.Equal(t, got, "checks", "String spells TableChecks")
}

// BenchmarkTable measures the spelling of a declared table.
func BenchmarkTable(b *testing.B) {
	b.Run("String", func(b *testing.B) {
		table := state.TableChecks
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = table.String()
		}
		assert.Equal(b, got, "checks", "String spells TableChecks")
	})
}
