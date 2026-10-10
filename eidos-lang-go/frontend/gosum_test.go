// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
)

// go.sum records two hashes of a module, and the round verifies the
// module cache against one, so which line decides is pinned.
func TestGoSum(t *testing.T) {
	t.Parallel()

	t.Run("parseSums", func(t *testing.T) {
		t.Parallel()

		t.Run("verifies against the tree's hash and not the go.mod's", func(t *testing.T) {
			t.Parallel()

			tree := depWorkspace()
			tree[appGoSum] = &fstest.MapFile{Data: []byte(
				libModule + " " + libVersion + goModField + " " + otherHash + "\n" +
					libModule + " " + libVersion + " " + libHash + "\n")}
			assert.NotEmpty(t, placed(t, tree, depStores(), libPackage), "the tree's hash matches the cache's")
		})

		t.Run("leaves out a line of another shape", func(t *testing.T) {
			t.Parallel()

			tree := depWorkspace()
			tree[appGoSum] = &fstest.MapFile{Data: []byte(
				libModule + " " + libVersion + "\n" + libModule + " " + libVersion + " " + libHash + " extra\n")}
			assert.Contains(t, refused(t, tree, depStores(), libPackage), "records a hash",
				"neither line records a tree hash")
		})

		t.Run("keeps the first go.sum's hash of a module", func(t *testing.T) {
			t.Parallel()

			tree := depWorkspace()
			tree[svcGoMod] = &fstest.MapFile{Data: []byte("module example.test/svc\n\nrequire " +
				libModule + " " + libVersion + "\n")}
			tree[svcGoSum] = &fstest.MapFile{Data: []byte(libModule + " " + libVersion + " " + otherHash + "\n")}
			assert.NotEmpty(t, placed(t, tree, depStores(), libPackage),
				"go.mod sorts before svc/go.mod, so its go.sum's hash is the one checked")
		})
	})
}
