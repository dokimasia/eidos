// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package graphql_test

import (
	"path"
	"path/filepath"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/conformance/lang/graphql"
	graphqllang "go.dokimi.dev/eidos/lang/graphql"
	graphqltesting "go.dokimi.dev/eidos/lang/graphql/testing"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The mark that opens a carrier of the fixture, the suffix of the stem of a
// mirror, the configuration of the catalog, and the documents with the
// descriptions of executable definitions that a case adds: an operation with
// a described variable, and an anonymous query.
const (
	carrierMark     = "#+" + string(graphql.Brand) + ":"
	mirrorSuffix    = "_mirror"
	configPath      = "catalog/.graphqlrc.yml"
	describedPath   = "catalog/documents/described.graphql"
	describedSource = "\"Reads one product.\"\nquery Described(\n  \"The key of the product.\"\n  $id: ID!\n) {\n" +
		"  product(id: $id) {\n    id\n  }\n}\n"
	versionPath   = "catalog/documents/version.graphql"
	versionSource = "\"Reads the version of the catalog.\"\nquery {\n  version\n}\n"
)

// TestMirror checks that the plan graphql-mirror writes each file of the
// catalog as its source states it, that the toolchain adapter validates what
// it writes, and that the frontend loads the mirror back with the stamps of
// its sources.
func TestMirror(t *testing.T) {
	t.Parallel()

	t.Run("MirrorPlan", func(t *testing.T) {
		t.Parallel()

		t.Run("writes each file of the catalog as its source states it", func(t *testing.T) {
			t.Parallel()

			root := catalogRoot(t)
			mirrors := mirrorsOf(t, root)
			assert.NotEmpty(t, mirrors, "the catalog has GraphQL files")
			cleanRun(t, root, graphql.CatalogPlans()...)
			for source, mirror := range mirrors {
				// A carrier is a directive of the run, which no declaration of
				// the model states, so the mirror does not restate it.
				var want strings.Builder
				for line := range strings.Lines(files.Read(t, filepath.Join(root, filepath.FromSlash(source)))) {
					if !strings.HasPrefix(strings.TrimSpace(line), carrierMark) {
						want.WriteString(line)
					}
				}
				written := files.Read(t, filepath.Join(root, filepath.FromSlash(mirror)))
				expect.Equal(t, bodyOf(t, written), want.String(), source+" is written as its source states it")
			}
		})

		t.Run("writes a mirror that the toolchain adapter validates", func(t *testing.T) {
			t.Parallel()

			root := catalogRoot(t)
			mirrors := mirrorsOf(t, root)
			cleanRun(t, root, graphql.CatalogPlans()...)
			g := toolchain.Generated{Files: map[string][]byte{}}
			for _, mirror := range mirrors {
				g.Files[mirror] = []byte(files.Read(t, filepath.Join(root, filepath.FromSlash(mirror))))
			}
			toolchain.AssertTypeChecks(t.Context(), t, graphqltesting.New(), g)
		})

		t.Run("writes the mirror of each file again from the mirror", func(t *testing.T) {
			t.Parallel()

			root := catalogRoot(t)
			mirrors := mirrorsOf(t, root)
			cleanRun(t, root, graphql.CatalogPlans()...)
			// The second tree has each mirror without its frame, because a
			// load excludes the files that the workspace stamped. A mirror has
			// no carriers, so the mirror plan runs alone.
			again := files.Tree{
				configPath: files.Text(files.Read(t, filepath.Join(root, filepath.FromSlash(configPath)))),
			}
			firsts := map[string]string{}
			for _, mirror := range mirrors {
				first := bodyOf(t, files.Read(t, filepath.Join(root, filepath.FromSlash(mirror))))
				p := strings.TrimPrefix(mirror, graphql.MirrorDir+"/")
				again[p] = files.Text(first)
				firsts[p] = first
			}
			second := files.Workspace(t, again)
			remirrors := mirrorsOf(t, second)
			cleanRun(t, second, graphql.MirrorPlan())
			for p, remirror := range remirrors {
				written := files.Read(t, filepath.Join(second, filepath.FromSlash(remirror)))
				expect.Equal(t, bodyOf(t, written), firsts[p], p+" loads back with the stamps of its source")
			}
		})

		t.Run("writes the descriptions of executable definitions as their source states them", func(t *testing.T) {
			t.Parallel()

			root := catalogRoot(t)
			files.Write(t, root, files.Tree{
				describedPath: files.Text(describedSource), versionPath: files.Text(versionSource),
			})
			mirrors := mirrorsOf(t, root)
			cleanRun(t, root, graphql.CatalogPlans()...)
			described := files.Read(t, filepath.Join(root, filepath.FromSlash(mirrors[describedPath])))
			expect.Equal(t, bodyOf(t, described), describedSource,
				"the September 2025 edition describes an operation and its variables")
			version := files.Read(t, filepath.Join(root, filepath.FromSlash(mirrors[versionPath])))
			expect.Equal(t, bodyOf(t, version), versionSource,
				"an anonymous query with a description keeps its keyword")
		})
	})
}

// mirrorsOf returns the slash-separated path of the mirror of each GraphQL
// file of the tree at root, keyed by the path of the file. The mirror of a
// file is the file <stem>_mirror.graphql under the mirror directory, at the
// directory of the file. A case calls mirrorsOf before the run, so the tree
// has no mirror yet.
func mirrorsOf(t *testing.T, root string) map[string]string {
	t.Helper()

	out := map[string]string{}
	for _, p := range graphqlPaths(t, root) {
		out[p] = path.Join(graphql.MirrorDir, strings.TrimSuffix(p, path.Ext(p))+mirrorSuffix+graphqllang.Extension)
	}
	return out
}
