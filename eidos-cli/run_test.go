// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/core/frontend"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/workspace"
)

// The run cases write sources at storePath and badPath, and a run writes the
// mirror of the package svc at mirrorPath. badSource has no package line,
// so the scripted frontend reports it as an Error. The mirror fails for
// brokenSource, and reports a finding for noisySource and chattySource.
const (
	storePath    = "svc/store.zz"
	mirrorPath   = "svc/mirror.txt"
	badPath      = "svc/bad.zz"
	badSource    = "type Bad string\n"
	brokenSource = "package svc\ntype " + brokenName + " string\n"
	noisySource  = "package svc\ntype " + noisyName + " string\n"
	chattySource = "package svc\ntype " + chattyName + " string\n"
)

// The packages of the narrowed cases, and the config file of a list of the
// workspaces svc and tools.
const (
	pkgA       = "svc/a"
	pkgB       = "svc/b"
	listConfig = "version: 1\nworkspaces: [{root: svc}, {root: tools}]\n"
	memberSvc  = "svc"
	memberTool = "tools"
)

// here is the pattern of a go:generate line. The go command runs the line
// in the directory of its package.
const here = "."

// The body of the mirror of the store, and the body that a person edited.
const (
	mirroredBody = "type ForStore struct{}"
	editedBody   = "type ForStore struct{ edited }"
)

// The actions of a file event that the cases read.
const (
	actionCreate = "create"
	actionStale  = "stale"
)

// storeVar is the variable that the locator of a locating composition
// reads.
const storeVar = "CLITEST_STORE"

// faultyConfig is a config file with a key that the format does not have.
const faultyConfig = "version: 1\nworkerz: 4\n"

// createdSummary is the summary line of text output for a run that created
// one file and committed one plan.
const createdSummary = "files: 1 create, 0 update, 0 unchanged, 0 stale, 0 drifted, 0 foreign, 0 withheld; " +
	"plans: 1 committed, 0 failed, 0 cancelled, 0 prepared, 0 skipped\n"

// refusedPattern is a pattern inside the root that a run refuses, because it
// has a wildcard inside a directory name.
const refusedPattern = "svc/a...b"

// handWritten is the content of a file without the frame of the brand.
const handWritten = "written by hand\n"

// errLocate is the error of the locator of the failing composition.
var errLocate = errors.New("cli_test: the store does not resolve")

// suppressing is the scripted line of a diag directive that removes the
// findings of the mirror at the struct above it.
var suppressing = "+diag off=" + noted.String() + "\n"

// The lines of text output for the error of the mirror on the struct
// Broken, and for the error of the locator of the failing composition.
var (
	brokenError = `error: workspace: plan "mirrors": generator mirror in bucket 1: eidos: mirror rule 0: ` +
		errBroken.Error() + "\n"
	locateError = "error: workspace: frontend locator: " + errLocate.Error() + "\n"
)

// failing is the fixture composition over a frontend whose locator returns
// errLocate.
var failing = locatingCompose(func(func(string) string) (map[string]fs.FS, error) { return nil, errLocate })

// fileEvent is a file event of JSON output.
type fileEvent struct {
	Path      string `json:"path"`
	Plan      string `json:"plan"`
	Action    string `json:"action"`
	Found     string `json:"found"`
	Hash      string `json:"hash"`
	Workspace string `json:"workspace"`
}

// outcomeEvent is an outcome event of JSON output.
type outcomeEvent struct {
	Plan   string `json:"plan"`
	Status string `json:"status"`
}

// Run runs each workspace once, and renders the report of each run.
func TestRun(t *testing.T) {
	t.Parallel()

	t.Run("Run", func(t *testing.T) {
		t.Parallel()

		t.Run("writes each change above the summary line to standard output", func(t *testing.T) {
			t.Parallel()

			status, stdout, stderr := invoke(t, compose, cmdRun, stored(t, nil))
			expect.Equal(t, status, cli.StatusOK, "the run succeeds")
			expect.Equal(t, stdout, "create svc/mirror.txt\n"+createdSummary, "the output lists the created file")
			expect.Empty(t, stderr, "the run reports nothing")
		})

		t.Run("commits the files of each plan", func(t *testing.T) {
			t.Parallel()

			root := stored(t, nil)
			ran(t, root)
			assert.Contains(t, files.Read(t, filepath.Join(root, "svc", "mirror.txt")), mirroredBody,
				"the run writes the mirror of the store")
		})

		t.Run("writes no line for a file that the run leaves unchanged", func(t *testing.T) {
			t.Parallel()

			root := stored(t, nil)
			ran(t, root)
			_, stdout, _ := invoke(t, compose, cmdRun, root, coldFlag)
			assert.Equal(t, stdout,
				"files: 0 create, 0 update, 1 unchanged, 0 stale, 0 drifted, 0 foreign, 0 withheld; "+
					"plans: 1 committed, 0 failed, 0 cancelled, 0 prepared, 0 skipped\n",
				"the summary of the cold run counts the unchanged file")
		})

		t.Run("writes only the summary line for a warm run over an unchanged tree", func(t *testing.T) {
			t.Parallel()

			root := stored(t, nil)
			ran(t, root)
			_, stdout, _ := invoke(t, compose, cmdRun, root)
			assert.Equal(t, stdout,
				"files: 0 create, 0 update, 0 unchanged, 0 stale, 0 drifted, 0 foreign, 0 withheld; "+
					"plans: 1 committed, 0 failed, 0 cancelled, 0 prepared, 0 skipped\n",
				"the warm run renders no file again")
		})

		t.Run("returns StatusFailed for an Error finding", func(t *testing.T) {
			t.Parallel()

			status, _, stderr := invoke(t, compose, cmdRun, stored(t, files.Tree{badPath: files.Text(badSource)}))
			expect.Equal(t, status, cli.StatusFailed, "the Error fails the run")
			expect.Equal(t, stderr,
				badPath+":1: error "+frontendtest.ScriptedBadFile.String()+
					`: svc/bad.zz opens with "type", not a package line (fakefront)`+"\n",
				"the run writes the finding and no other error")
		})

		t.Run("commits nothing under --dry-run", func(t *testing.T) {
			t.Parallel()

			root := stored(t, nil)
			status, stdout, _ := invoke(t, compose, cmdRun, root, dryRunFlag)
			expect.Equal(t, status, cli.StatusOK, "the dry run succeeds")
			expect.Equal(t, stdout, "create svc/mirror.txt\n"+
				"files: 1 create, 0 update, 0 unchanged, 0 stale, 0 drifted, 0 foreign, 0 withheld; "+
				"plans: 0 committed, 0 failed, 0 cancelled, 1 prepared, 0 skipped\n",
				"the dry run lists the change that it prepared")
			files.Absent(t, filepath.Join(root, "svc", "mirror.txt"), "the dry run writes no file")
		})

		t.Run("returns StatusFailed under --check for an output that is out of date", func(t *testing.T) {
			t.Parallel()

			root := stored(t, nil)
			status, _, stderr := invoke(t, compose, cmdRun, root, checkFlag)
			expect.Equal(t, status, cli.StatusFailed, "the missing mirror fails the check")
			expect.Contains(t, stderr, mirrorPath+": error "+workspace.OutOfDate.String(),
				"the check reports the path")
			files.Absent(t, filepath.Join(root, "svc", "mirror.txt"), "the check writes no file")
		})

		t.Run("returns StatusOK under --check for an output that is current", func(t *testing.T) {
			t.Parallel()

			root := stored(t, nil)
			ran(t, root)
			status, _, stderr := invoke(t, compose, cmdRun, root, checkFlag)
			assert.Equal(t, status, cli.StatusOK, "the committed output is current: "+stderr)
		})

		t.Run("commits only the changes inside the patterns", func(t *testing.T) {
			t.Parallel()

			root := packages(t)
			ran(t, root, pkgA)
			files.IsFile(t, filepath.Join(root, "svc", "a", "mirror.txt"), "the run commits the file of svc/a")
			files.Absent(t, filepath.Join(root, "svc", "b", "mirror.txt"), "the run withholds the file of svc/b")
		})

		t.Run("writes a change outside the patterns as withheld", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, compose, cmdRun, packages(t), pkgA)
			assert.Equal(t, stdout,
				"create svc/a/mirror.txt\nwithheld svc/b/mirror.txt\n"+
					"files: 1 create, 0 update, 0 unchanged, 0 stale, 0 drifted, 0 foreign, 1 withheld; "+
					"plans: 1 committed, 0 failed, 0 cancelled, 0 prepared, 0 skipped\n",
				"the output lists the withheld change")
		})

		t.Run("writes no line for a withheld change that changes nothing", func(t *testing.T) {
			t.Parallel()

			root := packages(t)
			ran(t, root)
			grown := "package svc/a\ntype A string\ntype Again string\n"
			files.Write(t, root, files.Tree{"svc/a/a.zz": files.Text(grown)})
			_, stdout, _ := invoke(t, compose, cmdRun, root, pkgA)
			assert.Equal(t, stdout,
				"update svc/a/mirror.txt\n"+
					"files: 0 create, 1 update, 0 unchanged, 0 stale, 0 drifted, 0 foreign, 0 withheld; "+
					"plans: 1 committed, 0 failed, 0 cancelled, 0 prepared, 0 skipped\n",
				"the output has no line for the unchanged mirror of svc/b")
		})

		t.Run("reads a pattern relative to the working directory", func(t *testing.T) {
			t.Parallel()

			root := packages(t)
			ran(t, filepath.Join(root, "svc"), "a")
			files.IsFile(t, filepath.Join(root, "svc", "a", "mirror.txt"), "the pattern a names svc/a")
			files.Absent(t, filepath.Join(root, "svc", "b", "mirror.txt"), "the run withholds svc/b")
		})

		t.Run("commits the package of the working directory for the pattern of a go:generate line", func(t *testing.T) {
			t.Parallel()

			root := packages(t)
			ran(t, filepath.Join(root, "svc", "a"), here)
			files.IsFile(t, filepath.Join(root, "svc", "a", "mirror.txt"), "the run commits the file of svc/a")
			files.Absent(t, filepath.Join(root, "svc", "b", "mirror.txt"), "the run withholds the file of svc/b")
		})

		t.Run("returns StatusUsage for a pattern outside the root", func(t *testing.T) {
			t.Parallel()

			outside := filepath.Join("..", "elsewhere")
			status, _, stderr := invoke(t, compose, cmdRun, stored(t, nil), outside)
			expect.Equal(t, status, cli.StatusUsage, "the pattern is a usage error")
			expect.HasPrefix(t, stderr,
				"error: cli: run: the pattern "+outside+" is outside the root of every workspace\n",
				"the error names the pattern")
		})

		t.Run("returns StatusUsage for a pattern that the run refuses", func(t *testing.T) {
			t.Parallel()

			status, _, stderr := invoke(t, compose, cmdRun, stored(t, nil), refusedPattern)
			expect.Equal(t, status, cli.StatusUsage, "the pattern is a usage error")
			expect.HasPrefix(t, stderr,
				`error: cli: run: workspace: the pattern "svc/a...b" names no directory of a workspace tree`+"\n",
				"the error is the error of the run")
		})

		t.Run("returns StatusUsage for a plan that the composition does not declare", func(t *testing.T) {
			t.Parallel()

			status, _, stderr := invoke(t, compose, cmdRun, stored(t, nil), planFlag, "ghost")
			expect.Equal(t, status, cli.StatusUsage, "the plan is a usage error")
			expect.Contains(t, stderr, `the plan "ghost"`, "the error names the plan")
		})

		t.Run("skips each plan that --plan leaves out", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, described, cmdRun, stored(t, nil), planFlag, planName, jsonFlag)
			assert.Equal(t, decoded[outcomeEvent](t, stdout, eventOutcome), []outcomeEvent{
				{Plan: planName, Status: workspace.PlanCommitted.String()},
				{Plan: bindingsPlan, Status: workspace.PlanSkipped.String()},
			}, "the run commits the plan of the mirror and skips the plan bindings")
		})

		t.Run("returns StatusUsage for a config error", func(t *testing.T) {
			t.Parallel()

			root := workspaceDir(t, files.Tree{confName: files.Text(faultyConfig)})
			status, _, stderr := invoke(t, compose, cmdRun, root)
			expect.Equal(t, status, cli.StatusUsage, "the config error is a usage error")
			expect.Contains(t, stderr, "field workerz not found", "the run writes the fault")
		})

		t.Run("returns StatusFailed for a generator that returns an error", func(t *testing.T) {
			t.Parallel()

			status, _, stderr := invoke(t, compose, cmdRun, stored(t, files.Tree{storePath: files.Text(brokenSource)}))
			expect.Equal(t, status, cli.StatusFailed, "the error fails the run")
			expect.Equal(t, stderr, brokenError, "the run writes the error of the generator")
		})

		t.Run("writes the error of a run below its Error findings", func(t *testing.T) {
			t.Parallel()

			root := stored(t, files.Tree{storePath: files.Text(brokenSource), badPath: files.Text(badSource)})
			_, _, stderr := invoke(t, compose, cmdRun, root)
			lines := strings.Split(strings.TrimSuffix(stderr, "\n"), "\n")
			assert.Length(t, lines, 2, "the run writes the finding and the error")
			expect.HasPrefix(t, lines[0], badPath+":1: error ", "the first line is the finding")
			expect.Equal(t, lines[1]+"\n", brokenError, "the second line is the error of the generator")
		})

		t.Run("returns StatusFailed when another process has the lock", func(t *testing.T) {
			t.Parallel()

			root := stored(t, nil)
			locked(t, root)
			status, _, stderr := invoke(t, compose, cmdRun, root)
			expect.Equal(t, status, cli.StatusFailed, "the lock fails the run")
			expect.Equal(t, stderr,
				".acme/lock: error "+workspace.StateLocked.String()+
					": the state directory is locked by process 1 on elsewhere since 1970-01-01T00:00:00Z (cli_test), "+
					"and the run writes nothing (load)\n",
				"the run writes the finding that names the holder")
		})

		t.Run("returns StatusFailed for a cancelled context", func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			var stdout, stderr bytes.Buffer
			getenv := func(string) string { return "" }
			stdio := cli.IO{Stdout: &stdout, Stderr: &stderr, Dir: stored(t, nil), Getenv: getenv}
			status := command(t, compose, cmdRun).Run(ctx, stdio, nil)
			expect.Equal(t, status, cli.StatusFailed, "the cancellation fails the run")
			expect.Contains(t, stdout.String(), "plans: 0 committed, 0 failed, 1 cancelled",
				"the summary counts the cancelled plan")
			expect.Equal(t, stderr.String(), "error: "+context.Canceled.Error()+"\n", "the run writes the cancellation")
		})

		t.Run("writes a refused write over a drifted file as drifted", func(t *testing.T) {
			t.Parallel()

			root := stored(t, nil)
			ran(t, root)
			drift(t, root)
			status, stdout, stderr := invoke(t, compose, cmdRun, root)
			expect.Equal(t, status, cli.StatusFailed, "the drifted file fails the plan")
			expect.HasPrefix(t, stdout, "drifted svc/mirror.txt\n", "the output lists the drifted file")
			expect.Contains(t, stderr, workspace.DriftedOutput.String(), "the run reports the drift")
		})

		t.Run("writes a refused write over a foreign file as foreign", func(t *testing.T) {
			t.Parallel()

			root := stored(t, files.Tree{mirrorPath: files.Text(handWritten)})
			status, stdout, stderr := invoke(t, compose, cmdRun, root)
			expect.Equal(t, status, cli.StatusFailed, "the foreign file fails the plan")
			expect.HasPrefix(t, stdout, "foreign svc/mirror.txt\n", "the output lists the foreign file")
			expect.Contains(t, stderr, workspace.ForeignFile.String(), "the run reports the foreign file")
		})

		t.Run("updates a drifted file under --overwrite-drift", func(t *testing.T) {
			t.Parallel()

			root := stored(t, nil)
			ran(t, root)
			drift(t, root)
			status, stdout, _ := invoke(t, compose, cmdRun, root, driftFlag)
			expect.Equal(t, status, cli.StatusOK, "the run writes over the drift")
			expect.HasPrefix(t, stdout, "update svc/mirror.txt\n", "the output lists the update")
		})

		t.Run("updates a foreign file under --adopt", func(t *testing.T) {
			t.Parallel()

			root := stored(t, files.Tree{mirrorPath: files.Text(handWritten)})
			status, stdout, _ := invoke(t, compose, cmdRun, root, adoptFlag)
			expect.Equal(t, status, cli.StatusOK, "the run adopts the file")
			expect.HasPrefix(t, stdout, "update svc/mirror.txt\n", "the output lists the update")
		})

		t.Run("writes the removal of a file that no plan produces as stale", func(t *testing.T) {
			t.Parallel()

			root := stored(t, nil)
			ran(t, root)
			assert.NoError(t, os.Remove(filepath.Join(root, "svc", "store.zz")), "the test removes the store")
			_, stdout, _ := invoke(t, compose, cmdRun, root)
			expect.HasPrefix(t, stdout, "stale svc/mirror.txt\n", "the output lists the removal")
			files.Absent(t, filepath.Join(root, "svc", "mirror.txt"), "the run removes the mirror")
		})

		t.Run("writes a removal that leaves a drifted file in place as drifted", func(t *testing.T) {
			t.Parallel()

			root := stored(t, nil)
			ran(t, root)
			drift(t, root)
			assert.NoError(t, os.Remove(filepath.Join(root, "svc", "store.zz")), "the test removes the store")
			status, stdout, _ := invoke(t, compose, cmdRun, root)
			expect.Equal(t, status, cli.StatusOK, "the kept file is a Warning")
			expect.HasPrefix(t, stdout, "drifted svc/mirror.txt\n", "the output lists the kept file")
		})

		t.Run("writes a removal of the sweep as stale", func(t *testing.T) {
			t.Parallel()

			root := stored(t, nil)
			ran(t, root)
			files.Write(t, root, files.Tree{confName: files.Text("version: 1\nplans: {mirrors: {enabled: false}}\n")})
			_, stdout, _ := invoke(t, compose, cmdRun, root, jsonFlag)
			got := decoded[fileEvent](t, stdout, eventFile)
			assert.Length(t, got, 1, "the sweep removes the mirror")
			expect.Equal(t, got[0].Path, mirrorPath, "the event has the path")
			expect.Equal(t, got[0].Action, actionStale, "the removal is stale")
			expect.Empty(t, got[0].Plan, "a removal of the sweep has no plan")
		})

		t.Run("reports a Warning as an Error under --strict", func(t *testing.T) {
			t.Parallel()

			root := stored(t, files.Tree{storePath: files.Text(noisySource)})
			status, _, stderr := invoke(t, compose, cmdRun, root, strictFlag)
			expect.Equal(t, status, cli.StatusFailed, "the Warning fails the run")
			expect.Contains(t, stderr, ": error "+noted.String()+": Noisy is noisy (mirror)",
				"the run reports the Warning as an Error")
		})

		t.Run("writes a file event for each change in JSON output", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, compose, cmdRun, stored(t, nil), jsonFlag)
			got := decoded[fileEvent](t, stdout, eventFile)
			assert.Length(t, got, 1, "the run creates one file")
			expect.Equal(t, got[0].Path, mirrorPath, "the event has the path")
			expect.Equal(t, got[0].Plan, planName, "the event has the plan")
			expect.Equal(t, got[0].Action, actionCreate, "the event has the action")
			expect.Equal(t, got[0].Found, output.FoundNothing.String(), "the path had no file")
			expect.HasPrefix(t, got[0].Hash, "sha256:", "the event has the digest of the file")
		})

		t.Run("writes an outcome event for each plan in JSON output", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, compose, cmdRun, stored(t, nil), jsonFlag)
			assert.Equal(t, decoded[outcomeEvent](t, stdout, eventOutcome),
				[]outcomeEvent{{Plan: planName, Status: workspace.PlanCommitted.String()}}, "the plan committed")
		})

		t.Run("writes the counts of the run into the summary event", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, compose, cmdRun, stored(t, nil), jsonFlag)
			assert.HasSuffix(t, stdout,
				`{"event":"summary","status":0,"files":{"create":1,"update":0,"unchanged":0,"stale":0,"drifted":0,`+
					`"foreign":0,"withheld":0},"plans":{"committed":1,"failed":0,"cancelled":0,"prepared":0,"skipped":0},`+
					`"errors":0,"warnings":0,"infos":0}`+"\n",
				"the summary counts the file and the plan")
		})

		t.Run("counts the findings that a diag directive removes in the summary event", func(t *testing.T) {
			t.Parallel()

			root := stored(t, files.Tree{storePath: files.Text(noisySource + suppressing)})
			_, stdout, _ := invoke(t, compose, cmdRun, root, jsonFlag)
			got := decoded[summaryEvent](t, stdout, eventSummary)
			assert.Length(t, got, 1, "the output ends with one summary")
			assert.Equal(t, got[0].Suppressed, map[string]int{noted.String(): 1},
				"the summary counts the removed Warning under its code")
		})

		t.Run("writes the name of the member into each event of a list", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, compose, cmdRun, listed(t), jsonFlag)
			got := decoded[fileEvent](t, stdout, eventFile)
			assert.Length(t, got, 2, "each member creates one file")
			expect.Equal(t, got[0].Workspace, memberSvc, "the first event names the first member")
			expect.Equal(t, got[0].Path, "mirror.txt", "the path is relative to the root of the member")
			expect.Equal(t, got[1].Workspace, memberTool, "the second event names the second member")
		})

		t.Run("puts the name of the member before each path in text output", func(t *testing.T) {
			t.Parallel()

			_, stdout, _ := invoke(t, compose, cmdRun, listed(t))
			assert.HasPrefix(t, stdout, "create svc/mirror.txt\ncreate tools/mirror.txt\n",
				"each path is relative to the directory of the list")
		})

		t.Run("runs only the members of a list that contain a pattern", func(t *testing.T) {
			t.Parallel()

			root := listed(t)
			_, stdout, _ := invoke(t, compose, cmdRun, root, jsonFlag, memberTool)
			got := decoded[fileEvent](t, stdout, eventFile)
			assert.Length(t, got, 1, "the run of tools alone creates one file")
			expect.Equal(t, got[0].Workspace, memberTool, "the event names the member of the pattern")
			files.Absent(t, filepath.Join(root, "svc", "mirror.txt"), "the other member does not run")
		})

		t.Run("returns StatusFailed for a locator that returns an error", func(t *testing.T) {
			t.Parallel()

			status, _, stderr := invoke(t, failing, cmdRun, stored(t, nil))
			expect.Equal(t, status, cli.StatusFailed, "the locator fails the run")
			expect.Equal(t, stderr, locateError, "the run writes the error of the locator")
		})
	})
}

// stored writes a workspace with the config file of the brand and the
// store into a new directory of the test, with the entries of extra over
// them, and returns the directory.
func stored(t *testing.T, extra files.Tree) string {
	t.Helper()

	tree := files.Tree{confName: files.Text(version), storePath: files.Text(source)}
	maps.Copy(tree, extra)
	return workspaceDir(t, tree)
}

// packages writes a workspace with the packages svc/a and svc/b into a new
// directory of the test, and returns the directory.
func packages(t *testing.T) string {
	t.Helper()

	return workspaceDir(t, files.Tree{
		confName:     files.Text(version),
		"svc/a/a.zz": files.Text("package svc/a\ntype A string\n"),
		"svc/b/b.zz": files.Text("package svc/b\ntype B string\n"),
	})
}

// listed writes a list of the two workspaces svc and tools, each with one
// struct, into a new directory of the test, and returns the directory.
func listed(t *testing.T) string {
	t.Helper()

	return workspaceDir(t, files.Tree{
		confName:         files.Text(listConfig),
		"svc/store.zz":   files.Text("package store\ntype Store string\n"),
		"tools/tools.zz": files.Text("package tools\ntype Tool string\n"),
	})
}

// ran runs the run command in dir with args, and fails the test unless the
// command returns StatusOK.
func ran(t *testing.T, dir string, args ...string) {
	t.Helper()

	status, _, stderr := invoke(t, compose, cmdRun, dir, args...)
	assert.Equal(t, status, cli.StatusOK, "the run succeeds: "+stderr)
}

// drift edits the body of the mirror of the store in root, and keeps its
// frame.
func drift(t *testing.T, root string) {
	t.Helper()

	mirrored := files.Read(t, filepath.Join(root, "svc", "mirror.txt"))
	assert.Contains(t, mirrored, mirroredBody, "the mirror has the body of the store")
	files.Write(t, root, files.Tree{mirrorPath: files.Text(strings.Replace(mirrored, mirroredBody, editedBody, 1))})
}

// locked takes the lock of the state directory of root for the rest of the
// test, as another process would.
func locked(t *testing.T, root string) {
	t.Helper()

	l, err := ledger.OpenDir(root, brand)
	assert.NoError(t, err, "the ledger of the state directory opens")
	holder := ledger.Holder{PID: 1, Host: "elsewhere", Caller: "cli_test", Since: time.Unix(0, 0)}
	release, err := l.Lock(t.Context(), holder)
	assert.NoError(t, err, "the test takes the lock")
	t.Cleanup(func() { expect.NoError(t, release(), "the test releases the lock") })
}

// locatingCompose returns the fixture composition over the frontend
// locator, in place of the scripted frontend. locator loads the scripted
// language, and locates its stores with locate.
func locatingCompose(locate func(getenv func(string) string) (map[string]fs.FS, error)) cli.Compose {
	return func() *workspace.Builder {
		inner := frontendtest.NewScriptedDependent()
		return composed(frontend.New("locator", frontendtest.ScriptedLang, inner.Syntax()).
			Version("1").
			Match("**/*.zz").
			Units(inner.Partition).
			Parse(inner.Parse).
			Resolve(inner.Resolve).
			Dependencies(inner.Dependencies).
			Stores(locate).
			Build())
	}
}
