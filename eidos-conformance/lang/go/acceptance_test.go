// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/cli/acceptancetest"
	golang "go.dokimi.dev/eidos/conformance/lang/go"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/output"
)

// The main packages of the two binaries of the acceptance fixture, the tree
// of the fixture, and the group under which the dispatcher of the group
// binary mounts the kernel commands.
const (
	mainHost       = "go.dokimi.dev/eidos/conformance/lang/go/testdata/acceptance/main"
	groupHost      = "go.dokimi.dev/eidos/conformance/lang/go/testdata/acceptance/group"
	acceptanceTree = "testdata/acceptance/tree"
	group          = "gen"
)

// crashUsage is the help text of the crash command.
const crashUsage = "usage: crash\n"

// The file that a case adds to a module so that the module does not
// compile, and the mode of the file.
const (
	brokenFile   = "svc/broken.go"
	brokenSource = "package svc\n\nfunc broken() int { return \"broken\" }\n"
	fileMode     = 0o644
)

// The binaries of the acceptance fixture pass the acceptance suite. One
// binary mounts the kernel commands through cli.Main, and the other binary
// mounts them under a group of its own dispatcher. CompileAcceptance returns
// the output of the go command for a module that does not compile, and
// Crash panics.
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
				name: "passes the acceptance suite in a binary that mounts the kernel commands through cli.Main",
				main: mainHost,
			},
			{
				name:   "passes the acceptance suite in a binary that mounts the kernel commands under a group",
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
					Compile: golang.CompileAcceptance,
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

	t.Run("CompileAcceptance", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the output of the go command for a module that does not compile", func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			copyTree(t, root, acceptanceTree)
			assert.NoError(t, os.WriteFile(filepath.Join(root, filepath.FromSlash(brokenFile)), []byte(brokenSource),
				fileMode), "the case writes a file that does not compile")
			err := golang.CompileAcceptance(t.Context(), root)
			assert.HasError(t, err, "the module does not compile")
			assert.Contains(t, err.Error(), filepath.FromSlash(brokenFile),
				"the error has the output of the go command about the file")
		})
	})

	t.Run("Crash", func(t *testing.T) {
		t.Parallel()

		t.Run("Name", func(t *testing.T) {
			t.Parallel()

			t.Run("returns CrashName", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, golang.Crash{}.Name(), golang.CrashName, "the name of the command is CrashName")
			})
		})

		t.Run("Synopsis", func(t *testing.T) {
			t.Parallel()

			t.Run("returns one line that ends with a full stop", func(t *testing.T) {
				t.Parallel()

				synopsis := golang.Crash{}.Synopsis()
				expect.HasSuffix(t, synopsis, ".", "the synopsis ends with a full stop")
				expect.NotContains(t, synopsis, "\n", "the synopsis has one line")
			})
		})

		t.Run("Usage", func(t *testing.T) {
			t.Parallel()

			t.Run("returns the form of the command", func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, golang.Crash{}.Usage(), crashUsage, "the usage is the form of the crash command")
			})
		})

		t.Run("Run", func(t *testing.T) {
			t.Parallel()

			t.Run("panics", func(t *testing.T) {
				t.Parallel()

				assert.Panics(t, func() { golang.Crash{}.Run(t.Context(), cli.IO{}, nil) }, "the command panics")
			})
		})
	})
}
