// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protoc_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"

	"go.dokimi.dev/eidos/protoc"
)

// badCarrierSrc is the rows file with a carrier that the grammar of
// directives refuses.
const badCarrierSrc = "syntax = \"proto3\";\n\npackage acme.v1;\n\nimport \"acme/v1/shared.proto\";\n\n" +
	"// Row is one row.\n//+acme:gen:table name=\nmessage Row {\n  Shared shared = 1;\n}\n"

// crossSrc is the rows file in a package of its own, so its reference to
// Shared crosses into the package of the imported file.
const crossSrc = "syntax = \"proto3\";\n\npackage acme.rows.v1;\n\nimport \"acme/v1/shared.proto\";\n\n" +
	"// Row is one row.\nmessage Row {\n  acme.v1.Shared shared = 1;\n}\n"

func TestResponse(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("states the optional label of proto3 and the editions", func(t *testing.T) {
			t.Parallel()

			resp, _ := answered(t, request(t, ""), compose)
			features := uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL) |
				uint64(pluginpb.CodeGeneratorResponse_FEATURE_SUPPORTS_EDITIONS)
			expect.Equal(t, resp.GetSupportedFeatures(), features, "the host states both features")
			expect.Equal(t, resp.GetMinimumEdition(), int32(descriptorpb.Edition_EDITION_PROTO2), "from proto2")
			expect.Equal(t, resp.GetMaximumEdition(), int32(descriptorpb.Edition_EDITION_2026), "to edition 2026")
		})

		t.Run("opens the error field with the strategy for a reference into an imported file", func(t *testing.T) {
			t.Parallel()

			resp, _ := answered(t, requestOf(t, crossSrc, ""), goCompose)
			expect.That(t, resp.GetError()).
				HasPrefix(protoc.ErrImportedReference.Error(), "the response states the strategy first").
				Contains("error EID-0070:", "the response keeps the finding")
		})

		t.Run("returns each Error finding in the error field without a file", func(t *testing.T) {
			t.Parallel()

			resp, _ := answered(t, requestOf(t, badCarrierSrc, ""), compose)
			expect.That(t, resp.GetError()).Contains("acme/v1/rows.proto:8:1: error PROTO-0003:",
				"the finding is at the carrier")
			assert.Empty(t, resp.GetFile(), "no file is written")
		})
	})
}
