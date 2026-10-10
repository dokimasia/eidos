// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package declarative_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/declarative"
)

// The template tree of a target is a file system over the plugin
// directory: each open opens its file in the directory, so the tree reads
// the templates as they are at each read.
func TestTree(t *testing.T) {
	t.Parallel()

	t.Run("Open", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the conformance test of a file system", func(t *testing.T) {
			t.Parallel()

			tree, _ := goTree(t)
			assert.NoError(t, fstest.TestFS(tree, functionTemplate, methodTemplate, testTemplate, "helper.tmpl"),
				"the tree meets the contract of io/fs")
		})

		t.Run("returns ErrInvalid for a name that is not a valid path", func(t *testing.T) {
			t.Parallel()

			tree, _ := goTree(t)
			_, err := tree.Open("../" + functionTemplate)
			assert.ErrorIs(t, err, fs.ErrInvalid, "a name that leaves the tree is no name of the tree")
		})

		t.Run("returns ErrNotExist for a template that the tree does not contain", func(t *testing.T) {
			t.Parallel()

			tree, _ := goTree(t)
			_, err := tree.Open("absent.tmpl")
			assert.ErrorIs(t, err, fs.ErrNotExist, "the open reports the missing file")
		})

		t.Run("reads a template that changed after the load", func(t *testing.T) {
			t.Parallel()

			tree, dir := goTree(t)
			edited := functionBody + "{{/* edited */}}"
			assert.NoError(t, os.WriteFile(filepath.Join(dir, filepath.FromSlash(goFunction)), []byte(edited), 0o600),
				"the template changes on disk")
			got, err := fs.ReadFile(tree, functionTemplate)
			assert.NoError(t, err, "the template reads")
			assert.Equal(t, string(got), edited, "the tree is no copy of the templates")
		})

		t.Run("returns an error for a link that leaves the directory", func(t *testing.T) {
			t.Parallel()

			tree, dir := goTree(t)
			outside := files.Workspace(t, files.Tree{"secret.tmpl": files.Text("secret")})
			assert.NoError(
				t,
				os.Symlink(
					filepath.Join(outside, "secret.tmpl"),
					filepath.Join(dir, "templates", "golang", "escape.tmpl"),
				),
				"the link is created after the load",
			)
			_, err := fs.ReadFile(tree, "escape.tmpl")
			assert.HasError(t, err, "the read does not leave the plugin directory")
		})
	})
}

// goTree loads the plugin directory of auditTree, and returns the
// generator's template tree of golang and the plugin directory.
func goTree(tb testing.TB) (fs.FS, string) {
	tb.Helper()

	dir := files.Workspace(tb, auditTree())
	g, err := declarative.Load(dir)
	assert.NoError(tb, err, "the plugin directory loads")
	provider, is := g.(plugin.TemplateProvider)
	assert.True(tb, is, "the generator declares template trees")
	tree, declared := provider.Templates(goTarget)
	assert.True(tb, declared, "the generator declares a tree for golang")
	return tree, dir
}
