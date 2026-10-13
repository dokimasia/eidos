// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/golden"
	gocli "go.stealthscale.io/cli"
	"go.stealthscale.io/cli/clitest"

	"go.dokimi.dev/eidos/cli"
)

// childVar is the variable of the environment that makes the test binary
// run main in place of its tests. The case of main runs the test binary
// again with it, so go test merges the coverage of main into the profile.
const childVar = "EIDOS_TEST_MAIN"

// The config files of the transcript. The config file of the fixture tree
// adds the plugin audit to the plans golang and typescript, and the config
// file of the bare tree extends no plan.
const (
	treeConfig = "testdata/tree/.eidos.yaml"
	bareConfig = "testdata/bare/.eidos.yaml"
)

// transcriptName is the golden file of the transcript below testdata/golden.
const transcriptName = "transcript.txt"

// roots are the toolchain roots of the Go frontend in the transcript. Both
// variables name the directory testdata, which has no standard library and
// no module cache. The sources of the trees import no package, so a run
// reads neither store.
var roots = map[string]string{"GOMODCACHE": "testdata", "GOROOT": "testdata"}

func TestMain(m *testing.M) {
	if os.Getenv(childVar) != "" {
		main()
	}
	os.Exit(m.Run())
}

func TestEidos(t *testing.T) {
	t.Parallel()

	t.Run("main", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the module of the binary for version", func(t *testing.T) {
			t.Parallel()

			binary, err := os.Executable()
			assert.NoError(t, err, "the path of the test binary")
			cmd := exec.CommandContext(t.Context(), binary, "version")
			cmd.Env = append(os.Environ(), childVar+"=1")
			out, err := cmd.Output()
			assert.NoError(t, err, "eidos version exits 0")
			assert.Contains(t, string(out), "brand eidos\nmodule go.dokimi.dev/eidos ",
				"eidos version writes the brand and the module of the binary")
		})
	})

	t.Run("compose", func(t *testing.T) {
		t.Parallel()

		t.Run("runs the plugins of the config file in the plans of the binary", func(t *testing.T) {
			t.Parallel()

			program := func(ctx context.Context, env *gocli.Env) int {
				return cli.Run(ctx, cli.New(help, compose), compose, env)
			}
			files := os.DirFS(".")
			got := clitest.Transcript(t.Context(), string(brand), program,
				clitest.Input{
					Comment: "The app lists the kernel commands.",
					Args:    []string{"--help"},
				},
				clitest.Input{
					Comment: "Each target has a plan. The config file adds the plugin audit to two of them.",
					Args:    []string{"--config", treeConfig, "plan"},
					Files:   files,
				},
				clitest.Input{
					Comment: "The plugin writes a Go implementation, its test, a Go mirror and a TypeScript mirror.",
					Args:    []string{"--config", treeConfig, "run", "--stdout"},
					Files:   files,
					Vars:    roots,
				},
				clitest.Input{
					Comment: "A config file that extends no plan writes no file.",
					Args:    []string{"--config", bareConfig, "run", "--stdout"},
					Files:   files,
					Vars:    roots,
				},
			)
			golden.Match(t, transcriptName, got, golden.ShouldUpdate())
		})
	})
}
