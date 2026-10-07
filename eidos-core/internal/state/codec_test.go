// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"testing"
	"time"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The record format writes every value the kernel stores, so each type
// of the fact vocabulary and each finding returns whole from a trip.
func TestCodec(t *testing.T) {
	t.Parallel()

	t.Run("value", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give any
		}{
			{name: "round-trips a string", give: "yes"},
			{name: "round-trips an integer", give: int64(-42)},
			{name: "round-trips a bool", give: true},
			{name: "round-trips a list of strings", give: []string{"a", "", "a"}},
			{name: "round-trips an identity", give: coretest.Struct(coretest.CachePath, "Cache").ID},
			{name: "round-trips no value", give: nil},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.RoundTrip(t, func(r *store.Region) ([]byte, error) { return state.AppendRegion(nil, r) },
					state.DecodeRegion, stamped(tt.give), "the value decodes to the value that encoded it")
			})
		}

		t.Run("returns an error for a value outside the fact vocabulary", func(t *testing.T) {
			t.Parallel()

			_, err := state.AppendRegion(nil, stamped(uint8(1)))
			assert.HasError(t, err, "a byte is no fact value")
		})
	})

	t.Run("finding", func(t *testing.T) {
		t.Parallel()

		t.Run("round-trips a finding with its related positions", func(t *testing.T) {
			t.Parallel()

			at := position.Pos{File: "a.zz", Line: 2, Col: 4}
			r := &store.Region{Findings: []diag.Diag{{
				Code: diag.Code{Prefix: "PLG", Number: 7}, Severity: diag.SeverityError, Pos: at,
				Msg: "broken", Origin: "plug", Related: []position.Pos{at, {File: "b.zz"}},
			}}}
			assert.RoundTrip(t, func(r *store.Region) ([]byte, error) { return state.AppendRegion(nil, r) },
				state.DecodeRegion, r, "the finding decodes to the finding that encoded it")
		})
	})

	t.Run("instant", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give time.Time
		}{
			{name: "round-trips the zero instant", give: time.Time{}},
			{name: "round-trips the epoch", give: time.Unix(0, 0)},
			{name: "round-trips an instant to the nanosecond", give: time.Unix(1_700_000_000, 123)},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				l := ledger.NewMem()
				_, err := state.NewCommit(nil, nil).Write(t.Context(), l, header(tt.give),
					manifest.Manifest{Version: manifest.Version})
				assert.NoError(t, err, "the commit writes")
				g, err := state.Open(t.Context(), l)
				assert.NoError(t, err, "and opens")
				assert.True(t, g.Header.Anchor.Equal(tt.give), "the anchor returns whole")
				assert.Equal(t, g.Header.Anchor.IsZero(), tt.give.IsZero(), "the zero instant reads back as zero")
			})
		}
	})
}

// stamped returns a region whose one subject has one stamp of a value.
func stamped(v any) *store.Region {
	s := coretest.Struct(coretest.StorePath, "Store")
	return &store.Region{
		Packages: []*node.Package{coretest.Package(coretest.StorePath, s)},
		Stamps:   map[symbol.Identity][]meta.RawStamp{s.ID: {{Key: "fake.value", Value: v}}},
	}
}
