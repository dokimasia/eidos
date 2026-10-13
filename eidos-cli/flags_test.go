// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"
	gocli "go.stealthscale.io/cli"

	"go.dokimi.dev/eidos/cli"
)

// The command of the binary that the Global cases run, and its help text.
const (
	probeName = "probe"
	probeHelp = "Records the global options of its run."
)

// globalConfig is a config file that sets every global option that has a
// key.
const globalConfig = "version: 1\nformat: json\nstrict: true\nverbose: true\nquiet: true\nno-color: true\n"

// The environment variables of the global options of the brand acme.
const (
	formatVar  = "ACME_FORMAT"
	configVar  = "ACME_CONFIG"
	chdirVar   = "ACME_CHDIR"
	strictVar  = "ACME_STRICT"
	verboseVar = "ACME_VERBOSE"
	quietVar   = "ACME_QUIET"
	noColorVar = "ACME_NO_COLOR"
)

// Every command accepts the global options, which go-cli reads from their
// flags, their variables and their keys.
func TestFlags(t *testing.T) {
	t.Parallel()

	t.Run("Format", func(t *testing.T) {
		t.Parallel()

		t.Run("Valid", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give cli.Format
				want bool
			}{
				{name: "reports true for FormatText", give: cli.FormatText, want: true},
				{name: "reports true for FormatJSON", give: cli.FormatJSON, want: true},
				{name: "reports false for a value above FormatJSON", give: cli.FormatJSON + 1, want: false},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, tt.give.Valid(), tt.want, "Valid accepts the two formats")
				})
			}
		})

		t.Run("String", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give cli.Format
				want string
			}{
				{name: "returns text for FormatText", give: cli.FormatText, want: "text"},
				{name: "returns json for FormatJSON", give: cli.FormatJSON, want: "json"},
				{name: "returns the number for a value above FormatJSON", give: 3, want: "Format(3)"},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, tt.give.String(), tt.want, "String returns the name of the format")
				})
			}
		})

		t.Run("Set", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give string
				want cli.Format
			}{
				{name: "sets FormatText for text", give: "text", want: cli.FormatText},
				{name: "sets FormatJSON for json", give: "json", want: cli.FormatJSON},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					var got cli.Format
					assert.NoError(t, got.Set(tt.give), "the name of a format sets the format")
					assert.Equal(t, got, tt.want, "the format has the name")
				})
			}

			t.Run("returns an error that wraps ErrFormat for a name of no format", func(t *testing.T) {
				t.Parallel()

				var got cli.Format
				assert.ErrorIs(t, got.Set("yaml"), cli.ErrFormat, "yaml is no output format")
			})

			t.Run("returns an error that lists the formats", func(t *testing.T) {
				t.Parallel()

				var got cli.Format
				err := got.Set("yaml")
				assert.HasError(t, err, "yaml is no output format")
				assert.Equal(t, err.Error(), `cli: unknown output format "yaml": use text or json`,
					"the error names the value and the formats")
			})
		})
	})

	t.Run("Global", func(t *testing.T) {
		t.Parallel()

		t.Run("has the format text and no other value without a source", func(t *testing.T) {
			t.Parallel()

			dir := workspaceDir(t, files.Tree{".git": files.Dir()})
			got := probed(t, &gocli.Env{Args: []string{chdirFlag, dir, probeName}})
			want := cli.Global{Format: cli.FormatText, Chdir: dir}
			assert.Equal(t, got, want, "every other option has its zero value")
		})

		t.Run("takes each option from its flag", func(t *testing.T) {
			t.Parallel()

			dir := workspaceDir(t, files.Tree{".git": files.Dir(), "ci.yaml": files.Text(version)})
			config := filepath.Join(dir, "ci.yaml")
			got := probed(t, &gocli.Env{Args: []string{
				chdirFlag, dir, jsonFlag, configFlag, config,
				strictFlag, verboseFlag, quietFlag, noColorFlag, probeName,
			}})
			assert.Equal(t, got, cli.Global{
				Format: cli.FormatJSON, Config: gocli.ConfigFile(config), Chdir: dir,
				Strict: true, Verbose: true, Quiet: true, NoColor: true,
			}, "each flag sets its option")
		})

		t.Run("takes each option from its environment variable", func(t *testing.T) {
			t.Parallel()

			dir := workspaceDir(t, files.Tree{".git": files.Dir(), "ci.yaml": files.Text(version)})
			config := filepath.Join(dir, "ci.yaml")
			got := probed(t, &gocli.Env{
				Lookup: gocli.Vars(map[string]string{
					formatVar: "json", configVar: config, chdirVar: dir, strictVar: "true",
					verboseVar: "true", quietVar: "true", noColorVar: "true",
				}),
				Args: []string{probeName},
			})
			assert.Equal(t, got, cli.Global{
				Format: cli.FormatJSON, Config: gocli.ConfigFile(config), Chdir: dir,
				Strict: true, Verbose: true, Quiet: true, NoColor: true,
			}, "each variable sets its option")
		})

		t.Run("takes each option with a key from the config file", func(t *testing.T) {
			t.Parallel()

			dir := workspaceDir(t, files.Tree{".git": files.Dir(), confName: files.Text(globalConfig)})
			got := probed(t, &gocli.Env{Args: []string{chdirFlag, dir, probeName}})
			assert.Equal(t, got, cli.Global{
				Format: cli.FormatJSON, Config: gocli.ConfigFile(filepath.Join(dir, confName)), Chdir: dir,
				Strict: true, Verbose: true, Quiet: true, NoColor: true,
			}, "each key sets its option")
		})

		t.Run("returns StatusUsage for the key chdir in the config file", func(t *testing.T) {
			t.Parallel()

			dir := workspaceDir(t, files.Tree{".git": files.Dir(), confName: files.Text("version: 1\nchdir: svc\n")})
			status, _, stderr := execute(t, cli.New(appHelp, compose), compose, chdirFlag, dir, cmdPlan)
			assert.Equal(t, status, cli.StatusUsage, "Chdir has no key: "+stderr)
		})
	})
}

// probed runs the command probe of an app that New returns over the fixture
// composition with env, and returns the global options of the run. The run
// opens the files of the process, and probed sets the standard streams of
// env. It fails the test when the run does not succeed.
func probed(t *testing.T, env *gocli.Env) cli.Global {
	t.Helper()

	var got cli.Global
	app := cli.New(appHelp, compose)
	app.Command(probeName, probeHelp, func(_ context.Context, run *gocli.Run[cli.Global, struct{}]) error {
		got = *run.Global()
		return nil
	})
	var stdout, stderr bytes.Buffer
	env.Open, env.Stdout, env.Stderr = gocli.OS().Open, &stdout, &stderr
	status := cli.Run(t.Context(), app, compose, env)
	assert.Equal(t, status, cli.StatusOK, "the probe succeeds: "+stderr.String())
	return got
}
