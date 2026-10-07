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

	rust "go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/sdk/symbol"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The Rust toolchain's spellings.
const (
	cargoBinary   = "cargo"
	rustfmtBinary = "rustfmt"
	manifestFile  = "Cargo.toml"
	edition       = "2024"
	// defaultCrate names the crate when the fixture's Module is empty,
	// so the laid-out project always has a name that tests import it
	// by.
	defaultCrate = "eidos_generated"
	// targetDir is where cargo builds: a directory no generated path
	// uses, left out of the project's own sources.
	targetDir = "eidos-target"
	// libRoot and binRoot are the crate roots cargo reads, the library
	// first.
	libRoot = "src/lib.rs"
	binRoot = "src/main.rs"
	// dirPerm and filePerm keep the scratch project private to the
	// user running the check.
	dirPerm  = 0o700
	filePerm = 0o600
	// unsatisfied is rustc's code for an unsatisfied trait bound, the
	// one refusal that makes a satisfaction probe report false.
	// codedError opens every error rustc reports with a code.
	unsatisfied = "error[E0277]"
	codedError  = "error[E"
	// probeModule is the module the satisfaction check writes and
	// removes.
	probeModule = "zz_eidos_probe"
)

// The test harness's summary lines, which the tally reads: one per
// test binary and one for the doc tests.
const (
	resultLine  = "test result: "
	passedWord  = " passed"
	failedWord  = " failed"
	ignoredWord = " ignored"
	fieldSep    = ";"
)

// runTimeout bounds one toolchain invocation, so a compiler that
// never returns fails the check with an error and leaves the suite
// running.
const runTimeout = 5 * time.Minute

// adapter drives cargo and rustfmt over generated Rust. It has no
// state, so one value serves every check.
type adapter struct{}

// New returns the Rust toolchain adapter.
func New() toolchain.Adapter { return adapter{} }

// Lang returns [rust.Lang], which the kernel's assertions name in
// their failure messages.
func (adapter) Lang() symbol.Lang { return rust.Lang }

// Available reports whether cargo and rustfmt are both on PATH, and
// names the first binary it did not find.
func (adapter) Available() (bool, string) {
	for _, binary := range []string{cargoBinary, rustfmtBinary} {
		if _, err := exec.LookPath(binary); err != nil {
			return false, fmt.Sprintf("%s is not on PATH: %v", binary, err)
		}
	}
	return true, ""
}

// Layout writes the generated output as a crate in a scratch
// directory: every file at its own path, and a Cargo.toml at the root
// declaring the fixture's module as the crate's name, or
// eidos_generated for a fixture stating none, unless the output has
// one there already. A write that fails, a path escaping the
// directory among them, returns an error and removes the directory.
func (adapter) Layout(g toolchain.Generated) (string, error) {
	dir, err := os.MkdirTemp("", "eidos-rust-*")
	if err != nil {
		return "", err
	}
	for path, body := range g.Files {
		if err := write(dir, path, body); err != nil {
			return "", errors.Join(err, os.RemoveAll(dir))
		}
	}
	if _, stated := g.Files[manifestFile]; !stated {
		crate := g.Module
		if crate == "" {
			crate = defaultCrate
		}
		manifest := "[package]\nname = " + strconv.Quote(crate) + "\nversion = \"0.0.0\"\nedition = " +
			strconv.Quote(edition) + "\n\n[dependencies]\n"
		if err := write(dir, manifestFile, []byte(manifest)); err != nil {
			return "", errors.Join(err, os.RemoveAll(dir))
		}
	}
	return dir, nil
}

// Parse reads every Rust source's syntax through rustfmt, which
// parses a file and resolves nothing, and returns an error naming the
// first file rustfmt cannot parse, with its position. A project with
// no Rust file returns an error, because a parse of nothing proves
// nothing. Rust has no parser in Go, so the parse needs the
// toolchain, and a caller passes [toolchain.Require] first.
func (adapter) Parse(ctx context.Context, dir string) error {
	var sources []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == targetDir {
			return filepath.SkipDir
		}
		if !d.IsDir() && filepath.Ext(path) == rust.Extension {
			sources = append(sources, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(sources) == 0 {
		return errors.New("testing: the laid-out project has no Rust file to parse")
	}
	for _, source := range sources {
		if _, err := run(ctx, dir, rustfmtBinary, "--edition", edition, "--emit", "stdout", source); err != nil {
			return err
		}
	}
	return nil
}

// TypeCheck checks every target of the crate, its tests included,
// with cargo check, which is Rust's type check.
func (adapter) TypeCheck(ctx context.Context, dir string) error {
	_, err := cargo(ctx, dir, "check", "--all-targets")
	return err
}

// RunTests runs the crate's tests with cargo test, every failing test
// run through, and reads the counts off each test binary's summary
// line. A failure the summaries do not report, such as a compile
// error, returns an error.
func (adapter) RunTests(ctx context.Context, dir string) (toolchain.TestReport, error) {
	out, err := cargo(ctx, dir, "test", "--no-fail-fast")
	report := tally(out)
	if err != nil && report.Failed == 0 {
		return report, err
	}
	return report, nil
}

// Satisfies asks rustc whether one type implements one trait: a probe
// module, declared from the crate root, calls a function bounded by
// the trait with the type, and the crate is checked again. The probe
// imports the crate root's names, so a root type is spelled by its
// name and any other by its path, such as crate::row::Row or
// std::fmt::Display. A probe that rustc refuses with unsatisfied
// trait bounds alone reports false. Any other refusal returns an
// error, and so does a crate that does not check without the probe.
func (a adapter) Satisfies(ctx context.Context, dir, typeName, contract string) (bool, error) {
	if err := a.TypeCheck(ctx, dir); err != nil {
		return false, fmt.Errorf("the crate does not check, so nothing can be asked of it: %w", err)
	}
	root, err := crateRoot(dir)
	if err != nil {
		return false, err
	}
	original, err := os.ReadFile(root)
	if err != nil {
		return false, err
	}
	probe := filepath.Join(filepath.Dir(root), probeModule+rust.Extension)
	source := "#![allow(dead_code, unused_imports)]\nuse super::*;\n\n" +
		"fn eidos_probe<T: ?Sized + " + contract + ">() {}\n\n" +
		"fn eidos_probe_check() {\n    eidos_probe::<" + typeName + ">();\n}\n"
	err = os.WriteFile(probe, []byte(source), filePerm)
	if err != nil {
		return false, err
	}
	defer func() { _ = os.Remove(probe) }()
	declared := slices.Concat(original, []byte("\nmod "+probeModule+";\n"))
	err = os.WriteFile(root, declared, filePerm)
	if err != nil {
		return false, err
	}
	defer func() { _ = os.WriteFile(root, original, filePerm) }()

	out, err := cargo(ctx, dir, "check")
	if err == nil {
		return true, nil
	}
	if refused := strings.Count(out, unsatisfied); refused > 0 && strings.Count(out, codedError) == refused {
		return false, nil
	}
	return false, fmt.Errorf("the probe does not check for a reason other than the trait: %w", err)
}

// crateRoot returns the file that declares the crate's top-level
// modules: the library root, or the binary root where the crate has
// no library. A crate with neither returns an error.
func crateRoot(dir string) (string, error) {
	for _, candidate := range []string{libRoot, binRoot} {
		root := filepath.Join(dir, filepath.FromSlash(candidate))
		if _, err := os.Stat(root); err == nil {
			return root, nil
		}
	}
	return "", errors.New("testing: the laid-out crate has no src/lib.rs or src/main.rs to probe from")
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

// cargo runs one cargo command over the crate under ctx, offline and
// building into [targetDir], so a check makes no network request and
// writes its build inside the scratch directory.
func cargo(ctx context.Context, dir, command string, args ...string) (string, error) {
	full := append([]string{command, "--offline", "--manifest-path", filepath.Join(dir, manifestFile)}, args...)
	return run(ctx, dir, cargoBinary, full...)
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
	cmd.Env = append(os.Environ(), "CARGO_TARGET_DIR="+filepath.Join(dir, targetDir), "CARGO_TERM_COLOR=never")
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

// tally sums the counts of every summary line, such as
// "test result: ok. 1 passed; 0 failed; 0 ignored; 0 measured; 0
// filtered out; finished in 0.00s".
func tally(stream string) toolchain.TestReport {
	report := toolchain.TestReport{Output: stream}
	for line := range strings.Lines(stream) {
		_, counts, summary := strings.Cut(line, resultLine)
		if !summary {
			continue
		}
		for field := range strings.SplitSeq(counts, fieldSep) {
			field = strings.TrimSpace(field)
			switch {
			case strings.HasSuffix(field, passedWord):
				report.Passed += number(field, passedWord)
			case strings.HasSuffix(field, failedWord):
				report.Failed += number(field, failedWord)
			case strings.HasSuffix(field, ignoredWord):
				report.Skipped += number(field, ignoredWord)
			}
		}
	}
	return report
}

// number reads the count in front of a summary field's label: the
// last word before it, after a status such as "ok." in the first
// field. A count that does not read is zero.
func number(field, label string) int {
	words := strings.Fields(strings.TrimSuffix(field, label))
	if len(words) == 0 {
		return 0
	}
	n, err := strconv.Atoi(words[len(words)-1])
	if err != nil {
		return 0
	}
	return n
}
