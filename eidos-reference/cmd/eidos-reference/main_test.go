// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/cli/acceptancetest"
	gotesting "go.dokimi.dev/eidos/lang/go/testing"
	tstesting "go.dokimi.dev/eidos/lang/typescript/testing"
	"go.dokimi.dev/eidos/reference"
	"go.dokimi.dev/eidos/reference/handlergen"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// mainEnv makes the test binary run main in place of its tests when it has
// the value runMain. A case then runs main as a process, with arguments, a
// working directory and an exit status. go test sets GOCOVERDIR for a test
// binary built with coverage. The process writes its coverage counters there
// when it exits, and the test binary merges them into its coverage profile.
const (
	mainEnv = "EIDOS_REFERENCE_TEST_MAIN"
	runMain = "1"
)

// graceShare is the share of the time left before the deadline of a test
// that a process of the test does not get: the test kills the process at
// the deadline less the time left divided by graceShare.
const graceShare = 20

// The main package of the binary, the workspace tree of the module relative
// to this package, and the directory of the TypeScript sources of the tree.
const (
	mainPackage = "go.dokimi.dev/eidos/reference/cmd/eidos-reference"
	tree        = "../../testdata/tree"
	webDir      = "web"
)

// The file that the Fail of the fixture writes, its source and its mode.
// The source puts a struct without the method Serve under handler, so
// handlergen reports an Error at it.
const (
	brokenFile   = "orders/broken.go"
	brokenSource = "package orders\n\n// Broken is under handler and declares no method Serve.\n//\n" +
		"//+reference:handler\ntype Broken struct{}\n"
	fileMode = 0o644
)

// versionCommand is the command that the process case runs.
const versionCommand = "version"

// TestMain runs main in place of the tests when mainEnv has the value
// runMain, and the tests otherwise.
func TestMain(m *testing.M) {
	if os.Getenv(mainEnv) == runMain {
		main()
	}
	os.Exit(m.Run())
}

// TestMainProcess runs main as a process of the test binary, and runs the
// binary that the go command builds from the package through the checks of
// the command line. Each process of the test binary writes the coverage
// meta-data file of the binary into GOCOVERDIR under a temporary name that
// Go 1.27 makes from the time in nanoseconds alone, so the test runs one
// such process at a time. A build of the go command writes no coverage.
func TestMainProcess(t *testing.T) {
	t.Run("main", func(t *testing.T) {
		t.Run("lists the plugins of the composition for version", func(t *testing.T) {
			self, err := os.Executable()
			assert.NoError(t, err, "the test binary has a path")
			ctx := t.Context()
			if deadline, ok := t.Deadline(); ok {
				var cancel context.CancelFunc
				ctx, cancel = context.WithDeadline(ctx, deadline.Add(-time.Until(deadline)/graceShare))
				defer cancel()
			}
			cmd := exec.CommandContext(ctx, self, versionCommand)
			cmd.Dir = t.TempDir()
			assert.NoError(t, os.CopyFS(cmd.Dir, os.DirFS(filepath.FromSlash(tree))),
				"the tree with the config file of the options copies into the working directory")
			cmd.Env = append(os.Environ(), mainEnv+"="+runMain, "PWD="+cmd.Dir)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			assert.NoError(t, cmd.Run(), "version exits with status 0: "+stderr.String())
			assert.Contains(t, stdout.String(), string(handlergen.ID), "version lists the plugins of the composition")
		})

		t.Run("passes the checks of the command line except the panic", func(t *testing.T) {
			if !toolchain.Require(t, tstesting.New()) {
				return
			}
			f := acceptancetest.Fixture{
				Main:  mainPackage,
				Brand: reference.Brand,
				Tree:  os.DirFS(filepath.FromSlash(tree)),
				Fail: func(root string) error {
					return os.WriteFile(
						filepath.Join(root, filepath.FromSlash(brokenFile)),
						[]byte(brokenSource),
						fileMode,
					)
				},
				Compile: func(ctx context.Context, root string) error {
					return errors.Join(
						gotesting.New().TypeCheck(ctx, root),
						tstesting.New().TypeCheck(ctx, filepath.Join(root, webDir)),
					)
				},
			}
			bin := acceptancetest.Build(t, f.Main)
			checks := []struct {
				name  string
				check func(assert.TB, acceptancetest.Fixture, string, string)
			}{
				{name: "exits with the statuses of the contract", check: acceptancetest.AssertStatuses},
				{name: "runs each kernel command under its name", check: acceptancetest.AssertNames},
				{name: "finds the config file of the root", check: acceptancetest.AssertDiscovery},
				{name: "writes JSON lines alone under --format=json", check: acceptancetest.AssertJSON},
				{name: "rewrites no file on a second run", check: acceptancetest.AssertIdempotent},
				{name: "compiles the generated output", check: acceptancetest.AssertCompiles},
				{
					name:  "runs each member of a list under a state directory of its own",
					check: acceptancetest.AssertLists,
				},
				{name: "exits with status 1 while another process has the lock", check: acceptancetest.AssertLocked},
			}
			for _, c := range checks {
				t.Run(c.name, func(t *testing.T) {
					t.Parallel()
					c.check(t, f, bin, t.TempDir())
				})
			}
		})
	})
}
