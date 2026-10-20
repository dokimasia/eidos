// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The spellings of the Python toolchain.
const (
	pythonBinary  = "python3"
	pyrightBinary = "pyright"
	configFile    = "pyrightconfig.json"
	// scratchPattern names the scratch directory of a laid-out project.
	scratchPattern = "eidos-python-*"
	// isolated runs python3 in isolated mode, so neither the environment
	// nor the directory of the project changes the modules that it imports.
	isolated = "-I"
	// script runs the program that the next argument states.
	script = "-c"
	// testFiles is the pattern of the test modules, which is the default
	// pattern of the discovery of unittest.
	testFiles = "test_*.py"
	// summaryMark opens the line of the counts that testScript prints last.
	summaryMark = `{"run":`
	// probeFile is the module that the satisfaction check writes and
	// removes.
	probeFile = "zz_eidos_probe.py"
	// returnRule is the rule of pyright for a returned value whose type is
	// not assignable to the declared result.
	returnRule = "reportReturnType"
	// errorSeverity is the severity of a diagnostic of pyright that fails a
	// check.
	errorSeverity = "error"
	// qualifiedSep separates the module of a class name from the name of the
	// class within the module, as the names of objects of importlib do.
	qualifiedSep = ":"
	// moduleSep separates the names of a dotted name of Python.
	moduleSep = "."
)

// parseScript is the program that python3 runs to parse the files of a
// project. Its first argument is the minor version of the target, and the
// others are the files. It parses each file with the grammar of the target
// version, prints each syntax error with its file, its line and its column,
// and exits with 1 where a file has one. An interpreter older than the
// target version exits with a message, because it does not know the newer
// syntax.
const parseScript = `import ast
import sys

minor = int(sys.argv[1])
if sys.version_info < (3, minor):
    sys.exit(f"python3 {sys.version_info[0]}.{sys.version_info[1]} is older than the target version 3.{minor}")
failed = False
for path in sys.argv[2:]:
    with open(path, "rb") as source:
        text = source.read()
    try:
        ast.parse(text, path, feature_version=(3, minor))
    except SyntaxError as error:
        failed = True
        print(f"{path}:{error.lineno}:{error.offset}: {error.msg}")
sys.exit(1 if failed else 0)
`

// testScript is the program that python3 runs to run the tests of a
// project. Its argument is the pattern of the test modules. It runs the
// discovery of unittest over the directory, writes the report of the runner
// to the standard error, and prints the counts of the run as one line of
// JSON last.
const testScript = `import json
import sys
import unittest

suite = unittest.defaultTestLoader.discover(".", pattern=sys.argv[1])
result = unittest.TextTestRunner(stream=sys.stderr, verbosity=2).run(suite)
failed = len(result.failures) + len(result.errors) + len(result.unexpectedSuccesses)
skipped = len(result.skipped) + len(result.expectedFailures)
print(json.dumps({"run": result.testsRun, "failed": failed, "skipped": skipped}))
`

// The refusals of the adapter: a version that the policy python.version
// does not declare, a project without a Python file, a run of the tests
// that ends without its counts, and an output of pyright without a report.
var (
	errVersion   = errors.New("testing: the policy python.version has no such version")
	errNoPython  = errors.New("testing: the laid-out project has no Python file")
	errNoSummary = errors.New("testing: the run of the tests ended without its counts")
	errNoReport  = errors.New("testing: pyright wrote no report")
)

// adapter drives python3 and pyright over generated Python for one target
// version. Its version is all of its state, so one value serves every check
// of the version.
type adapter struct {
	version plugin.Choice
	minor   int
	err     error
}

// summary is the line of the counts that testScript prints.
type summary struct {
	Run     int `json:"run"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

// report is the part of the JSON report of pyright that the adapter reads.
type report struct {
	GeneralDiagnostics []diagnostic `json:"generalDiagnostics"`
}

// diagnostic is one diagnostic of a JSON report of pyright. Its lines and
// its characters count from zero.
type diagnostic struct {
	File     string `json:"file"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Rule     string `json:"rule"`
	Range    struct {
		Start struct {
			Line      int `json:"line"`
			Character int `json:"character"`
		} `json:"start"`
	} `json:"range"`
}

// line writes the diagnostic as file:line:column: message (rule), with the
// path of the file relative to dir where it is below dir, and the line and
// the column counted from one.
func (d diagnostic) line(dir string) string {
	file := d.File
	if rel, err := filepath.Rel(dir, d.File); err == nil && filepath.IsLocal(rel) {
		file = rel
	}
	text := fmt.Sprintf("%s:%d:%d: %s", file, d.Range.Start.Line+1, d.Range.Start.Character+1, d.Message)
	if d.Rule != "" {
		text += " (" + d.Rule + ")"
	}
	return text
}

// New returns the Python toolchain adapter for the target version, a
// choice of [python.VersionPolicy], such as [python.Version312]. The empty
// choice is the default of the policy, 3.11. The adapter of a choice that
// the policy does not declare returns an error that wraps errVersion from
// Layout and from every check.
func New(version plugin.Choice) toolchain.Adapter {
	version = cmp.Or(version, python.Version311)
	minor, valid := python.Minor(string(version))
	if !valid || !slices.Contains(python.Policies()[0].Choices, version) {
		return adapter{version: version, err: fmt.Errorf("%w: %q", errVersion, version)}
	}
	return adapter{version: version, minor: minor}
}

// Lang returns [python.Lang], with which the messages of the assertions of
// the kernel start.
func (adapter) Lang() symbol.Lang { return python.Lang }

// Available reports whether python3 and pyright are both on PATH, and names
// the first binary that it did not find.
func (adapter) Available() (bool, string) { return toolchain.LookPaths(pythonBinary, pyrightBinary) }

// Layout writes the generated output as a project in a scratch directory.
// Every file is at its own path, and a pyrightconfig.json is at the root
// unless the output has one there. The configuration checks the project in
// the strict mode of pyright for the target version. A write that fails,
// such as the write of a path that leaves the directory, returns an error
// and removes the directory.
func (a adapter) Layout(g toolchain.Generated) (string, error) {
	if a.err != nil {
		return "", a.err
	}
	config := "{\n  \"pythonVersion\": \"" + string(a.version) + "\",\n  \"typeCheckingMode\": \"strict\"\n}\n"
	return toolchain.Scratch(scratchPattern, g, map[string][]byte{configFile: []byte(config)})
}

// Parse reads the syntax of every module and stub file of the project with
// the ast module of python3, under the grammar of the target version. It
// returns an error that contains each syntax error with its file, its line
// and its column. A type error passes. Parse returns an error for a project
// without a Python file, because a parse of nothing proves nothing, and for
// an interpreter older than the target version. The grammar of an earlier
// version is the best effort of the feature_version of ast: it refuses a
// type statement below 3.12, and it does not refuse every construct of a
// later version, such as a lazy import below 3.15.
//
// Parse returns the error of a python3 that does not run to its exit, such
// as a python3 that is not on PATH. A context that ends stops python3, and
// Parse returns an error that wraps the error of the context.
func (a adapter) Parse(ctx context.Context, dir string) error {
	if a.err != nil {
		return a.err
	}
	sources, err := toolchain.Sources(dir, []string{python.Extension, python.StubExtension})
	switch {
	case err != nil:
		return err
	case len(sources) == 0:
		return errNoPython
	}
	args := append([]string{isolated, script, parseScript, strconv.Itoa(a.minor)}, sources...)
	out, err := toolchain.Run(ctx, dir, nil, pythonBinary, args...)
	if _, exited := errors.AsType[*exec.ExitError](err); err != nil && !exited {
		return err
	}
	if err != nil {
		return fmt.Errorf("testing: python3 refuses the project:\n%s", strings.TrimSpace(out))
	}
	return nil
}

// TypeCheck checks the project with pyright for the target version, under
// the configuration of the project. It returns an error that contains each
// diagnostic of the severity error, with its file, its line, its column,
// its message and its rule. A warning passes. TypeCheck returns the error
// of a pyright that does not run, of one whose output has no report, and of
// one that fails without a diagnostic, such as for a configuration that
// pyright cannot read. A context that ends stops pyright, and TypeCheck
// returns an error that wraps the error of the context.
func (a adapter) TypeCheck(ctx context.Context, dir string) error {
	if a.err != nil {
		return a.err
	}
	found, err := a.check(ctx, dir)
	if err != nil || len(found) == 0 {
		return err
	}
	lines := make([]string, 0, len(found))
	for _, d := range found {
		lines = append(lines, d.line(dir))
	}
	return fmt.Errorf("testing: pyright refuses the project:\n%s", strings.Join(lines, "\n"))
}

// RunTests runs the test modules of the project, the files test_*.py, with
// the discovery of unittest. The runner writes its report to the output,
// and the counts come from the line that the program prints last. A test
// that fails, that raises or that passes against its expected failure
// counts as failed, and a test that skips or that fails as expected counts
// as skipped. A project without a test module returns an empty report,
// which fails the assertion of the kernel.
//
// RunTests returns the error of a python3 that does not start, and an error
// that wraps errNoSummary for a run that ends without its counts, such as a
// test that ends the interpreter. A context that ends stops python3, and
// RunTests returns an error that wraps the error of the context.
func (a adapter) RunTests(ctx context.Context, dir string) (toolchain.TestReport, error) {
	if a.err != nil {
		return toolchain.TestReport{}, a.err
	}
	out, err := toolchain.Run(ctx, dir, nil, pythonBinary, isolated, script, testScript, testFiles)
	report := toolchain.TestReport{Output: out}
	if _, exited := errors.AsType[*exec.ExitError](err); err != nil && !exited {
		return report, err
	}
	var s summary
	start := max(strings.LastIndex(out, summaryMark), 0)
	if decodeErr := json.NewDecoder(strings.NewReader(out[start:])).Decode(&s); decodeErr != nil {
		return report, fmt.Errorf("%w: %w:\n%s", errNoSummary, decodeErr, out)
	}
	report.Passed, report.Failed, report.Skipped = s.Run-s.Failed-s.Skipped, s.Failed, s.Skipped
	return report, nil
}

// Satisfies asks pyright whether a value of one class is assignable to one
// contract, which is satisfaction under the nominal subtyping of a class and
// the structural subtyping of a Protocol. typeName and contract name a class
// as module:Class or module.Class, as shop.models:Order.Line or
// shop.models.Order, and a builtin by its name alone. A probe module at the
// root of the project imports both modules and returns a value of the type
// from a function whose result is the contract, and pyright checks the
// project again.
//
// A probe that pyright refuses with the rule of a returned type alone
// reports false. Satisfies returns an error for any other refusal, with the
// source of the probe, and for a project that does not type-check without
// the probe.
func (a adapter) Satisfies(ctx context.Context, dir, typeName, contract string) (bool, error) {
	if err := a.TypeCheck(ctx, dir); err != nil {
		return false, fmt.Errorf("testing: the project does not type-check, so nothing can be asked of it: %w", err)
	}
	typeModule, typeRef := qualified(typeName)
	contractModule, contractRef := qualified(contract)
	var b strings.Builder
	for _, module := range slices.Compact([]string{typeModule, contractModule}) {
		if module != "" {
			b.WriteString("import " + module + "\n")
		}
	}
	b.WriteString("\n\ndef eidos_probe(value: " + typeRef + ") -> " + contractRef + ":\n    return value\n")
	source := b.String()
	var found []diagnostic
	if err := toolchain.Probe(dir, probeFile, source, func() error {
		var checkErr error
		found, checkErr = a.check(ctx, dir)
		return checkErr
	}); err != nil {
		return false, err
	}
	for _, d := range found {
		if d.Rule != returnRule {
			return false, fmt.Errorf("testing: the probe does not type-check for a reason other than the "+
				"contract: %s\nthe probe is:\n%s", d.line(dir), source)
		}
	}
	return len(found) == 0, nil
}

// check runs pyright over the project and returns its diagnostics of the
// severity error. It returns the error of a pyright that does not run, an
// error that wraps errNoReport for an output without a report, and the
// error of a run that fails without such a diagnostic.
func (a adapter) check(ctx context.Context, dir string) ([]diagnostic, error) {
	out, err := toolchain.Run(ctx, dir, nil, pyrightBinary, "--outputjson", "--pythonversion", string(a.version),
		"-p", dir)
	if _, exited := errors.AsType[*exec.ExitError](err); err != nil && !exited {
		return nil, err
	}
	var r report
	start := max(strings.IndexByte(out, '{'), 0)
	if decodeErr := json.NewDecoder(strings.NewReader(out[start:])).Decode(&r); decodeErr != nil {
		return nil, fmt.Errorf("%w: %w:\n%s", errNoReport, decodeErr, out)
	}
	var found []diagnostic
	for _, d := range r.GeneralDiagnostics {
		if d.Severity == errorSeverity {
			found = append(found, d)
		}
	}
	if err != nil && len(found) == 0 {
		return nil, err
	}
	return found, nil
}

// qualified returns the module to import for a class name, and the
// reference of the class in a module that imports it: the module before
// the colon of module:Class, the module before the last dot of
// module.Class, and no module for a name without either, such as a
// builtin.
func qualified(name string) (module, ref string) {
	if module, class, found := strings.Cut(name, qualifiedSep); found {
		return module, module + moduleSep + class
	}
	if module, _, found := strings.CutLast(name, moduleSep); found {
		return module, name
	}
	return "", name
}
