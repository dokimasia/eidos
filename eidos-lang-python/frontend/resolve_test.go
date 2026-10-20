// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/python/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// resolveAllocs is the resolution of a name that an import binds: the list
// of its candidates and the list of its tiers, which share the first list.
const resolveAllocs = 2

// TestResolve pins the two doors of the resolution of a file: what a
// spelling could mean, and what a module publishes.
func TestResolve(t *testing.T) {
	t.Parallel()

	t.Run("resolve", func(t *testing.T) {
		t.Parallel()

		t.Run("resolves nothing in a file whose parse recorded no scope", func(t *testing.T) {
			t.Parallel()

			assert.Nil(t, frontend.New(nil).Resolve(plugin.ImportScope{}, "Order"), "a scope of another language")
		})
	})

	t.Run("exports", func(t *testing.T) {
		t.Parallel()

		t.Run("publishes nothing in a file whose parse recorded no scope", func(t *testing.T) {
			t.Parallel()

			exporter, _ := frontend.New(nil).(plugin.Exporter)
			assert.Nil(t, exporter.Exports(plugin.ImportScope{}, "Order"), "a scope of another language")
		})
	})
}

// TestResolveAllocs checks the ceiling of the resolution of a name in the
// ordinary run, which runs no benchmark.
func TestResolveAllocs(t *testing.T) {
	checkAllocs(t, resolveCalls(t))
}

// BenchmarkResolve measures the resolution that the load makes for each
// reference of the graph.
func BenchmarkResolve(b *testing.B) {
	benchCalls(b, resolveCalls(b))
}

// resolveCalls returns a call of Resolve for a name that an import binds.
func resolveCalls(tb testing.TB) []allocCall {
	tb.Helper()

	f := frontend.New(nil)
	s := scopeOf(tb, scopeSource)
	var got plugin.Candidates
	return []allocCall{
		{
			name: "Resolve", allocs: resolveAllocs,
			call: func() { got = f.Resolve(s, "Order") },
			check: func(tb assert.TB) {
				tb.Helper()

				assert.Length(tb, got, 3, "the import, the star import and builtins")
			},
		},
	}
}
