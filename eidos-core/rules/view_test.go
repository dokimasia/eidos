// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/symbol"
)

// The view is the one door a projection reads through, so what it
// records and what a zero view refuses are pinned here.
func TestView(t *testing.T) {
	t.Parallel()

	t.Run("Lookup", func(t *testing.T) {
		t.Parallel()

		t.Run("reads a declaration and records the read", func(t *testing.T) {
			t.Parallel()

			v, reads, _ := viewOver(t, coretest.Frozen(t, hierarchy()))
			want := coretest.ID(svcPath, rowName, symbol.KindStruct)
			decl, held := v.Lookup(want)
			assert.True(t, held, "the declaration is held")
			assert.Equal(t, decl.(interface{ Identity() symbol.Identity }).Identity(), want, "as itself")
			pkg, held := v.PackageOf(want)
			assert.True(t, held, "and its package")
			assert.Equal(t, pkg.ID.Package, svcPath, "is the fixture's")
			recorded := 0
			for range reads.Identities() {
				recorded++
			}
			assert.True(t, recorded >= 2, "both reads recorded")
		})

		t.Run("holds nothing on the zero view", func(t *testing.T) {
			t.Parallel()

			var v rules.View
			assert.True(t, v.IsZero(), "the zero view reads nothing")
			_, held := v.Lookup(coretest.ID(svcPath, rowName, symbol.KindStruct))
			assert.False(t, held, "no declaration")
			_, held = v.PackageOf(coretest.ID(svcPath, rowName, symbol.KindStruct))
			assert.False(t, held, "no package")
			_, held = rules.Fact(v, coretest.ID(svcPath, rowName, symbol.KindStruct), meta.Key[string]{})
			assert.False(t, held, "no fact")
		})
	})

	t.Run("Fact", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the winning value and records the read", func(t *testing.T) {
			t.Parallel()

			v, reads, facts := viewOver(t, coretest.Frozen(t, hierarchy()))
			subject := coretest.PackageID(svcPath)
			assert.NoError(t, meta.Stamp(facts, v.Kernel.Module, "example.test/svc",
				meta.Claim{Subject: subject}), "a fact stamps")
			got, held := rules.Fact(v, subject, v.Kernel.Module)
			assert.True(t, held, "and reads back")
			assert.Equal(t, got, "example.test/svc", "as stamped")
			recorded := 0
			for range reads.Facts() {
				recorded++
			}
			assert.Equal(t, recorded, 1, "the read is on the set")
		})

		t.Run("reads untracked without a recorder", func(t *testing.T) {
			t.Parallel()

			v, _, facts := viewOver(t, coretest.Frozen(t, hierarchy()))
			v.Reads = nil
			subject := coretest.PackageID(svcPath)
			assert.NoError(t, meta.Stamp(facts, v.Kernel.Module, "m", meta.Claim{Subject: subject}), "stamps")
			got, held := rules.Fact(v, subject, v.Kernel.Module)
			assert.True(t, held && got == "m", "the value still reads")
		})
	})
}
