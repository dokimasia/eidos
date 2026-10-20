// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protobuf_test

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/conformance/lang/protobuf"
	"go.dokimi.dev/eidos/core/workspace"
	protolang "go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// frameEnd ends the lines of the frame that the output contract writes
// before the body of a file.
const frameEnd = "\n\n"

// cleanBuild builds the composition b with its plans over root and runs
// it, and fails the test where the run is not clean. Each Error of the run
// fails the test on a line of its own, before the run's error does.
func cleanBuild(t *testing.T, root string, b *workspace.Builder) {
	t.Helper()

	w, err := b.Build()
	assert.NoError(t, err, "the composition builds")
	report, err := w.Run(t.Context(), workspace.Input{Tree: os.DirFS(root)})
	for d := range report.Sink.All() {
		expect.NotEqual(t, d.Severity, diag.SeverityError,
			fmt.Sprintf("%s at %s is not an Error of the run: %s", d.Code, d.Pos, d.Msg))
	}
	assert.NoError(t, err, "the run is clean")
}

// protoPaths returns the slash-separated path of every proto file of the
// tree at dir, in lexical order.
func protoPaths(t *testing.T, dir string) []string {
	t.Helper()

	var paths []string
	err := fs.WalkDir(os.DirFS(dir), ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && path.Ext(p) == protolang.Extension {
			paths = append(paths, p)
		}
		return err
	})
	assert.NoError(t, err, "the tree walks")
	return paths
}

// bodyOf returns the text of a written file inside the frame that the
// output contract adds: the lines up to the first blank line, and the
// blank line and the trailer at the end.
func bodyOf(t *testing.T, stamped string) string {
	t.Helper()

	_, rest, framed := strings.Cut(stamped, frameEnd)
	assert.True(t, framed, "the written file has a frame")
	end := strings.LastIndex(strings.TrimSuffix(rest, "\n"), frameEnd)
	assert.NotEqual(t, end, -1, "the written file has a blank line before its trailer")
	return rest[:end+1]
}

// mirrored returns the files that a mirror plan wrote under root, each at
// its path below the mirror directory, which is its import path.
func mirrored(t *testing.T, root string) toolchain.Generated {
	t.Helper()

	dir := filepath.Join(root, protobuf.MirrorDir)
	g := toolchain.Generated{Files: map[string][]byte{}}
	for _, p := range protoPaths(t, dir) {
		g.Files[p] = []byte(files.Read(t, filepath.Join(dir, filepath.FromSlash(p))))
	}
	return g
}
