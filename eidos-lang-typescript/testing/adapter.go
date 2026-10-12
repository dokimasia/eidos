// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/symbol"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The spellings of the TypeScript toolchain.
const (
	tscBinary  = "tsc"
	nodeBinary = "node"
	configFile = "tsconfig.json"
	// scratchPattern names the scratch directory of a laid-out project.
	scratchPattern = "eidos-ts-*"
	// buildDir is the directory into which a test run emits JavaScript.
	// No generated path uses it, and the project leaves it out of its own
	// sources.
	buildDir = "eidos-build"
	// testModule ends the emitted name of a test module.
	testModule = ".test.js"
	// errorMark opens the code of every error that tsc reports, and
	// codeEnd closes it.
	errorMark = "error TS"
	codeEnd   = ":"
	// syntaxClass and configClass open the four-digit codes of the parser
	// and the grammar check, TS1005 among them, and of the project
	// configuration, which leaves nothing parsed.
	syntaxClass = '1'
	configClass = '5'
	classDigits = 4
	// probeFile is the module that the satisfaction check writes and
	// removes.
	probeFile = "zz-eidos-probe.ts"
)

// unassignable are the codes of the checker for a type that does not
// satisfy a contract: not assignable, sharing no property, missing several
// properties, missing many, and missing one.
var unassignable = []string{"2322", "2559", "2739", "2740", "2741"}

// The summary lines of the tap reporter, which the tally reads.
const (
	tapPass    = "# pass "
	tapFail    = "# fail "
	tapSkipped = "# skipped "
)

// config is the project configuration of a fixture that states none. It
// checks strictly, refuses an unused local, an unused parameter and an
// undefined written into an optional property, and emits CommonJS into
// [buildDir], so node runs the emitted tests. It names no ambient type
// package, because a scratch project installs none.
const config = `{
  "compilerOptions": {
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "exactOptionalPropertyTypes": true,
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

// adapter drives tsc and node over generated TypeScript. It has no state,
// so one value serves every check.
type adapter struct{}

// New returns the TypeScript toolchain adapter.
func New() toolchain.Adapter { return adapter{} }

// Lang returns [typescript.Lang], with which the messages of the
// assertions of the kernel start.
func (adapter) Lang() symbol.Lang { return typescript.Lang }

// Available reports whether tsc and node are both on PATH, and names the
// first binary that it did not find.
func (adapter) Available() (bool, string) { return toolchain.LookPaths(tscBinary, nodeBinary) }

// Layout writes the generated output as a project in a scratch directory.
// Every file is at its own path, and a tsconfig.json is at the root unless
// the output has one there. A write that fails, such as the write of a
// path that leaves the directory, returns an error and removes the
// directory.
func (adapter) Layout(g toolchain.Generated) (string, error) {
	return toolchain.Scratch(scratchPattern, g, map[string][]byte{configFile: []byte(config)})
}

// Parse reads the syntax of the project through tsc and returns the
// diagnostics of the parser and the grammar check alone, so a type error
// passes and a syntax error names its file and position. It returns an
// error for a configuration that tsc cannot read, and for a project
// without a TypeScript file, because a parse of nothing proves nothing.
// TypeScript has no parser in Go, so the parse needs the toolchain, and a
// caller passes [toolchain.Require] first.
//
// Parse reads the diagnostics of tsc and not its exit status, which a type
// error sets. Parse returns the error of a tsc that does not run to its
// exit, such as a tsc that is not on PATH. A context that ends stops tsc,
// and Parse returns an error that wraps the error of the context.
func (adapter) Parse(ctx context.Context, dir string) error {
	sources, err := toolchain.Sources(dir, []string{typescript.Extension}, buildDir)
	if err != nil {
		return err
	}
	if len(sources) == 0 {
		return errors.New("testing: the laid-out project has no TypeScript file to parse")
	}
	out, err := toolchain.Run(ctx, dir, nil, tscBinary, "-p", dir, "--noEmit", "--pretty", "false")
	if _, exited := errors.AsType[*exec.ExitError](err); err != nil && !exited {
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

// TypeCheck checks the project against the strict type rules of TypeScript
// with tsc and emits nothing. The error contains the output of tsc.
func (adapter) TypeCheck(ctx context.Context, dir string) error {
	_, err := toolchain.Run(ctx, dir, nil, tscBinary, "-p", dir, "--noEmit", "--pretty", "false")
	return err
}

// RunTests emits the project into [buildDir] and runs every emitted test
// module, a source named *.test.ts, with the test runner of node. The
// counts come from the tap summary of the runner. A type error does not
// stop the run, because tsc emits past one and [TypeCheck] reports it. A
// project that emits no test module returns an empty report, which fails
// the assertion of the kernel. A context that ends stops tsc or node, and
// RunTests returns an error that wraps the error of the context.
func (adapter) RunTests(ctx context.Context, dir string) (toolchain.TestReport, error) {
	emitted, err := toolchain.Run(ctx, dir, nil, tscBinary, "-p", dir, "--pretty", "false")
	if err != nil && ctx.Err() != nil {
		return toolchain.TestReport{Output: emitted}, err
	}
	tests, err := toolchain.Sources(filepath.Join(dir, buildDir), []string{testModule})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return toolchain.TestReport{Output: emitted}, err
	}
	if len(tests) == 0 {
		return toolchain.TestReport{Output: emitted}, nil
	}
	out, err := toolchain.Run(ctx, dir, nil, nodeBinary, append([]string{"--test", "--test-reporter=tap"}, tests...)...)
	report := tally(out)
	if err != nil && report.Failed == 0 {
		// node exited with an error and reported no failed test.
		return report, err
	}
	return report, nil
}

// Satisfies asks tsc whether one type is assignable to one contract, which
// is satisfaction under the structural typing of TypeScript. A probe
// module assigns a value of the type to a binding of the contract, and tsc
// checks the project again. Both are type expressions that resolve at the
// root of the project, so a type that another module declares is spelled
// import("./row").Row.
//
// A probe that tsc refuses with assignability codes alone reports false.
// Satisfies returns an error for any other refusal, with the source of the
// probe, and for a project that does not type-check without the probe.
func (a adapter) Satisfies(ctx context.Context, dir, typeName, contract string) (bool, error) {
	if err := a.TypeCheck(ctx, dir); err != nil {
		return false, fmt.Errorf("the project does not type-check, so nothing can be asked of it: %w", err)
	}
	source := "export const eidosProbe: " + contract + " = undefined as unknown as " + typeName + ";\n"
	var (
		out      string
		checkErr error
	)
	err := toolchain.Probe(dir, probeFile, source, func() error {
		out, checkErr = toolchain.Run(ctx, dir, nil, tscBinary, "-p", dir, "--noEmit", "--pretty", "false")
		return nil
	})
	if err != nil {
		return false, err
	}
	if checkErr == nil {
		return true, nil
	}
	found := diagnostics(out)
	for _, d := range found {
		if !slices.Contains(unassignable, d.code) {
			return false, fmt.Errorf(
				"the probe does not type-check for a reason other than the contract: %w\nthe probe is:\n%s",
				checkErr, source)
		}
	}
	if len(found) == 0 {
		return false, fmt.Errorf("the probe does not type-check and tsc names no error: %w\nthe probe is:\n%s",
			checkErr, source)
	}
	return false, nil
}

// diagnostic is one error line that tsc printed, and the code of the error.
type diagnostic struct {
	code string
	line string
}

// diagnostics returns the error lines of the output of tsc with their
// codes, in the order in which tsc printed them. It leaves out a
// continuation line, which explains the error above it and has no code.
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

// tally reads the case counts from the summary lines of the tap reporter.
// node reports each test module and each test inside one as a case, so a
// module without a test call counts once.
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

// count reads the number after the label of a summary line, and returns
// zero for a line whose number does not parse.
func count(line, label string) int {
	n, err := strconv.Atoi(strings.TrimPrefix(line, label))
	if err != nil {
		return 0
	}
	return n
}
