// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/cli"
)

// coreModule is the module path of the kernel. The command version writes
// the version of this module.
const coreModule = "go.dokimi.dev/eidos/core"

// digestPrefix opens a digest in the output of version.
const digestPrefix = "sha256:"

// copyMode is the mode of the copy of the test binary, which the process
// case runs.
const copyMode fs.FileMode = 0o755

// moduleEvent is the main module of the binary in a version event.
type moduleEvent struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}

// versionEvent is a version event of JSON output, with the fields that the
// cases read.
type versionEvent struct {
	Module     moduleEvent `json:"module"`
	Executable string      `json:"executable"`
	Workspace  string      `json:"workspace"`
}

// Version writes the versions of the binary, of eidos and of the plugins of
// each workspace, and the digests of the composition and of the executable.
func TestVersion(t *testing.T) {
	t.Parallel()

	info, ok := debug.ReadBuildInfo()
	assert.True(t, ok, "the test binary has build information")
	core := ""
	for _, d := range info.Deps {
		if d.Path == coreModule {
			core = d.Version
		}
	}

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("writes the versions and the digests as text", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text(version)})
			status, stdout, stderr := invoke(t, described, cmdVersion, root)
			expect.Equal(t, status, cli.StatusOK, "version succeeds")
			expect.Equal(t, stdout, strings.Join([]string{
				"module " + info.Main.Path + " " + info.Main.Version,
				"core " + core,
				"cli " + info.Main.Version,
				"frontend fakefront 1",
				"annotator flagger 2",
				"generator mirror",
				"backend printer",
				"generator binder 3",
				"check lister",
				"composition " + fingerprinted(t, root),
				"executable " + executableDigest(t),
			}, "\n")+"\n", "the output lists each plugin once")
			expect.Empty(t, stderr, "version reports nothing")
		})

		t.Run("writes a version event for each member of a list", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, compose, cmdVersion, listed(t), jsonFlag)
			got := decoded[versionEvent](t, stdout, eventVersion)
			assert.Length(t, got, 2, "each member has one event")
			expect.Equal(t, got[0].Workspace, memberSvc, "the first event names the first member")
			expect.Equal(t, got[0].Executable, executableDigest(t), "the event has the digest of the executable")
			expect.Equal(t, got[0].Module, moduleEvent{Path: info.Main.Path, Version: info.Main.Version},
				"the event has the main module of the binary")
			expect.Equal(t, got[1].Workspace, memberTool, "the second event names the second member")
		})

		t.Run("returns StatusUsage for a config error", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text(faultyConfig)})
			status, _, stderr := invoke(t, compose, cmdVersion, root)
			expect.Equal(t, status, cli.StatusUsage, "the config error is a usage error")
			expect.Contains(t, stderr, "field workerz not found", "version writes the fault")
		})
	})
}

// Version returns StatusFailed for an executable that does not read. The
// case runs a copy of the test binary as the binary, and removes the copy
// before version reads it, so it runs alone.
func TestVersionProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not remove the executable of a running process")
	}
	self, err := os.Executable()
	assert.NoError(t, err, "the test binary has a path")
	data, err := os.ReadFile(self)
	assert.NoError(t, err, "the test binary reads")
	binary := filepath.Join(t.TempDir(), filepath.Base(self))
	assert.NoError(t, os.WriteFile(binary, data, copyMode), "the test writes the copy of the binary")
	cmd := process(t, binary, t.TempDir(), cmdVersion)
	stdin, err := cmd.StdinPipe()
	assert.NoError(t, err, "the test opens the standard input of the copy")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	assert.NoError(t, cmd.Start(), "the copy starts")
	assert.NoError(t, os.Remove(binary), "the test removes the copy")
	assert.NoError(t, stdin.Close(), "the test lets the copy run")
	err = cmd.Wait()
	_, exited := errors.AsType[*exec.ExitError](err)
	assert.True(t, exited, "the copy exits with a status")
	expect.Equal(t, cmd.ProcessState.ExitCode(), cli.StatusFailed, "the removed executable fails version")
	expect.HasPrefix(t, stderr.String(), "error: cli: read the executable: ", "version writes the error of the read")
	expect.Empty(t, stdout.String(), "version writes no version")
}

// fingerprinted returns the digest of the composition fingerprint of the
// described composition in root, as "sha256:" and the hex digest.
func fingerprinted(t *testing.T, root string) string {
	t.Helper()

	stdio := cli.IO{Stdout: io.Discard, Stderr: io.Discard, Dir: root, Getenv: os.Getenv}
	members, err := cli.Open(stdio, cli.Flags{}, described)
	assert.NoError(t, err, "the workspace opens")
	assert.Length(t, members, 1, "the root is one workspace")
	sum := sha256.Sum256(members[0].Workspace.Describe().Fingerprint)
	return digestPrefix + hex.EncodeToString(sum[:])
}

// executableDigest returns the digest of the running test binary, as
// "sha256:" and the hex digest.
func executableDigest(t *testing.T) string {
	t.Helper()

	self, err := os.Executable()
	assert.NoError(t, err, "the test binary has a path")
	data, err := os.ReadFile(self)
	assert.NoError(t, err, "the test binary reads")
	sum := sha256.Sum256(data)
	return digestPrefix + hex.EncodeToString(sum[:])
}
