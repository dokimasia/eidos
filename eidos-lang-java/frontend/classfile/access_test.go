// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package classfile_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/java/frontend/classfile"
)

// Has checks several flags at once. A set that lacks one of them
// reports false.
func TestAccess(t *testing.T) {
	t.Parallel()

	t.Run("Has", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for a set that has every flag", func(t *testing.T) {
			t.Parallel()

			set := classfile.AccPublic | classfile.AccStatic | classfile.AccFinal
			assert.True(t, set.Has(classfile.AccPublic|classfile.AccFinal), "both are set")
		})

		t.Run("reports false for a set that lacks one of the flags", func(t *testing.T) {
			t.Parallel()

			assert.False(t, classfile.AccPublic.Has(classfile.AccPublic|classfile.AccStatic), "static is not set")
		})
	})
}

// Has allocates nothing in the ordinary run, which runs no benchmark.
func TestAccessZeroAlloc(t *testing.T) {
	set := classfile.AccPublic | classfile.AccStatic | classfile.AccFinal
	var got bool
	assert.MaxAllocs(t, func() { got = set.Has(classfile.AccPublic | classfile.AccFinal) }, 0,
		"Has allocates nothing")
	assert.True(t, got, "Has reports both flags set")
}

// BenchmarkAccess measures the flag check the reader makes for every
// class, member and parameter.
func BenchmarkAccess(b *testing.B) {
	b.Run("Has", func(b *testing.B) {
		set := classfile.AccPublic | classfile.AccStatic | classfile.AccFinal
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got bool
		for c.Loop() {
			got = set.Has(classfile.AccPublic | classfile.AccFinal)
		}
		assert.True(b, got, "Has reports both flags set")
	})
}
