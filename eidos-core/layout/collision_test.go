// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package layout_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/layout"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// Routed paths that one tree cannot contain refuse the declarations
// routed to them, and every other file renders.
func TestCollision(t *testing.T) {
	t.Parallel()

	t.Run("Route", func(t *testing.T) {
		t.Parallel()

		collisions := []struct {
			name string
			give func() *fixture
		}{
			{
				name: "reports PathCollision for declarations of two packages routed to one file",
				give: func() *fixture {
					return newFixture(storeStub(), cacheStub()).
						on(storeID, written(stubDirective, 1, keyOut, "../shared/stub.go")).
						on(cacheID, written(stubDirective, 1, keyOut, "../shared/stub.go"))
				},
			},
			{
				name: "reports PathCollision for two paths that differ only in case",
				give: func() *fixture {
					return newFixture(storeStub(), rowStub()).
						on(storeID, written(stubDirective, 1, keyOut, "Stub.go")).
						on(rowID, written(stubDirective, 1, keyOut, "stub.go"))
				},
			},
			{
				name: "reports PathCollision for a file under a path routed as a file",
				give: func() *fixture {
					return newFixture(storeStub(), rowStub()).
						on(storeID, written(stubDirective, 1, keyOut, "../mocks")).
						on(rowID, written(stubDirective, 1, keyOut, "../mocks/"))
				},
			},
			{
				name: "reports PathCollision for a file at a path another file needs as a directory",
				give: func() *fixture {
					return newFixture(storeStub(), rowStub()).
						on(storeID, written(stubDirective, 1, keyOut, "../Mocks/")).
						on(rowID, written(stubDirective, 1, keyOut, "../mocks"))
				},
			},
		}
		for _, tt := range collisions {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				files, sink := tt.give().route(t)
				assert.Empty(t, files, "the declarations of both files are refused")
				coretest.AssertCodes(t, sink, layout.PathCollision)
			})
		}

		t.Run("returns the files beside a collision", func(t *testing.T) {
			t.Parallel()

			files, sink := newFixture(storeStub(), rowStub(), cacheStub()).
				on(storeID, written(stubDirective, 1, keyOut, "Stub.go")).
				on(rowID, written(stubDirective, 1, keyOut, "stub.go")).route(t)
			assert.Equal(t, layoutOf(files), []string{"svc/cache/cache_stub.go: CacheStub"},
				"the file outside the collision routes")
			coretest.AssertCodes(t, sink, layout.PathCollision)
		})

		t.Run("reports a clash at the second file's origin with the first's as related", func(t *testing.T) {
			t.Parallel()

			_, sink := newFixture(storeStub(), rowStub()).
				on(storeID, written(stubDirective, 1, keyOut, "Stub.go")).
				on(rowID, written(stubDirective, 1, keyOut, "stub.go")).route(t)
			for d := range sink.All() {
				assert.Equal(t, d.Pos, at(rowFile), "svc/store/stub.go sorts second")
				assert.Equal(t, d.Related, []position.Pos{at(storeFile)}, "svc/store/Stub.go sorts first")
			}
		})

		t.Run("reports two packages at the second unit's origin with the first's as related", func(t *testing.T) {
			t.Parallel()

			_, sink := newFixture(storeStub(), cacheStub()).
				on(storeID, written(stubDirective, 1, keyOut, "../shared/stub.go")).
				on(cacheID, written(stubDirective, 1, keyOut, "../shared/stub.go")).route(t)
			for d := range sink.All() {
				assert.Equal(t, d.Pos, at(storeFile), "the store unit sorts after the cache unit")
				assert.Equal(t, d.Related, []position.Pos{at(cacheFile)}, "the cache unit sorts first")
			}
		})

		t.Run("returns a plan unit and a package's unit at one path as one file", func(t *testing.T) {
			t.Parallel()

			f := newFixture(storeStub(),
				unitOf(stubgen, families()[stubgen][3], "", symbol.Identity{}, generated(storeID, "StoreIndex")))
			f.config = layout.Config{Families: map[layout.Family]layout.Refinement{
				{Plugin: stubgen, Tag: tagPlan}: {Dir: storePkg, File: "store_stub.go"},
			}}
			files, sink := f.route(t)
			assert.Equal(t, layoutOf(files), []string{"svc/store/store_stub.go: StoreStub StoreIndex"},
				"a plan unit derives from no package")
			coretest.AssertCodes(t, sink)
		})
	})
}

// rowStub returns stubgen's primary unit of row.go with one stub of
// Row.
func rowStub() plugin.Unit {
	return stubOf(rowFile, storePkg, generated(rowID, "RowStub"))
}

// cacheStub returns stubgen's primary unit of cache.go with one stub
// of Cache.
func cacheStub() plugin.Unit {
	return stubOf(cacheFile, cachePkg, generated(cacheID, "CacheStub"))
}
