// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"context"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	protobuf "go.dokimi.dev/eidos/lang/protobuf"
	protofrontend "go.dokimi.dev/eidos/lang/protobuf/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The frontend's contract surface is what the load drives it
// through, so the identity, the claim and the unit grain are each
// pinned.
func TestFrontend(t *testing.T) {
	t.Parallel()

	f := protofrontend.New()

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, f.Name(), protobuf.Name, "findings report under the satellite's identity")
	})

	t.Run("Lang", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, f.Lang(), protobuf.Lang, "every declaration the frontend loads is in the satellite's language")
		assert.Equal(t, protofrontend.Lang, protobuf.Lang, "restated for the frontend's own callers")
	})

	t.Run("Syntax", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, f.Syntax(), protobuf.Syntax(), "the comment forms are the satellite's")
	})

	t.Run("Version", func(t *testing.T) {
		t.Parallel()

		versioned, held := f.(plugin.Versioned)
		assert.True(t, held, "every unit key folds a version, so the frontend states one")
		assert.Equal(t, versioned.Version(), protobuf.Version, "the satellite's own")
	})

	t.Run("Selection", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, f.Selection(), []string{"**/*.proto"},
			"every proto file is claimed, and nothing is negated")
	})

	t.Run("Partition", func(t *testing.T) {
		t.Parallel()

		t.Run("returns one unit per file, in path order", func(t *testing.T) {
			t.Parallel()

			units, err := f.Partition(context.Background(), []plugin.SourceRef{
				{Path: "b/second.proto"}, {Path: "a/first.proto"},
			}, nil)
			assert.NoError(t, err, "the grain needs no bytes to settle")
			assert.Length(t, units, 2, "one file is one unit")
			assert.Equal(t, units[0][0].Path, "a/first.proto", "sorted, so two runs partition alike")
			assert.Equal(t, units[1][0].Path, "b/second.proto", "in path order")
			assert.Empty(t, units[0][0].Shared, "and no unit declares a shared input")
		})

		t.Run("returns nothing for no files", func(t *testing.T) {
			t.Parallel()

			units, err := f.Partition(context.Background(), nil, nil)
			assert.NoError(t, err, "an empty claim partitions")
			assert.Empty(t, units, "into no units")
		})
	})

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the context's error and no half-built unit", func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			tree := fstest.MapFS{fixturePath: {Data: []byte("syntax = \"proto3\";\n")}}
			u := plugin.NewSourceUnit(
				[]plugin.SourceRef{{Path: fixturePath}}, tree, plugin.DepthFull,
				f.Syntax(), sinkOf(), f.Name(),
			)
			assert.HasError(t, f.Parse(ctx, u), "a cancelled load stops before it parses")
		})

		t.Run("reports a file it cannot read", func(t *testing.T) {
			t.Parallel()

			u := plugin.NewSourceUnit(
				[]plugin.SourceRef{{Path: "absent.proto"}}, fstest.MapFS{}, plugin.DepthFull,
				f.Syntax(), sinkOf(), f.Name(),
			)
			assert.HasError(t, f.Parse(context.Background(), u),
				"a unit whose file is missing from the tree is the load's fault, not the schema's")
		})
	})
}
