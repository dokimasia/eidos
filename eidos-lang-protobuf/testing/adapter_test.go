// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/lang/protobuf"
	protobuftesting "go.dokimi.dev/eidos/lang/protobuf/testing"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The files of the healthy fixture, and the file that a case breaks.
const (
	typesFile  = "acme/store/v1/types.proto"
	storeFile  = "acme/store/v1/store.proto"
	pageFile   = "acme/store/v2/page.proto"
	brokenFile = "acme/store/v1/broken.proto"
)

// The sources of the fixtures. The types import a well-known type, the
// service imports the types, and the page is a file of edition 2026.
const (
	typesSource = "syntax = \"proto3\";\n\npackage acme.store.v1;\n\n" +
		"import \"google/protobuf/timestamp.proto\";\n\n" +
		"// Row is one record.\nmessage Row {\n  string id = 1;\n  google.protobuf.Timestamp created = 2;\n}\n"
	storeSource = "syntax = \"proto3\";\n\npackage acme.store.v1;\n\nimport \"acme/store/v1/types.proto\";\n\n" +
		"// Store serves rows.\nservice Store {\n  rpc Get(Row) returns (Row);\n}\n"
	pageSource = "edition = \"2026\";\n\npackage acme.store.v2;\n\n" +
		"// Page is one page of rows.\nexport message Page {\n  int32 size = 1;\n}\n"
	syntaxErrorSource = "syntax = \"proto3\";\n\nmessage {\n"
	typeErrorSource   = "syntax = \"proto3\";\n\nmessage Row {\n  Missing missing = 1;\n}\n"
	optionErrorSource = "syntax = \"proto3\";\n\noption (acme.missing) = true;\n"
)

// TestAdapter checks the adapter through the assertions of the kernel and
// through each of its methods. The parse and the compile run in process, so
// no case needs a toolchain on the machine.
func TestAdapter(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an adapter that parses a healthy project", func(t *testing.T) {
			t.Parallel()

			toolchain.AssertParses(t.Context(), t, protobuftesting.New(), healthy())
		})

		t.Run("returns an adapter that type-checks a healthy project", func(t *testing.T) {
			t.Parallel()

			toolchain.AssertTypeChecks(t.Context(), t, protobuftesting.New(), healthy())
		})
	})

	t.Run("Lang", func(t *testing.T) {
		t.Parallel()

		t.Run("returns protobuf", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, protobuftesting.New().Lang(), protobuf.Lang, "the language that the assertions name")
		})
	})

	t.Run("Available", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true without a reason", func(t *testing.T) {
			t.Parallel()

			present, why := protobuftesting.New().Available()
			assert.True(t, present, "the parse and the compile run in process")
			assert.Empty(t, why, "a toolchain that is there states no reason")
		})
	})

	t.Run("Layout", func(t *testing.T) {
		t.Parallel()

		t.Run("writes every generated file", func(t *testing.T) {
			t.Parallel()

			files.HasContent(t, filepath.Join(laidOut(t, healthy()), filepath.FromSlash(storeFile)), storeSource,
				"the generated file is on disk with its own bytes")
		})

		t.Run("returns an error for a path that leaves the scratch directory", func(t *testing.T) {
			t.Parallel()

			_, err := protobuftesting.New().Layout(only("../escape.proto", typesSource))
			assert.HasError(t, err, "a fixture writes only inside its project")
			assert.Contains(t, err.Error(), "leaves the directory", "the error contains the reason")
		})
	})

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nil for a healthy project", func(t *testing.T) {
			t.Parallel()

			assert.NoError(t, protobuftesting.New().Parse(t.Context(), laidOut(t, healthy())),
				"the healthy output parses")
		})

		t.Run("returns an error that contains the file and the line of a syntax error", func(t *testing.T) {
			t.Parallel()

			err := protobuftesting.New().Parse(t.Context(), laidOut(t, with(brokenFile, syntaxErrorSource)))
			assert.HasError(t, err, "a message without a name is a syntax error")
			assert.Contains(t, err.Error(), brokenFile+":3:", "the error states the file and the line")
		})

		t.Run("returns nil for a project whose one error is a type error", func(t *testing.T) {
			t.Parallel()

			err := protobuftesting.New().Parse(t.Context(), laidOut(t, with(brokenFile, typeErrorSource)))
			assert.NoError(t, err, "a reference that does not resolve is no syntax error")
		})

		t.Run("returns an error for a project without a proto file", func(t *testing.T) {
			t.Parallel()

			err := protobuftesting.New().Parse(t.Context(), laidOut(t, only("notes.txt", typesSource)))
			assert.HasError(t, err, "a parse of nothing proves nothing")
			assert.Contains(t, err.Error(), "no proto file", "the error states the reason")
		})

		t.Run("returns an error for a directory that does not exist", func(t *testing.T) {
			t.Parallel()

			err := protobuftesting.New().Parse(t.Context(), filepath.Join(t.TempDir(), "missing"))
			assert.ErrorIs(t, err, os.ErrNotExist, "the walk of a missing directory fails")
		})

		t.Run("returns an error for a proto file that does not read", func(t *testing.T) {
			t.Parallel()

			dir := files.Workspace(t, files.Tree{"dangling.proto": files.Link("missing.proto")})
			err := protobuftesting.New().Parse(t.Context(), dir)
			assert.ErrorIs(t, err, os.ErrNotExist, "the read of a link without a target fails")
		})

		t.Run("returns the error of the context for a context that ended", func(t *testing.T) {
			t.Parallel()

			dir := laidOut(t, healthy())
			assert.HonoursCancellation(t, func(ctx context.Context) error {
				return protobuftesting.New().Parse(ctx, dir)
			}, "an ended context parses no file")
		})
	})

	t.Run("TypeCheck", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nil for a healthy project", func(t *testing.T) {
			t.Parallel()

			assert.NoError(t, protobuftesting.New().TypeCheck(t.Context(), laidOut(t, healthy())),
				"the output compiles with its imports and a well-known type")
		})

		t.Run("returns an error that contains the file and the line of a reference that does not resolve",
			func(t *testing.T) {
				t.Parallel()

				err := protobuftesting.New().TypeCheck(t.Context(), laidOut(t, with(brokenFile, typeErrorSource)))
				assert.HasError(t, err, "a reference to an undeclared message fails the compile")
				assert.Contains(t, err.Error(), brokenFile+":4:", "the error states the file and the line")
			})

		t.Run("returns an error for an option that no file declares", func(t *testing.T) {
			t.Parallel()

			err := protobuftesting.New().TypeCheck(t.Context(), laidOut(t, with(brokenFile, optionErrorSource)))
			assert.HasError(t, err, "the compile interprets the options of a file")
			assert.Contains(t, err.Error(), brokenFile+":3:", "the error states the file and the line")
		})

		t.Run("returns an error for a project without a proto file", func(t *testing.T) {
			t.Parallel()

			err := protobuftesting.New().TypeCheck(t.Context(), laidOut(t, only("notes.txt", typesSource)))
			assert.HasError(t, err, "a compile of nothing proves nothing")
			assert.Contains(t, err.Error(), "no proto file", "the error states the reason")
		})

		t.Run("returns the error of the context for a context that ended", func(t *testing.T) {
			t.Parallel()

			dir := laidOut(t, healthy())
			assert.HonoursCancellation(t, func(ctx context.Context) error {
				return protobuftesting.New().TypeCheck(ctx, dir)
			}, "an ended context compiles no file")
		})
	})

	t.Run("RunTests", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error for every project", func(t *testing.T) {
			t.Parallel()

			report, err := protobuftesting.New().RunTests(t.Context(), laidOut(t, healthy()))
			assert.HasError(t, err, "a protobuf file declares no test")
			assert.Equal(t, err.Error(), "testing: a protobuf file declares no test, so protobuf runs none",
				"the error states the missing capability")
			assert.False(t, report.OK(), "a run of nothing is not a pass")
		})
	})

	t.Run("Satisfies", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error for every question", func(t *testing.T) {
			t.Parallel()

			got, err := protobuftesting.New().Satisfies(t.Context(), laidOut(t, healthy()), "acme.store.v1.Row",
				"acme.store.v1.Store")
			assert.HasError(t, err, "a protobuf message implements no contract")
			assert.Equal(t, err.Error(),
				"testing: a protobuf message implements no contract, so protobuf checks none",
				"the error states the missing capability")
			assert.False(t, got, "a refusal reports false")
		})
	})
}

// healthy returns generated output that parses and compiles: a file of
// types, a service that imports it, and a file of edition 2026.
func healthy() toolchain.Generated {
	return toolchain.Generated{Files: map[string][]byte{
		typesFile: []byte(typesSource),
		storeFile: []byte(storeSource),
		pageFile:  []byte(pageSource),
	}}
}

// with returns the healthy fixture with one file added, for a case that
// breaks one thing.
func with(path, body string) toolchain.Generated {
	g := healthy()
	g.Files[path] = []byte(body)
	return g
}

// only returns a fixture with one file.
func only(path, body string) toolchain.Generated {
	return toolchain.Generated{Files: map[string][]byte{path: []byte(body)}}
}

// laidOut lays a fixture out and removes it when the case ends.
func laidOut(t *testing.T, g toolchain.Generated) string {
	t.Helper()

	dir, err := protobuftesting.New().Layout(g)
	assert.NoError(t, err, "the fixture lays out")
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}
