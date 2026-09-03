// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta_test

import (
	"slices"
	"strconv"
	"sync/atomic"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The fixture vocabulary: one struct subject and the claims plugins
// make about it.
var (
	subject = symbol.Identity{
		Lang: "golang", Package: "svc/store", Name: "Store", Kind: symbol.KindStruct,
	}
	carrier = position.Pos{File: "svc/store.go", Line: 7, Col: 1}
)

// fixture returns a registry with the fixture keys registered and a
// fact store over it.
func fixture(tb assert.TB) (*meta.Registry, *meta.Facts,
	meta.Key[string], meta.Key[bool],
) {
	tb.Helper()

	r := claimed(tb)
	role, err := meta.Register[string](r, meta.KeySpec{
		Name: "shape.role", Group: "shape.writer",
		Kinds: []symbol.Kind{symbol.KindStruct},
		Doc:   "the classified role",
	})
	assert.NoError(tb, err, "the role key registers")
	flag, err := meta.Register[bool](r, meta.KeySpec{
		Name: "shape.comparable", Doc: "values compare with ==",
	})
	assert.NoError(tb, err, "the boolean key registers")
	return r, meta.NewFacts(r), role, flag
}

// by returns a plugin-authority claim on the fixture subject.
func by(plugin diag.Origin, seq int) meta.Claim {
	return meta.Claim{Subject: subject, Plugin: plugin, Seq: seq, Pos: carrier}
}

// recorder records fact reads for the tracked-read cases.
type recorder struct {
	reads []meta.Read
}

func (r *recorder) RecordFact(subject symbol.Identity, key meta.KeyName) {
	r.reads = append(r.reads, meta.Read{Subject: subject, Key: key})
}

func TestFacts(t *testing.T) {
	t.Parallel()

	t.Run("Registry", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the registry the store was created over", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			f := meta.NewFacts(r)
			assert.True(t, f.Registry() == r, "a reader resolves keys by name against it")
		})
	})

	t.Run("Stamp", func(t *testing.T) {
		t.Parallel()

		t.Run("a stamped fact reads back", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)),
				"a claim on an admitted kind stamps")

			got, held := meta.Get(f, subject, role)
			assert.True(t, held, "the fact then reads present")
			assert.Equal(t, got, "writer", "with the claimed value")
		})

		t.Run("refuses a zero key", func(t *testing.T) {
			t.Parallel()

			_, f, _, _ := fixture(t)
			var unregistered meta.Key[string]
			err := meta.Stamp(f, unregistered, "writer", by("shape", 1))
			assert.HasError(t, err, "a key that was never registered stamps nothing")
			assert.HasPrefix(t, err.Error(), "meta: ", "under the package prefix")
		})

		t.Run("refuses a kind the key does not admit", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			onFunction := by("shape", 1)
			onFunction.Subject = symbol.Identity{
				Lang: "golang", Package: "svc/store", Name: "Open",
				Kind: symbol.KindFunction,
			}
			err := meta.Stamp(f, role, "writer", onFunction)
			assert.HasError(t, err, "the key admits structs alone")

			_, held := meta.Get(f, onFunction.Subject, role)
			assert.False(t, held, "and nothing was recorded")
		})

		t.Run("admits every kind for a key that lists none", func(t *testing.T) {
			t.Parallel()

			_, f, _, flag := fixture(t)
			assert.NoError(t, meta.Stamp(f, flag, true, by("shape", 1)),
				"an empty kind list admits every kind")
		})

		t.Run("refuses a false boolean", func(t *testing.T) {
			t.Parallel()

			_, f, _, flag := fixture(t)
			err := meta.Stamp(f, flag, false, by("shape", 1))
			assert.HasError(t, err,
				"absence is the negative: false is never stamped, so deletion stays load-bearing")
		})

		t.Run("an identical re-stamp changes nothing", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			claim := by("shape", 1)
			assert.NoError(t, meta.Stamp(f, role, "writer", claim), "the first stamp arrives")
			assert.NoError(t, meta.Stamp(f, role, "writer", claim), "the re-stamp is accepted")

			assert.Length(t, slices.Collect(f.Claims(subject, role.ID())), 1,
				"and records nothing: idempotence is what early cutoff prunes")
		})
	})

	t.Run("drops", func(t *testing.T) {
		t.Parallel()

		t.Run("an identical re-drop changes nothing", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			drop := by("defaults", 1)
			drop.Authority = meta.AuthorityDirective
			assert.NoError(t, f.DropGroup("shape.writer", drop), "the drop arrives")
			assert.NoError(t, f.DropGroup("shape.writer", drop), "and repeats")

			assert.Length(t, slices.Collect(f.Claims(subject, role.ID())), 1,
				"recording one tombstone: idempotence holds for drops too")
		})

		t.Run("a directive drop beats a plugin stamp whenever it arrives", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			drop := by("defaults", 1)
			drop.Authority = meta.AuthorityDirective
			assert.NoError(t, f.DropKey(role.ID(), drop), "the drop arrives first")
			assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)),
				"the stamp arrives second")

			_, held := meta.Get(f, subject, role)
			assert.False(t, held,
				"the tombstone does not race the stamp, it outranks it")
		})

		t.Run("a manual write beats a directive drop", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			drop := by("defaults", 1)
			drop.Authority = meta.AuthorityDirective
			assert.NoError(t, f.DropKey(role.ID(), drop), "the drop arrives")
			manual := by("migrate", 1)
			manual.Authority = meta.AuthorityManual
			assert.NoError(t, meta.Stamp(f, role, "kept", manual), "the manual write arrives")

			got, held := meta.Get(f, subject, role)
			assert.True(t, held, "manual outranks the drop")
			assert.Equal(t, got, "kept", "and its value stands")
		})

		t.Run("a group drop covers a member stamped afterwards", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			drop := by("defaults", 1)
			drop.Authority = meta.AuthorityDirective
			assert.NoError(t, f.DropGroup("shape.writer", drop), "the group drop arrives")
			assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)),
				"a member stamps afterwards")

			_, held := meta.Get(f, subject, role)
			assert.False(t, held,
				"the tombstone covers the group, so arbitration finds it "+
					"whichever member is read")
		})

		t.Run("a group drop does not touch a key outside the group", func(t *testing.T) {
			t.Parallel()

			_, f, _, flag := fixture(t)
			drop := by("defaults", 1)
			drop.Authority = meta.AuthorityDirective
			assert.NoError(t, f.DropGroup("shape.writer", drop), "the group drop arrives")
			assert.NoError(t, meta.Stamp(f, flag, true, by("shape", 1)),
				"a key outside the group stamps")

			_, held := meta.Get(f, subject, flag)
			assert.True(t, held, "and stays present")
		})

		t.Run("refuses a key and a group nothing registered", func(t *testing.T) {
			t.Parallel()

			_, f, _, _ := fixture(t)
			claim := by("defaults", 1)
			assert.HasError(t, f.DropKey(meta.KeyID(0), claim),
				"the zero key drops nothing")
			assert.HasError(t, f.DropKey(meta.KeyID(999), claim),
				"an id never assigned drops nothing")
			assert.HasError(t, f.DropGroup("shape.nonexistent", claim),
				"a group nothing registered into drops nothing")
		})
	})

	t.Run("DropGroup", func(t *testing.T) {
		t.Parallel()

		t.Run("a higher-authority write outranks the group tombstone", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			drop := by("defaults", 1)
			drop.Authority = meta.AuthorityDirective
			assert.NoError(t, f.DropGroup("shape.writer", drop), "the group drop arrives")
			manual := by("migrate", 1)
			manual.Authority = meta.AuthorityManual
			assert.NoError(t, meta.Stamp(f, role, "kept", manual),
				"a manual write on a member arrives after it")

			got, held := meta.Get(f, subject, role)
			assert.True(t, held,
				"a tombstone covering the group is still a claim, and rank decides it")
			assert.Equal(t, got, "kept", "so the outranking value stands")
		})
	})

	t.Run("Get", func(t *testing.T) {
		t.Parallel()

		t.Run("returns absent for a fact never stamped", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			got, held := meta.Get(f, subject, role)
			assert.False(t, held, "a fact nobody wrote reads absent, and absence is legitimate")
			assert.Equal(t, got, "", "with the zero value")
		})
	})

	t.Run("Fact", func(t *testing.T) {
		t.Parallel()

		t.Run("returns what Get does and records the read", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)), "the fact stamps")

			rec := &recorder{}
			got, held := meta.Fact(f, rec, subject, role)
			assert.True(t, held, "the tracked read returns the fact")
			assert.Equal(t, got, "writer", "with its value")
			assert.Equal(t, rec.reads, []meta.Read{{Subject: subject, Key: "shape.role"}},
				"and records the read at (subject, key)")
		})

		t.Run("a miss records too", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			rec := &recorder{}
			_, held := meta.Fact(f, rec, subject, role)
			assert.False(t, held, "the fact is absent")
			assert.Length(t, rec.reads, 1,
				"and the read still records: the reader runs again when it appears")
		})
	})
}

// The scale the store is sized for: two hundred thousand subjects,
// which is dozens of lifted keys across a large workspace's
// declarations.
const benchSubjects = 200_000

// benchIdentities returns n distinct struct subjects.
func benchIdentities(n int) []symbol.Identity {
	out := make([]symbol.Identity, 0, n)
	for i := range n {
		out = append(out, symbol.Identity{
			Lang: "golang", Package: "svc/store/" + strconv.Itoa(i%1000),
			Name: "Decl" + strconv.Itoa(i), Kind: symbol.KindStruct,
		})
	}
	return out
}

// stamped returns a fact store with the role fact on every subject.
func stamped(b *testing.B, subjects []symbol.Identity) (*meta.Facts, meta.Key[string]) {
	b.Helper()

	_, f, role, _ := fixture(b)
	for i, id := range subjects {
		claim := by("shape", i)
		claim.Subject = id
		if err := meta.Stamp(f, role, "writer", claim); err != nil {
			b.Fatalf("Stamp: unexpected error: %v", err)
		}
	}
	return f, role
}

func BenchmarkFacts(b *testing.B) {
	subjects := benchIdentities(benchSubjects)

	b.Run("Stamp", func(b *testing.B) {
		b.ReportAllocs()

		_, f, role, _ := fixture(b)
		next := 0
		for b.Loop() {
			claim := by("shape", next)
			claim.Subject = subjects[next%len(subjects)]
			if err := meta.Stamp(f, role, "writer", claim); err != nil {
				b.Fatalf("Stamp: unexpected error: %v", err)
			}
			next++
		}
	})

	b.Run("Get/a miss on an unstamped subject", func(b *testing.B) {
		b.ReportAllocs()

		_, f, role, _ := fixture(b)
		unstamped := benchIdentities(benchSubjects)
		next := 0
		for b.Loop() {
			if _, held := meta.Get(f, unstamped[next%len(unstamped)], role); held {
				b.Fatal("Get returned present for an unstamped subject")
			}
			next++
		}
	})

	b.Run("Get", func(b *testing.B) {
		b.ReportAllocs()

		f, role := stamped(b, subjects)
		next := 0
		for b.Loop() {
			if _, held := meta.Get(f, subjects[next%len(subjects)], role); !held {
				b.Fatal("Get returned absent for a stamped fact")
			}
			next++
		}
	})

	b.Run("Fact", func(b *testing.B) {
		b.ReportAllocs()

		f, role := stamped(b, subjects)
		reads := store.NewReadSet()
		next := 0
		for b.Loop() {
			if _, held := meta.Fact(f, reads, subjects[next%len(subjects)], role); !held {
				b.Fatal("Fact returned absent for a stamped fact")
			}
			next++
		}
	})

	b.Run("ByKey", func(b *testing.B) {
		b.ReportAllocs()

		f, role := stamped(b, subjects)
		for b.Loop() {
			seen := 0
			for range f.ByKey(role.ID()) {
				seen++
			}
			if seen != len(subjects) {
				b.Fatalf("ByKey returned %d subjects, want %d", seen, len(subjects))
			}
		}
	})
}

// Contention: bags shard by subject, so parallel writers
// on distinct subjects and parallel readers of stamped facts both
// scale rather than serialize.
func BenchmarkFactsParallel(b *testing.B) {
	subjects := benchIdentities(benchSubjects)

	b.Run("Stamp", func(b *testing.B) {
		b.ReportAllocs()

		_, f, role, _ := fixture(b)
		var next atomic.Int64
		b.ResetTimer()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				i := int(next.Add(1) - 1)
				claim := by("shape", i)
				claim.Subject = subjects[i%len(subjects)]
				if err := meta.Stamp(f, role, "writer", claim); err != nil {
					b.Errorf("Stamp: unexpected error: %v", err)
				}
			}
		})
	})

	b.Run("Get", func(b *testing.B) {
		b.ReportAllocs()

		f, role := stamped(b, subjects)
		var next atomic.Int64
		b.ResetTimer()
		b.RunParallel(func(p *testing.PB) {
			for p.Next() {
				i := int(next.Add(1) - 1)
				if _, held := meta.Get(f, subjects[i%len(subjects)], role); !held {
					b.Error("Get returned absent for a stamped fact")
				}
			}
		})
	})

	b.Run("identical re-stamp", func(b *testing.B) {
		b.ReportAllocs()

		// The idempotence path: an annotator re-run stamps what it
		// stamped before, and the dedupe scan is O(claims on the
		// key), which the design bounds to a few claimants per fact.
		_, f, role, _ := fixture(b)
		claim := by("shape", 1)
		if err := meta.Stamp(f, role, "writer", claim); err != nil {
			b.Fatalf("Stamp: unexpected error: %v", err)
		}
		for b.Loop() {
			if err := meta.Stamp(f, role, "writer", claim); err != nil {
				b.Fatalf("Stamp: unexpected error: %v", err)
			}
		}
	})
}
