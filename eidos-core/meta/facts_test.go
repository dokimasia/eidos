// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta_test

import (
	"cmp"
	"slices"
	"strconv"
	"sync/atomic"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// benchSubjects is the number of subjects that a benchmark stamps. A
// large workspace has as many declarations, and the store is sized for
// them.
const benchSubjects = 200_000

// allocSubjects is the number of subjects that an allocation check
// stamps. Each check calls its write on 101 subjects, one call for each,
// so the checks of one store take disjoint ranges of the subjects.
const allocSubjects = 1_000

// stampBatch is the number of first claims that one iteration of the
// first-claim benchmark stamps into an empty store.
const stampBatch = 1_000

// These constants are the allocations of the construction of the store
// and of each write that keeps something. TestFactsAllocs checks them in
// the ordinary run, and BenchmarkFacts checks them in a benchmark run.
const (
	// newFactsAllocs is the count of an empty store. NewFacts allocates
	// the store and the table of the groups and kind restrictions of the
	// keys.
	newFactsAllocs = 2
	// firstClaimAllocs is the count of a first claim on a new subject.
	// The claim allocates the bag, the boxed identity, the sync.Map entry
	// and the boxed string. sync.Map also adds a trie node where the
	// hashes of two subjects share a prefix. In a store of one subject
	// that happens in about one claim of 15. Over 300 runs the mean of a
	// check measured 4.07, and the highest mean was 4.17.
	firstClaimAllocs = 4
	// secondKeyAllocs is the count of the first claim on a second key of
	// a subject. The claim allocates the key map of the bag, the first
	// group of the map and the state of the key. Go boxes a boolean
	// without an allocation.
	secondKeyAllocs = 3
	// secondSourceAllocs is the count of a claim from a second rank
	// source on a fact. The claim allocates the claim slice of the fact
	// and the boxed string.
	secondSourceAllocs = 2
	// dropAllocs is the count of a drop on a stamped fact. The drop
	// allocates the claim slice of the fact and has no value to box.
	dropAllocs = 1
	// groupDropAllocs is the count of the first group drop on a stamped
	// subject. The drop allocates the group map of the bag, the first
	// group of the map and the state of the group.
	groupDropAllocs = 3
	// firstClaimsAllocs is the count of one iteration of the first-claim
	// benchmark. The first claims of the batch allocate 4,000 times. The
	// map of the index allocates at most 21 times as it grows to 1,001
	// members. sync.Map adds trie nodes at random. Over 300 runs they
	// averaged 360 with a standard deviation of 10. The ceiling allows 440
	// trie nodes, which is eight standard deviations above the mean.
	firstClaimsAllocs = stampBatch*firstClaimAllocs + 21 + 440
)

// These variables are the fixture vocabulary. subject is a struct, and
// sibling is a struct of the same package that sorts before it. function
// is a function that the role key does not admit. carrier is the
// position of the claims about them.
var (
	subject = symbol.Identity{
		Lang: "golang", Package: "svc/store", Name: "Store", Kind: symbol.KindStruct,
	}
	sibling = symbol.Identity{
		Lang: "golang", Package: "svc/store", Name: "Cache", Kind: symbol.KindStruct,
	}
	function = symbol.Identity{
		Lang: "golang", Package: "svc/store", Name: "Open", Kind: symbol.KindFunction,
	}
	carrier = position.Pos{File: "svc/store.go", Line: 7, Col: 1}
)

// recorder records fact reads for the tracked-read cases.
type recorder struct {
	reads []meta.Read
}

// RecordFact appends the read.
func (r *recorder) RecordFact(subject symbol.Identity, key meta.KeyName) {
	r.reads = append(r.reads, meta.Read{Subject: subject, Key: key})
}

// counter counts fact reads and keeps no read, so a benchmark of
// [meta.Fact] measures the read of the store alone. It allocates nothing.
type counter int

// RecordFact counts the read.
func (c *counter) RecordFact(symbol.Identity, meta.KeyName) { *c++ }

// Every plugin writes and reads facts through the store, so the contract
// covers what a write keeps and what a read returns.
func TestFacts(t *testing.T) {
	t.Parallel()

	t.Run("NewFacts", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a store without a claim", func(t *testing.T) {
			t.Parallel()

			r, _, role, _ := fixture(t)
			f := meta.NewFacts(r)
			_, held := meta.Get(f, subject, role)
			assert.False(t, held, "no fact reads present")
			assert.Empty(t, slices.Collect(f.ByKey(role.ID())), "no key lists a subject")
		})
	})

	t.Run("Stamp", func(t *testing.T) {
		t.Parallel()

		t.Run("records a value that Get returns", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)),
				"a claim on an admitted kind stamps")

			got, held := meta.Get(f, subject, role)
			assert.True(t, held, "the fact then reads present")
			assert.Equal(t, got, "writer", "Get returns the claimed value")
		})

		t.Run("returns an error for a zero key", func(t *testing.T) {
			t.Parallel()

			_, f, _, _ := fixture(t)
			var unregistered meta.Key[string]
			err := meta.Stamp(f, unregistered, "writer", by("shape", 1))
			assert.HasError(t, err, "a key that was never registered stamps nothing")
			assert.HasPrefix(t, err.Error(), "meta: ", "the error starts with the package prefix")
		})

		t.Run("returns an error for a named handle", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			err := meta.Stamp(f, meta.Named[string](role.Name()), "writer", by("shape", 1))
			assert.HasError(t, err, "a named handle writes nothing")
			assert.Contains(t, err.Error(), "named handle", "the error states that the handle is named")
		})

		t.Run("returns an error for a kind the key does not admit", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			err := meta.Stamp(f, role, "writer", on(function, "shape", 1))
			assert.HasError(t, err, "the key admits structs alone")

			_, held := meta.Get(f, function, role)
			assert.False(t, held, "the store records nothing")
		})

		t.Run("records a claim on any kind for a key that lists no kinds", func(t *testing.T) {
			t.Parallel()

			_, f, _, flag := fixture(t)
			assert.NoError(t, meta.Stamp(f, flag, true, on(function, "shape", 1)),
				"an empty kind list admits every kind")
		})

		t.Run("returns an error for a false boolean", func(t *testing.T) {
			t.Parallel()

			_, f, _, flag := fixture(t)
			err := meta.Stamp(f, flag, false, by("shape", 1))
			assert.HasError(t, err,
				"absence is the negative, so a fact turns false through a drop alone")
		})

		t.Run("records one claim for an identical re-stamp", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			claim := by("shape", 1)
			assert.NoError(t, meta.Stamp(f, role, "writer", claim), "the first stamp arrives")
			assert.NoError(t, meta.Stamp(f, role, "writer", claim), "the re-stamp is accepted")

			assert.Length(t, slices.Collect(f.Claims(subject, role.ID())), 1,
				"the store keeps one claim, so early cutoff sees no change")
		})
	})

	t.Run("Registry", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the registry that NewFacts received", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			f := meta.NewFacts(r)
			assert.Equal(t, f.Registry(), r, "Registry returns the registry that NewFacts received",
				assert.ByIdentity())
		})
	})

	t.Run("DropKey", func(t *testing.T) {
		t.Parallel()

		t.Run("outranks a later plugin stamp at directive authority", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, f.DropKey(role.ID(), dropBy("defaults", 1)), "the drop arrives first")
			assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)),
				"the stamp arrives second")

			_, held := meta.Get(f, subject, role)
			assert.False(t, held, "the drop outranks the later stamp")
		})

		t.Run("ranks below a later manual write", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, f.DropKey(role.ID(), dropBy("defaults", 1)), "the drop arrives")
			manual := by("migrate", 1)
			manual.Authority = meta.AuthorityManual
			assert.NoError(t, meta.Stamp(f, role, "kept", manual), "the manual write arrives")

			got, held := meta.Get(f, subject, role)
			assert.True(t, held, "manual outranks the drop")
			assert.Equal(t, got, "kept", "the fact reads the manual value")
		})

		tests := []struct {
			name string
			give meta.KeyID
		}{
			{name: "returns an error for the zero key", give: 0},
			{name: "returns an error for an id never assigned", give: 999},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, f, _, _ := fixture(t)
				assert.HasError(t, f.DropKey(tt.give, dropBy("defaults", 1)), "the key drops nothing")
			})
		}
	})

	t.Run("DropGroup", func(t *testing.T) {
		t.Parallel()

		t.Run("records one tombstone for an identical re-drop", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			drop := dropBy("defaults", 1)
			assert.NoError(t, f.DropGroup("shape.writer", drop), "the drop arrives")
			assert.NoError(t, f.DropGroup("shape.writer", drop), "the drop arrives again")

			assert.Length(t, slices.Collect(f.Claims(subject, role.ID())), 1,
				"the store keeps one tombstone")
		})

		t.Run("covers a member stamped after the drop", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, f.DropGroup("shape.writer", dropBy("defaults", 1)), "the group drop arrives")
			assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)),
				"a member stamps afterwards")

			_, held := meta.Get(f, subject, role)
			assert.False(t, held,
				"the tombstone covers the group, so arbitration finds it whichever member is read")
		})

		t.Run("leaves a key outside the group present", func(t *testing.T) {
			t.Parallel()

			_, f, _, flag := fixture(t)
			assert.NoError(t, f.DropGroup("shape.writer", dropBy("defaults", 1)), "the group drop arrives")
			assert.NoError(t, meta.Stamp(f, flag, true, by("shape", 1)),
				"a key outside the group stamps")

			_, held := meta.Get(f, subject, flag)
			assert.True(t, held, "the key reads present")
		})

		t.Run("ranks below a later write of higher authority", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, f.DropGroup("shape.writer", dropBy("defaults", 1)), "the group drop arrives")
			manual := by("migrate", 1)
			manual.Authority = meta.AuthorityManual
			assert.NoError(t, meta.Stamp(f, role, "kept", manual),
				"a manual write on a member arrives after the drop")

			got, held := meta.Get(f, subject, role)
			assert.True(t, held,
				"a tombstone covering the group is still a claim, and rank decides it")
			assert.Equal(t, got, "kept", "the fact reads the outranking value")
		})

		t.Run("returns an error for a group without registered keys", func(t *testing.T) {
			t.Parallel()

			_, f, _, _ := fixture(t)
			assert.HasError(t, f.DropGroup("shape.nonexistent", dropBy("defaults", 1)),
				"a group without registered keys drops nothing")
		})
	})

	t.Run("Withdraw", func(t *testing.T) {
		t.Parallel()

		t.Run("removes the claim of the rank source", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("alpha", 1)), "the first plugin stamps")
			assert.NoError(t, meta.Stamp(f, role, "reader", by("beta", 1)), "the second plugin stamps")
			assert.NoError(t, f.Withdraw(role.ID(), by("alpha", 1)), "the first claim is withdrawn")

			got, held := meta.Get(f, subject, role)
			assert.True(t, held, "the fact remains present")
			assert.Equal(t, got, "reader", "the fact reads the value of the remaining claim")
			assert.Length(t, slices.Collect(f.Claims(subject, role.ID())), 1, "one claim remains")
		})

		t.Run("removes the subject from ByKey with its last claim", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("alpha", 1)), "the plugin stamps")
			assert.Length(t, slices.Collect(f.ByKey(role.ID())), 1, "the subject has the key")
			assert.NoError(t, f.Withdraw(role.ID(), by("alpha", 1)), "the claim is withdrawn")
			assert.Empty(t, slices.Collect(f.ByKey(role.ID())), "the subject no longer has the key")
		})

		t.Run("records a claim stamped after the last claim was withdrawn", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("alpha", 1)), "the first plugin stamps")
			assert.NoError(t, f.Withdraw(role.ID(), by("alpha", 1)), "its claim is withdrawn")
			assert.NoError(t, meta.Stamp(f, role, "reader", by("beta", 1)), "the second plugin stamps")

			got, held := meta.Get(f, subject, role)
			assert.True(t, held, "the fact reads present")
			assert.Equal(t, got, "reader", "the fact reads the value of the second plugin")
		})

		t.Run("returns nil for a claim the store does not contain", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("alpha", 1)), "the plugin stamps")
			assert.NoError(t, f.Withdraw(role.ID(), by("alpha", 2)), "another instance made no claim")
			_, held := meta.Get(f, subject, role)
			assert.True(t, held, "the stamped claim remains")
		})

		t.Run("returns nil for a subject nothing claimed", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, f.Withdraw(role.ID(), by("alpha", 1)), "the subject has no claim")
		})

		t.Run("returns an error for a key nothing registered", func(t *testing.T) {
			t.Parallel()

			_, f, _, _ := fixture(t)
			assert.HasError(t, f.Withdraw(meta.KeyID(999), by("alpha", 1)),
				"Withdraw returns an error for an unregistered key")
		})
	})

	t.Run("WithdrawGroup", func(t *testing.T) {
		t.Parallel()

		t.Run("restores the member a group drop covered", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			drop := dropBy("defaults", 1)
			assert.NoError(t, f.DropGroup("shape.writer", drop), "the group drop arrives")
			assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)), "a member stamps")
			assert.NoError(t, f.WithdrawGroup("shape.writer", drop), "the group drop is withdrawn")

			got, held := meta.Get(f, subject, role)
			assert.True(t, held, "the member reads present again")
			assert.Equal(t, got, "writer", "the member reads its value")
			assert.Equal(t, slices.Collect(f.ByKey(role.ID())), []symbol.Identity{subject}, "ByKey lists the member")
		})

		t.Run("returns nil for a drop the store does not contain", func(t *testing.T) {
			t.Parallel()

			_, f, _, _ := fixture(t)
			assert.NoError(t, f.WithdrawGroup("shape.writer", dropBy("defaults", 1)), "no drop is no error")
		})

		t.Run("returns an error for a group without registered keys", func(t *testing.T) {
			t.Parallel()

			_, f, _, _ := fixture(t)
			assert.HasError(t, f.WithdrawGroup("shape.nonexistent", dropBy("defaults", 1)),
				"WithdrawGroup returns an error for a group without registered keys")
		})
	})

	t.Run("Damaged", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nil for a store that restores nothing", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("alpha", 1)), "the plugin stamps")
			assert.NoError(t, f.Damaged(), "nothing restores, so nothing is damaged")
		})
	})

	t.Run("Get", func(t *testing.T) {
		t.Parallel()

		t.Run("reports false for a fact never stamped", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			got, held := meta.Get(f, subject, role)
			assert.False(t, held, "a fact nobody wrote reads absent, and absence is legitimate")
			assert.Equal(t, got, "", "Get returns the zero value")
		})

		t.Run("returns the value of a named handle", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)), "the fact stamps")
			got, held := meta.Get(f, subject, meta.Named[string](role.Name()))
			assert.True(t, held, "the name resolves to the registered key")
			assert.Equal(t, got, "writer", "the handle reads the stamped value")
		})

		t.Run("reports false for a named handle that nothing registered", func(t *testing.T) {
			t.Parallel()

			_, f, _, _ := fixture(t)
			_, held := meta.Get(f, subject, meta.Named[string]("shape.ghost"))
			assert.False(t, held, "a name that the registry does not contain reads absent")
		})

		t.Run("reports false for a named handle of another value type", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)), "the fact stamps")
			_, held := meta.Get(f, subject, meta.Named[bool](role.Name()))
			assert.False(t, held, "the registry has the name under another value type")
		})
	})

	t.Run("GetAtLeast", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the value of a claim at the floor's authority", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)), "the plugin stamps")
			got, held := meta.GetAtLeast(f, subject, role, meta.AuthorityPlugin)
			assert.True(t, held, "a plugin's claim reads present at the floor of plugin authority")
			assert.Equal(t, got, "writer", "GetAtLeast returns the claimed value")
		})

		t.Run("reports false for a claim below the floor's authority", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)), "the plugin stamps")
			got, held := meta.GetAtLeast(f, subject, role, meta.AuthorityDirective)
			assert.False(t, held, "a plugin's claim is not an override")
			assert.Equal(t, got, "", "GetAtLeast returns the zero value")
		})

		t.Run("returns the value of a directive claim that ranks above a plugin's claim", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)), "the plugin stamps")
			override := by("author", 1)
			override.Authority = meta.AuthorityDirective
			assert.NoError(t, meta.Stamp(f, role, "reader", override), "the directive stamps")
			got, held := meta.GetAtLeast(f, subject, role, meta.AuthorityDirective)
			assert.True(t, held, "the directive's claim reads present")
			assert.Equal(t, got, "reader", "GetAtLeast returns the value of the directive")
		})

		t.Run("reports false where a drop ranks first", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)), "the plugin stamps")
			assert.NoError(t, f.DropKey(role.ID(), dropBy("defaults", 1)), "the directive drops the fact")
			_, held := meta.GetAtLeast(f, subject, role, meta.AuthorityPlugin)
			assert.False(t, held, "a dropped fact reads absent")
		})

		t.Run("reports false for a fact never stamped", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			_, held := meta.GetAtLeast(f, subject, role, meta.AuthorityPlugin)
			assert.False(t, held, "a fact nobody wrote reads absent")
		})

		t.Run("returns the value of a named handle", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)), "the fact stamps")
			got, held := meta.GetAtLeast(f, subject, meta.Named[string](role.Name()), meta.AuthorityPlugin)
			assert.True(t, held, "the name resolves to the registered key")
			assert.Equal(t, got, "writer", "the handle reads the stamped value")
		})

		t.Run("reports false for a named handle that nothing registered", func(t *testing.T) {
			t.Parallel()

			_, f, _, _ := fixture(t)
			_, held := meta.GetAtLeast(f, subject, meta.Named[string]("shape.ghost"), meta.AuthorityPlugin)
			assert.False(t, held, "a name that the registry does not contain reads absent")
		})

		t.Run("reports false for a named handle of another value type", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)), "the fact stamps")
			_, held := meta.GetAtLeast(f, subject, meta.Named[bool](role.Name()), meta.AuthorityPlugin)
			assert.False(t, held, "the registry has the name under another value type")
		})
	})

	t.Run("Fact", func(t *testing.T) {
		t.Parallel()

		t.Run("records the read of a stamped fact", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)), "the fact stamps")

			rec := &recorder{}
			got, held := meta.Fact(f, rec, subject, role)
			assert.True(t, held, "the tracked read returns the fact")
			assert.Equal(t, got, "writer", "the tracked read returns the stamped value")
			assert.Equal(t, rec.reads, []meta.Read{{Subject: subject, Key: "shape.role"}},
				"Fact records the read at (subject, key)")
		})

		t.Run("records the read of a fact never stamped", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			rec := &recorder{}
			_, held := meta.Fact(f, rec, subject, role)
			assert.False(t, held, "the fact is absent")
			assert.Length(t, rec.reads, 1, "Fact records the read, so the reader runs again when the fact appears")
		})

		t.Run("records nothing on a kind the key does not admit", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			rec := &recorder{}
			_, held := meta.Fact(f, rec, function, role)
			assert.False(t, held, "the fact is absent, because Stamp refuses the kind")
			assert.Empty(t, rec.reads, "Fact records no read, because the fact can never appear")
		})

		t.Run("returns the value of a named handle", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			assert.NoError(t, meta.Stamp(f, role, "writer", by("shape", 1)), "the fact stamps")
			got, held := meta.Fact(f, &recorder{}, subject, meta.Named[string](role.Name()))
			assert.True(t, held, "the name resolves to the registered key")
			assert.Equal(t, got, "writer", "the handle reads the stamped value")
		})

		t.Run("records nothing for a named handle on a kind the key does not admit", func(t *testing.T) {
			t.Parallel()

			_, f, role, _ := fixture(t)
			rec := &recorder{}
			_, held := meta.Fact(f, rec, function, meta.Named[string](role.Name()))
			assert.False(t, held, "the fact is absent, because Stamp refuses the kind")
			assert.Empty(t, rec.reads, "the kinds of the registered key apply to the name")
		})

		t.Run("records the read of a named handle that nothing registered under its name", func(t *testing.T) {
			t.Parallel()

			_, f, _, _ := fixture(t)
			rec := &recorder{}
			_, held := meta.Fact(f, rec, subject, meta.Named[string]("shape.ghost"))
			assert.False(t, held, "a name that the registry does not contain reads absent")
			assert.Equal(t, rec.reads, []meta.Read{{Subject: subject, Key: "shape.ghost"}},
				"the read records under the name of the handle")
		})
	})
}

// Each write allocates what the store keeps, and each read allocates
// nothing. Every check counts the mean over a hundred calls, each on a
// subject of its own. A counted first claim goes into a fresh store of
// one subject, so sync.Map adds a trie node in about one call of 15. The
// test does not call t.Parallel, because the count includes the
// allocations of every goroutine.
func TestFactsAllocs(t *testing.T) {
	subjects := benchIdentities(allocSubjects)
	f, role, flag := stamped(t, subjects)
	absent := absentIdentities(allocSubjects)

	var built *meta.Facts
	assert.MaxAllocs(t, func() { built = meta.NewFacts(f.Registry()) }, newFactsAllocs,
		"NewFacts allocates the store and its key table")
	assert.Equal(t, built.Registry(), f.Registry(), "NewFacts returns a store over the registry",
		assert.ByIdentity())

	// Each count keeps the first error of its calls, which cmp.Or
	// returns without allocating. fresh builds the store of each counted
	// first claim outside the count.
	var err error
	fresh := func() *meta.Facts {
		s := meta.NewFacts(f.Registry())
		err = cmp.Or(err, meta.Stamp(s, role, "writer", by("shape", 0)))
		return s
	}
	assert.MaxAllocsWithSetup(t, fresh, func(s *meta.Facts) {
		err = cmp.Or(err, meta.Stamp(s, role, "writer", on(subjects[0], "shape", 0)))
	}, firstClaimAllocs, "Stamp allocates the bag, its key, the map entry and the value of a first claim")
	assert.NoError(t, err, "every first claim is admitted")

	assert.NoError(t, meta.Stamp(f, flag, true, on(subjects[0], "shape", 0)), "the flag's index exists")

	// Each counted call takes the next index of its range of subjects,
	// which next returns outside the count.
	at := 1
	next := func() int {
		at++
		return at - 1
	}
	assert.MaxAllocsWithSetup(t, next, func(i int) {
		err = cmp.Or(err, meta.Stamp(f, flag, true, on(subjects[i], "shape", i)))
	}, secondKeyAllocs, "Stamp allocates the key map and the state of a subject's second key")
	assert.NoError(t, err, "every second key's claim is admitted")

	at = 0
	assert.MaxAllocsWithSetup(t, next, func(i int) {
		err = cmp.Or(err, meta.Stamp(f, role, "reader", on(subjects[i], "weaver", i)))
	}, secondSourceAllocs, "Stamp allocates the claim slice and the value of a second source's claim")
	assert.NoError(t, err, "every second source's claim is admitted")

	assert.MaxAllocs(t, func() {
		err = cmp.Or(err, meta.Stamp(f, role, "writer", on(subjects[0], "shape", 0)))
	}, 0, "Stamp allocates nothing for an identical re-stamp")
	assert.NoError(t, err, "every re-stamp is accepted")

	at = 200
	assert.MaxAllocsWithSetup(t, next, func(i int) {
		err = cmp.Or(err, f.DropKey(role.ID(), dropOn(subjects[i], i)))
	}, dropAllocs, "DropKey allocates the claim slice of a drop on a stamped fact")
	assert.NoError(t, err, "every drop is admitted")

	at = 400
	assert.MaxAllocsWithSetup(t, next, func(i int) {
		err = cmp.Or(err, f.DropGroup("shape.writer", dropOn(subjects[i], i)))
	}, groupDropAllocs, "DropGroup allocates the group map and the state of a subject's first group drop")
	assert.NoError(t, err, "every group drop is admitted")

	at = 400
	assert.MaxAllocsWithSetup(t, next, func(i int) {
		err = cmp.Or(err, f.WithdrawGroup("shape.writer", dropOn(subjects[i], i)))
	}, 0, "WithdrawGroup allocates nothing for a group drop the store keeps")
	assert.NoError(t, err, "every group drop is withdrawn")

	at = 600
	assert.MaxAllocsWithSetup(t, next, func(i int) {
		err = cmp.Or(err, f.Withdraw(role.ID(), on(subjects[i], "shape", i)))
	}, 0, "Withdraw allocates nothing for a claim the store keeps")
	assert.NoError(t, err, "every claim is withdrawn")

	at = 0
	assert.MaxAllocsWithSetup(t, next, func(i int) {
		err = cmp.Or(err, f.Withdraw(role.ID(), on(absent[i], "shape", i)))
	}, 0, "Withdraw allocates nothing for a subject nothing claimed")
	assert.NoError(t, err, "a withdrawal from a subject nothing claimed returns nil")

	var held bool
	assert.MaxAllocs(t, func() { _, held = meta.Get(f, subjects[800], role) }, 0,
		"Get allocates nothing for a stamped string")
	assert.True(t, held, "Get finds a stamped fact")

	assert.MaxAllocs(t, func() { _, held = meta.Get(f, absent[0], role) }, 0,
		"Get allocates nothing for a subject nothing claimed")
	assert.False(t, held, "Get finds nothing on a subject nothing claimed")

	named := meta.Named[string](role.Name())
	assert.MaxAllocs(t, func() { _, held = meta.Get(f, subjects[800], named) }, 0,
		"Get allocates nothing for a named handle")
	assert.True(t, held, "Get finds a stamped fact through a named handle")

	assert.MaxAllocs(t, func() { _, held = meta.GetAtLeast(f, subjects[800], role, meta.AuthorityPlugin) }, 0,
		"GetAtLeast allocates nothing for a stamped string at the floor's authority")
	assert.True(t, held, "GetAtLeast finds a stamped fact at the floor's authority")

	assert.MaxAllocs(t, func() { _, held = meta.GetAtLeast(f, subjects[800], role, meta.AuthorityDirective) }, 0,
		"GetAtLeast allocates nothing for a stamped string below the floor's authority")
	assert.False(t, held, "GetAtLeast skips a stamped fact below the floor's authority")

	var reads counter
	assert.MaxAllocs(t, func() { _, held = meta.Fact(f, &reads, subjects[800], role) }, 0,
		"Fact allocates nothing beyond what its recorder allocates")
	assert.True(t, held, "Fact finds a stamped fact")

	seen := 0
	assert.MaxAllocs(t, func() {
		seen = 0
		for range f.ByKey(role.ID()) {
			seen++
		}
	}, 0, "ByKey allocates nothing for a key whose presence did not change")
	assert.NotEqual(t, seen, 0, "ByKey enumerates the stamped subjects")

	var registry *meta.Registry
	assert.MaxAllocs(t, func() { err, registry = f.Damaged(), f.Registry() }, 0,
		"Damaged and Registry allocate nothing")
	assert.NoError(t, err, "a store that restores nothing is never damaged")
	assert.NotNil(t, registry, "the store has its registry")
}

// BenchmarkFacts measures each method of the store over 200,000 subjects
// with one fact each. The store is sized for that scale. A write that
// consumes the state of its subject takes the next subject. When the
// subjects run out, the benchmark builds the store again outside the
// measurement.
func BenchmarkFacts(b *testing.B) {
	subjects := benchIdentities(benchSubjects)
	absent := absentIdentities(benchSubjects)

	b.Run("NewFacts", func(b *testing.B) {
		r, _, _, _ := fixture(b)
		c := bench.Start(b).MaxAllocs(newFactsAllocs)
		defer c.End()
		var f *meta.Facts
		for c.Loop() {
			f = meta.NewFacts(r)
		}
		assert.Equal(b, f.Registry(), r, "NewFacts returns a store over the registry", assert.ByIdentity())
	})

	b.Run("Stamp", func(b *testing.B) {
		b.Run("a first claim on each of a thousand new subjects", func(b *testing.B) {
			r, _, role, _ := fixture(b)
			batch := subjects[:stampBatch]
			var f *meta.Facts
			fresh := func() {
				f = meta.NewFacts(r)
				assert.NoError(b, meta.Stamp(f, role, "writer", on(subjects[stampBatch], "shape", stampBatch)),
					"a claim outside the batch creates the store's map and index")
			}
			c := bench.Start(b).MaxAllocs(firstClaimsAllocs)
			defer c.End()
			var err error
			for c.Loop() {
				c.Excluding(fresh)
				for i, id := range batch {
					if err = meta.Stamp(f, role, "writer", on(id, "shape", i)); err != nil {
						break
					}
				}
			}
			assert.NoError(b, err, "every first claim is admitted")
		})

		b.Run("a first claim on a second key of a subject", func(b *testing.B) {
			var (
				f    *meta.Facts
				flag meta.Key[bool]
				next int
			)
			build := func() {
				f, _, flag = stamped(b, subjects)
				assert.NoError(b, meta.Stamp(f, flag, true, on(subjects[0], "shape", 0)), "the flag's index exists")
				next = 1
			}
			build()
			c := bench.Start(b).MaxAllocs(secondKeyAllocs)
			defer c.End()
			var err error
			for c.Loop() {
				if next == len(subjects) {
					c.Excluding(build)
				}
				err = meta.Stamp(f, flag, true, on(subjects[next], "shape", next))
				next++
			}
			assert.NoError(b, err, "the second key's claim is admitted")
		})

		b.Run("a claim from a second source", func(b *testing.B) {
			var (
				f    *meta.Facts
				role meta.Key[string]
				next int
			)
			build := func() {
				f, role, _ = stamped(b, subjects)
				next = 0
			}
			build()
			c := bench.Start(b).MaxAllocs(secondSourceAllocs)
			defer c.End()
			var err error
			for c.Loop() {
				if next == len(subjects) {
					c.Excluding(build)
				}
				err = meta.Stamp(f, role, "reader", on(subjects[next], "weaver", next))
				next++
			}
			assert.NoError(b, err, "the second source's claim is admitted")
		})

		b.Run("an identical re-stamp", func(b *testing.B) {
			f, role, _ := stamped(b, subjects[:1])
			claim := on(subjects[0], "shape", 0)
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var err error
			for c.Loop() {
				err = meta.Stamp(f, role, "writer", claim)
			}
			assert.NoError(b, err, "the re-stamp is accepted")
		})
	})

	b.Run("DropKey", func(b *testing.B) {
		b.Run("a drop on a stamped fact", func(b *testing.B) {
			var (
				f    *meta.Facts
				role meta.Key[string]
				next int
			)
			build := func() {
				f, role, _ = stamped(b, subjects)
				next = 0
			}
			build()
			c := bench.Start(b).MaxAllocs(dropAllocs)
			defer c.End()
			var err error
			for c.Loop() {
				if next == len(subjects) {
					c.Excluding(build)
				}
				err = f.DropKey(role.ID(), dropOn(subjects[next], next))
				next++
			}
			assert.NoError(b, err, "the drop is admitted")
		})
	})

	b.Run("DropGroup", func(b *testing.B) {
		b.Run("a first group drop on a stamped subject", func(b *testing.B) {
			var (
				f    *meta.Facts
				next int
			)
			build := func() {
				f, _, _ = stamped(b, subjects)
				next = 0
			}
			build()
			c := bench.Start(b).MaxAllocs(groupDropAllocs)
			defer c.End()
			var err error
			for c.Loop() {
				if next == len(subjects) {
					c.Excluding(build)
				}
				err = f.DropGroup("shape.writer", dropOn(subjects[next], next))
				next++
			}
			assert.NoError(b, err, "the group drop is admitted")
		})
	})

	b.Run("Withdraw", func(b *testing.B) {
		b.Run("the only claim on a fact", func(b *testing.B) {
			f, role, _ := stamped(b, subjects[:1])
			claim := on(subjects[0], "shape", 0)
			restamp := func() {
				assert.NoError(b, meta.Stamp(f, role, "writer", claim), "the claim is stamped again")
			}
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var err error
			for c.Loop() {
				err = f.Withdraw(role.ID(), claim)
				c.Excluding(restamp)
			}
			assert.NoError(b, err, "the claim is withdrawn")
		})
	})

	b.Run("WithdrawGroup", func(b *testing.B) {
		b.Run("the only group drop on a subject", func(b *testing.B) {
			f, _, _ := stamped(b, subjects[:1])
			drop := dropOn(subjects[0], 0)
			redrop := func() {
				assert.NoError(b, f.DropGroup("shape.writer", drop), "the group drop is made again")
			}
			redrop()
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var err error
			for c.Loop() {
				err = f.WithdrawGroup("shape.writer", drop)
				c.Excluding(redrop)
			}
			assert.NoError(b, err, "the group drop is withdrawn")
		})
	})

	b.Run("Get", func(b *testing.B) {
		b.Run("a miss on an unstamped subject", func(b *testing.B) {
			f, role, _ := stamped(b, subjects)
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			held, next := false, 0
			for c.Loop() {
				_, held = meta.Get(f, absent[next%len(absent)], role)
				next++
			}
			assert.False(b, held, "Get reports false for an unstamped subject")
		})

		b.Run("a stamped fact", func(b *testing.B) {
			f, role, _ := stamped(b, subjects)
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			got, next := "", 0
			for c.Loop() {
				got, _ = meta.Get(f, subjects[next%len(subjects)], role)
				next++
			}
			assert.Equal(b, got, "writer", "Get returns the stamped value")
		})

		b.Run("a stamped fact through a named handle", func(b *testing.B) {
			f, role, _ := stamped(b, subjects)
			named := meta.Named[string](role.Name())
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			got, next := "", 0
			for c.Loop() {
				got, _ = meta.Get(f, subjects[next%len(subjects)], named)
				next++
			}
			assert.Equal(b, got, "writer", "Get returns the stamped value through the name")
		})
	})

	b.Run("GetAtLeast", func(b *testing.B) {
		floors := []struct {
			name  string
			floor meta.Authority
			want  bool
		}{
			{name: "a stamped fact at the floor's authority", floor: meta.AuthorityPlugin, want: true},
			{name: "a stamped fact below the floor's authority", floor: meta.AuthorityDirective, want: false},
		}
		for _, tt := range floors {
			b.Run(tt.name, func(b *testing.B) {
				f, role, _ := stamped(b, subjects)
				c := bench.Start(b).MaxAllocs(0)
				defer c.End()
				held, next := false, 0
				for c.Loop() {
					_, held = meta.GetAtLeast(f, subjects[next%len(subjects)], role, tt.floor)
					next++
				}
				assert.Equal(b, held, tt.want,
					"GetAtLeast reports the fact where its claim has the floor's authority or a higher one")
			})
		}
	})

	b.Run("Fact", func(b *testing.B) {
		f, role, _ := stamped(b, subjects)
		var reads counter
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		got, next := "", 0
		for c.Loop() {
			got, _ = meta.Fact(f, &reads, subjects[next%len(subjects)], role)
			next++
		}
		assert.Equal(b, got, "writer", "Fact returns the stamped value")
	})

	b.Run("ByKey", func(b *testing.B) {
		f, role, _ := stamped(b, subjects)
		for range f.ByKey(role.ID()) { // the first enumeration sorts the index
			break
		}
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		seen := 0
		for c.Loop() {
			seen = 0
			for range f.ByKey(role.ID()) {
				seen++
			}
		}
		assert.Equal(b, seen, len(subjects), "ByKey enumerates every stamped subject")
	})

	b.Run("Damaged", func(b *testing.B) {
		f, _, _ := stamped(b, subjects[:1])
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var err error
		for c.Loop() {
			err = f.Damaged()
		}
		assert.NoError(b, err, "a store that restores nothing is never damaged")
	})

	b.Run("Registry", func(b *testing.B) {
		r, f, _, _ := fixture(b)
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got *meta.Registry
		for c.Loop() {
			got = f.Registry()
		}
		assert.Equal(b, got, r, "Registry returns the store's registry", assert.ByIdentity())
	})
}

// BenchmarkFactsParallel measures contention on GOMAXPROCS goroutines.
// Writers stamp first claims on distinct subjects, and readers read
// stamped facts. Each bag has its own lock, so neither side serializes.
// A read allocates nothing. The writers have no ceiling. Once b.N passes
// 200,000, the claim of a writer is a second claim on its subject, and
// it allocates what BenchmarkFacts states for a second source.
func BenchmarkFactsParallel(b *testing.B) {
	subjects := benchIdentities(benchSubjects)

	b.Run("Stamp", func(b *testing.B) {
		_, f, role, _ := fixture(b)
		var next atomic.Int64
		c := bench.Start(b)
		defer c.End()
		c.RunParallel(func(pb *bench.PB) {
			for pb.Next() {
				i := int(next.Add(1) - 1)
				expect.NoError(b, meta.Stamp(f, role, "writer", on(subjects[i%len(subjects)], "shape", i)),
					"every racing claim is admitted")
			}
		})
	})

	b.Run("Get", func(b *testing.B) {
		f, role, _ := stamped(b, subjects)
		var next atomic.Int64
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		c.RunParallel(func(pb *bench.PB) {
			for pb.Next() {
				i := int(next.Add(1) - 1)
				_, held := meta.Get(f, subjects[i%len(subjects)], role)
				expect.True(b, held, "Get finds every stamped fact")
			}
		})
	})
}

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

// by returns a plugin-authority claim on the fixture subject at one
// instance of its order.
func by(plugin diag.Origin, instance int) meta.Claim {
	return meta.Claim{
		Subject: subject,
		Plugin:  plugin,
		Order:   meta.Order{Subject: subject, Instance: instance},
		Pos:     carrier,
	}
}

// on returns the claim of [by] with id as its subject.
func on(id symbol.Identity, plugin diag.Origin, instance int) meta.Claim {
	c := by(plugin, instance)
	c.Subject = id
	return c
}

// dropBy returns a directive-authority claim on the fixture subject at
// one instance of its order. Drops have directive authority.
func dropBy(plugin diag.Origin, instance int) meta.Claim {
	c := by(plugin, instance)
	c.Authority = meta.AuthorityDirective
	return c
}

// dropOn returns a drop of the defaults plugin on another subject.
func dropOn(id symbol.Identity, instance int) meta.Claim {
	c := dropBy("defaults", instance)
	c.Subject = id
	return c
}

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

// absentIdentities returns n distinct struct subjects that
// [benchIdentities] never returns.
func absentIdentities(n int) []symbol.Identity {
	out := make([]symbol.Identity, 0, n)
	for i := range n {
		out = append(out, symbol.Identity{
			Lang: "golang", Package: "svc/absent/" + strconv.Itoa(i%1000),
			Name: "Absent" + strconv.Itoa(i), Kind: symbol.KindStruct,
		})
	}
	return out
}

// stamped returns a fact store and its two keys. The store has the role
// fact on every subject, and the shape plugin claims each fact at the
// index of its subject.
func stamped(tb assert.TB, subjects []symbol.Identity) (*meta.Facts, meta.Key[string], meta.Key[bool]) {
	tb.Helper()

	_, f, role, flag := fixture(tb)
	for i, id := range subjects {
		assert.NoError(tb, meta.Stamp(f, role, "writer", on(id, "shape", i)), "the subject's fact stamps")
	}
	return f, role, flag
}
