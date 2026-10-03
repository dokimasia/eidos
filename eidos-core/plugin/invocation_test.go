// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
)

// The references the order cases compare: a unit, and the unit each
// field of the order moves past it.
var (
	baseUnit = plugin.UnitRef{
		Plugin: "stubgen", Tag: "test", Pkg: coretest.PackageID(coretest.StorePath), Key: "svc/store/a.go",
	}
	laterPlugin = plugin.UnitRef{
		Plugin: "weaver", Tag: "", Pkg: coretest.PackageID(coretest.CachePath), Key: "",
	}
	laterTag = plugin.UnitRef{
		Plugin: "stubgen", Tag: "zz", Pkg: coretest.PackageID(coretest.CachePath), Key: "",
	}
	laterPkg = plugin.UnitRef{
		Plugin: "stubgen", Tag: "test", Pkg: coretest.PackageID(coretest.StorePath + "/sub"), Key: "",
	}
	laterKey = plugin.UnitRef{
		Plugin: "stubgen", Tag: "test", Pkg: coretest.PackageID(coretest.StorePath), Key: "svc/store/b.go",
	}
)

// baseMatch is the key the match order cases move each field past.
var baseMatch = plugin.MatchKey{
	Plugin:   "stubgen",
	Rule:     1,
	Subject:  structID("Beta"),
	Instance: 1,
	Host:     plugin.EmitRef{Unit: baseUnit, Index: 3},
}

// The identities of a phase call's invocations are the keys a warm run
// re-executes by, so each order is total and pinned field by field.
func TestInvocation(t *testing.T) {
	t.Parallel()

	t.Run("UnitRef.Compare", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			later plugin.UnitRef
		}{
			{name: "orders by plugin before tag, package and key", later: laterPlugin},
			{name: "orders by tag before package and key", later: laterTag},
			{name: "orders by package before key", later: laterPkg},
			{name: "orders by key", later: laterKey},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, baseUnit.Compare(tt.later), -1, "the base sorts first")
				assert.Equal(t, tt.later.Compare(baseUnit), 1, "the order is antisymmetric")
			})
		}

		t.Run("returns zero for equal references", func(t *testing.T) {
			t.Parallel()

			same := baseUnit
			assert.Equal(t, baseUnit.Compare(same), 0, "a reference equals an equal one")
		})
	})

	t.Run("EmitRef.Compare", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name         string
			early, later plugin.EmitRef
		}{
			{
				name:  "orders by unit before place",
				early: plugin.EmitRef{Unit: baseUnit, Index: 9},
				later: plugin.EmitRef{Unit: laterKey, Index: 0},
			},
			{
				name:  "orders by place within one unit",
				early: plugin.EmitRef{Unit: baseUnit, Index: 2},
				later: plugin.EmitRef{Unit: baseUnit, Index: 3},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.early.Compare(tt.later), -1, "the earlier sorts first")
				assert.Equal(t, tt.later.Compare(tt.early), 1, "the order is antisymmetric")
			})
		}

		t.Run("returns zero for equal references", func(t *testing.T) {
			t.Parallel()

			ref, same := plugin.EmitRef{Unit: baseUnit, Index: 2}, plugin.EmitRef{Unit: baseUnit, Index: 2}
			assert.Equal(t, ref.Compare(same), 0, "a reference equals an equal one")
		})
	})

	t.Run("MatchKey.Compare", func(t *testing.T) {
		t.Parallel()

		later := func(edit func(k *plugin.MatchKey)) plugin.MatchKey {
			k := baseMatch
			edit(&k)
			return k
		}
		tests := []struct {
			name  string
			later plugin.MatchKey
		}{
			{
				name: "orders by plugin before every other field",
				later: plugin.MatchKey{
					Plugin: "weaver", Rule: plugin.WholeCall, Subject: structID("Alpha"),
				},
			},
			{
				name: "orders by rule before subject, instance and host",
				later: later(func(k *plugin.MatchKey) {
					k.Rule, k.Subject, k.Instance, k.Host = 2, structID("Alpha"), 0, plugin.EmitRef{}
				}),
			},
			{
				name: "orders by subject before instance and host",
				later: later(func(k *plugin.MatchKey) {
					k.Subject, k.Instance, k.Host = structID("Gamma"), 0, plugin.EmitRef{}
				}),
			},
			{
				name: "orders by instance before host",
				later: later(func(k *plugin.MatchKey) {
					k.Instance, k.Host = 2, plugin.EmitRef{}
				}),
			},
			{
				name: "orders by host",
				later: later(func(k *plugin.MatchKey) {
					k.Host = plugin.EmitRef{Unit: baseUnit, Index: 4}
				}),
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, baseMatch.Compare(tt.later), -1, "the base sorts first")
				assert.Equal(t, tt.later.Compare(baseMatch), 1, "the order is antisymmetric")
			})
		}

		t.Run("returns zero for equal keys", func(t *testing.T) {
			t.Parallel()

			same := baseMatch
			assert.Equal(t, baseMatch.Compare(same), 0, "a key equals an equal one")
		})

		t.Run("orders a whole call before every rule of its plugin", func(t *testing.T) {
			t.Parallel()

			whole := plugin.MatchKey{Plugin: baseMatch.Plugin, Rule: plugin.WholeCall}
			assert.Equal(t, whole.Compare(plugin.MatchKey{Plugin: baseMatch.Plugin}), -1,
				"the whole call's rule is below the first ordinal")
		})
	})
}

// The orders allocate nothing: a warm run sorts and searches its
// records by them. The checks run alone, because AllocsPerRun counts
// every goroutine's allocations and refuses to run beside parallel
// tests.
func TestInvocationZeroAlloc(t *testing.T) {
	later := baseMatch
	later.Host.Index++
	assert.MaxAllocs(t, func() {
		if baseMatch.Compare(later) >= 0 {
			t.Fatal("MatchKey.Compare puts the earlier host second")
		}
	}, 0, "MatchKey.Compare allocates nothing")
	assert.MaxAllocs(t, func() {
		if baseMatch.Host.Compare(later.Host) >= 0 {
			t.Fatal("EmitRef.Compare puts the earlier place second")
		}
	}, 0, "EmitRef.Compare allocates nothing")
	assert.MaxAllocs(t, func() {
		if baseUnit.Compare(laterKey) >= 0 {
			t.Fatal("UnitRef.Compare puts the earlier key second")
		}
	}, 0, "UnitRef.Compare allocates nothing")
}

// BenchmarkInvocation measures the orders over keys that agree on every
// field the order compares before the last, which is the longest
// comparison.
func BenchmarkInvocation(b *testing.B) {
	later := baseMatch
	later.Host.Index++

	b.Run("MatchKey.Compare", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got int
		for c.Loop() {
			got = baseMatch.Compare(later)
		}
		if got != -1 {
			b.Fatalf("MatchKey.Compare returns %d", got)
		}
	})

	b.Run("EmitRef.Compare", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got int
		for c.Loop() {
			got = baseMatch.Host.Compare(later.Host)
		}
		if got != -1 {
			b.Fatalf("EmitRef.Compare returns %d", got)
		}
	})

	b.Run("UnitRef.Compare", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got int
		for c.Loop() {
			got = baseUnit.Compare(laterKey)
		}
		if got != -1 {
			b.Fatalf("UnitRef.Compare returns %d", got)
		}
	})
}
