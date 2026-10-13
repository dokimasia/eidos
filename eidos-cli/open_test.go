// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"
	gocli "go.stealthscale.io/cli"

	"go.dokimi.dev/eidos/cli"
	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/backend"
	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/workspace"
)

// The fixture composition and its files have these names and contents.
const (
	brand    output.Brand  = "acme"
	target   plugin.Target = "text"
	planName               = "mirrors"
	// confName is the name of the config file of the brand.
	confName = ".acme.yaml"
	// stateName is the name of the state directory of the brand.
	stateName = ".acme"
	// version is a config file that sets only the version.
	version = "version: 1\n"
	// optionsConfig is a config file with the value of an option of the
	// command line beside the name of the workspace.
	optionsConfig = "version: 1\nstrict: true\nworkspace: platform\n"
	// source is a scripted source file with one struct.
	source = "package svc\ntype Store string\n"
	// windowsOS is the GOOS of Windows.
	windowsOS = "windows"
)

// The mirror of the fixture composition fails or reports a finding for a
// struct with one of these names. It returns errBroken for Broken, reports
// a Warning under noted for Noisy, and reports an Info under noted for
// Chatty.
const (
	brokenName = "Broken"
	noisyName  = "Noisy"
	chattyName = "Chatty"
)

// The Extension cases read the extension extensionName. extendedConfig is a
// config file with the extension, and strayConfig is a config file whose
// extension has a key that canarySettings does not declare, at line 3.
const (
	extensionName  = "reference"
	extendedConfig = "version: 1\nextensions:\n    reference: {rounds: 4}\n"
	strayConfig    = "version: 1\nextensions:\n    reference: {roundz: 4}\n"
	strayMsg       = "field roundz not found in type cli_test.canarySettings"
)

// errBroken is the error that the mirror returns for a struct named Broken.
var errBroken = errors.New("cli_test: the mirror cannot mirror Broken")

// canarySettings are the settings of a consumer under the extension of the
// Extension cases.
type canarySettings struct {
	Rounds int `yaml:"rounds"`
}

// noted is the code of the findings of the mirror.
var noted = diag.MustRegister(diag.Prefix("CLITEST"), diag.CodeSpec{
	Number:  1,
	Meaning: "a finding that the mirror of the cli tests reports",
})

// Open finds the config file of the brand, and builds one member for each
// workspace that the file covers.
func TestOpen(t *testing.T) {
	t.Parallel()

	t.Run("Open", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the directory of the config file as the root", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text("version: 1\nworkspace: platform\n")})
			members := opened(t, root, cli.Global{})
			assert.Length(t, members, 1, "a document covers one workspace")
			expect.Equal(t, members[0].Name, "", "a workspace outside a list has no name")
			expect.Equal(t, members[0].Root, root, "the root is the directory of the file")
			expect.Equal(t, members[0].Config, filepath.Join(root, confName), "the member has the file")
			expect.Equal(t, members[0].Workspace.Describe().Name, "platform", "the file configures the workspace")
		})

		t.Run("searches the parent directories for the config file", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text(version), "svc/store": files.Dir()})
			members := opened(t, filepath.Join(root, "svc", "store"), cli.Global{})
			assert.Length(t, members, 1, "a document covers one workspace")
			assert.Equal(t, members[0].Root, root, "the search finds the file two directories up")
		})

		markers := []struct {
			name   string
			marker string
			give   files.Entry
		}{
			{name: "stops the search after a directory with a .git directory", marker: ".git", give: files.Dir()},
			{
				name:   "stops the search after a directory with a .git file",
				marker: ".git",
				give:   files.Text("gitdir: ../.git/worktrees/repo\n"),
			},
			{name: "stops the search after a directory with a .hg directory", marker: ".hg", give: files.Dir()},
			{name: "stops the search after a directory with a .jj directory", marker: ".jj", give: files.Dir()},
			{name: "stops the search after a directory with a .svn directory", marker: ".svn", give: files.Dir()},
		}
		for _, tt := range markers {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				root := workspaceDir(t, files.Tree{
					confName:            files.Text(version),
					"repo/" + tt.marker: tt.give,
					"repo/svc":          files.Dir(),
				})
				members := opened(t, filepath.Join(root, "repo", "svc"), cli.Global{})
				assert.Length(t, members, 1, "the repository is one workspace")
				expect.Equal(t, members[0].Root, filepath.Join(root, "repo"), "the root is the root of the repository")
				expect.Equal(t, members[0].Config, "", "the search does not leave the repository")
			})
		}

		t.Run("returns the working directory as the root outside a repository", func(t *testing.T) {
			t.Parallel()

			dir := filepath.Join(workspaceDir(t, files.Tree{"svc": files.Dir()}), "svc")
			members := opened(t, dir, cli.Global{})
			assert.Length(t, members, 1, "the working directory is one workspace")
			expect.Equal(t, members[0].Root, dir, "the root is the working directory")
			expect.Equal(t, members[0].Config, "", "the workspace has no config file")
		})

		t.Run("reads the config file of a Config relative to the working directory", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{"ci/acme.yaml": files.Text("version: 1\nworkspace: ci\n")})
			members := opened(t, root, cli.Global{Config: gocli.ConfigFile(filepath.Join("ci", "acme.yaml"))})
			assert.Length(t, members, 1, "a document covers one workspace")
			expect.Equal(t, members[0].Root, filepath.Join(root, "ci"), "the root is the directory of the file")
			expect.Equal(t, members[0].Workspace.Describe().Name, "ci", "the file configures the workspace")
		})

		t.Run("reads the config file of a Config at an absolute path", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{"ci/acme.yaml": files.Text(version)})
			config := gocli.ConfigFile(filepath.Join(root, "ci", "acme.yaml"))
			members := opened(t, t.TempDir(), cli.Global{Config: config})
			assert.Length(t, members, 1, "a document covers one workspace")
			assert.Equal(t, members[0].Config, filepath.Join(root, "ci", "acme.yaml"), "the member has the file")
		})

		t.Run("returns the directory of a linked config file of a Config as the root", func(t *testing.T) {
			t.Parallel()

			if runtime.GOOS == windowsOS {
				t.Skip("Windows creates a symbolic link only with a privilege")
			}
			root := workspaceDir(t, files.Tree{
				"shared/acme.yaml": files.Text("version: 1\nworkspace: shared\n"),
				"app/" + confName:  files.Link(filepath.Join("..", "shared", "acme.yaml")),
			})
			app := filepath.Join(root, "app")
			searched := opened(t, app, cli.Global{})
			given := opened(t, root, cli.Global{Config: gocli.ConfigFile(filepath.Join(app, confName))})
			assert.Length(t, searched, 1, "the search finds one workspace")
			assert.Length(t, given, 1, "the Config covers one workspace")
			expect.Equal(t, searched[0].Root, app, "the root of the search is the directory of the link")
			expect.Equal(t, given[0].Root, app, "the root of the Config is the directory of the link")
			expect.Equal(t, given[0].Workspace.Describe().Name, "shared", "the link reads the shared file")
		})

		t.Run("returns an error for a working directory that does not resolve", func(t *testing.T) {
			t.Parallel()

			err := refused(t, filepath.Join(t.TempDir(), "gone"), cli.Global{})
			assert.ErrorIs(t, err, os.ErrNotExist, "the error is the error of the resolution")
		})

		t.Run("returns an error for a config file of a Config that does not exist", func(t *testing.T) {
			t.Parallel()

			err := refused(t, t.TempDir(), cli.Global{Config: "absent.yaml"})
			assert.ErrorIs(t, err, os.ErrNotExist, "the error is the error of the read")
		})

		t.Run("returns an error for a Config in a directory that does not exist", func(t *testing.T) {
			t.Parallel()

			err := refused(t, t.TempDir(), cli.Global{Config: gocli.ConfigFile(filepath.Join("gone", "absent.yaml"))})
			assert.ErrorIs(t, err, os.ErrNotExist, "the error is the error of the resolution of the directory")
		})

		t.Run("returns an error for a config file of a Config that does not read", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{"ci": files.Dir()})
			err := refused(t, root, cli.Global{Config: "ci"})
			assert.Contains(t, err.Error(), "read the config file", "the error describes the read")
		})

		t.Run("returns an error for a config file that does not read", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Dir()})
			err := refused(t, root, cli.Global{})
			assert.Contains(t, err.Error(), "read the config file", "the error describes the read")
		})

		t.Run("skips the keys of the options of the command line in the config file", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text(optionsConfig)})
			members := opened(t, root, cli.Global{})
			assert.Length(t, members, 1, "a document covers one workspace")
			assert.Equal(t, members[0].Workspace.Describe().Name, "platform", "the file configures the workspace")
		})

		t.Run("returns the faults of a config file that does not decode", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text(faultyConfig)})
			err := refused(t, root, cli.Global{})
			assert.HasPrefix(t, err.Error(), confName+":2: field limt not found",
				"the error has the file relative to the root and the line of the fault")
		})

		t.Run("returns an error that wraps a ConfigError for a fault of a config file", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text(faultyConfig)})
			fault := assert.ErrorAs[*cli.ConfigError](t, refused(t, root, cli.Global{}), "the fault is a ConfigError")
			assert.Equal(t, *fault, cli.ConfigError{
				File: confName, Line: 2, Msg: "field limt not found in type config.Memo",
			}, "the fault has the file, the line and the message")
		})

		t.Run("returns the faults of Build", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text("version: 1\nplans: {ghost: {}}\n")})
			err := refused(t, root, cli.Global{})
			assert.Contains(t, err.Error(), `"ghost"`, "the error names the plan that the composition lacks")
		})

		t.Run("returns the faults of a plugin directory that does not load", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{
				confName: files.Text("version: 1\nplans: {" + planName + ": {plugins: [plugins/acme-audit]}}\n"),
			})
			err := refused(t, root, cli.Global{})
			assert.Contains(t, err.Error(), "the directory has no plugin.yaml", "the error is the fault of the plugin")
		})

		t.Run("returns the fault of Build for a composition without a valid brand", func(t *testing.T) {
			t.Parallel()

			stdio := cli.IO{Stdout: io.Discard, Stderr: io.Discard, Dir: t.TempDir(), Getenv: os.Getenv}
			_, err := cli.Open(stdio, cli.Global{}, func() *workspace.Builder { return compose().Brand("Acme") })
			assert.HasError(t, err, "a composition without a valid brand does not open")
			assert.Contains(t, err.Error(), `"Acme" is not a brand`, "the error is the fault of the brand")
		})

		t.Run("returns a member for each workspace of a list", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{
				confName: files.Text(
					"version: 1\nworkspaces:\n    - {root: ./platform, config: platform/ci.yaml}\n    - {root: tools/gen}\n",
				),
				"platform/ci.yaml": files.Text("version: 1\nworkspace: platform\n"),
				"tools/gen":        files.Dir(),
			})
			members := opened(t, root, cli.Global{})
			assert.Length(t, members, 2, "the list has two workspaces")
			expect.Equal(t, members[0].Name, "platform", "the name is the clean root of the list")
			expect.Equal(t, members[0].Root, filepath.Join(root, "platform"), "the root is below the list")
			expect.Equal(t, members[0].Config, filepath.Join(root, "platform", "ci.yaml"), "the entry sets the file")
			expect.Equal(t, members[0].Workspace.Describe().Name, "platform", "the file configures the workspace")
			expect.Equal(t, members[1].Name, "tools/gen", "the name has slashes")
			expect.Equal(t, members[1].Config, "", "the root of the entry has no config file")
		})

		t.Run("reads the config file of the brand in the root of a list entry", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{
				confName:               files.Text("version: 1\nworkspaces: [{root: platform}]\n"),
				"platform/" + confName: files.Text("version: 1\nworkspace: platform\n"),
			})
			members := opened(t, root, cli.Global{})
			assert.Length(t, members, 1, "the list has one workspace")
			assert.Equal(t, members[0].Workspace.Describe().Name, "platform", "the file configures the workspace")
		})

		t.Run("returns an error for a root of a list that does not exist", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text("version: 1\nworkspaces: [{root: ghost}]\n")})
			err := refused(t, root, cli.Global{})
			assert.Contains(t, err.Error(), "cli: the workspace root ghost does not resolve",
				"the error names the root")
		})

		t.Run("returns an error for two roots of a list that nest", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{
				confName:  files.Text("version: 1\nworkspaces: [{root: svc}, {root: tools}, {root: svc/api}]\n"),
				"svc/api": files.Dir(),
				"tools":   files.Dir(),
			})
			err := refused(t, root, cli.Global{})
			assert.Equal(t, err.Error(), "cli: the workspace roots svc and svc/api nest", "the error names both roots")
		})

		t.Run("returns an error for a root that a list names twice", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{
				confName: files.Text("version: 1\nworkspaces: [{root: svc}, {root: ./svc}]\n"),
				"svc":    files.Dir(),
			})
			err := refused(t, root, cli.Global{})
			assert.Equal(t, err.Error(), "cli: the workspace roots svc and ./svc nest", "the error names both roots")
		})

		t.Run("returns an error for a config file of a list entry that is a list", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{
				confName:          files.Text("version: 1\nworkspaces: [{root: svc}]\n"),
				"svc/" + confName: files.Text("version: 1\nworkspaces: [{root: api}]\n"),
			})
			fault := assert.ErrorAs[*cli.ConfigError](t, refused(t, root, cli.Global{}), "the fault is a ConfigError")
			assert.Equal(t, *fault, cli.ConfigError{
				File: "svc/" + confName, Line: 2, Msg: "the config file of a member of a list lists workspaces",
			}, "the fault has the file relative to the list and the line of the key")
		})

		t.Run("returns an error for the key of an option in the config file of a list entry", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{
				confName:          files.Text("version: 1\nstrict: true\nworkspaces: [{root: svc}]\n"),
				"svc/" + confName: files.Text("version: 1\nstrict: true\n"),
			})
			fault := assert.ErrorAs[*cli.ConfigError](t, refused(t, root, cli.Global{}), "the fault is a ConfigError")
			assert.Equal(t, *fault, cli.ConfigError{
				File: "svc/" + confName, Line: 2, Msg: "field strict not found in type config.Document",
			}, "the config file of a member sets no options")
		})

		t.Run("returns an error for a config file of a list entry that does not exist", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{
				confName: files.Text("version: 1\nworkspaces: [{root: svc, config: svc/ci.yaml}]\n"),
				"svc":    files.Dir(),
			})
			err := refused(t, root, cli.Global{})
			assert.ErrorIs(t, err, os.ErrNotExist, "the error is the error of the read")
		})

		t.Run("returns the faults of a config file of a list entry that does not decode", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{
				confName:          files.Text("version: 1\nworkspaces: [{root: svc}]\n"),
				"svc/" + confName: files.Text("version: 2\n"),
			})
			err := refused(t, root, cli.Global{})
			assert.HasPrefix(t, err.Error(), "svc/"+confName+":1: the file has version 2",
				"the error has the file relative to the list and the fault of the file")
		})

		t.Run("returns the faults of Build after the name of the list entry", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{
				confName:          files.Text("version: 1\nworkspaces: [{root: svc}]\n"),
				"svc/" + confName: files.Text("version: 1\nplans: {ghost: {}}\n"),
			})
			err := refused(t, root, cli.Global{})
			assert.HasPrefix(t, err.Error(), "cli: the workspace svc does not build\n",
				"the first line names the entry")
			assert.Contains(t, err.Error(), `"ghost"`, "the next line is the fault of Build")
		})

		t.Run("returns a workspace that writes its output and its state below its root", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text(version), "svc/store.zz": files.Text(source)})
			members := opened(t, root, cli.Global{})
			assert.Length(t, members, 1, "a document covers one workspace")
			_, err := members[0].Workspace.Run(t.Context(), workspace.Input{Tree: os.DirFS(root)})
			assert.NoError(t, err, "the run succeeds")
			files.IsFile(t, filepath.Join(root, "svc", "mirror.txt"), "the run writes the output below the root")
			files.IsDir(t, filepath.Join(root, stateName), "the run writes the state directory into the root")
		})

		t.Run("returns a workspace that writes to disk where the composition sets an output", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text(version), "svc/store.zz": files.Text(source)})
			sink := output.NewMem()
			stdio := cli.IO{Stdout: io.Discard, Stderr: io.Discard, Dir: root, Getenv: os.Getenv}
			members, err := cli.Open(stdio, cli.Global{}, func() *workspace.Builder {
				return compose().
					Output(func() (output.Sink, error) { return sink, nil }).
					Ledger(func() (ledger.Ledger, error) { return ledger.NewMem(), nil })
			})
			assert.NoError(t, err, "the workspaces open")
			assert.Length(t, members, 1, "a document covers one workspace")
			_, err = members[0].Workspace.Run(t.Context(), workspace.Input{Tree: os.DirFS(root)})
			assert.NoError(t, err, "the run succeeds")
			files.IsFile(t, filepath.Join(root, "svc", "mirror.txt"), "the run writes the output below the root")
			assert.Empty(t, sink.Files(), "the output of the composition receives nothing")
		})

		t.Run("builds one composition for a workspace outside a list", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text(version)})
			stdio := cli.IO{Stdout: io.Discard, Stderr: io.Discard, Dir: root, Getenv: os.Getenv}
			var calls atomic.Int32
			members, err := cli.Open(stdio, cli.Global{}, func() *workspace.Builder {
				calls.Add(1)
				return compose()
			})
			assert.NoError(t, err, "the workspace opens")
			assert.Length(t, members, 1, "a document covers one workspace")
			assert.Equal(t, calls.Load(), int32(1), "the composition that names the brand builds the workspace")
		})

		t.Run("panics for an IO with a relative working directory", func(t *testing.T) {
			t.Parallel()

			stdio := cli.IO{Stdout: io.Discard, Stderr: io.Discard, Dir: "svc", Getenv: os.Getenv}
			assert.Panics(t, func() { _, _ = cli.Open(stdio, cli.Global{}, compose) },
				"a relative working directory is a defect of the host")
		})
	})

	t.Run("Member", func(t *testing.T) {
		t.Parallel()

		t.Run("Extension", func(t *testing.T) {
			t.Parallel()

			t.Run("decodes the extension of the config file of the member", func(t *testing.T) {
				t.Parallel()

				members := opened(t, workspaceDir(t, files.Tree{confName: files.Text(extendedConfig)}), cli.Global{})
				assert.Length(t, members, 1, "a document covers one workspace")
				var got canarySettings
				found, err := members[0].Extension(extensionName, &got)
				assert.NoError(t, err, "the extension decodes")
				assert.True(t, found, "the config file has the extension")
				assert.Equal(t, got, canarySettings{Rounds: 4}, "the settings have the value of the extension")
			})

			t.Run("decodes the extension of the config file of a member of a list", func(t *testing.T) {
				t.Parallel()

				root := workspaceDir(t, files.Tree{
					confName:          files.Text("version: 1\nworkspaces: [{root: svc}]\n"),
					"svc/" + confName: files.Text(extendedConfig),
				})
				members := opened(t, root, cli.Global{})
				assert.Length(t, members, 1, "the list has one workspace")
				var got canarySettings
				_, err := members[0].Extension(extensionName, &got)
				assert.NoError(t, err, "the extension decodes")
				assert.Equal(t, got, canarySettings{Rounds: 4}, "the member reads its own config file")
			})

			t.Run("reports false for a member without a config file", func(t *testing.T) {
				t.Parallel()

				members := opened(t, workspaceDir(t, files.Tree{".git": files.Dir()}), cli.Global{})
				assert.Length(t, members, 1, "the repository is one workspace")
				var got canarySettings
				found, err := members[0].Extension(extensionName, &got)
				assert.NoError(t, err, "a member without a config file has no fault")
				assert.False(t, found, "the member has no extension")
			})

			faults := []struct {
				name string
				give files.Tree
				want cli.ConfigError
			}{
				{
					name: "returns an error that wraps a ConfigError for a key that v does not declare",
					give: files.Tree{confName: files.Text(strayConfig)},
					want: cli.ConfigError{File: confName, Line: 3, Msg: strayMsg},
				},
				{
					name: "returns a ConfigError with the file of a member of a list relative to the list",
					give: files.Tree{
						confName:          files.Text("version: 1\nworkspaces: [{root: svc}]\n"),
						"svc/" + confName: files.Text(strayConfig),
					},
					want: cli.ConfigError{File: "svc/" + confName, Line: 3, Msg: strayMsg},
				},
			}
			for _, tt := range faults {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					members := opened(t, workspaceDir(t, tt.give), cli.Global{})
					assert.Length(t, members, 1, "the file covers one workspace")
					var got canarySettings
					_, err := members[0].Extension(extensionName, &got)
					fault := assert.ErrorAs[*cli.ConfigError](t, err, "the fault is a ConfigError")
					assert.Equal(t, *fault, tt.want, "the fault has the file, the line and the message")
				})
			}
		})
	})
}

// compose returns the fixture composition over the scripted frontend. It is
// the [cli.Compose] of most cases.
func compose() *workspace.Builder {
	return composed(frontendtest.NewScripted())
}

// composed returns the fixture composition over the frontend f, which loads
// the scripted language. Its one plan mirrors each struct into the text
// file of its package.
func composed(f plugin.Frontend) *workspace.Builder {
	return workspace.New().
		Brand(brand).
		Frontends(f).
		Targets(target).
		Plans(workspace.Plan{Name: planName, Generators: []plugin.Generator{mirror()}, Backend: printer()})
}

// mirror returns the generator of the plan of the fixture. It mirrors each
// struct into the file mirror.txt of its package. It fails for a struct
// named Broken, and reports a finding for a struct named Noisy or Chatty.
func mirror() plugin.Generator {
	return eidos.NewPlugin("mirror").
		Output(plugin.Output{Per: plugin.PerPackage, Word: "mirror"}).
		Handle(eidos.OnStruct(func(m *eidos.StructMatch, e *eidos.Emitter) error {
			switch m.Struct.Name {
			case brokenName:
				return errBroken
			case noisyName:
				m.Warnf(noted, "%s is noisy", m.Struct.Name)
			case chattyName:
				m.Infof(noted, "%s is chatty", m.Struct.Name)
			}
			e.PackageFile().Append(&emit.Struct{Origin: m.Struct.Identity(), Name: "For" + m.Struct.Name})
			return nil
		})).
		Generator()
}

// printer returns the backend of the fixture plans. It renders each struct
// as one line into the text file whose name is the word of the unit, and it
// declares the lowering policies policies.
func printer(policies ...plugin.PolicySpec) plugin.Backend {
	coverage := map[symbol.Fact]render.Verdict{}
	for _, f := range symbol.Facts() {
		coverage[f] = render.Renders
	}
	return backend.New("printer", target, plugin.CommentSyntax{Line: []string{"//"}}).
		KindTemplates(map[symbol.Kind]string{symbol.KindStruct: "type {{.Name}} struct{}\n"}).
		Naming(func(u plugin.Unit) string { return u.Word + ".txt" }).
		Scaffold(func(emit.Stmt, *render.ImportSet) ([]byte, error) {
			return nil, errors.New("cli_test: the printer writes no statements")
		}).
		Imports(func(*render.ImportSet) string { return "" }).
		Finalise(func(src []byte) ([]byte, error) { return src, nil }).
		Coverage(render.Coverage{Facts: coverage}).
		Policies(policies...).
		Build()
}

// workspaceDir writes tree into a new directory of the test with
// files.Workspace, and returns the directory with its symbolic links
// resolved, as Open resolves the working directory.
func workspaceDir(t *testing.T, tree files.Tree) string {
	t.Helper()

	dir, err := filepath.EvalSymlinks(files.Workspace(t, tree))
	assert.NoError(t, err, "the directory of the test resolves")
	return dir
}

// opened opens the workspaces for the working directory dir with the global
// options global, and fails the test when Open returns an error.
func opened(t *testing.T, dir string, global cli.Global) []cli.Member {
	t.Helper()

	stdio := cli.IO{Stdout: io.Discard, Stderr: io.Discard, Dir: dir, Getenv: os.Getenv}
	members, err := cli.Open(stdio, global, compose)
	assert.NoError(t, err, "the workspaces open")
	return members
}

// refused opens the workspaces for the working directory dir with the
// global options global, and returns the error of Open. It fails the test
// when Open returns no error.
func refused(t *testing.T, dir string, global cli.Global) error {
	t.Helper()

	stdio := cli.IO{Stdout: io.Discard, Stderr: io.Discard, Dir: dir, Getenv: os.Getenv}
	_, err := cli.Open(stdio, global, compose)
	assert.HasError(t, err, "Open returns a config error")
	return err
}
