// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package tree_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/reference/internal/tree"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The target of the cases, the name of its template, the text of the
// template, and a target whose directory leaves the directory templates.
const (
	target   plugin.Target = "golang"
	name                   = "middleware.tmpl"
	text                   = "return h\n"
	escaping plugin.Target = "../../escaping"
)

// Of returns the directory of one target below the directory templates.
func TestTree(t *testing.T) {
	t.Parallel()

	t.Run("Of", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the templates of the target under their names", func(t *testing.T) {
			t.Parallel()

			files := fstest.MapFS{tree.Dir + "/" + string(target) + "/" + name: {Data: []byte(text)}}
			got, err := fs.ReadFile(tree.Of(files, target), name)
			assert.NoError(t, err, "the tree of the target contains the template")
			assert.Equal(t, string(got), text, "the template has its text")
		})

		t.Run("panics for a target whose directory leaves the directory templates", func(t *testing.T) {
			t.Parallel()

			assert.Panics(
				t,
				func() { tree.Of(fstest.MapFS{}, escaping) },
				"the directory of the target is no valid path",
			)
		})
	})
}
