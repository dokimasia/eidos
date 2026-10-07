// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The comment split is the kernel's. What is pinned here is Go's
// own filtering over it, the constraint directive, and the folding
// across a group's comments, through the parsed output, black-box.
func TestSplit(t *testing.T) {
	t.Parallel()

	t.Run("split", func(t *testing.T) {
		t.Parallel()

		t.Run("filters go:build out of the annotations", func(t *testing.T) {
			t.Parallel()

			file := onlyFile(t, parsedFile(t, nil, plugin.DepthFull,
				"package p\n\n// D is documented.\n//go:build ignore\nvar D int\n"))
			d := file.Decls[0].(*node.Variable)
			assert.Empty(t, d.Annotations, "a constraint is configuration, never an annotation")
			assert.Equal(t, d.Doc, []string{"D is documented."}, "a constraint is never documentation")
		})

		t.Run("folds a continuation across the group's comments", func(t *testing.T) {
			t.Parallel()

			gb := parsedFile(t, nil, plugin.DepthFull,
				"package p\n\n// D is documented.\n// +fixture:gen:out user.go \\\n// plugin=buildergen\nvar D int\n")
			assert.Length(t, gb.Attachments(), 1, "one folded instance attaches")
			raw := gb.Attachments()[0].Raw
			assert.Equal(t, string(raw.Name), "gen:out", "the instance has its own name")
			assert.Length(t, raw.Args, 2, "the continued line's argument is joined")
			d := onlyFile(t, gb).Decls[0].(*node.Variable)
			assert.Equal(t, d.Doc, []string{"D is documented."}, "the continuation lines are no documentation")
		})

		t.Run("attaches a negated carrier as a negated instance", func(t *testing.T) {
			t.Parallel()

			gb := parsedFile(t, nil, plugin.DepthFull,
				"package p\n\n// D is documented.\n// -fixture:gen:out\nvar D int\n")
			assert.Length(t, gb.Attachments(), 1, "the instance attaches")
			assert.True(t, gb.Attachments()[0].Raw.Negated, "the instance is negated")
		})
	})
}
