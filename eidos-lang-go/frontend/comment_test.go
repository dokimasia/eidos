// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The comment split is the kernel's; what is pinned here is Go's
// own filtering over it — the legacy constraint form and the
// constraint directive — through the parsed output, black-box.
func TestSplit(t *testing.T) {
	t.Parallel()

	t.Run("filters go:build out of the annotations", func(t *testing.T) {
		t.Parallel()

		file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
			"package p\n\n// D holds.\n//go:build ignore\nvar D int\n"))
		d := file.Decls[0].(*node.Variable)
		assert.Length(t, d.Annotations, 0,
			"a constraint is configuration, never an annotation")
		assert.Equal(t, d.Doc, []string{"D holds."}, "and never documentation")
	})

	t.Run("filters the legacy build form out of the carriers", func(t *testing.T) {
		t.Parallel()

		gb := parsedFile(t, nil, plugin.DepthFull,
			"package p\n\n// D holds.\n// +build linux\nvar D int\n")
		assert.Length(t, gb.Attachments(), 0,
			"a legacy line never attaches a directive named build")
		d := onlyFile(t, gb).Decls[0].(*node.Variable)
		assert.Equal(t, d.Doc, []string{"D holds."}, "and is not documentation")
	})

	t.Run("folds a continuation across the group's comments", func(t *testing.T) {
		t.Parallel()

		gb := parsedFile(t, nil, plugin.DepthFull,
			"package p\n\n// D holds.\n// +gen:out user.go \\\n// plugin=buildergen\nvar D int\n")
		assert.Length(t, gb.Attachments(), 1, "one folded instance attaches")
		raw := gb.Attachments()[0].Raw
		assert.Equal(t, string(raw.Name), "gen:out", "under its own name")
		assert.Length(t, raw.Args, 2, "the continued line's argument arrives joined")
		d := onlyFile(t, gb).Decls[0].(*node.Variable)
		assert.Equal(t, d.Doc, []string{"D holds."},
			"the continuation lines are spent, not documentation")
	})

	t.Run("keeps a carrier whose name begins with build", func(t *testing.T) {
		t.Parallel()

		gb := parsedFile(t, nil, plugin.DepthFull,
			"package p\n\n//+builder name=b\nvar D int\n")
		assert.Length(t, gb.Attachments(), 1,
			"the legacy filter matches the form, not the prefix")
		assert.Equal(t, string(gb.Attachments()[0].Raw.Name), "builder", "as written")
	})
}
