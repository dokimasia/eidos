// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package descriptor_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	protofrontend "go.dokimi.dev/eidos/lang/protobuf/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
)

// The schemas of the comment cases: a carrier that a blank line detaches
// from the message below it, and a carrier on an import.
const (
	detachedFile = "parity/detached.proto"
	detachedSrc  = "syntax = \"proto3\";\n\npackage detached;\n\n//+fixture:table name=rows\n\n" +
		"message Row {\n  string id = 1;\n}\n"
	importedFile = "parity/imported.proto"
	importedSrc  = "syntax = \"proto3\";\n\npackage imported;\n\n//+fixture:table name=rows\n" +
		"import \"parity/detached.proto\";\n\nmessage Row {\n  detached.Row row = 1;\n}\n"
)

func TestSource(t *testing.T) {
	t.Parallel()

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		fds := compiledFrom(t, map[string]string{detachedFile: detachedSrc, importedFile: importedSrc})

		t.Run("reports UnaddressedCarrier for a carrier in a detached comment", func(t *testing.T) {
			t.Parallel()

			gb, sink, err := viaBytes(t, detachedFile, encoded(t, fds[detachedFile]), false)
			assert.NoError(t, err, "the parse succeeds")
			expect.Equal(t, codes(sink), []diag.Code{protofrontend.UnaddressedCarrier}, "the carrier is refused")
			expect.Empty(t, gb.Attachments(), "the carrier attaches nothing")
		})

		t.Run("reports UnaddressedCarrier for a carrier on an import", func(t *testing.T) {
			t.Parallel()

			gb, sink, err := viaBytes(t, importedFile, encoded(t, fds[importedFile]), false)
			assert.NoError(t, err, "the parse succeeds")
			expect.Equal(t, codes(sink), []diag.Code{protofrontend.UnaddressedCarrier}, "the carrier is refused")
			expect.Empty(t, gb.Attachments(), "the carrier attaches nothing")
		})

		t.Run("reports what the source frontend reports for a detached carrier", func(t *testing.T) {
			t.Parallel()

			fromSource, sourceSink := viaSource(t, detachedFile, detachedSrc)
			fromDescriptor, descriptorSink, err := viaBytes(t, detachedFile, encoded(t, fds[detachedFile]), false)
			assert.NoError(t, err, "the parse succeeds")
			expect.Equal(t, dump(fromDescriptor), dump(fromSource), "the two frontends build one graph")
			expect.Equal(t, codes(descriptorSink), codes(sourceSink), "the two frontends report the same findings")
		})
	})
}
