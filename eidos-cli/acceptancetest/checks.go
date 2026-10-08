// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package acceptancetest

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/cli"
	"go.dokimi.dev/eidos/core/ledger"
)

// The kernel commands that the checks run by name.
const (
	runName     = "run"
	planName    = "plan"
	explainName = "explain"
	pruneName   = "prune"
	doctorName  = "doctor"
	versionName = "version"
)

// The flags that the checks pass to the kernel commands.
const (
	jsonFlag    = "--format=json"
	coldFlag    = "--cold"
	configFlag  = "--config"
	helpFlag    = "-h"
	unknownFlag = "--acceptancetest-unknown"
)

// The events of JSON output and the action of a file event that the checks
// read.
const (
	eventStart      = "start"
	eventSummary    = "summary"
	eventFile       = "file"
	actionUnchanged = "unchanged"
)

// Go exits a process with status 2 after a panic. It writes the panic, and
// then the stack trace of each goroutine, to standard error.
const (
	panicStatus = 2
	panicMark   = "panic: "
	stackMark   = "goroutine "
)

// configExt is the extension of the config file of a brand. The name of the
// file is the name of the state directory of the brand and the extension,
// such as .acme.yaml.
const configExt = ".yaml"

// gitDir is the version control marker that AssertDiscovery writes.
const gitDir = ".git"

// The directories that the checks create in their root.
const (
	cleanDir     = "clean"
	malformedDir = "malformed"
	failedDir    = "failed"
	belowDir     = "below"
	deeperDir    = "deeper"
	markedDir    = "marked"
	repoDir      = "repo"
	innerDir     = "inner"
	givenDir     = "given"
	listedDir    = "listed"
	firstMember  = "first"
	secondMember = "second"
	nestedDir    = "nested"
	outerDir     = "outer"
)

// The checks write these config files. malformedConfig has a key that the
// format does not have, and plainConfig has the version alone. givenName is
// the file name that a check passes to --config. listConfig lists two
// members, and the roots of nestedConfig nest.
const (
	malformedConfig = "version: 1\nacceptancetest: a key that the format does not have\n"
	plainConfig     = "version: 1\n"
	givenName       = "given.yaml"
	listConfig      = "version: 1\nworkspaces: [{root: " + firstMember + "}, {root: " + secondMember + "}]\n"
	nestedConfig    = "version: 1\nworkspaces: [{root: " + outerDir + "}, {root: " + outerDir + "/" + innerDir + "}]\n"
)

// The modes of the directories and of the files that the checks write.
const (
	dirMode  fs.FileMode = 0o755
	fileMode fs.FileMode = 0o644
)

// lockCaller is the caller in the holder record of the lock that
// AssertLocked takes.
const lockCaller = "acceptancetest"

// past is the modification time that AssertIdempotent gives each file
// before the second run.
var past = time.Unix(0, 0)

// event is an event of JSON output, with the fields that the checks read.
type event struct {
	Event  string `json:"event"`
	Path   string `json:"path"`
	Action string `json:"action"`
}

// AssertStatuses checks the exit statuses of run over copies of the tree of
// the fixture in root. run exits 0 over the tree, 64 for a flag that no
// command defines, 64 for a config file with a key that the format does not
// have, and 1 after the Fail of the fixture.
func AssertStatuses(tb assert.TB, f Fixture, bin, root string) {
	tb.Helper()

	assert.NotNil(tb, f.Fail, "the fixture has a Fail")
	_, config := stateNames(tb, f)
	run := slices.Concat(f.Prefix, []string{runName})

	clean := filepath.Join(root, cleanDir)
	copied(tb, f, clean)
	got := Exec(tb, bin, clean, nil, run...)
	assert.Equal(tb, got.Status, cli.StatusOK, "run exits 0 over the tree of the fixture: "+string(got.Stderr))
	got = Exec(tb, bin, clean, nil, slices.Concat(run, []string{unknownFlag})...)
	assert.Equal(tb, got.Status, cli.StatusUsage,
		"run exits 64 for a flag that no command defines: "+string(got.Stderr))

	malformed := filepath.Join(root, malformedDir)
	copied(tb, f, malformed)
	assert.NoError(tb, os.WriteFile(filepath.Join(malformed, config), []byte(malformedConfig), fileMode),
		"the check writes a config file with a key that the format does not have")
	got = Exec(tb, bin, malformed, nil, run...)
	assert.Equal(tb, got.Status, cli.StatusUsage,
		"run exits 64 for a config file with a key that the format does not have: "+string(got.Stderr))

	failed := filepath.Join(root, failedDir)
	copied(tb, f, failed)
	assert.NoError(tb, f.Fail(failed), "the Fail of the fixture edits the tree")
	got = Exec(tb, bin, failed, nil, run...)
	assert.Equal(tb, got.Status, cli.StatusFailed, "run exits 1 after the Fail of the fixture: "+string(got.Stderr))
}

// AssertNames checks that each kernel command runs under its own name after
// the prefix of the fixture. For -h after the name, the command exits 0 and
// writes its usage to standard output.
func AssertNames(tb assert.TB, f Fixture, bin, root string) {
	tb.Helper()

	for _, k := range cli.Kernels(nil) {
		got := Exec(tb, bin, root, nil, slices.Concat(f.Prefix, []string{k.Name(), helpFlag})...)
		assert.Equal(tb, got.Status, cli.StatusOK, k.Name()+" -h exits 0: "+string(got.Stderr))
		assert.Equal(tb, string(got.Stdout), k.Usage(), k.Name()+" -h writes the usage of "+k.Name())
	}
}

// AssertPanic checks that the command of the fixture that panics exits 2,
// and writes the panic and the stack trace of Go to standard error.
func AssertPanic(tb assert.TB, f Fixture, bin, root string) {
	tb.Helper()

	assert.NotEmpty(tb, f.Panic, "the fixture has the arguments of a command that panics")
	got := Exec(tb, bin, root, nil, f.Panic...)
	assert.Equal(tb, got.Status, panicStatus, "the command that panics exits 2: "+string(got.Stderr))
	assert.Contains(tb, string(got.Stderr), panicMark, "the command writes the panic to standard error")
	assert.Contains(tb, string(got.Stderr), stackMark, "the command writes the stack trace to standard error")
}

// AssertDiscovery checks the search for the config file in copies of the
// tree of the fixture in root. It checks these rules:
//
//   - A run from a directory below the root finds the config file of the
//     root, and writes the state directory into the root.
//   - A version control marker between the working directory and a config
//     file stops the search.
//   - --config takes precedence over the config file of the working
//     directory.
func AssertDiscovery(tb assert.TB, f Fixture, bin, root string) {
	tb.Helper()

	state, config := stateNames(tb, f)
	run := slices.Concat(f.Prefix, []string{runName})
	plan := slices.Concat(f.Prefix, []string{planName})

	below := filepath.Join(root, belowDir)
	copied(tb, f, below)
	deeper := filepath.Join(below, deeperDir)
	assert.NoError(tb, os.MkdirAll(deeper, dirMode), "the check creates a directory below the root")
	got := Exec(tb, bin, deeper, nil, run...)
	assert.Equal(tb, got.Status, cli.StatusOK, "a run from a directory below the root exits 0: "+string(got.Stderr))
	files.IsDir(tb, filepath.Join(below, state), "the run writes the state directory into the root")
	files.Absent(tb, filepath.Join(deeper, state), "the run writes no state directory into the working directory")

	marked := filepath.Join(root, markedDir)
	inner := filepath.Join(marked, repoDir, innerDir)
	assert.NoError(tb, os.MkdirAll(inner, dirMode), "the check creates a directory inside a repository")
	assert.NoError(tb, os.Mkdir(filepath.Join(marked, repoDir, gitDir), dirMode),
		"the check marks the root of the repository")
	assert.NoError(tb, os.WriteFile(filepath.Join(marked, config), []byte(malformedConfig), fileMode),
		"the check writes a config file with a key that the format does not have above the repository")
	got = Exec(tb, bin, marked, nil, plan...)
	assert.Equal(tb, got.Status, cli.StatusUsage,
		"plan exits 64 beside the config file with a key that the format does not have: "+string(got.Stderr))
	got = Exec(tb, bin, inner, nil, plan...)
	assert.Equal(tb, got.Status, cli.StatusOK,
		"plan exits 0 inside the repository, because the search stops at its marker: "+string(got.Stderr))

	given := filepath.Join(root, givenDir)
	assert.NoError(tb, os.MkdirAll(given, dirMode), "the check creates the working directory of --config")
	assert.NoError(tb, os.WriteFile(filepath.Join(given, config), []byte(malformedConfig), fileMode),
		"the check writes a config file with a key that the format does not have")
	assert.NoError(tb, os.WriteFile(filepath.Join(given, givenName), []byte(plainConfig), fileMode),
		"the check writes the config file of --config")
	got = Exec(tb, bin, given, nil, slices.Concat(plan, []string{configFlag, givenName})...)
	assert.Equal(tb, got.Status, cli.StatusOK,
		"plan exits 0 for --config beside a config file that does not decode: "+string(got.Stderr))
}

// AssertJSON checks the JSON output of each kernel command except watch
// over a copy of the tree of the fixture in root. watch runs until an
// interrupt, and Exec waits for the process to exit. explain explains the
// first file that run reports. Each line of standard output is a JSON
// object with an event, the first event is start, the last event is
// summary, and standard error is empty.
func AssertJSON(tb assert.TB, f Fixture, bin, root string) {
	tb.Helper()

	copied(tb, f, root)
	// events runs the command name with args under --format=json, checks
	// its output, and returns its events.
	events := func(name string, args ...string) []event {
		got := Exec(tb, bin, root, nil, slices.Concat(f.Prefix, []string{name, jsonFlag}, args)...)
		assert.Empty(tb, string(got.Stderr), name+" writes nothing to standard error")
		var out []event
		for line := range strings.Lines(string(got.Stdout)) {
			var e event
			assert.NoError(tb, json.Unmarshal([]byte(line), &e), "each line of "+name+" is a JSON object: "+line)
			assert.NotEqual(tb, e.Event, "", "each line of "+name+" has an event: "+line)
			out = append(out, e)
		}
		assert.NotEmpty(tb, out, name+" writes events")
		assert.Equal(tb, out[0].Event, eventStart, "the first event of "+name+" is start")
		assert.Equal(tb, out[len(out)-1].Event, eventSummary, "the last event of "+name+" is summary")
		return out
	}
	ran := events(runName)
	i := slices.IndexFunc(ran, func(e event) bool { return e.Event == eventFile })
	assert.NotEqual(tb, i, -1, "run reports a file for explain to explain")
	events(explainName, ran[i].Path)
	for _, name := range []string{planName, pruneName, doctorName, versionName} {
		events(name)
	}
}

// AssertIdempotent checks that a second run over a copy of the tree of the
// fixture in root rewrites no file. The second run is cold, so it renders
// every file again. It reports each file unchanged, and moves no
// modification time of a file outside the state directory.
func AssertIdempotent(tb assert.TB, f Fixture, bin, root string) {
	tb.Helper()

	state, _ := stateNames(tb, f)
	copied(tb, f, root)
	run := slices.Concat(f.Prefix, []string{runName})
	got := Exec(tb, bin, root, nil, run...)
	assert.Equal(tb, got.Status, cli.StatusOK, "the first run exits 0: "+string(got.Stderr))
	paths := outside(tb, root, state)
	for _, p := range paths {
		assert.NoError(tb, os.Chtimes(p, past, past), "the check sets the modification time of "+p)
	}
	got = Exec(tb, bin, root, nil, slices.Concat(run, []string{coldFlag, jsonFlag})...)
	assert.Equal(tb, got.Status, cli.StatusOK, "the second run exits 0: "+string(got.Stdout))
	rendered := 0
	for line := range strings.Lines(string(got.Stdout)) {
		var e event
		assert.NoError(tb, json.Unmarshal([]byte(line), &e), "each line of the second run is a JSON object: "+line)
		if e.Event == eventFile {
			rendered++
			assert.Equal(tb, e.Action, actionUnchanged, "the second run leaves "+e.Path+" unchanged")
		}
	}
	assert.NotEqual(tb, rendered, 0, "the second run renders the files of the first run again")
	for _, p := range paths {
		info, err := os.Stat(p)
		assert.NoError(tb, err, "the check reads the modification time of "+p)
		assert.True(tb, info.ModTime().Equal(past), "the second run moves no modification time of "+p)
	}
}

// AssertCompiles checks that the Compile of the fixture compiles the output
// of a run over a copy of the tree of the fixture in root.
func AssertCompiles(tb assert.TB, f Fixture, bin, root string) {
	tb.Helper()

	assert.NotNil(tb, f.Compile, "the fixture has a Compile")
	copied(tb, f, root)
	got := Exec(tb, bin, root, nil, slices.Concat(f.Prefix, []string{runName})...)
	assert.Equal(tb, got.Status, cli.StatusOK, "the run exits 0: "+string(got.Stderr))
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	assert.NoError(tb, f.Compile(ctx, root), "the generated output compiles")
}

// AssertLists checks lists of workspaces over copies of the tree of the
// fixture in root. A run over a list of two copies of the tree runs each
// member under a state directory of its own. A run over a list whose roots
// nest exits 64 before it creates a state directory.
func AssertLists(tb assert.TB, f Fixture, bin, root string) {
	tb.Helper()

	state, config := stateNames(tb, f)
	run := slices.Concat(f.Prefix, []string{runName})

	listed := filepath.Join(root, listedDir)
	copied(tb, f, filepath.Join(listed, firstMember))
	copied(tb, f, filepath.Join(listed, secondMember))
	assert.NoError(tb, os.WriteFile(filepath.Join(listed, config), []byte(listConfig), fileMode),
		"the check writes a list of two members")
	got := Exec(tb, bin, listed, nil, run...)
	assert.Equal(tb, got.Status, cli.StatusOK, "a run over the list exits 0: "+string(got.Stderr))
	files.IsDir(tb, filepath.Join(listed, firstMember, state),
		"the run writes the state directory of the first member into its root")
	files.IsDir(tb, filepath.Join(listed, secondMember, state),
		"the run writes the state directory of the second member into its root")
	files.Absent(tb, filepath.Join(listed, state), "the run writes no state directory into the directory of the list")

	nested := filepath.Join(root, nestedDir)
	assert.NoError(tb, os.MkdirAll(filepath.Join(nested, outerDir, innerDir), dirMode),
		"the check creates the roots of the list")
	assert.NoError(tb, os.WriteFile(filepath.Join(nested, config), []byte(nestedConfig), fileMode),
		"the check writes a list whose roots nest")
	got = Exec(tb, bin, nested, nil, run...)
	assert.Equal(tb, got.Status, cli.StatusUsage, "a run over a list whose roots nest exits 64: "+string(got.Stderr))
	for _, dir := range []string{nested, filepath.Join(nested, outerDir), filepath.Join(nested, outerDir, innerDir)} {
		files.Absent(tb, filepath.Join(dir, state), "the run creates no state directory in "+dir)
	}
}

// AssertLocked checks that a run over a copy of the tree of the fixture in
// root exits 1 while the check has the lock of the state directory, and
// that the output of the run has the process ID of the check.
func AssertLocked(tb assert.TB, f Fixture, bin, root string) {
	tb.Helper()

	copied(tb, f, root)
	l, err := ledger.OpenDir(root, f.Brand)
	assert.NoError(tb, err, "the state directory of the copy of the tree opens")
	host, _ := os.Hostname()
	release, err := l.Lock(context.Background(), ledger.Holder{
		PID: os.Getpid(), Host: host, Caller: lockCaller, Since: time.Now(),
	})
	assert.NoError(tb, err, "the check takes the lock of the state directory")
	got := Exec(tb, bin, root, nil, slices.Concat(f.Prefix, []string{runName})...)
	assert.NoError(tb, release(), "the check releases the lock of the state directory")
	assert.Equal(tb, got.Status, cli.StatusFailed,
		"a run exits 1 while another process has the lock: "+string(got.Stderr))
	assert.Contains(tb, string(got.Stdout)+string(got.Stderr), strconv.Itoa(os.Getpid()),
		"the output of the run has the process ID of the holder of the lock")
}

// copied copies the tree of the fixture into dir, which it creates. It
// stops the check when the fixture has no tree, and when the tree does not
// copy.
func copied(tb assert.TB, f Fixture, dir string) {
	tb.Helper()

	assert.NotNil(tb, f.Tree, "the fixture has a tree")
	assert.NoError(tb, os.CopyFS(dir, f.Tree), "the tree of the fixture copies into "+dir)
}

// stateNames returns the name of the state directory of the brand of the
// fixture, and the name of the config file of the brand. It stops the check
// when the brand of the fixture is not valid.
func stateNames(tb assert.TB, f Fixture) (string, string) {
	tb.Helper()

	assert.True(tb, f.Brand.Valid(), "the fixture has a valid brand")
	state := ledger.StateDir(f.Brand)
	return state, state + configExt
}

// outside returns the path of each regular file under root, except the
// files of the state directory state in root. It stops the check when root
// does not walk.
func outside(tb assert.TB, root, state string) []string {
	tb.Helper()

	skipped := filepath.Join(root, state)
	var paths []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if p == skipped {
			return filepath.SkipDir
		}
		if err == nil && d.Type().IsRegular() {
			paths = append(paths, p)
		}
		return err
	})
	assert.NoError(tb, err, "the files under "+root+" walk")
	return paths
}
