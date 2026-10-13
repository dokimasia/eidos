// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/cli"
)

// configuredName is the name of the workspace that the config files of
// the Configure cases set.
const configuredName = "platform"

// Configure applies the config file of the brand in one root to a
// composition, so a test configures a workspace as the binary does.
func TestConfigure(t *testing.T) {
	t.Parallel()

	t.Run("Configure", func(t *testing.T) {
		t.Parallel()

		t.Run("applies the config file of the brand in the root", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text("version: 1\nworkspace: " + configuredName + "\n")})
			b := compose()
			assert.NoError(t, cli.Configure(b, root), "the config file applies")
			w, err := b.Build()
			assert.NoError(t, err, "the configured composition builds")
			assert.Equal(t, w.Describe().Name, configuredName, "the config file names the workspace")
		})

		t.Run("leaves the composition as it is for a root without a config file", func(t *testing.T) {
			t.Parallel()

			b := compose()
			assert.NoError(t, cli.Configure(b, t.TempDir()), "a root without a config file configures nothing")
			w, err := b.Build()
			assert.NoError(t, err, "the composition builds")
			plain, err := compose().Build()
			assert.NoError(t, err, "the plain composition builds")
			assert.Equal(t, w.Describe().Name, plain.Describe().Name, "the composition keeps its own name")
		})

		t.Run("searches no parent directory for the config file", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{
				confName: files.Text("version: 1\nworkspace: " + configuredName + "\n"),
				"svc":    files.Dir(),
			})
			b := compose()
			assert.NoError(t, cli.Configure(b, filepath.Join(root, "svc")), "the subdirectory has no config file")
			w, err := b.Build()
			assert.NoError(t, err, "the composition builds")
			assert.NotEqual(t, w.Describe().Name, configuredName, "the config file of the parent does not apply")
		})

		t.Run("returns the faults of a config file that does not decode", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text(faultyConfig)})
			err := cli.Configure(compose(), root)
			assert.HasError(t, err, "the config file does not decode")
			assert.HasPrefix(t, err.Error(), confName+":2: field limt not found",
				"the error has the file relative to the root and the line of the fault")
		})

		t.Run("returns an error that wraps a ConfigError for a fault of the config file", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text(faultyConfig)})
			fault := assert.ErrorAs[*cli.ConfigError](t, cli.Configure(compose(), root), "the fault is a ConfigError")
			assert.Equal(t, fault.Line, 2, "the fault has the line of the key")
		})

		t.Run("skips the keys of the options of the command line", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text(optionsConfig)})
			b := compose()
			assert.NoError(t, cli.Configure(b, root), "the keys of the options are no faults")
			w, err := b.Build()
			assert.NoError(t, err, "the composition builds")
			assert.Equal(t, w.Describe().Name, "platform", "the config file configures the composition")
		})

		t.Run("returns an error for a config file that lists workspaces", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text("version: 1\nworkspaces: [{root: svc}]\n")})
			err := cli.Configure(compose(), root)
			assert.HasError(t, err, "a list configures no single composition")
			assert.Contains(t, err.Error(), "lists workspaces", "the error describes the file")
		})

		t.Run("returns an error for a config file that does not read", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Dir()})
			err := cli.Configure(compose(), root)
			assert.HasError(t, err, "a directory does not read as a file")
			assert.Contains(t, err.Error(), "read the config file", "the error describes the read")
		})

		t.Run("returns an error for a composition without a valid brand", func(t *testing.T) {
			t.Parallel()

			err := cli.Configure(compose().Brand("Acme"), t.TempDir())
			assert.HasError(t, err, "a composition without a valid brand has no config file")
			assert.Contains(t, err.Error(), `"Acme"`, "the error names the brand")
		})
	})
}
