// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/cli"
	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/backend"
	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
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
	// source is a scripted source file with one struct.
	source = "package svc\ntype Store string\n"
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

// errBroken is the error that the mirror returns for a struct named Broken.
var errBroken = errors.New("cli_test: the mirror cannot mirror Broken")

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
			members := opened(t, root, cli.Flags{})
			assert.Length(t, members, 1, "a document covers one workspace")
			expect.Equal(t, members[0].Name, "", "a workspace outside a list has no name")
			expect.Equal(t, members[0].Root, root, "the root is the directory of the file")
			expect.Equal(t, members[0].Config, filepath.Join(root, confName), "the member has the file")
			expect.Equal(t, members[0].Workspace.Describe().Name, "platform", "the file configures the workspace")
		})

		t.Run("searches the parent directories for the config file", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text(version), "svc/store": files.Dir()})
			members := opened(t, filepath.Join(root, "svc", "store"), cli.Flags{})
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
				members := opened(t, filepath.Join(root, "repo", "svc"), cli.Flags{})
				assert.Length(t, members, 1, "the repository is one workspace")
				expect.Equal(t, members[0].Root, filepath.Join(root, "repo"), "the root is the root of the repository")
				expect.Equal(t, members[0].Config, "", "the search does not leave the repository")
			})
		}

		t.Run("returns the working directory as the root outside a repository", func(t *testing.T) {
			t.Parallel()

			dir := filepath.Join(workspaceDir(t, files.Tree{"svc": files.Dir()}), "svc")
			members := opened(t, dir, cli.Flags{})
			assert.Length(t, members, 1, "the working directory is one workspace")
			expect.Equal(t, members[0].Root, dir, "the root is the working directory")
			expect.Equal(t, members[0].Config, "", "the workspace has no config file")
		})

		t.Run("reads the config file of --config relative to the working directory", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{"ci/acme.yaml": files.Text("version: 1\nworkspace: ci\n")})
			members := opened(t, root, cli.Flags{Config: filepath.Join("ci", "acme.yaml")})
			assert.Length(t, members, 1, "a document covers one workspace")
			expect.Equal(t, members[0].Root, filepath.Join(root, "ci"), "the root is the directory of the file")
			expect.Equal(t, members[0].Workspace.Describe().Name, "ci", "the file configures the workspace")
		})

		t.Run("reads the config file of --config at an absolute path", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{"ci/acme.yaml": files.Text(version)})
			members := opened(t, t.TempDir(), cli.Flags{Config: filepath.Join(root, "ci", "acme.yaml")})
			assert.Length(t, members, 1, "a document covers one workspace")
			assert.Equal(t, members[0].Config, filepath.Join(root, "ci", "acme.yaml"), "the member has the file")
		})

		t.Run("returns an error for a working directory that does not resolve", func(t *testing.T) {
			t.Parallel()

			err := refused(t, filepath.Join(t.TempDir(), "gone"), cli.Flags{})
			assert.ErrorIs(t, err, os.ErrNotExist, "the error is the error of the resolution")
		})

		t.Run("returns an error for a config file of --config that does not exist", func(t *testing.T) {
			t.Parallel()

			err := refused(t, t.TempDir(), cli.Flags{Config: "absent.yaml"})
			assert.ErrorIs(t, err, os.ErrNotExist, "the error is the error of the read")
		})

		t.Run("returns an error for a config file of --config that does not read", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{"ci": files.Dir()})
			err := refused(t, root, cli.Flags{Config: "ci"})
			assert.Contains(t, err.Error(), "read the config file", "the error describes the read")
		})

		t.Run("returns an error for a config file that does not read", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Dir()})
			err := refused(t, root, cli.Flags{})
			assert.Contains(t, err.Error(), "read the config file", "the error describes the read")
		})

		t.Run("returns the faults of a config file that does not decode", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text("version: 1\nworkerz: 4\n")})
			err := refused(t, root, cli.Flags{})
			assert.Contains(t, err.Error(), filepath.Join(root, confName)+":2: field workerz not found",
				"the error has the file and the line of the fault")
		})

		t.Run("returns the faults of Build", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text("version: 1\nplans: {ghost: {}}\n")})
			err := refused(t, root, cli.Flags{})
			assert.Contains(t, err.Error(), `"ghost"`, "the error names the plan that the composition lacks")
		})

		t.Run("returns the fault of Build for a composition without a valid brand", func(t *testing.T) {
			t.Parallel()

			stdio := cli.IO{Stdout: io.Discard, Stderr: io.Discard, Dir: t.TempDir(), Getenv: os.Getenv}
			_, err := cli.Open(stdio, cli.Flags{}, func() *workspace.Builder { return compose().Brand("Acme") })
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
			members := opened(t, root, cli.Flags{})
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
			members := opened(t, root, cli.Flags{})
			assert.Length(t, members, 1, "the list has one workspace")
			assert.Equal(t, members[0].Workspace.Describe().Name, "platform", "the file configures the workspace")
		})

		t.Run("returns an error for a root of a list that does not exist", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text("version: 1\nworkspaces: [{root: ghost}]\n")})
			err := refused(t, root, cli.Flags{})
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
			err := refused(t, root, cli.Flags{})
			assert.Equal(t, err.Error(), "cli: the workspace roots svc and svc/api nest", "the error names both roots")
		})

		t.Run("returns an error for a root that a list names twice", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{
				confName: files.Text("version: 1\nworkspaces: [{root: svc}, {root: ./svc}]\n"),
				"svc":    files.Dir(),
			})
			err := refused(t, root, cli.Flags{})
			assert.Equal(t, err.Error(), "cli: the workspace roots svc and ./svc nest", "the error names both roots")
		})

		t.Run("returns an error for a config file of a list entry that is a list", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{
				confName:          files.Text("version: 1\nworkspaces: [{root: svc}]\n"),
				"svc/" + confName: files.Text("version: 1\nworkspaces: [{root: api}]\n"),
			})
			err := refused(t, root, cli.Flags{})
			assert.Contains(t, err.Error(), "of a workspace in a list is a list", "the error describes the file")
		})

		t.Run("returns an error for a config file of a list entry that does not exist", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{
				confName: files.Text("version: 1\nworkspaces: [{root: svc, config: svc/ci.yaml}]\n"),
				"svc":    files.Dir(),
			})
			err := refused(t, root, cli.Flags{})
			assert.ErrorIs(t, err, os.ErrNotExist, "the error is the error of the read")
		})

		t.Run("returns the faults of a config file of a list entry that does not decode", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{
				confName:          files.Text("version: 1\nworkspaces: [{root: svc}]\n"),
				"svc/" + confName: files.Text("version: 2\n"),
			})
			err := refused(t, root, cli.Flags{})
			assert.Contains(t, err.Error(), "the file has version 2", "the error is the fault of the file")
		})

		t.Run("returns the faults of Build after the name of the list entry", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{
				confName:          files.Text("version: 1\nworkspaces: [{root: svc}]\n"),
				"svc/" + confName: files.Text("version: 1\nplans: {ghost: {}}\n"),
			})
			err := refused(t, root, cli.Flags{})
			assert.HasPrefix(t, err.Error(), "cli: the workspace svc does not build\n",
				"the first line names the entry")
			assert.Contains(t, err.Error(), `"ghost"`, "the next line is the fault of Build")
		})

		t.Run("returns a workspace that writes its output and its state below its root", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text(version), "svc/store.zz": files.Text(source)})
			members := opened(t, root, cli.Flags{})
			assert.Length(t, members, 1, "a document covers one workspace")
			_, err := members[0].Workspace.Run(t.Context(), workspace.Input{Tree: os.DirFS(root)})
			assert.NoError(t, err, "the run succeeds")
			files.IsFile(t, filepath.Join(root, "svc", "mirror.txt"), "the run writes the output below the root")
			files.IsDir(t, filepath.Join(root, stateName), "the run writes the state directory into the root")
		})

		t.Run("panics for an IO with a relative working directory", func(t *testing.T) {
			t.Parallel()

			stdio := cli.IO{Stdout: io.Discard, Stderr: io.Discard, Dir: "svc", Getenv: os.Getenv}
			assert.Panics(t, func() { _, _ = cli.Open(stdio, cli.Flags{}, compose) },
				"a relative working directory is a defect of the host")
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
		Build().(plugin.Generator)
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

// opened opens the workspaces for the working directory dir, and fails the
// test when Open returns an error.
func opened(t *testing.T, dir string, f cli.Flags) []cli.Member {
	t.Helper()

	stdio := cli.IO{Stdout: io.Discard, Stderr: io.Discard, Dir: dir, Getenv: os.Getenv}
	members, err := cli.Open(stdio, f, compose)
	assert.NoError(t, err, "the workspaces open")
	return members
}

// refused opens the workspaces for the working directory dir, and returns
// the error of Open. It fails the test when Open returns no error.
func refused(t *testing.T, dir string, f cli.Flags) error {
	t.Helper()

	stdio := cli.IO{Stdout: io.Discard, Stderr: io.Discard, Dir: dir, Getenv: os.Getenv}
	_, err := cli.Open(stdio, f, compose)
	assert.HasError(t, err, "Open returns a config error")
	return err
}
