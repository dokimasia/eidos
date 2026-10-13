// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	gocli "go.stealthscale.io/cli"

	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/cli/acceptancetest"
	golang "go.dokimi.dev/eidos/conformance/lang/go"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/workspace"
	gotesting "go.dokimi.dev/eidos/lang/go/testing"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/output"
)

// The main packages of the two binaries of the acceptance fixture, the tree
// of the fixture, and the group below which the group binary mounts the
// kernel commands.
const (
	mainHost       = "go.dokimi.dev/eidos/conformance/lang/go/testdata/acceptance/main"
	groupHost      = "go.dokimi.dev/eidos/conformance/lang/go/testdata/acceptance/group"
	acceptanceTree = "testdata/acceptance/tree"
	group          = "gen"
)

// The binaries of the acceptance fixture pass the acceptance suite. The app
// of one binary has the kernel commands at its root, and the app of the
// other binary mounts them below a group. The suite compiles the output of
// a run through the type check of the Go adapter, which compiles every
// package and every test file. Crash panics.
func TestAcceptance(t *testing.T) {
	t.Parallel()

	t.Run("ComposeAcceptance", func(t *testing.T) {
		t.Parallel()

		hosts := []struct {
			name   string
			main   string
			prefix []string
		}{
			{
				name: "passes the acceptance suite in a binary with the kernel commands at the root of its app",
				main: mainHost,
			},
			{
				name:   "passes the acceptance suite in a binary that mounts the kernel commands below a group",
				main:   groupHost,
				prefix: []string{group},
			},
		}
		for _, tt := range hosts {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				acceptancetest.RunAcceptanceSuite(t, acceptancetest.Fixture{
					Main:   tt.main,
					Brand:  golang.Brand,
					Prefix: tt.prefix,
					Tree:   os.DirFS(filepath.FromSlash(acceptanceTree)),
					Panic:  []string{golang.CrashName},
					Fail: func(root string) error {
						return os.CopyFS(root, os.DirFS(filepath.FromSlash(checkedTree)))
					},
					Compile: gotesting.New().TypeCheck,
				})
			})
		}

		t.Run("returns a composition whose check reports an Error at an interface without a double",
			func(t *testing.T) {
				t.Parallel()

				root := t.TempDir()
				copyTree(t, root, acceptanceTree)
				copyTree(t, root, checkedTree)
				w, err := golang.ComposeAcceptance().
					Output(func() (output.Sink, error) { return output.NewDisk(root, golang.Brand) }).
					Ledger(func() (ledger.Ledger, error) { return ledger.OpenDir(root, golang.Brand) }).
					Build()
				assert.NoError(t, err, "the composition builds with the sink and the ledger of the root")
				report, err := w.Run(t.Context(), workspace.Input{Tree: os.DirFS(root)})
				assert.ErrorIs(t, err, workspace.ErrRunFailed, "the Error of the check fails the run")
				var reported []diag.Diag
				for d := range report.Sink.All() {
					if d.Code == golang.Unstubbed {
						reported = append(reported, d)
					}
				}
				assert.Length(t, reported, 1,
					"the check reports the one interface outside the sources of the stubs plan")
				assert.Equal(t, reported[0].Pos.File, auditorFile, "the check reports the interface in admin")
			})
	})

	t.Run("Crash", func(t *testing.T) {
		t.Parallel()

		t.Run("panics", func(t *testing.T) {
			t.Parallel()

			run, err := gocli.NewRun(&cli.Global{}, &struct{}{}, &gocli.Env{Stdout: io.Discard, Stderr: io.Discard})
			assert.NoError(t, err, "the run has its options and both streams")
			assert.Panics(t, func() { _ = golang.Crash(t.Context(), run) }, "the command panics")
		})
	})
}
