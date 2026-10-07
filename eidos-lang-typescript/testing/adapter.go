// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	typescript "go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/symbol"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The TypeScript toolchain's spellings.
const (
	tscBinary  = "tsc"
	nodeBinary = "node"
	configFile = "tsconfig.json"
	// buildDir is where a test run emits JavaScript: a directory no
	// generated path uses, left out of the project's own sources.
	buildDir = "eidos-build"
	// testModule ends the emitted name of a test module.
	testModule = ".test.js"
	// dirPerm and filePerm keep the scratch project private to the
	// user running the check.
	dirPerm  = 0o700
	filePerm = 0o600
	// errorMark opens the code of every error tsc reports, and
	// codeEnd closes it.
	errorMark = "error TS"
	codeEnd   = ":"
	// syntaxClass and configClass open a four-digit code of the parser
	// or the grammar check, TS1005 among them, and of the project
	// configuration, which leaves nothing parsed.
	syntaxClass = '1'
	configClass = '5'
	classDigits = 4
	// probeFile is the module the satisfaction check writes and
	// removes.
	probeFile = "zz-eidos-probe.ts"
)

// unassignable are the checker's codes for a type that does not
// satisfy a contract: not assignable, sharing no property, missing
// several properties, missing many, and missing one.
var unassignable = []string{"2322", "2559", "2739", "2740", "2741"}

// The tap reporter's summary lines, which the tally reads.
const (
	tapPass    = "# pass "
	tapFail    = "# fail "
	tapSkipped = "# skipped "
)

// config is the project configuration a fixture stating none is laid
// out with: strict checking, CommonJS output into [buildDir] so node
// runs the emitted tests, and no ambient type package, because a
// scratch project installs none.
const config = `{
  "compilerOptions": {
    "strict": true,
    "target": "es2022",
    "module": "commonjs",
    "outDir": "` + buildDir + `",
    "rootDir": ".",
    "types": [],
    "skipLibCheck": true
  },
  "include": ["**/*` + typescript.Extension + `"],
  "exclude": ["` + buildDir + `"]
}
`

// runTimeout bounds one toolchain invocation, so a compiler that
// never returns fails the check with an error and leaves the suite
// running.
const runTimeout = 5 * time.Minute

// adapter drives tsc and node over generated TypeScript. It has no
// state, so one value serves every check.
type adapter struct{}

// New returns the TypeScript toolchain adapter.
func New() toolchain.Adapter { return adapter{} }

// Lang returns [typescript.Lang], which the kernel's assertions name
// in their failure messages.
func (adapter) Lang() symbol.Lang { return typescript.Lang }

// Available reports whether tsc and node are both on PATH, and names
// the first binary it did not find.
func (adapter) Available() (bool, string) {
	for _, binary := range []string{tscBinary, nodeBinary} {
		if _, err := exec.LookPath(binary); err != nil {
			return false, fmt.Sprintf("%s is not on PATH: %v", binary, err)
		}
	}
	return true, ""
}

// Layout writes the generated output as a project in a scratch
// directory: every file at its own path, and a tsconfig.json at the
// root unless the output has one there already. A write that fails,
// a path escaping the directory among them, returns an error and
// removes the directory.
func (adapter) Layout(g toolchain.Generated) (string, error) {
	dir, err := os.MkdirTemp("", "eidos-ts-*")
	if err != nil {
		return "", err
	}
	for path, body := range g.Files {
		if err := write(dir, path, body); err != nil {
			return "", errors.Join(err, os.RemoveAll(dir))
		}
	}
	if _, stated := g.Files[configFile]; !stated {
		if err := write(dir, configFile, []byte(config)); err != nil {
			return "", errors.Join(err, os.RemoveAll(dir))
		}
	}
	return dir, nil
}

// Parse reads the project's syntax through tsc and returns the
// diagnostics of the parser and the grammar check alone, so a type
// error passes and a syntax error names its file and position. It
// returns an error for a configuration tsc cannot read, and for a
// project with no TypeScript file, because a parse of nothing proves
// nothing. TypeScript has no parser in Go, so the parse needs the
// toolchain, and a caller passes [toolchain.Require] first. Parse reads
// tsc's diagnostics and not its exit status, which a type error sets.
// A context that ends stops tsc, and Parse returns an error that wraps
// the context's error.
func (adapter) Parse(ctx context.Context, dir string) error {
	if err := hasSource(dir); err != nil {
		return err
	}
	out, err := run(ctx, dir, tscBinary, "-p", dir, "--noEmit", "--pretty", "false")
	if err != nil && ctx.Err() != nil {
		return err
	}
	var refused []string
	for _, d := range diagnostics(out) {
		if len(d.code) == classDigits && (d.code[0] == syntaxClass || d.code[0] == configClass) {
			refused = append(refused, d.line)
		}
	}
	if len(refused) > 0 {
		return fmt.Errorf("testing: tsc refuses the syntax:\n%s", strings.Join(refused, "\n"))
	}
	return nil
}

// TypeCheck checks the project against TypeScript's strict type rules
// with tsc and emits nothing. The error contains tsc's output.
func (adapter) TypeCheck(ctx context.Context, dir string) error {
	_, err := run(ctx, dir, tscBinary, "-p", dir, "--noEmit", "--pretty", "false")
	return err
}

// RunTests emits the project into [buildDir] and runs every emitted
// test module, a source named *.test.ts, with node's test runner. The
// counts read off the runner's tap summary. A type error does not stop
// the run, because tsc emits past one and [TypeCheck] reports it. A
// project that emits no test module returns an empty report, which
// fails the kernel's assertion. A context that ends stops tsc or node,
// and RunTests returns an error that wraps the context's error.
func (adapter) RunTests(ctx context.Context, dir string) (toolchain.TestReport, error) {
	emitted, err := run(ctx, dir, tscBinary, "-p", dir, "--pretty", "false")
	if err != nil && ctx.Err() != nil {
		return toolchain.TestReport{Output: emitted}, err
	}
	var tests []string
	err = filepath.WalkDir(filepath.Join(dir, buildDir), func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, testModule) {
			tests = append(tests, path)
		}
		return err
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return toolchain.TestReport{Output: emitted}, err
	}
	if len(tests) == 0 {
		return toolchain.TestReport{Output: emitted}, nil
	}
	out, err := run(ctx, dir, nodeBinary, append([]string{"--test", "--test-reporter=tap"}, tests...)...)
	report := tally(out)
	if err != nil && report.Failed == 0 {
		// node exited with an error and reported no failed test.
		return report, err
	}
	return report, nil
}

// Satisfies asks tsc whether one type is assignable to one contract,
// which is satisfaction under TypeScript's structural typing: a probe
// module assigns a value of the type to a binding of the contract and
// the project is checked again. Both are type expressions resolved at
// the project's root, so a type another module declares is spelled
// import("./row").Row. A probe that tsc refuses with assignability
// codes alone reports false. Any other refusal returns an error, and
// so does a project that does not type-check without the probe.
func (a adapter) Satisfies(ctx context.Context, dir, typeName, contract string) (bool, error) {
	if err := a.TypeCheck(ctx, dir); err != nil {
		return false, fmt.Errorf("the project does not type-check, so nothing can be asked of it: %w", err)
	}
	probe := filepath.Join(dir, probeFile)
	source := "export const eidosProbe: " + contract + " = undefined as unknown as " + typeName + ";\n"
	if err := os.WriteFile(probe, []byte(source), filePerm); err != nil {
		return false, err
	}
	defer func() { _ = os.Remove(probe) }()

	out, checkErr := run(ctx, dir, tscBinary, "-p", dir, "--noEmit", "--pretty", "false")
	if checkErr == nil {
		return true, nil
	}
	found := diagnostics(out)
	for _, d := range found {
		if !slices.Contains(unassignable, d.code) {
			return false, fmt.Errorf("the probe does not type-check for a reason other than the contract: %w", checkErr)
		}
	}
	if len(found) == 0 {
		return false, fmt.Errorf("the probe does not type-check and tsc names no error: %w", checkErr)
	}
	return false, nil
}

// diagnostic is one error line tsc printed, and the code it names.
type diagnostic struct {
	code string
	line string
}

// diagnostics returns the error lines of tsc's output with their
// codes, in the order tsc printed them. A continuation line, which
// explains the error above it, names no code and is left out.
func diagnostics(out string) []diagnostic {
	var found []diagnostic
	for line := range strings.Lines(out) {
		_, rest, isError := strings.Cut(line, errorMark)
		if !isError {
			continue
		}
		code, _, _ := strings.Cut(rest, codeEnd)
		found = append(found, diagnostic{code: code, line: strings.TrimSpace(line)})
	}
	return found
}

// hasSource returns an error for a project with no TypeScript file
// outside [buildDir].
func hasSource(dir string) error {
	found := false
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || found {
			return err
		}
		if d.IsDir() && d.Name() == buildDir {
			return filepath.SkipDir
		}
		found = !d.IsDir() && filepath.Ext(path) == typescript.Extension
		return nil
	})
	if err != nil {
		return err
	}
	if !found {
		return errors.New("testing: the laid-out project has no TypeScript file to parse")
	}
	return nil
}

// write writes one fixture file under the scratch directory, and
// returns an error for a path that climbs out of it.
func write(dir, path string, body []byte) error {
	target := filepath.Join(dir, filepath.FromSlash(path))
	rel, err := filepath.Rel(dir, target)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("the generated path %q climbs out of the scratch project", path)
	}
	if err := os.MkdirAll(filepath.Dir(target), dirPerm); err != nil {
		return err
	}
	return os.WriteFile(target, body, filePerm)
}

// run runs one toolchain binary in dir under ctx and returns its
// combined output. A failure wraps that output, so the error contains
// what the tool reported. The run is bounded by [runTimeout] as well,
// and a run that ctx or the bound ends returns an error that wraps the
// context's error.
func run(ctx context.Context, dir, binary string, args ...string) (string, error) {
	bounded, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()

	cmd := exec.CommandContext(bounded, binary, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return string(out), fmt.Errorf(
				"testing: %s %s stopped with its caller: %w", binary, strings.Join(args, " "), ctx.Err(),
			)
		}
		if bounded.Err() != nil {
			return string(out), fmt.Errorf(
				"testing: %s %s did not return within %s: %w",
				binary, strings.Join(args, " "), runTimeout, bounded.Err(),
			)
		}
		return string(out), fmt.Errorf("%s %s: %w\n%s", binary, strings.Join(args, " "), err, out)
	}
	return string(out), nil
}

// tally reads the case counts off the tap reporter's summary lines.
// node reports each test module and each test inside one as a case,
// so a module without a test call counts once.
func tally(stream string) toolchain.TestReport {
	report := toolchain.TestReport{Output: stream}
	for line := range strings.Lines(stream) {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, tapPass):
			report.Passed = count(line, tapPass)
		case strings.HasPrefix(line, tapFail):
			report.Failed = count(line, tapFail)
		case strings.HasPrefix(line, tapSkipped):
			report.Skipped = count(line, tapSkipped)
		}
	}
	return report
}

// count reads the number after a summary line's label, and zero for a
// line whose number does not read.
func count(line, label string) int {
	n, err := strconv.Atoi(strings.TrimPrefix(line, label))
	if err != nil {
		return 0
	}
	return n
}
