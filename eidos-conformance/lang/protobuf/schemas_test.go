// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protobuf_test

import (
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/conformance/lang/protobuf"
	protobuftesting "go.dokimi.dev/eidos/lang/protobuf/testing"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// schemasTree is the tree of the schemas fixture, whose directories are the
// versions of protobuf.
const schemasTree = "testdata/schemas"

// TestSchemas checks that the plan proto-mirror writes each schema of the
// tree of every protobuf version as its source states it, and that the
// toolchain adapter compiles what it writes.
func TestSchemas(t *testing.T) {
	t.Parallel()

	t.Run("SchemaPlans", func(t *testing.T) {
		t.Parallel()

		for _, version := range versions {
			t.Run("writes each schema of the "+version+" tree as its source states it", func(t *testing.T) {
				t.Parallel()

				root := schemasRoot(t, version)
				cleanBuild(t, root, protobuf.ComposeSchemas(root).Plans(protobuf.SchemaPlans()...))
				sources := protoPaths(t, filepath.Join(filepath.FromSlash(schemasTree), version))
				assert.NotEmpty(t, sources, "the tree of "+version+" has schemas")
				for _, p := range sources {
					src := files.Read(t, filepath.Join(root, filepath.FromSlash(p)))
					written := files.Read(t, filepath.Join(root, protobuf.MirrorDir, filepath.FromSlash(p)))
					expect.Equal(t, bodyOf(t, written), src, p+" is written as its source states it")
				}
			})

			t.Run("writes schemas of the "+version+" tree that the toolchain adapter compiles", func(t *testing.T) {
				t.Parallel()

				root := schemasRoot(t, version)
				cleanBuild(t, root, protobuf.ComposeSchemas(root).Plans(protobuf.SchemaPlans()...))
				toolchain.AssertTypeChecks(t.Context(), t, protobuftesting.New(), mirrored(t, root))
			})
		}
	})
}

// schemasRoot copies the schemas of one version into a new directory, and
// returns the directory.
func schemasRoot(t *testing.T, version string) string {
	t.Helper()

	root := t.TempDir()
	assert.NoError(t, os.CopyFS(root, os.DirFS(filepath.Join(filepath.FromSlash(schemasTree), version))),
		"the "+version+" tree copies")
	return root
}
