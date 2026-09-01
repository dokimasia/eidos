// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta_test

import (
	"math/rand/v2"
	"slices"
	"sync"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/meta"
)

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
			for round := range 8 {
				_, f, role, _ := fixture(t)
				drop := by("defaults", 1)
				drop.Authority = meta.AuthorityDirective

				var wg sync.WaitGroup
				wg.Go(func() {
					expect.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)),
						"the racing stamp is admitted")
				})
				wg.Go(func() {
					expect.NoError(t, f.DropKey(role.ID(), drop),
						"the racing drop is admitted")
				})
				wg.Wait()

				assert.Empty(t, slices.Collect(f.ByKey(role.ID())),
					"the drop outranks the stamp, so the subject is no match, "+
						"whichever write finished last")
				_ = round
			}
		})

		t.Run("one winner however the writes interleave", func(t *testing.T) {
			t.Parallel()

			// Every scheduling of the same claims returns the same
			// winner, which is what lets a parallel run report what a
			// serial one does.
			claims := []struct {
				value string
				claim meta.Claim
			}{
				{"a", by("alpha", 1)},
				{"b", by("beta", 1)},
				{"c", by("beta", 2)},
				{"d", func() meta.Claim {
					c := by("gamma", 1)
					c.Bucket = 1
					return c
				}()},
			}

			winner := ""
			for round := range 8 {
				_, f, role, _ := fixture(t)
				shuffled := slices.Clone(claims)
				seed := rand.New(rand.NewPCG(uint64(round), 0))
				seed.Shuffle(len(shuffled), func(i, j int) {
					shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
				})

				var wg sync.WaitGroup
				for _, entry := range shuffled {
					wg.Go(func() {
						expect.NoError(t, meta.Stamp(f, role, entry.value, entry.claim),
							"every racing claim is admitted")
					})
				}
				wg.Wait()

				got, held := meta.Get(f, subject, role)
				assert.True(t, held, "a winner is chosen")
				if winner == "" {
					winner = got
				}
				assert.Equal(t, got, winner, "and it is the same one every round")
			}
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

// FuzzArbitration applies arbitrary claim sequences forwards and
// backwards and holds the winner equal: rank decides, arrival never
// does, whatever the claims are.
func FuzzArbitration(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 0, 1, 2, 1, 0, 3, 1, 0})
	f.Add([]byte{1, 0, 0, 0, 1, 0})
	f.Add([]byte{})

	plugins := []diag.Origin{"alpha", "beta", "gamma", "delta"}
	values := []string{"a", "b", "c", "d"}

	f.Fuzz(func(t *testing.T, in []byte) {
		// Decode fixed-width records, then keep one claim per rank
		// source: a source claiming two values is refused by
		// design, and which of the two survives would otherwise
		// depend on arrival.
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
			_, facts, role, _ := fixture(t)
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
				if err != nil {
					t.Fatalf("a deduplicated claim was refused: %v", err)
				}
			}
			got, held := meta.Get(facts, subject, role)
			return got, held, len(slices.Collect(facts.Claims(subject, role.ID())))
		}

		forward := records
		backward := slices.Clone(records)
		slices.Reverse(backward)

		gotF, heldF, countF := apply(forward)
		gotB, heldB, countB := apply(backward)
		if heldF != heldB || gotF != gotB {
			t.Fatalf("arrival order decided: forward (%q, %t), backward (%q, %t)",
				gotF, heldF, gotB, heldB)
		}
		if countF != countB {
			t.Fatalf("the record depends on arrival: %d claims forward, %d backward",
				countF, countB)
		}
	})
}
