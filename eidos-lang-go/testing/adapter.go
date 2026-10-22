// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"context"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/symbol"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The spellings of the go tool.
const (
	goBinary    = "go"
	modFile     = "go.mod"
	allPackages = "./..."
	// scratchPattern names the scratch directory of a laid-out project.
	scratchPattern = "eidos-go-*"
	// noTest is a test pattern that no test name matches, so go test
	// compiles every test binary and runs none.
	noTest = "^$"
	// defaultModule is the module of a fixture that states none, so the
	// laid-out project always has an import path.
	defaultModule = "eidos.test/generated"
	// goVersion is the language version that the scratch module declares.
	// It is the version of the repository, because no one can build output
	// that the toolchain of the repository cannot build.
	goVersion = "1.27.3"
	// testSuffix ends the name of a test file, whose package the probe
	// never joins.
	testSuffix = "_test" + golang.Extension
	// qualifierSep separates a package from a name: the import path of a
	// contract from its name, and the name of a package from a type that a
	// reference names.
	qualifierSep = "."
	// typeArgsOpen opens the argument list of a generic type. The search
	// for the qualifier of a contract stops before it.
	typeArgsOpen = "["
	// importPathSep separates the elements of an import path. A type
	// expression of Go never contains it.
	importPathSep = "/"
	// probeImport is the name under which the probe imports the package of
	// a contract. No generated package declares it.
	probeImport = "eidosprobecontract"
	// notImplemented is the text of the compiler for the false answer.
	notImplemented = "does not implement"
)

// probeFile is the file that the satisfaction check writes and removes. The
// name has the marker that the Go convention reserves for generated files,
// so a tool that reads the scratch tree knows that no one wrote it by hand.
const probeFile = "zz_eidos_probe_gen.go"

// goEnv are the variables of each run of the go tool. The scratch module
// resolves nothing from the network, so a fixture that needs a dependency
// states it.
var goEnv = []string{"GOFLAGS=-mod=mod", "GOPROXY=off", "GOWORK=off"}

// adapter drives the Go toolchain over generated output. It has no state,
// so one value serves every check.
type adapter struct{}

// New returns the Go toolchain adapter.
func New() toolchain.Adapter { return adapter{} }

// Lang returns [golang.Lang].
func (adapter) Lang() symbol.Lang { return golang.Lang }

// Available reports whether the go tool is on PATH, and names the binary
// where it is not.
func (adapter) Available() (bool, string) { return toolchain.LookPaths(goBinary) }

// Layout writes the generated output as a module in a scratch directory.
// Every file is at its own path. A go.mod at the root declares the module
// of the fixture, unless the output has a go.mod at the root. A go.mod
// deeper in the output declares a nested module and leaves the root
// without one. A path that leaves the directory returns an error, because
// a fixture writes only inside its project.
func (adapter) Layout(g toolchain.Generated) (string, error) {
	module := g.Module
	if module == "" {
		module = defaultModule
	}
	mod := "module " + module + "\n\ngo " + goVersion + "\n"
	return toolchain.Scratch(scratchPattern, g, map[string][]byte{modFile: []byte(mod)})
}

// Parse reads the syntax of every Go file through the standard library,
// which needs no toolchain, and returns an error for the first file that
// does not parse, with its position. A project without a Go file returns an
// error, because a parse of nothing proves nothing. A context that ends
// stops the parse before the next file, and Parse returns its error.
func (adapter) Parse(ctx context.Context, dir string) error {
	sources, err := toolchain.Sources(dir, []string{golang.Extension})
	if err != nil {
		return err
	}
	if len(sources) == 0 {
		return errors.New("testing: the laid-out project has no Go file to parse")
	}
	fset := token.NewFileSet()
	for _, path := range sources {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution); err != nil {
			rel, relErr := filepath.Rel(dir, path)
			if relErr != nil {
				rel = path
			}
			return fmt.Errorf("%s: %w", rel, err)
		}
	}
	return nil
}

// TypeCheck compiles every package and the tests of every package without
// running one, which is the type check of Go. The compiler is the only
// complete check: go/types over a tree without its resolved dependencies
// answers a narrower question, and a build alone never compiles a _test.go
// file, which contains most generated checks. Vet is off, because a
// finding of vet is no type error, and [AssertVets] asks for it.
func (adapter) TypeCheck(ctx context.Context, dir string) error {
	_, err := toolchain.Run(ctx, dir, goEnv, goBinary, "test", "-vet=off", "-count=1", "-run="+noTest, allPackages)
	return err
}

// RunTests runs the tests of the project and reads the counts from the
// JSON stream of the tool, so a report states how many cases ran and not
// only whether the command exited with zero.
func (adapter) RunTests(ctx context.Context, dir string) (toolchain.TestReport, error) {
	out, err := toolchain.Run(ctx, dir, goEnv, goBinary, "test", "-json", "-count=1", allPackages)
	report := tally(out)
	if err != nil && report.Failed == 0 {
		// The command failed for a reason that the stream does not report,
		// such as a build error or a missing package.
		return report, err
	}
	return report, nil
}

// Satisfies asks the compiler whether one type implements one interface.
// It writes an assignment into the project and builds it. The type is
// probed as named: Row asks about the method set of the value, and *Row
// about the method set of the pointer. The probe imports the package of a
// contract that its import path qualifies, such as io.Reader or
// example.com/x.Store. An unqualified contract is an interface of the root
// package or a predeclared one.
//
// A build that fails because the type does not implement the contract is
// the false answer. Satisfies returns an error for any other build
// failure, with the source of the probe, and for a project that does not
// build without the probe. A claim that the probe cannot spell returns an
// error before the build: a type that contains an import path, and a type
// or a contract that is not a Go expression once the probe has replaced
// the import path of the contract.
func (a adapter) Satisfies(ctx context.Context, dir, typeName, contract string) (bool, error) {
	if err := checkClaim(typeName, contract); err != nil {
		return false, err
	}
	if err := a.TypeCheck(ctx, dir); err != nil {
		return false, fmt.Errorf("the project does not build, so nothing can be asked of it: %w", err)
	}
	pkg, err := probePackage(dir)
	if err != nil {
		return false, err
	}
	var (
		out      string
		buildErr error
	)
	source := probeSource(pkg, typeName, contract)
	err = toolchain.Probe(dir, probeFile, source, func() error {
		out, buildErr = toolchain.Run(ctx, dir, goEnv, goBinary, "build", allPackages)
		return nil
	})
	switch {
	case err != nil:
		return false, err
	case buildErr == nil:
		return true, nil
	case strings.Contains(out, notImplemented):
		return false, nil
	default:
		return false, fmt.Errorf("the probe does not build for a reason other than the contract: %w\nthe probe is:\n%s",
			buildErr, source)
	}
}

// checkClaim returns an error for a claim that the probe cannot spell.
// The probe is in the root package and imports only the package of the
// contract, so a type that contains an import path is the mistake of a
// caller who passes the path of the type. A type or a contract that is not
// a Go expression would fail the build of a file that the caller never
// sees.
func checkClaim(typeName, contract string) error {
	_, spelled := probeContract(contract)
	claims := []struct{ name, spelled string }{{name: typeName, spelled: typeName}, {name: contract, spelled: spelled}}
	for _, c := range claims {
		if strings.Contains(c.spelled, importPathSep) {
			return fmt.Errorf("testing: %q contains an import path, which the probe cannot spell as a type", c.name)
		}
		if _, err := parser.ParseExpr(c.spelled); err != nil {
			return fmt.Errorf("testing: %q is not a Go type expression: %w", c.name, err)
		}
	}
	return nil
}

// probeSource returns the source of the probe file. The file assigns a
// value of the named type to the contract, and imports the package of a
// qualified contract under [probeImport].
func probeSource(pkg, typeName, contract string) string {
	var b strings.Builder
	b.WriteString("package " + pkg + "\n\n")
	path, spelled := probeContract(contract)
	if path != "" {
		b.WriteString("import " + probeImport + " " + strconv.Quote(path) + "\n\n")
	}
	b.WriteString("var _ " + spelled + " = *new(" + typeName + ")\n")
	return b.String()
}

// probeContract returns the import path of the qualifier of contract, and
// the contract as the probe spells it, with the import path replaced by
// [probeImport]. A contract without a qualifier returns an empty path and
// the contract.
func probeContract(contract string) (string, string) {
	head, _, _ := strings.Cut(contract, typeArgsOpen)
	at := strings.LastIndex(head, qualifierSep)
	if at <= 0 {
		return "", contract
	}
	return contract[:at], probeImport + qualifierSep + contract[at+len(qualifierSep):]
}

// probePackage returns the package clause of the probe. The probe is
// written beside the files of the root directory, so its package is the
// package that those files declare. The package of a test file is never
// the package of the probe, because an external test package declares
// another name.
func probePackage(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("testing: %w", err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != golang.Extension ||
			strings.HasSuffix(e.Name(), testSuffix) {

			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()),
			nil, parser.PackageClauseOnly|parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		return f.Name.Name, nil
	}
	return "", errors.New("testing: the laid-out project's root declares no Go package to probe from")
}

// The action words of the JSON stream of go test that the tally counts.
const (
	actionPass = `"Action":"pass"`
	actionFail = `"Action":"fail"`
	actionSkip = `"Action":"skip"`
	testField  = `"Test":`
)

// tally reads the case counts from the JSON stream of go test. It counts
// only a line with a Test, so the results of each package, which the stream
// also reports, do not double every number.
func tally(stream string) toolchain.TestReport {
	report := toolchain.TestReport{Output: stream}
	for line := range strings.Lines(stream) {
		if !strings.Contains(line, testField) {
			continue
		}
		switch {
		case strings.Contains(line, actionPass):
			report.Passed++
		case strings.Contains(line, actionFail):
			report.Failed++
		case strings.Contains(line, actionSkip):
			report.Skipped++
		}
	}
	return report
}
