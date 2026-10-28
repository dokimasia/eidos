// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protoc_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
	"testing/iotest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"

	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/protoc"
)

// misspelledSrc is the rows file with a carrier whose mark is near the
// brand and is not the brand.
const misspelledSrc = "syntax = \"proto3\";\n\npackage acme.v1;\n\nimport \"acme/v1/shared.proto\";\n\n" +
	"// Row is one row.\n//+acne:gen:table\nmessage Row {\n  Shared shared = 1;\n}\n"

func TestProtoc(t *testing.T) {
	t.Parallel()

	t.Run("Main", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the response to standard output and exits with status 0", func(t *testing.T) {
			t.Parallel()

			in, err := proto.Marshal(request(t, ""))
			assert.NoError(t, err, "the request encodes")
			out, _, status := mainRun(t, in)
			assert.Equal(t, status, 0, "the plugin succeeds")
			resp := &pluginpb.CodeGeneratorResponse{}
			assert.NoError(t, proto.Unmarshal(out, resp), "the response decodes")
			expect.Equal(t, resp.GetFile()[0].GetName(), rowsList, "the response has the file")
		})

		t.Run("exits with status 1 and the error on standard error for a request that does not decode",
			func(t *testing.T) {
				t.Parallel()

				out, errOut, status := mainRun(t, []byte{0xff})
				assert.Equal(t, status, 1, "the plugin fails")
				expect.Empty(t, out, "no response is written")
				expect.That(t, errOut).Contains("protoc: decode the request", "the error names the step")
			})
	})

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

		t.Run("returns the error of a request that does not read", func(t *testing.T) {
			t.Parallel()

			var out, log bytes.Buffer
			err := protoc.Run(t.Context(), iotest.ErrReader(errRead), &out, &log, compose)
			assert.ErrorIs(t, err, errRead, "the read fails")
		})

		t.Run("returns the error of a run that a cancellation ends", func(t *testing.T) {
			t.Parallel()

			in, err := proto.Marshal(request(t, ""))
			assert.NoError(t, err, "the request encodes")
			assert.HonoursCancellation(t, func(ctx context.Context) error {
				var out, log bytes.Buffer
				return protoc.Run(ctx, bytes.NewReader(in), &out, &log, compose)
			}, "a cancelled run returns its error")
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

// mainEnv makes the test binary run [protoc.Main] with the composition of
// the specs, as a plugin binary does.
const mainEnv = "PROTOC_SPEC_MAIN"

func TestMain(m *testing.M) {
	if os.Getenv(mainEnv) != "" {
		protoc.Main(compose)
		return
	}
	os.Exit(m.Run())
}

// mainRun runs the test binary as a plugin over stdin, and returns its
// standard output, its standard error and its exit status.
func mainRun(tb testing.TB, stdin []byte) ([]byte, string, int) {
	tb.Helper()

	cmd := exec.CommandContext(tb.Context(), os.Args[0])
	cmd.Env = append(os.Environ(), mainEnv+"=1")
	cmd.Stdin = bytes.NewReader(stdin)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	if exit, failed := errors.AsType[*exec.ExitError](err); failed {
		return out.Bytes(), errOut.String(), exit.ExitCode()
	}
	assert.NoError(tb, err, "the plugin runs")
	return out.Bytes(), errOut.String(), 0
}

// errRead is the failure of the reader of a request.
var errRead = errors.New("the request does not read")

// errWrite is the failure of the writer of a response.
var errWrite = errors.New("the response does not write")

// failingWriter is a writer that fails every write.
type failingWriter struct{}

// Write fails with errWrite.
func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }
