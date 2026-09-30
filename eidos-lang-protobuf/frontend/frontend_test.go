// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"context"
	"slices"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	protobuf "go.dokimi.dev/eidos/lang/protobuf"
	protofrontend "go.dokimi.dev/eidos/lang/protobuf/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// unparsedCode pins the code a syntax error reports under: the
// satellite's prefix, protobuf.CodePrefix, and the first number.
const unparsedCode = "PROTO-0001"

// The frontend's contract surface is what the load drives it
// through, so the identity, the claim and the unit grain are each
// pinned.
func TestFrontend(t *testing.T) {
	t.Parallel()

	f := protofrontend.New()

	t.Run("Name", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the satellite's name", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, f.Name(), protobuf.Name, "findings report under the satellite's identity")
		})
	})

	t.Run("Lang", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the satellite's language", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, f.Lang(), protobuf.Lang, "every loaded declaration is in the satellite's language")
			assert.Equal(t, protofrontend.Lang, protobuf.Lang, "the frontend restates it for its own callers")
		})
	})

	t.Run("Syntax", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the satellite's comment forms", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, f.Syntax(), protobuf.Syntax(), "the comment forms are the satellite's")
		})
	})

	t.Run("Overloads", func(t *testing.T) {
		t.Parallel()

		t.Run("reports false", func(t *testing.T) {
			t.Parallel()

			assert.False(t, f.Overloads(), "a service declares each RPC once by name")
		})
	})

	t.Run("Version", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the satellite's version", func(t *testing.T) {
			t.Parallel()

			versioned, held := f.(plugin.Versioned)
			assert.True(t, held, "every unit key folds a version, so the frontend states one")
			assert.Equal(t, versioned.Version(), protobuf.Version, "the version is the satellite's")
		})
	})

	t.Run("Selection", func(t *testing.T) {
		t.Parallel()

		t.Run("claims every proto file", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, f.Selection(), []string{"**/*.proto"}, "the claim negates nothing")
		})
	})

	t.Run("Partition", func(t *testing.T) {
		t.Parallel()

		t.Run("returns one unit per file in path order", func(t *testing.T) {
			t.Parallel()

			units, err := f.Partition(context.Background(), []plugin.SourceRef{
				{Path: "b/second.proto"}, {Path: "a/first.proto"},
			}, nil)
			assert.NoError(t, err, "the grain needs no bytes to settle")
			assert.Length(t, units, 2, "one file is one unit")
			assert.Equal(t, units[0][0].Path, "a/first.proto", "the units are sorted, so two runs partition alike")
			assert.Equal(t, units[1][0].Path, "b/second.proto", "the units are in path order")
			assert.Empty(t, units[0][0].Shared, "no unit declares a shared input")
		})

		t.Run("returns nothing for no files", func(t *testing.T) {
			t.Parallel()

			units, err := f.Partition(context.Background(), nil, nil)
			assert.NoError(t, err, "an empty claim partitions")
			assert.Empty(t, units, "there is no unit")
		})
	})

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the context's error for a cancelled load", func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			tree := fstest.MapFS{fixturePath: {Data: []byte("syntax = \"proto3\";\n")}}
			u := plugin.NewSourceUnit(
				[]plugin.SourceRef{{Path: fixturePath}}, tree, plugin.DepthFull,
				f.Syntax(), brand, sinkOf(), f.Name(),
			)
			assert.HasError(t, f.Parse(ctx, u), "a cancelled load stops before it parses")
		})

		t.Run("returns an error for a file it cannot read", func(t *testing.T) {
			t.Parallel()

			u := plugin.NewSourceUnit(
				[]plugin.SourceRef{{Path: "absent.proto"}}, fstest.MapFS{}, plugin.DepthFull,
				f.Syntax(), brand, sinkOf(), f.Name(),
			)
			assert.HasError(t, f.Parse(context.Background(), u),
				"a unit whose file is missing from the tree is the load's fault, not the schema's")
		})

		t.Run("reports a syntax error as PROTO-0001", func(t *testing.T) {
			t.Parallel()

			tree := fstest.MapFS{fixturePath: {Data: []byte("syntax = \"proto3\";\nmessage {\n")}}
			sink := sinkOf()
			u := plugin.NewSourceUnit(
				[]plugin.SourceRef{{Path: fixturePath}}, tree, plugin.DepthFull,
				f.Syntax(), brand, sink, f.Name(),
			)
			assert.NoError(t, f.Parse(context.Background(), u), "a syntax error is the schema's problem")
			found := slices.Collect(sink.All())
			assert.NotEmpty(t, found, "the syntax error reports")
			assert.Equal(t, found[0].Code.String(), unparsedCode, "under the satellite's prefix and the first number")
		})
	})
}
