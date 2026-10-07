// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta_test

import (
	"encoding/binary"
	"slices"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/history"
	"go.dokimi.dev/assert/prop"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/meta"
)

// rankDecides is the property [FuzzArbitration] and its ForAll twin in
// [TestBag] state.
const rankDecides = "a claim sequence and its reverse must leave one winner and one record of claims"

// Arbitration is the bag's whole contract: rank decides, arrival
// never does.
func TestBag(t *testing.T) {
	t.Parallel()

	t.Run("arbitration", func(t *testing.T) {
		t.Parallel()

		t.Run("refuses one source claiming two values", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)),
				"the first claim arrives")

			err := meta.Stamp(f, role, "reader", by("shape", 1))
			assert.HasError(t, err,
				"the same source claiming a second value is refused: rank could "+
					"not order the two, so arrival would")
		})

		t.Run("higher authority wins whatever arrives first", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "inferred", by("shape", 1)),
				"the plugin stamps first")
			directive := by("defaults", 1)
			directive.Authority = meta.AuthorityDirective
			assert.NoError(t, meta.Stamp(f, role, "declared", directive),
				"the directive stamps second")

			got, _ := meta.Get(f, subject, role)
			assert.Equal(t, got, "declared", "authority outranks arrival order")
		})

		t.Run("the earlier bucket wins within one authority", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			late := by("weaver", 1)
			late.Bucket = 2
			assert.NoError(t, meta.Stamp(f, role, "late", late), "the later bucket stamps first")
			early := by("classifier", 1)
			early.Bucket = 1
			assert.NoError(t, meta.Stamp(f, role, "early", early), "the earlier stamps second")

			got, _ := meta.Get(f, subject, role)
			assert.Equal(t, got, "early", "the earlier capability bucket wins")
		})

		t.Run("the alphabetically first plugin wins within one bucket", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "second", by("zeta", 1)), "zeta stamps first")
			assert.NoError(t, meta.Stamp(f, role, "first", by("alpha", 1)), "alpha stamps second")

			got, _ := meta.Get(f, subject, role)
			assert.Equal(t, got, "first", "the tie-break is the name, not the schedule")
		})

		t.Run("the first claim wins within one plugin", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "later", by("shape", 2)),
				"the later claim stamps first")
			assert.NoError(t, meta.Stamp(f, role, "earlier", by("shape", 1)),
				"the earlier stamps second")

			got, _ := meta.Get(f, subject, role)
			assert.Equal(t, got, "earlier",
				"canonical match order decides, and a later claim carrying a "+
					"different value does nothing")
		})

		t.Run("one index however stamps and drops interleave", func(t *testing.T) {
			t.Parallel()

			// The presence transitions re-record under the bag lock,
			// so racing a stamp against an outranking drop leaves the
			// index the same whichever finished last.
			for range 8 {
				_, f, role, _ := fixture(t)
				drop := by("defaults", 1)
				drop.Authority = meta.AuthorityDirective

				outcomes := history.Concurrently(2, time.Minute, func(client int) (any, error) {
					if client == 0 {
						return client, meta.Stamp(f, role, "writer", by("shape", 1))
					}
					return client, f.DropKey(role.ID(), drop)
				})
				for _, o := range outcomes {
					expect.True(t, o.Finished, "the racing write finishes")
					expect.NoError(t, o.Error, "the racing stamp and drop are admitted")
				}

				assert.Empty(t, slices.Collect(f.ByKey(role.ID())),
					"the drop outranks the stamp, so the subject is no match, "+
						"whichever write finished last")
			}
		})

		t.Run("one winner however the writes interleave", func(t *testing.T) {
			t.Parallel()

			// Every scheduling of the same claims returns the winner a
			// serial run returns, which is what lets a parallel run report
			// what a serial one does.
			type stamp struct {
				value string
				claim meta.Claim
			}
			claims := []stamp{
				{"a", by("alpha", 1)},
				{"b", by("beta", 1)},
				{"c", by("beta", 2)},
				{"d", func() meta.Claim {
					c := by("gamma", 1)
					c.Bucket = 1
					return c
				}()},
			}
			_, serial, role, _ := fixture(t)
			assert.Total(t, func(s stamp) error { return meta.Stamp(serial, role, s.value, s.claim) }, claims,
				"every claim is admitted in a serial run")
			want, held := meta.Get(serial, subject, role)
			assert.True(t, held, "the serial run chooses a winner")

			prop.ForAll(t, "every scheduling of the claims must choose the serial run's winner", func(c *prop.Case) {
				shuffled := c.Draw(prop.Permutation(claims...), "claims")
				_, f, role, _ := fixture(c)
				outcomes := history.Concurrently(len(shuffled), time.Minute, func(client int) (any, error) {
					entry := shuffled[client]
					return client, meta.Stamp(f, role, entry.value, entry.claim)
				})
				for _, o := range outcomes {
					assert.True(c, o.Finished, "the racing claim must finish")
					assert.NoError(c, o.Error, "every racing claim must be admitted")
				}
				got, held := meta.Get(f, subject, role)
				assert.True(c, held, "a winner must be chosen")
				assert.Equal(c, got, want, "the winner must be the serial run's")
			})
		})

		t.Run("one winner whichever order arbitrary claims arrive in", func(t *testing.T) {
			t.Parallel()

			prop.ForAll(t, rankDecides, arbitrates)
		})
	})
}

// claimRecord is one fuzz-decoded write.
type claimRecord struct {
	authority meta.Authority
	bucket    int
	plugin    diag.Origin
	seq       int
	drop      bool
	value     string
}

// rankSource is the identity arbitration refuses duplicates under.
type rankSource struct {
	authority meta.Authority
	bucket    int
	plugin    diag.Origin
	seq       int
}

// FuzzArbitration checks [rankDecides] on claim sequences nothing in
// this repository wrote. Each seed is the choices of one case: a
// two-byte little-endian length, then six bytes per claim.
func FuzzArbitration(f *testing.F) {
	for _, records := range [][]byte{{0, 1, 2, 3, 0, 1, 2, 1, 0, 3, 1, 0}, {1, 0, 0, 0, 1, 0}, {}} {
		f.Add(append(binary.LittleEndian.AppendUint16(nil, uint16(len(records))), records...))
	}

	prop.Fuzz(f, rankDecides, arbitrates)
}

// arbitrates checks [rankDecides] on a claim sequence that the case
// draws as bytes. It decodes fixed-width records and keeps one claim per
// rank source, because a source claiming two values is refused by
// design, and which of the two survives would otherwise depend on
// arrival. It applies the claims forwards and backwards and compares the
// two outcomes.
func arbitrates(c *prop.Case) {
	plugins := []diag.Origin{"alpha", "beta", "gamma", "delta"}
	values := []string{"a", "b", "c", "d"}
	in := c.Draw(prop.Bytes(), "claims")

	const record = 6
	seen := map[rankSource]struct{}{}
	var records []claimRecord
	for i := 0; i+record <= len(in) && len(records) < 32; i += record {
		r := claimRecord{
			authority: meta.Authority(in[i] % 3),
			bucket:    int(in[i+1] % 3),
			plugin:    plugins[int(in[i+2])%len(plugins)],
			seq:       int(in[i+3] % 4),
			drop:      in[i+4]%2 == 1,
			value:     values[int(in[i+5])%len(values)],
		}
		source := rankSource{r.authority, r.bucket, r.plugin, r.seq}
		if _, held := seen[source]; held {
			continue
		}
		seen[source] = struct{}{}
		records = append(records, r)
	}

	apply := func(ordered []claimRecord) (string, bool, int) {
		_, facts, role, _ := fixture(c)
		for _, r := range ordered {
			claim := by(r.plugin, r.seq)
			claim.Authority = r.authority
			claim.Bucket = r.bucket
			var err error
			if r.drop {
				err = facts.DropKey(role.ID(), claim)
			} else {
				err = meta.Stamp(facts, role, r.value, claim)
			}
			assert.NoError(c, err, "a deduplicated claim must be admitted")
		}
		got, held := meta.Get(facts, subject, role)
		return got, held, len(slices.Collect(facts.Claims(subject, role.ID())))
	}

	backward := slices.Clone(records)
	slices.Reverse(backward)
	gotF, heldF, countF := apply(records)
	gotB, heldB, countB := apply(backward)
	assert.Equal(c, heldB, heldF, "arrival order must decide no winner's presence")
	assert.Equal(c, gotB, gotF, "arrival order must decide no winner")
	assert.Equal(c, countB, countF, "the record of claims must not depend on arrival")
}
