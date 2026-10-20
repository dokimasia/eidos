// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package python_test

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/conformance/lang/python"
	"go.dokimi.dev/eidos/core/workspace"
	pythonlang "go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/spell"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The tree of the shop fixture.
const shopTree = "testdata/shop/tree"

// frameEnd ends the lines of the frame that the output contract writes
// before the body of a file.
const frameEnd = "\n\n"

// mirrorWord is the family word of the mirror generator, which the filename
// of each mirror joins to the stem of its source.
const mirrorWord = "mirror"

// shopRoot copies the tree of the shop fixture into a new directory, and
// returns the directory.
func shopRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	assert.NoError(t, os.CopyFS(root, os.DirFS(filepath.FromSlash(shopTree))), "the shop tree copies")
	return root
}

// cleanRun runs the shop composition with the plans over root, and fails
// the test where the run is not clean. Each Error of the run fails the test
// on a line of its own, before the run's error does.
func cleanRun(t *testing.T, root string, plans ...workspace.Plan) {
	t.Helper()

	w, err := python.ComposeShop(root).Plans(plans...).Build()
	assert.NoError(t, err, "the composition builds")
	report, err := w.Run(t.Context(), workspace.Input{Tree: os.DirFS(root)})
	for d := range report.Sink.All() {
		expect.NotEqual(t, d.Severity, diag.SeverityError,
			fmt.Sprintf("%s at %s is not an Error of the run: %s", d.Code, d.Pos, d.Msg))
	}
	assert.NoError(t, err, "the run is clean")
}

// mirrorsOf returns the slash-separated path of the mirror of each Python
// file of the tree at root, keyed by the path of the file. The mirror of a
// file <stem>.py or <stem>.pyi is the module beside it that the Python
// backend names after the stem and the family word, as models_mirror.py. A
// case calls mirrorsOf before the run, so the tree has no mirror yet.
func mirrorsOf(t *testing.T, root string) map[string]string {
	t.Helper()

	out := map[string]string{}
	err := fs.WalkDir(os.DirFS(root), ".", func(p string, d fs.DirEntry, err error) error {
		ext := path.Ext(p)
		if err == nil && !d.IsDir() && slices.Contains([]string{pythonlang.Extension, pythonlang.StubExtension}, ext) {
			mirror := plugin.Unit{Per: plugin.PerSource, Key: p, Word: mirrorWord}
			out[p] = path.Join(path.Dir(p), spell.Filename(mirror))
		}
		return err
	})
	assert.NoError(t, err, "the tree walks")
	return out
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
