// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"
	gocli "go.stealthscale.io/cli"

	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/core/workspace"
)

// The config files of the Prepare cases: a file with an option of run, a
// file whose top-level key neither the document nor an option has, a file
// with a misspelled key of an option, a file that selects JSON output, and
// a file that disables the plan of the fixture.
const (
	dryConfig      = "version: 1\nrun:\n    dry-run: true\n"
	strayKeyConfig = "version: 1\nworkerz: 4\n"
	typoConfig     = "version: 1\nstrikt: true\n"
	jsonConfig     = "version: 1\nformat: json\n"
	offConfig      = "version: 1\nplans: {" + planName + ": {enabled: false}}\n"
)

// helpCapture is a go-cli renderer that keeps the help of a run in place of
// writing it.
type helpCapture struct {
	gocli.Plain
	help *gocli.Help
}

// Help keeps h and writes nothing. It returns no error.
func (c *helpCapture) Help(_ io.Writer, _ []string, h *gocli.Help) error {
	c.help = h
	return nil
}

// An app runs the kernel commands at its root or below a group, and reads
// the options of its commands from the config file of the workspace.
func TestApp(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an app with the kernel commands at its root in order", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, commandsOf(t, cli.New(appHelp, compose), helpFlag), helpNames,
				"the help of the app lists the kernel commands, then the command of go-cli")
		})

		t.Run("returns an app with the name of the brand", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := execute(t, cli.New(appHelp, compose), compose, helpFlag)
			assert.HasPrefix(t, stdout, "Usage: "+string(brand)+" ", "the usage line names the brand")
		})
	})

	t.Run("Prepare", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the options of a command from the config file", func(t *testing.T) {
			t.Parallel()

			root := stored(t, files.Tree{confName: files.Text(dryConfig)})
			status, stdout, _ := invoke(t, compose, cmdRun, root)
			expect.Equal(t, status, cli.StatusOK, "the dry run succeeds")
			expect.Contains(t, stdout, "1 prepared", "the key run.dry-run makes the run a dry run")
			files.Absent(t, filepath.Join(root, filepath.FromSlash(mirrorPath)), "the dry run writes no file")
		})

		t.Run("returns StatusUsage for a top-level key that no option and no document has", func(t *testing.T) {
			t.Parallel()

			root := stored(t, files.Tree{confName: files.Text(strayKeyConfig)})
			status, _, stderr := invoke(t, compose, cmdRun, root)
			expect.Equal(t, status, cli.StatusUsage, "the key is a usage error")
			expect.Equal(t, stderr, "error: cli: run: unknown key workerz in "+filepath.Join(root, confName)+"\n",
				"go-cli names the command, the key and the file")
		})

		t.Run("suggests the key of an option for a misspelled key", func(t *testing.T) {
			t.Parallel()

			root := stored(t, files.Tree{confName: files.Text(typoConfig)})
			status, _, stderr := invoke(t, compose, cmdRun, root)
			expect.Equal(t, status, cli.StatusUsage, "the key is a usage error")
			expect.HasSuffix(t, stderr, " (did you mean strict?)\n", "go-cli suggests the key")
		})

		t.Run("searches the parent directories of Chdir for the config file", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text(jsonConfig), "svc": files.Dir()})
			_, stdout, _ := invoke(t, compose, cmdPlan, filepath.Join(root, "svc"))
			assert.HasPrefix(t, stdout, `{"event":"start"`, "the config file of the parent selects JSON output")
		})

		t.Run("stops the search after a directory with a version control marker", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{
				confName:    files.Text(strayKeyConfig),
				"repo/.git": files.Dir(),
				"repo/svc":  files.Dir(),
			})
			status, _, stderr := invoke(t, compose, cmdPlan, filepath.Join(root, "repo", "svc"))
			assert.Equal(t, status, cli.StatusOK, "the run does not read the file above the marker: "+stderr)
		})

		t.Run("writes the source of a located config file under --flag-debug", func(t *testing.T) {
			t.Parallel()

			_, _, stderr := invoke(t, compose, cmdPlan, stored(t, nil), debugFlag)
			assert.Contains(t, stderr, "located", "the path of the config file comes from the search")
		})

		t.Run("reads a relative Config against the working directory of the process under Chdir", func(t *testing.T) {
			t.Parallel()

			cwd, err := os.Getwd()
			assert.NoError(t, err, "the test reads the working directory")
			given := workspaceDir(t, files.Tree{"ci.yaml": files.Text(offConfig)})
			rel, err := filepath.Rel(cwd, filepath.Join(given, "ci.yaml"))
			if err != nil {
				t.Skip("the temporary directory is on another volume than the working directory")
			}
			status, stdout, stderr := invoke(t, compose, cmdPlan, stored(t, nil), configFlag, rel)
			expect.Equal(t, status, cli.StatusOK, "the config file of the Config opens: "+stderr)
			expect.Contains(t, stdout, "disabled by the config file", "Open reads the file that go-cli read")
		})

		t.Run("returns StatusUsage for a Chdir that does not exist", func(t *testing.T) {
			t.Parallel()

			status, _, stderr := invoke(t, compose, cmdPlan, filepath.Join(t.TempDir(), missingDir))
			expect.Equal(t, status, cli.StatusUsage, "the directory is a usage error")
			expect.HasPrefix(t, stderr, "error: cli: find the working directory: ", "the search writes the error")
		})

		t.Run("returns StatusUsage for a Chdir that does not exist beside a Config", func(t *testing.T) {
			t.Parallel()

			config := filepath.Join(workspaceDir(t, files.Tree{"ci.yaml": files.Text(version)}), "ci.yaml")
			status, _, stderr := invoke(t, compose, cmdPlan, filepath.Join(t.TempDir(), missingDir), configFlag, config)
			expect.Equal(t, status, cli.StatusUsage, "the directory is a usage error")
			expect.HasPrefix(t, stderr, "error: cli: find the working directory: ", "the hook writes the error")
		})

		t.Run("returns StatusUsage for a config file that the search finds and cannot read", func(t *testing.T) {
			t.Parallel()

			status, _, stderr := invoke(t, compose, cmdPlan, workspaceDir(t, files.Tree{confName: files.Dir()}))
			expect.Equal(t, status, cli.StatusUsage, "the config file is a config error")
			expect.HasPrefix(t, stderr, "error: cli: read the config file: ", "the search writes the error")
		})

		t.Run("searches no config file for a brand that is not valid", func(t *testing.T) {
			t.Parallel()

			invalid := func() *workspace.Builder { return compose().Brand("Acme") }
			app := gocli.New[cli.Global]("acme", appHelp)
			cli.Prepare(app, invalid)
			cli.Mount(app.Command[struct{}](groupName, groupHelp, nil), invalid)
			root := workspaceDir(t, files.Tree{confName: files.Text(strayKeyConfig)})
			status, _, stderr := execute(t, app, invalid, chdirFlag, root, groupName, cmdPlan)
			expect.Equal(t, status, cli.StatusUsage, "the brand is a config error")
			expect.Contains(t, stderr, `"Acme" is not a brand`, "Open writes the fault of Build")
		})
	})

	t.Run("Mount", func(t *testing.T) {
		t.Parallel()

		mounted := func() *gocli.App[cli.Global] {
			app := gocli.New[cli.Global]("acme", appHelp)
			cli.Prepare(app, compose)
			cli.Mount(app.Command[struct{}](groupName, groupHelp, nil), compose)
			return app
		}

		t.Run("adds the kernel commands as subcommands of the group in order", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, commandsOf(t, mounted(), groupName, helpFlag), kernelNames,
				"the help of the group lists the kernel commands")
		})

		t.Run("runs a kernel command below the group", func(t *testing.T) {
			t.Parallel()

			status, stdout, _ := execute(t, mounted(), compose, chdirFlag, t.TempDir(), groupName, cmdPlan)
			expect.Equal(t, status, cli.StatusOK, "plan succeeds")
			expect.HasPrefix(t, stdout, "frontend fakefront 1\n", "plan writes the composition")
		})

		t.Run("writes the path of the command into the start event", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := execute(t, mounted(), compose, chdirFlag, t.TempDir(), jsonFlag, groupName, cmdPlan)
			starts := decoded[startEvent](t, stdout, eventStart)
			assert.Length(t, starts, 1, "the output has one start")
			assert.Equal(t, starts[0].Command, groupName+" "+cmdPlan, "the start has the path of the command")
		})

		t.Run("names the path of the command in a usage error of the command", func(t *testing.T) {
			t.Parallel()

			_, _, stderr := execute(t, mounted(), compose, chdirFlag, t.TempDir(), groupName, cmdPlan, quietFlag,
				verboseFlag)
			assert.Equal(t, stderr, "error: cli: gen plan: the flags --quiet and --verbose exclude each other\n",
				"the error names the path of the command")
		})
	})
}

// The search for the config file and the hook of Prepare return a usage
// error when the working directory of the process does not exist. The cases
// change the working directory of the process, so they run alone.
func TestAppProcess(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("Windows does not remove the working directory of a process")
	}
	config := filepath.Join(workspaceDir(t, files.Tree{"ci.yaml": files.Text(version)}), "ci.yaml")
	tests := []struct {
		name string
		args []string
	}{
		{name: "returns StatusUsage from the search for a removed working directory", args: []string{cmdPlan}},
		{
			name: "returns StatusUsage from the hook for a removed working directory beside a Config",
			args: []string{configFlag, config, cmdPlan},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			assert.NoError(t, os.Remove(dir), "the test removes the working directory")
			status, _, stderr := execute(t, cli.New(appHelp, compose), compose, tt.args...)
			expect.Equal(t, status, cli.StatusUsage, "the removed directory is a usage error")
			expect.HasPrefix(t, stderr, "error: cli: find the working directory: ", "Run writes the error")
		})
	}
}

// commandsOf returns the names of the commands that the help of app lists
// for the command line args, in order.
func commandsOf(t *testing.T, app *gocli.App[cli.Global], args ...string) []string {
	t.Helper()

	capture := &helpCapture{}
	app.Render(capture)
	err := app.Execute(t.Context(), &gocli.Env{Stdout: io.Discard, Stderr: io.Discard, Args: args})
	assert.NoError(t, err, "the help writes")
	assert.NotNil(t, capture.help, "the run has a help")
	names := make([]string, 0, len(capture.help.Commands))
	for _, c := range capture.help.Commands {
		names = append(names, c.Name)
	}
	return names
}
