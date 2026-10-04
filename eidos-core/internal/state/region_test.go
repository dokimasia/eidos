// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/internal/wire"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// A region crosses runs and workspaces as bytes, so its encoding
// returns every part whole and refuses every damaged blob.
func TestRegion(t *testing.T) {
	t.Parallel()

	t.Run("AppendRegion", func(t *testing.T) {
		t.Parallel()

		t.Run("encodes one region to the same bytes twice", func(t *testing.T) {
			t.Parallel()

			one, err := state.AppendRegion(nil, fullRegion())
			assert.NoError(t, err, "the region encodes")
			two, err := state.AppendRegion(nil, fullRegion())
			assert.NoError(t, err, "and again")
			assert.True(t, bytes.Equal(one, two), "the maps encode in identity order, so the bytes agree")
		})

		t.Run("returns an error for a stamp value outside the fact vocabulary", func(t *testing.T) {
			t.Parallel()

			r := fullRegion()
			s := coretest.Struct(coretest.StorePath, "Store")
			r.Stamps[s.ID] = []meta.RawStamp{{Key: "fake.ratio", Value: 0.5}}
			dst := []byte{7}
			got, err := state.AppendRegion(dst, r)
			assert.HasError(t, err, "a float is no fact value")
			assert.Equal(t, got, []byte{7}, "and dst comes back unchanged")
		})

		t.Run("returns an error for a symbol the model does not declare", func(t *testing.T) {
			t.Parallel()

			r := fullRegion()
			r.Packages[0].Files[0].Decls = append(r.Packages[0].Files[0].Decls,
				coretest.Foreign(coretest.StorePath, "Alien", symbol.KindStruct))
			_, err := state.AppendRegion(nil, r)
			assert.HasError(t, err, "a foreign symbol has no encoding")
		})
	})

	t.Run("DecodeRegion", func(t *testing.T) {
		t.Parallel()

		t.Run("returns every part of the region it encodes", func(t *testing.T) {
			t.Parallel()

			b, err := state.AppendRegion(nil, fullRegion())
			assert.NoError(t, err, "the region encodes")
			got, err := state.DecodeRegion(b)
			assert.NoError(t, err, "and decodes")
			assert.Equal(t, got, fullRegion(), "to the region that encoded it")
		})

		t.Run("returns an empty region for an empty region's blob", func(t *testing.T) {
			t.Parallel()

			b, err := state.AppendRegion(nil, &store.Region{})
			assert.NoError(t, err, "the empty region encodes")
			got, err := state.DecodeRegion(b)
			assert.NoError(t, err, "and decodes")
			assert.Equal(t, got, &store.Region{}, "to nothing")
		})

		blob, err := state.AppendRegion(nil, fullRegion())
		assert.NoError(t, err, "the region encodes")
		flipped := bytes.Clone(blob)
		flipped[len(flipped)/2] ^= 0xff
		tests := []struct {
			name string
			give []byte
		}{
			{name: "returns ErrDamaged for a blob shorter than its trailer", give: blob[:2]},
			{name: "returns ErrDamaged for a blob whose CRC-32C fails", give: flipped},
			{name: "returns ErrDamaged for a blob cut short", give: resealed(blob[:len(blob)/2])},
			{name: "returns ErrDamaged for bytes after the body", give: resealed(append(body(blob), 0))},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := state.DecodeRegion(tt.give)
				assert.ErrorIs(t, err, state.ErrDamaged, "a damaged region does not decode")
			})
		}

		t.Run("returns ErrDamaged wrapping ErrMalformed for a table that does not decode", func(t *testing.T) {
			t.Parallel()

			_, err := state.DecodeRegion(resealed([]byte{0xff, 0xff}))
			assert.ErrorIs(t, err, state.ErrDamaged, "the region is damaged")
		})

		t.Run("returns ErrDamaged wrapping ErrMalformed for a body that does not decode", func(t *testing.T) {
			t.Parallel()

			table, err := (&node.StringTable{}).AppendBinary(nil)
			assert.NoError(t, err, "an empty table encodes")
			_, err = state.DecodeRegion(resealed(append(table, 0xff)))
			assert.ErrorIs(t, err, state.ErrDamaged, "the region is damaged")
			assert.ErrorIs(t, err, wire.ErrMalformed, "because its body is malformed")
		})
	})
}

// TestRegionAllocs checks the allocation contract of the region codec:
// an encoding into a buffer with room allocates nothing once the pool
// contains its scratch, and a decode of one canonical package allocates
// its table, its region, its package list and each declaration and
// list of the package once. The check runs alone, because AllocsPerRun
// refuses to run beside parallel tests.
func TestRegionAllocs(t *testing.T) {
	r := fullRegion()
	dst := make([]byte, 0, 1<<16)
	assert.MaxAllocs(t, func() {
		if _, err := state.AppendRegion(dst[:0], r); err != nil {
			t.Fatal(err)
		}
	}, 0, "AppendRegion allocates nothing past its pooled scratch")

	blob, err := state.AppendRegion(nil, &store.Region{Packages: coretest.Workspace(1, 10, 20)})
	assert.NoError(t, err, "the canonical package encodes")
	assert.MaxAllocs(t, func() {
		if _, err := state.DecodeRegion(blob); err != nil {
			t.Fatal(err)
		}
	}, canonicalDecodeAllocs, "DecodeRegion allocates the region it returns")
}

// BenchmarkRegion measures a region's encoding and decoding over one
// package of the canonical corpus.
func BenchmarkRegion(b *testing.B) {
	r := &store.Region{Packages: coretest.Workspace(1, 10, 20)}
	blob, err := state.AppendRegion(nil, r)
	assert.NoError(b, err, "the region encodes")

	b.Run("AppendRegion", func(b *testing.B) {
		dst := make([]byte, 0, 2*len(blob))
		// The harness collects garbage before this run and runs it on a
		// goroutine of its own, so the pool can miss the scratch of the
		// parent's encoding. One encoding before the contract counts pools
		// the scratch again.
		_, err = state.AppendRegion(dst[:0], r)
		assert.NoError(b, err, "the encoding before the measurement succeeds")
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got []byte
		for c.Loop() {
			got, err = state.AppendRegion(dst[:0], r)
		}
		assert.NoError(b, err, "every encoding succeeds")
		assert.Equal(b, len(got), len(blob), "to the same bytes")
	})

	b.Run("DecodeRegion", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(canonicalDecodeAllocs)
		defer c.End()
		var got *store.Region
		for c.Loop() {
			got, err = state.DecodeRegion(blob)
		}
		assert.NoError(b, err, "every decode succeeds")
		assert.Length(b, got.Packages, 1, "to the one package")
	})
}

// fullRegion returns a region with a value in every part: two packages,
// a directive with a nested argument on two subjects, a stamp of every
// value type of the fact vocabulary, a link with tiers, a follow and a
// finding, and a finding of the region's own.
func fullRegion() *store.Region {
	s := coretest.Struct(coretest.StorePath, "Store")
	at := position.Pos{File: "svc/store/unit.go", Line: 3, Col: 1}
	finding := diag.Diag{
		Code: diag.Code{Prefix: diag.KernelPrefix, Number: 38}, Severity: diag.SeverityWarning,
		Pos: at, Msg: "ambiguous", Origin: "fakefront", Related: []position.Pos{at},
	}
	return &store.Region{
		Packages: []*node.Package{coretest.Package(coretest.StorePath, s), coretest.EveryKind(coretest.CachePath)},
		Directives: map[symbol.Identity][]directive.Raw{
			s.ID: {{
				Name: "gen:table", Pos: at, Negated: true, DirectiveShaped: true,
				Args: []directive.RawArg{{
					Key: "name", Col: 9,
					Value: directive.RawValue{Text: "users", Quoted: true, List: []directive.RawValue{{Text: "x"}}},
				}},
			}},
			coretest.PackageID(coretest.StorePath): {{Name: "gen:skip", Pos: at}},
		},
		Stamps: map[symbol.Identity][]meta.RawStamp{
			s.ID: {
				{Key: "fake.text", Value: "yes", Pos: at, Origin: "fakefront"},
				{Key: "fake.count", Value: int64(-4), Pos: at, Origin: "fakefront"},
				{Key: "fake.flag", Value: true, Pos: at, Origin: "fakefront"},
				{Key: "fake.list", Value: []string{"a", "b"}, Pos: at, Origin: "fakefront"},
				{Key: "fake.target", Value: s.ID, Pos: at, Origin: "fakefront"},
			},
		},
		Links: []store.Link{{
			Ref:      2,
			Tiers:    [][]symbol.Identity{{s.ID}, {coretest.PackageID(coretest.CachePath)}},
			Followed: []symbol.Identity{s.ID},
			Findings: []diag.Diag{finding},
		}},
		Findings: []diag.Diag{finding},
	}
}

// canonicalDecodeAllocs is the ceiling of one decode of a region of one
// canonical package: the string table, its list and its string, the
// region, its package list, and the package's 223 declarations and
// lists.
const canonicalDecodeAllocs = 3 + 1 + 1 + 223

// body returns a blob without its trailer.
func body(blob []byte) []byte { return bytes.Clone(blob[:len(blob)-4]) }

// resealed returns a body with a fresh CRC-32C trailer, so the decode
// gets past the checksum to the body however damaged it is.
func resealed(b []byte) []byte {
	sum := crc32.Checksum(b, crc32.MakeTable(crc32.Castagnoli))
	return binary.LittleEndian.AppendUint32(bytes.Clone(b), sum)
}
