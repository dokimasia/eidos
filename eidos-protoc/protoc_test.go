// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protoc_test

import (
	"bytes"
	"errors"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"google.golang.org/protobuf/proto"

	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/protoc"
)

// misspelledSrc is the rows file with a carrier whose mark is near the
// brand and is not the brand.
const misspelledSrc = "syntax = \"proto3\";\n\npackage acme.v1;\n\nimport \"acme/v1/shared.proto\";\n\n" +
	"// Row is one row.\n//+acne:gen:table\nmessage Row {\n  Shared shared = 1;\n}\n"

func TestProtoc(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the file of each file to generate", func(t *testing.T) {
			t.Parallel()

			resp, _ := answered(t, request(t, ""), compose)
			assert.Equal(t, resp.GetError(), "", "the run reports no error")
			assert.Length(t, resp.GetFile(), 1, "the imported file is not generated")
			expect.Equal(t, resp.GetFile()[0].GetName(), rowsList, "the file is beside its proto file")
			expect.That(t, resp.GetFile()[0].GetContent()).
				Contains("type RowList struct{}", "the file lists the message")
		})

		t.Run("sets an option of a plugin from the parameter", func(t *testing.T) {
			t.Parallel()

			resp, _ := answered(t, request(t, "lister.suffix=Table"), compose)
			assert.Length(t, resp.GetFile(), 1, "the file is generated")
			assert.That(t, resp.GetFile()[0].GetContent()).
				Contains("type RowTable struct{}", "the option names the struct")
		})

		t.Run("writes each Warning of the run to the log", func(t *testing.T) {
			t.Parallel()

			resp, log := answered(t, requestOf(t, misspelledSrc, ""), compose)
			assert.Equal(t, resp.GetError(), "", "a warning fails nothing")
			assert.That(t, log).Contains("acme/v1/rows.proto:8:1: warning EID-0083:",
				"the log has the warning of the mark near the brand")
		})

		t.Run("returns an error for a request that does not decode", func(t *testing.T) {
			t.Parallel()

			var out, log bytes.Buffer
			err := protoc.Run(t.Context(), bytes.NewReader([]byte{0xff}), &out, &log, compose)
			assert.HasError(t, err, "the request does not decode")
			assert.Equal(t, out.Len(), 0, "no response is written")
		})

		t.Run("returns an error for a composition that does not build", func(t *testing.T) {
			t.Parallel()

			in, err := proto.Marshal(request(t, ""))
			assert.NoError(t, err, "the request encodes")
			var out, log bytes.Buffer
			err = protoc.Run(t.Context(), bytes.NewReader(in), &out, &log, workspace.New)
			assert.HasError(t, err, "a composition without a brand does not build")
		})

		t.Run("returns the error of a response that does not write", func(t *testing.T) {
			t.Parallel()

			in, err := proto.Marshal(request(t, ""))
			assert.NoError(t, err, "the request encodes")
			var log bytes.Buffer
			err = protoc.Run(t.Context(), bytes.NewReader(in), failingWriter{}, &log, compose)
			assert.ErrorIs(t, err, errWrite, "the write fails")
		})
	})
}

// errWrite is the failure of the writer of a response.
var errWrite = errors.New("the response does not write")

// failingWriter is a writer that fails every write.
type failingWriter struct{}

// Write fails with errWrite.
func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }
