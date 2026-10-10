// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace_test

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/frontend"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/plugin"
)

// The stores that the locating fixtures return, and the variable that
// they read.
const (
	cacheStore = "cache"
	rootStore  = "root"
	storeVar   = "FAKEROOT"
)

// A run reads the stores of every frontend that locates its own.
func TestStores(t *testing.T) {
	t.Parallel()

	t.Run("Stores", func(t *testing.T) {
		t.Parallel()

		cache, root := fstest.MapFS{}, fstest.MapFS{}

		t.Run("returns the stores of each frontend that locates stores", func(t *testing.T) {
			t.Parallel()

			w := built(t, valid().Frontends(
				frontendtest.NewScripted(),
				locating("first", func(func(string) string) (map[string]fs.FS, error) {
					return map[string]fs.FS{cacheStore: cache}, nil
				}),
				locating("second", func(func(string) string) (map[string]fs.FS, error) {
					return map[string]fs.FS{rootStore: root}, nil
				}),
			))
			got, err := w.Stores(func(string) string { return "" })
			assert.NoError(t, err, "every locator finds its stores")
			assert.Length(t, got, 2, "the map has the stores of both locators")
			expect.Equal(t, got[cacheStore], fs.FS(cache), "the first locator returns the cache", assert.ByIdentity())
			expect.Equal(t, got[rootStore], fs.FS(root), "the second locator returns the root", assert.ByIdentity())
		})

		t.Run("returns an empty map for a composition without a locator", func(t *testing.T) {
			t.Parallel()

			w := built(t, valid().Frontends(frontendtest.NewScripted()))
			got, err := w.Stores(func(string) string { return "" })
			assert.NoError(t, err, "a composition without a locator has no stores")
			assert.Empty(t, got, "the map is empty")
		})

		t.Run("passes getenv to each locator", func(t *testing.T) {
			t.Parallel()

			var read string
			locate := func(getenv func(string) string) (map[string]fs.FS, error) {
				read = getenv(storeVar)
				return nil, nil
			}
			w := built(t, valid().Frontends(locating("first", locate)))
			_, err := w.Stores(func(key string) string { return key + "=/srv/cache" })
			assert.NoError(t, err, "the locator finds no stores")
			assert.Equal(t, read, storeVar+"=/srv/cache", "the locator reads the environment of the caller")
		})

		t.Run("returns the error of a locator after the name of its frontend", func(t *testing.T) {
			t.Parallel()

			missing := errors.New("workspace_test: set FAKEROOT")
			w := built(t, valid().Frontends(locating("first", func(func(string) string) (map[string]fs.FS, error) {
				return nil, missing
			})))
			_, err := w.Stores(func(string) string { return "" })
			assert.ErrorIs(t, err, missing, "the error wraps the error of the locator")
			assert.Equal(t, err.Error(), "workspace: frontend first: workspace_test: set FAKEROOT",
				"the error names the frontend")
		})

		t.Run("returns an error for a store that two frontends return", func(t *testing.T) {
			t.Parallel()

			twice := func(func(string) string) (map[string]fs.FS, error) {
				return map[string]fs.FS{cacheStore: cache}, nil
			}
			w := built(t, valid().Frontends(locating("first", twice), locating("second", twice)))
			_, err := w.Stores(func(string) string { return "" })
			assert.HasError(t, err, "one store name has one frontend")
			assert.Equal(t, err.Error(), `workspace: frontends first and second both return the store "cache"`,
				"the error names both frontends")
		})
	})
}

// locating returns a frontend of the scripted language with the name name,
// whose dependency rounds read the stores that locate returns.
func locating(name plugin.ID, locate func(func(string) string) (map[string]fs.FS, error)) plugin.Frontend {
	inner := frontendtest.NewScriptedDependent()
	return frontend.New(name, frontendtest.ScriptedLang, inner.Syntax()).
		Version("1").
		Match("**/*.zz").
		Units(inner.Partition).
		Parse(inner.Parse).
		Resolve(inner.Resolve).
		Dependencies(inner.Dependencies).
		Stores(locate).
		Build()
}
