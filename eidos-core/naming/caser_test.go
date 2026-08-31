// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package naming_test

import (
	"slices"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/naming"
)

// The Caser holds the one thing case conversion is configurable
// in, so what it accepts as an initialism, and that a derived
// Caser leaves its parent alone, are both contract.
func TestCaser(t *testing.T) {
	t.Parallel()

	t.Run("Default carries the common initialisms", func(t *testing.T) {
		t.Parallel()

		got := naming.Default().Initialisms()
		want := slices.Sorted(slices.Values(naming.CommonInitialisms))
		assert.Equal(t, got, want, "sorted, and the list the package declares")
		first, second := naming.Default(), naming.Default()
		assert.True(t, first == second,
			"one value serves every caller, because it is immutable")
	})

	t.Run("every common initialism is one the API would accept", func(t *testing.T) {
		t.Parallel()

		_, err := naming.New().WithInitialisms(naming.CommonInitialisms...)
		assert.NoError(t, err,
			"the list is fixed here, so nothing revalidates it at construction")
	})

	t.Run("New recognises none", func(t *testing.T) {
		t.Parallel()

		assert.Length(t, naming.New().Initialisms(), 0, "an empty set")
		assert.Equal(t, naming.New().Pascal("url_path"), "UrlPath",
			"so an acronym is title-cased like any other word")
	})

	t.Run("WithInitialisms", func(t *testing.T) {
		t.Parallel()

		t.Run("adds to what the receiver recognised", func(t *testing.T) {
			t.Parallel()

			c, err := naming.Default().WithInitialisms("ULID")
			assert.NoError(t, err, "an upper-case word is an initialism")
			assert.Equal(t, c.Pascal("ulid_key"), "ULIDKey", "the new one is recognised")
			assert.Equal(t, c.Pascal("url_path"), "URLPath", "and the old ones stand")
		})

		t.Run("leaves the receiver alone", func(t *testing.T) {
			t.Parallel()

			base := naming.New()
			derived, err := base.WithInitialisms("ULID")
			assert.NoError(t, err, "the derived Caser composes")
			assert.Length(t, base.Initialisms(), 0,
				"a Caser is immutable, which is what makes it safe to share")
			assert.Length(t, derived.Initialisms(), 1, "and the copy carries the addition")
		})

		t.Run("refuses what no initialism may be", func(t *testing.T) {
			t.Parallel()

			for _, bad := range []string{"", "url", "Url", "8K", "U-L", "UÉ"} {
				_, err := naming.New().WithInitialisms(bad)
				assert.ErrorIs(t, err, naming.ErrInvalidInitialism,
					"upper-case letters and digits, starting with a letter: "+bad)
			}
		})

		t.Run("accepts a digit after the first letter", func(t *testing.T) {
			t.Parallel()

			c, err := naming.New().WithInitialisms("UTF8", "HTTP2")
			assert.NoError(t, err, "a version digit is part of the acronym")
			assert.Equal(t, c.Pascal("utf8_reader"), "UTF8Reader", "and is recognised as one")
		})
	})

	t.Run("recognises an initialism however it was written", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, naming.Pascal("http_server"), "HTTPServer", "lower-case spells it")
		assert.Equal(t, naming.Pascal("Http_server"), "HTTPServer", "mixed case too")
		assert.Equal(t, naming.Pascal("ıd_token"), "IDToken",
			"and a rune whose upper-case form is the acronym, which is why "+
				"the probe does not stop at the first non-ASCII byte")
	})
}
