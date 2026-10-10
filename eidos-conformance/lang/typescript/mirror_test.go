// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/conformance/lang/typescript"
	"go.dokimi.dev/eidos/core/workspace"
	eidos "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/position"
)

// The modules of the mirror cases, the Go file that the plan writes
// beside each, and the declarations and lines that the cases check.
const (
	rowModule    = "svc/row.ts"
	rowMirror    = "svc/row_mirror.go"
	rowSource    = "export interface Row {\n  name: string;\n  size: number;\n}\n"
	unionSource  = "export interface Row {\n  id: string | number;\n}\n"
	mirroredName = "\tName string\n"
	mirroredSize = "\tSize float64\n"
	unionLine    = 2
	unionColumn  = 3
)

// TestMirror checks that a Go plan over TypeScript source translates a
// TypeScript type into Go through the hub, and refuses a TypeScript
// union, which Go cannot spell, at the property that has it.
func TestMirror(t *testing.T) {
	t.Parallel()

	t.Run("ComposeMirror", func(t *testing.T) {
		t.Parallel()

		t.Run("mirrors an interface as a Go struct of translated fields", func(t *testing.T) {
			t.Parallel()

			root := files.Workspace(t, files.Tree{rowModule: files.Text(rowSource)})
			report, err := mirrorRun(t, root)
			for d := range report.Sink.All() {
				expect.NotEqual(t, d.Severity, diag.SeverityError,
					fmt.Sprintf("%s at %s is not an Error of the run: %s", d.Code, d.Pos, d.Msg))
			}
			assert.NoError(t, err, "the run is clean")
			b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rowMirror)))
			assert.NoError(t, err, "the plan writes the struct beside the module")
			expect.That(t, b).
				Contains(mirroredName, "a string property is a Go string").
				Contains(mirroredSize, "a number property is a Go float64")
		})

		t.Run("reports RefusedType at a property of a union", func(t *testing.T) {
			t.Parallel()

			report, err := mirrorRun(t, files.Workspace(t, files.Tree{rowModule: files.Text(unionSource)}))
			assert.ErrorIs(t, err, workspace.ErrRunFailed, "the refused type fails the plan")
			var refused []diag.Diag
			for d := range report.Sink.All() {
				if d.Code == eidos.RefusedType {
					refused = append(refused, d)
				}
			}
			assert.Length(t, refused, 1, "the run reports the one refused type")
			assert.Equal(t, refused[0].Pos, position.Pos{File: rowModule, Line: unionLine, Col: unionColumn},
				"the refusal is at the property with the union")
		})
	})
}

// mirrorRun runs the mirror composition over the tree at root, and
// returns the run's report and error.
func mirrorRun(t *testing.T, root string) (*workspace.Report, error) {
	t.Helper()

	w, err := typescript.ComposeMirror(root)
	assert.NoError(t, err, "the composition builds")
	return w.Run(t.Context(), workspace.Input{Tree: os.DirFS(root)})
}
