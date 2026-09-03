// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"context"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/symbol"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The go tool's own spellings.
const (
	goBinary    = "go"
	modFile     = "go.mod"
	allPackages = "./..."
	// defaultModule is the module a fixture stating none declares,
	// so the laid-out project always has an import path.
	defaultModule = "eidos.test/generated"
	// goVersion is the language version the scratch module
	// declares. It tracks the repository's own, because output the
	// repository's toolchain cannot build is output nobody can.
	goVersion = "1.27.0"
	// dirPerm and filePerm are what the scratch project is written
	// with: a private tree, because it holds nothing anyone else
	// reads.
	dirPerm  = 0o700
	filePerm = 0o600
)

// adapter drives the Go toolchain over generated output. It holds
// no state, so one value serves every check.
type adapter struct{}

// New returns the Go toolchain adapter.
func New() toolchain.Adapter { return adapter{} }

// Lang names the language.
func (adapter) Lang() symbol.Lang { return golang.Lang }

// Available reports whether the go tool is on PATH, and says which
// binary was looked for where it is not.
func (adapter) Available() (bool, string) {
	if _, err := exec.LookPath(goBinary); err != nil {
		return false, fmt.Sprintf("%s is not on PATH: %v", goBinary, err)
	}
	return true, ""
}

// Layout writes the generated output as a module in a scratch
// directory: every file at its own path, and a go.mod declaring
// the fixture's module unless the output carries one already. A
// path escaping the directory refuses, because a fixture is not a
// place to write from.
func (adapter) Layout(g toolchain.Generated) (string, error) {
	dir, err := os.MkdirTemp("", "eidos-go-*")
	if err != nil {
		return "", err
	}
	written := false
	for path, body := range g.Files {
		target, err := resolve(dir, path)
		if err != nil {
			return "", errors.Join(err, os.RemoveAll(dir))
		}
		if err := os.MkdirAll(filepath.Dir(target), dirPerm); err != nil {
			return "", errors.Join(err, os.RemoveAll(dir))
		}
		if err := os.WriteFile(target, body, filePerm); err != nil {
			return "", errors.Join(err, os.RemoveAll(dir))
		}
		written = written || filepath.Base(path) == modFile
	}
	if !written {
		module := g.Module
		if module == "" {
			module = defaultModule
		}
		mod := "module " + module + "\n\ngo " + goVersion + "\n"
		if err := os.WriteFile(filepath.Join(dir, modFile), []byte(mod), filePerm); err != nil {
			return "", errors.Join(err, os.RemoveAll(dir))
		}
	}
	return dir, nil
}

// Parse reads every Go file's syntax through the standard library,
// which needs no toolchain, and reports the first file that
// refuses with its own position.
func (adapter) Parse(dir string) error {
	fset := token.NewFileSet()
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != golang.Extension {
			return err
		}
		if _, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution); err != nil {
			rel, relErr := filepath.Rel(dir, path)
			if relErr != nil {
				rel = path
			}
			return fmt.Errorf("%s: %w", rel, err)
		}
		return nil
	})
}

// TypeCheck builds every package, which is Go's type check: the
// compiler is the only complete one, and go/types over a tree
// without its dependencies resolved would answer a narrower
// question.
func (adapter) TypeCheck(dir string) error {
	_, err := run(dir, "build", allPackages)
	return err
}

// RunTests runs the project's tests and reads the counts off the
// tool's own JSON stream, so a report says how many cases ran
// rather than only whether the command exited zero.
func (adapter) RunTests(dir string) (toolchain.TestReport, error) {
	out, err := run(dir, "test", "-json", "-count=1", allPackages)
	report := tally(out)
	if err != nil && report.Failed == 0 {
		// The command failed for a reason the stream does not
		// carry: a build error, a missing package.
		return report, err
	}
	return report, nil
}

// Satisfies asks the compiler whether one type implements one
// interface, by writing an assertion into the project and building
// it. A type that does not implement the interface is a build
// error, which is the answer rather than a fault, so the check
// tells a false answer apart from a broken project by building the
// project alone first.
func (a adapter) Satisfies(dir, typeName, contract string) (bool, error) {
	if err := a.TypeCheck(dir); err != nil {
		return false, fmt.Errorf("the project does not build, so nothing can be asked of it: %w", err)
	}
	pkg, err := probePackage(dir)
	if err != nil {
		return false, err
	}
	probe := filepath.Join(dir, probeFile)
	body := "package " + pkg + "\n\nvar _ " + contract + " = (*" + typeName + ")(nil)\n"
	if err := os.WriteFile(probe, []byte(body), filePerm); err != nil {
		return false, err
	}
	defer func() { _ = os.Remove(probe) }()

	if _, err := run(dir, "build", allPackages); err != nil {
		return false, nil
	}
	return true, nil
}

// probeFile is the file the satisfaction check writes and removes.
// The name carries the marker the Go convention reserves, so a
// tool reading the scratch tree knows nobody wrote it by hand.
const probeFile = "zz_eidos_probe_gen.go"

// probePackage returns the package clause the probe must carry:
// the one the root directory's own files declare, because the
// probe sits beside them.
func probePackage(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != golang.Extension {
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

// resolve joins a fixture path onto the scratch directory and
// refuses one that climbs out of it.
func resolve(dir, path string) (string, error) {
	target := filepath.Join(dir, filepath.FromSlash(path))
	rel, err := filepath.Rel(dir, target)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("the generated path %q climbs out of the scratch project", path)
	}
	return target, nil
}

// runTimeout bounds one go invocation. A compiler that has not
// answered in this long is a harness failure rather than a slow
// machine, and an unbounded run would hang a suite instead of
// reporting.
const runTimeout = 5 * time.Minute

// run runs the go tool in dir and returns its combined output,
// wrapping a failure with that output so a message names what the
// tool said. The run is bounded, so a toolchain that never returns
// fails the assertion rather than the suite.
func run(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, goBinary, args...)
	cmd.Dir = dir
	// The scratch module resolves nothing from the network: a
	// fixture that needs a dependency is a fixture that states it.
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off", "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return string(out), fmt.Errorf(
				"testing: go %s did not answer within %s: %w", strings.Join(args, " "), runTimeout, ctx.Err(),
			)
		}
		return string(out), fmt.Errorf("go %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return string(out), nil
}

// The go test JSON stream's action words the tally counts.
const (
	actionPass = `"Action":"pass"`
	actionFail = `"Action":"fail"`
	actionSkip = `"Action":"skip"`
	testField  = `"Test":`
)

// tally reads the case counts off the go test JSON stream. Only a
// line naming a Test is counted, so the per-package results the
// stream also carries do not double every number.
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
