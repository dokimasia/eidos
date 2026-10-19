// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/graphql/backend"
	"go.dokimi.dev/eidos/sdk/render"
)

// TestImports pins the import block of a GraphQL file, which is empty.
func TestImports(t *testing.T) {
	t.Parallel()

	t.Run("Imports", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the empty block", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, backend.Imports(&render.ImportSet{}), "a GraphQL file has no imports")
		})
	})
}

// Imports allocates nothing. The ordinary run, which runs no benchmark,
// checks that ceiling here.
func TestImportsAllocs(t *testing.T) {
	set := &render.ImportSet{}
	var got string
	assert.MaxAllocs(t, func() { got = backend.Imports(set) }, 0, "Imports allocates nothing")
	assert.Empty(t, got, "Imports writes no block")
}

// BenchmarkImports measures the import block that every file writes.
func BenchmarkImports(b *testing.B) {
	b.Run("Imports", func(b *testing.B) {
		set := &render.ImportSet{}
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = backend.Imports(set)
		}
		assert.Empty(b, got, "Imports writes no block")
	})
}
