// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load_test

import (
	"sync"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/store"
)

// memoRecorder is a parse memo that misses every lookup and records
// every region put into it, by key.
type memoRecorder map[string]*store.Region

// Get misses.
func (memoRecorder) Get([]byte) (*store.Region, bool) { return nil, false }

// Put records the region under its key.
func (m memoRecorder) Put(key []byte, r *store.Region) { m[string(key)] = r }

// remembered is a parse memo that keeps each region's encoding under its
// key, as the sealed state's memo does, and decodes a fresh region on
// each hit, which it counts.
type remembered struct {
	mu      sync.Mutex
	entries map[string][]byte
	hits    int
}

// newRemembered returns an empty memo.
func newRemembered() *remembered { return &remembered{entries: map[string][]byte{}} }

// Get decodes the region recorded under a key, and misses a key it
// records nothing under.
func (m *remembered) Get(key []byte) (*store.Region, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, held := m.entries[string(key)]
	if !held {
		return nil, false
	}
	r, err := state.DecodeRegion(b)
	if err != nil {
		return nil, false
	}
	m.hits++
	return r, true
}

// Put records a region's encoding under its key.
func (m *remembered) Put(key []byte, r *store.Region) {
	b, err := state.AppendRegion(nil, r)
	if err != nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[string(key)] = b
}

// A report names where each unit's region came from, so the spellings
// are what a reader of the report sees.
func TestPrior(t *testing.T) {
	t.Parallel()

	t.Run("String", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give load.From
			want string
		}{
			{name: "spells a parsed unit parse", give: load.FromParse, want: "parse"},
			{name: "spells a memo hit memo", give: load.FromMemo, want: "memo"},
			{name: "spells a kept unit generation", give: load.FromGeneration, want: "generation"},
			{name: "spells a value outside the set by its number", give: load.From(9), want: "From(9)"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.give.String(), tt.want, "the spelling of the source")
			})
		}
	})

	t.Run("Load", func(t *testing.T) {
		t.Parallel()

		t.Run("reports a cold load's units parsed", func(t *testing.T) {
			t.Parallel()

			_, report, _ := loadTree(t, stdTree())
			for _, u := range report.Units {
				assert.Equal(t, u.From, load.FromParse, "a cold load parses every unit")
				assert.NotNil(t, u.Region, "and reports the region it built")
			}
		})

		t.Run("records every parsed unit in the memo under its key", func(t *testing.T) {
			t.Parallel()

			memo := memoRecorder{}
			_, report, _ := loadTree(t, stdTree(), func(cfg *load.Config) { cfg.Memo = memo })
			assert.Equal(t, len(memo), len(report.Units), "one entry for each unit")
			for _, u := range report.Units {
				assert.True(t, memo[string(u.Key)] == u.Region, "the entry is the unit's region")
			}
		})

		t.Run("restores every unit the memo holds the key of", func(t *testing.T) {
			t.Parallel()

			memo := newRemembered()
			remember := func(cfg *load.Config) { cfg.Memo = memo }
			loadOf(t, stdTree(), remember)
			restored := loadOf(t, stdTree(), remember)
			for _, u := range restored.report.Units {
				assert.Equal(t, u.From, load.FromMemo, "each unit restores without a parse")
			}
			assertSameLoad(t, restored, loadOf(t, stdTree()))
		})

		t.Run("parses a unit of an importing language the memo holds the key of", func(t *testing.T) {
			t.Parallel()

			memo := newRemembered()
			importer := with(&importing{frontendtest.NewScripted()})
			remember := func(cfg *load.Config) { cfg.Memo = memo }
			loadOf(t, stdTree(), importer, remember)
			again := loadOf(t, stdTree(), importer, remember)
			assert.Equal(t, memo.hits, 0, "the memo is not asked")
			for _, u := range again.report.Units {
				assert.Equal(t, u.From, load.FromParse, "each unit parses for its imports")
			}
		})
	})
}
