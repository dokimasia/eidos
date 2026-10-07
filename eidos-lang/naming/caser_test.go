// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package naming_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/naming"
)

// ulid is an initialism the common set does not contain.
const ulid = "ULID"

// The allocations of a Caser's construction and of its listing.
const (
	// newCaserAllocs is a Caser without initialisms: the Caser and its
	// map.
	newCaserAllocs = 2
	// derivedAllocs is a Caser that adds one initialism to a Caser
	// without any: the Caser, the clone of the empty map, and the
	// clone's first group.
	derivedAllocs = 3
	// initialismsAllocs is the sorted list of the initialisms.
	initialismsAllocs = 1
)

// A Caser's initialisms are the one part of case conversion a consumer
// configures. What it accepts as an initialism, and that a derived
// Caser leaves its parent unchanged, are pinned.
func TestCaser(t *testing.T) {
	t.Parallel()

	t.Run("Default", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a Caser of the common initialisms", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, naming.Default().Initialisms(), slices.Sorted(slices.Values(naming.CommonInitialisms)),
				"the list the package declares")
		})

		t.Run("returns one Caser to every caller", func(t *testing.T) {
			t.Parallel()

			first, second := naming.Default(), naming.Default()
			assert.Equal(t, second, first, "one immutable value serves every caller", assert.ByIdentity())
		})
	})

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a Caser without an initialism", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, naming.New().Initialisms(), "an empty set")
		})

		t.Run("returns a Caser that title-cases an acronym", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, naming.New().Pascal("url_path"), "UrlPath", "url is a word like any other")
		})
	})

	t.Run("WithInitialisms", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a Caser that recognises the given initialism", func(t *testing.T) {
			t.Parallel()

			c, err := naming.Default().WithInitialisms(ulid)
			assert.NoError(t, err, "an upper-case word is an initialism")
			assert.Equal(t, c.Pascal("ulid_key"), "ULIDKey", "the new one is recognised")
		})

		t.Run("returns a Caser that keeps the receiver's initialisms", func(t *testing.T) {
			t.Parallel()

			c, err := naming.Default().WithInitialisms(ulid)
			assert.NoError(t, err, "the derived Caser composes")
			assert.Equal(t, c.Pascal("url_path"), "URLPath", "the receiver's ones remain")
		})

		t.Run("leaves the receiver unchanged", func(t *testing.T) {
			t.Parallel()

			base := naming.New()
			_, err := base.WithInitialisms(ulid)
			assert.NoError(t, err, "the derived Caser composes")
			assert.Empty(t, base.Initialisms(), "a Caser is immutable, which makes it safe to share")
		})

		t.Run("returns a Caser for an initialism with digits after its first letter", func(t *testing.T) {
			t.Parallel()

			c, err := naming.New().WithInitialisms("UTF8", "HTTP2")
			assert.NoError(t, err, "a version digit is part of the acronym")
			assert.Equal(t, c.Pascal("utf8_reader"), "UTF8Reader", "and is recognised as one")
		})

		t.Run("returns a Caser of the given initialism from the zero Caser", func(t *testing.T) {
			t.Parallel()

			var zero naming.Caser
			c, err := zero.WithInitialisms(ulid)
			assert.NoError(t, err, "the zero Caser derives a Caser")
			assert.Equal(t, c.Initialisms(), []string{ulid}, "with the one initialism")
		})

		t.Run("returns a Caser for every common initialism", func(t *testing.T) {
			t.Parallel()

			_, err := naming.New().WithInitialisms(naming.CommonInitialisms...)
			assert.NoError(t, err, "the common list satisfies the rule the Default Caser skips checking")
		})

		for _, bad := range []string{"", "url", "Url", "8K", "U-L", "UÉ"} {
			t.Run("returns ErrInvalidInitialism for "+candidate(bad), func(t *testing.T) {
				t.Parallel()

				_, err := naming.New().WithInitialisms(bad)
				assert.ErrorIs(t, err, naming.ErrInvalidInitialism,
					"an initialism is upper-case letters and digits, opening with a letter")
			})
		}
	})

	t.Run("Initialisms", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a copy the caller can change", func(t *testing.T) {
			t.Parallel()

			first := naming.Default().Initialisms()
			first[0] = ulid
			assert.NotEqual(t, naming.Default().Initialisms()[0], ulid, "the Caser's set is unchanged")
		})
	})
}

// A Caser allocates itself and its map, and a listing allocates its
// list. The ordinary run, which runs no benchmark, checks those
// ceilings here.
func TestCaserAllocs(t *testing.T) {
	base := naming.New()
	var (
		c   *naming.Caser
		err error
		got []string
	)
	assert.MaxAllocs(t, func() { c = naming.Default() }, 0, "Default allocates nothing")
	assert.NotNil(t, c, "Default returns the Caser")
	assert.MaxAllocs(t, func() { c = naming.New() }, newCaserAllocs, "New allocates the Caser and its map")
	assert.NotNil(t, c, "New returns a Caser")
	assert.MaxAllocs(t, func() { c, err = base.WithInitialisms(ulid) }, derivedAllocs,
		"WithInitialisms allocates the Caser and the clone of its map")
	assert.NoError(t, err, "WithInitialisms returns the Caser")
	assert.MaxAllocs(t, func() { got = c.Initialisms() }, initialismsAllocs, "Initialisms allocates the list")
	assert.Equal(t, got, []string{ulid}, "Initialisms lists the one initialism")
}

// BenchmarkCaser measures a Caser's construction, a derivation with one
// more initialism, and the listing of the common set.
func BenchmarkCaser(b *testing.B) {
	b.Run("Default", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got *naming.Caser
		for c.Loop() {
			got = naming.Default()
		}
		assert.NotNil(b, got, "Default returns the Caser")
	})

	b.Run("New", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(newCaserAllocs)
		defer c.End()
		var got *naming.Caser
		for c.Loop() {
			got = naming.New()
		}
		assert.NotNil(b, got, "New returns a Caser")
	})

	b.Run("WithInitialisms", func(b *testing.B) {
		base := naming.New()
		c := bench.Start(b).MaxAllocs(derivedAllocs)
		defer c.End()
		var (
			got *naming.Caser
			err error
		)
		for c.Loop() {
			got, err = base.WithInitialisms(ulid)
		}
		assert.NoError(b, err, "WithInitialisms returns the Caser")
		assert.Length(b, got.Initialisms(), 1, "the Caser has the one initialism")
	})

	b.Run("Initialisms", func(b *testing.B) {
		caser := naming.Default()
		c := bench.Start(b).MaxAllocs(initialismsAllocs)
		defer c.End()
		var got []string
		for c.Loop() {
			got = caser.Initialisms()
		}
		assert.Length(b, got, len(naming.CommonInitialisms), "Initialisms lists the common set")
	})
}

// candidate spells a candidate initialism for a case name, with the
// empty one spelled as such.
func candidate(s string) string {
	if s == "" {
		return "the empty word"
	}
	return s
}
