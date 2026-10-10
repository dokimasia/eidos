// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load_test

import (
	"sync"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

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
			assert.Length(t, memo, len(report.Units), "one entry for each unit")
			for _, u := range report.Units {
				expect.Equal(t, memo[string(u.Key)], u.Region, "the entry is the unit's region", assert.ByIdentity())
			}
		})

		t.Run("restores every unit whose key the memo records", func(t *testing.T) {
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

		t.Run("parses a unit of an importing language whose key the memo records", func(t *testing.T) {
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

		t.Run("reports a unit of an importing language as not restorable", func(t *testing.T) {
			t.Parallel()

			_, report, _ := loadTree(t, stdTree(), with(&importing{frontendtest.NewScripted()}))
			assert.NotEmpty(t, report.Units, "the tree loads units")
			for _, u := range report.Units {
				expect.False(t, u.Restorable, "a memo cannot restore a unit whose references need its parse")
			}
		})

		t.Run("reports a unit of a language without the importer role as restorable", func(t *testing.T) {
			t.Parallel()

			_, report, _ := loadTree(t, stdTree())
			assert.NotEmpty(t, report.Units, "the tree loads units")
			for _, u := range report.Units {
				expect.True(t, u.Restorable, "a memo can restore the unit")
			}
		})
	})
}

// A declared source spells without allocating, and a value outside the
// set allocates its spelling, in the ordinary run, which runs no
// benchmark.
func TestPriorAllocs(t *testing.T) {
	from, outside := load.FromMemo, load.From(9)
	var got string
	assert.MaxAllocs(t, func() { got = from.String() }, 0, "String allocates nothing for a declared source")
	assert.Equal(t, got, "memo", "String spells FromMemo")
	assert.MaxAllocs(t, func() { got = outside.String() }, 1,
		"String allocates the spelling of a value outside the set")
	assert.Equal(t, got, "From(9)", "String spells the value's number")
}

// BenchmarkPrior measures the spelling of where a unit's region came
// from.
func BenchmarkPrior(b *testing.B) {
	b.Run("String", func(b *testing.B) {
		b.Run("a value of the set", func(b *testing.B) {
			from := load.FromMemo
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var got string
			for c.Loop() {
				got = from.String()
			}
			assert.Equal(b, got, "memo", "String spells FromMemo")
		})

		b.Run("a value outside the set", func(b *testing.B) {
			// The number is below 100, so strconv returns a constant string
			// and the concatenation is the one allocation.
			outside := load.From(9)
			c := bench.Start(b).MaxAllocs(1)
			defer c.End()
			var got string
			for c.Loop() {
				got = outside.String()
			}
			assert.Equal(b, got, "From(9)", "String spells the value's number")
		})
	})
}
