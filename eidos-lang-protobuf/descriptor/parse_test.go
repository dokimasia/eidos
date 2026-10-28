// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package descriptor_test

import (
	"context"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/lang/protobuf/descriptor"
	protofrontend "go.dokimi.dev/eidos/lang/protobuf/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
)

// The proto paths of the parse cases.
const (
	unstableFile = "parity/unstable.proto"
	brokenFile   = "parity/broken.proto"
	proto3File   = "parity/proto3.proto"
	customFile   = "parity/custom.proto"
)

func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		fds := compiled(t)

		t.Run("returns an error for bytes that are no descriptor", func(t *testing.T) {
			t.Parallel()

			_, _, err := viaBytes(t, brokenFile, []byte{0xff}, false)
			assert.HasError(t, err, "the bytes do not decode")
		})

		t.Run("returns the error of a context that is done", func(t *testing.T) {
			t.Parallel()

			f := descriptor.New()
			assert.HonoursCancellation(t, func(ctx context.Context) error { return f.Parse(ctx, nil) },
				"a done context ends the parse")
		})

		t.Run("reports UnknownEdition for an edition outside the table", func(t *testing.T) {
			t.Parallel()

			fd := &descriptorpb.FileDescriptorProto{
				Name:        new(unstableFile),
				Syntax:      new("editions"),
				Edition:     descriptorpb.Edition_EDITION_UNSTABLE.Enum(),
				MessageType: []*descriptorpb.DescriptorProto{{Name: new("Row")}},
			}
			gb, sink, err := viaBytes(t, unstableFile, encoded(t, fd), false)
			assert.NoError(t, err, "the parse succeeds")
			expect.Equal(t, codes(sink), []diag.Code{protofrontend.UnknownEdition}, "the edition is refused")
			expect.Empty(t, gb.Packages(), "the file loads no declaration")
		})

		t.Run("reports MissingSourceInfo for a file of the workspace without source code info", func(t *testing.T) {
			t.Parallel()

			fd := proto.CloneOf(fds[proto3File])
			fd.SourceCodeInfo = nil
			gb, sink, err := viaBytes(t, proto3File, encoded(t, fd), false)
			assert.NoError(t, err, "the parse succeeds")
			expect.Equal(
				t,
				codes(sink),
				[]diag.Code{descriptor.MissingSourceInfo},
				"the loss of the comments is reported",
			)
			expect.NotEmpty(t, gb.Packages(), "the file loads its declarations")
		})

		t.Run("reports nothing for a file of a dependency without source code info", func(t *testing.T) {
			t.Parallel()

			fd := proto.CloneOf(fds[proto3File])
			fd.SourceCodeInfo = nil
			_, sink, err := viaBytes(t, proto3File, encoded(t, fd), true)
			assert.NoError(t, err, "the parse succeeds")
			assert.Empty(t, codes(sink), "a dependency reports no finding")
		})

		t.Run("stamps no custom option without the registry", func(t *testing.T) {
			t.Parallel()

			gb, _, err := viaBytes(t, customFile, encoded(t, fds[customFile]), false)
			assert.NoError(t, err, "the parse succeeds")
			assert.Empty(t, stamped(gb, string(protobuf.OptionsKey)), "the option is unknown without its extension")
		})

		t.Run("returns an error for a file that the tree does not contain", func(t *testing.T) {
			t.Parallel()

			assert.HasError(t, parseErr(t, customFile, nil, nil), "the file does not read")
		})

		t.Run("returns an error for a registry that does not decode", func(t *testing.T) {
			t.Parallel()

			err := parseErr(t, customFile, encoded(t, fds[customFile]), []byte{0xff})
			assert.HasError(t, err, "the registry does not decode")
			expect.That(t, err.Error()).Contains("the registry:", "the error names the registry")
		})

		t.Run("returns an error for a registry whose files do not build", func(t *testing.T) {
			t.Parallel()

			lone := registryOf(t, map[string]*descriptorpb.FileDescriptorProto{customFile: fds[customFile]})
			err := parseErr(t, customFile, encoded(t, fds[customFile]), lone)
			assert.HasError(t, err, "the import of the file is absent")
			expect.That(t, err.Error()).
				Contains("the files of the registry:", "the error names the files of the registry")
		})
	})
}
