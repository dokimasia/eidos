// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/lang/rust"
	"go.dokimi.dev/eidos/sdk/symbol"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The spellings of the Rust toolchain.
const (
	cargoBinary   = "cargo"
	rustfmtBinary = "rustfmt"
	manifestFile  = "Cargo.toml"
	edition       = "2024"
	// scratchPattern names the scratch directory of a laid-out crate.
	scratchPattern = "eidos-rust-*"
	// defaultCrate names the crate when the Module of the fixture is empty,
	// so the laid-out project always has a name under which tests import
	// it.
	defaultCrate = "eidos_generated"
	// targetDir is the directory into which cargo builds. No generated path
	// uses it, and the project leaves it out of its own sources.
	targetDir = "eidos-target"
	// libRoot and binRoot are the crate roots that cargo reads, the library
	// first.
	libRoot = "src/lib.rs"
	binRoot = "src/main.rs"
	// rootMode is the mode with which the satisfaction check writes the
	// crate root, private to the user who runs the check.
	rootMode = 0o600
	// unsatisfied is the code of rustc for an unsatisfied trait bound, the
	// one refusal that makes a satisfaction probe report false. codedError
	// opens every error that rustc reports with a code.
	unsatisfied = "error[E0277]"
	codedError  = "error[E"
	// probeModule is the module that the satisfaction check writes and
	// removes.
	probeModule = "zz_eidos_probe"
)

// The summary lines of the test harness, which the tally reads: one for
// each test binary and one for the doc tests.
const (
	resultLine  = "test result: "
	passedWord  = " passed"
	failedWord  = " failed"
	ignoredWord = " ignored"
	fieldSep    = ";"
)

// adapter drives cargo and rustfmt over generated Rust. It has no state, so
// one value serves every check.
type adapter struct{}

// New returns the Rust toolchain adapter.
func New() toolchain.Adapter { return adapter{} }

// Lang returns [rust.Lang], with which the messages of the assertions of
// the kernel start.
func (adapter) Lang() symbol.Lang { return rust.Lang }

// Available reports whether cargo and rustfmt are both on PATH, and names
// the first binary that it did not find.
func (adapter) Available() (bool, string) { return toolchain.LookPaths(cargoBinary, rustfmtBinary) }

// Layout writes the generated output as a crate in a scratch directory.
// Every file is at its own path. A Cargo.toml at the root declares the
// module of the fixture as the name of the crate, or eidos_generated for a
// fixture without a module, unless the output has a Cargo.toml there. A
// write that fails, such as the write of a path that leaves the directory,
// returns an error and removes the directory.
func (adapter) Layout(g toolchain.Generated) (string, error) {
	crate := g.Module
	if crate == "" {
		crate = defaultCrate
	}
	manifest := "[package]\nname = " + strconv.Quote(crate) + "\nversion = \"0.0.0\"\nedition = " +
		strconv.Quote(edition) + "\n\n[dependencies]\n"
	return toolchain.Scratch(scratchPattern, g, map[string][]byte{manifestFile: []byte(manifest)})
}

// Parse reads the syntax of every Rust source through rustfmt, which parses
// a file and resolves nothing. It returns an error that contains the first
// file that rustfmt cannot parse, with its position. A project without a
// Rust file returns an error, because a parse of nothing proves nothing.
// Rust has no parser in Go, so the parse needs the toolchain, and a caller
// passes [toolchain.Require] first.
func (adapter) Parse(ctx context.Context, dir string) error {
	sources, err := toolchain.Sources(dir, []string{rust.Extension}, targetDir)
	if err != nil {
		return err
	}
	if len(sources) == 0 {
		return errors.New("testing: the laid-out project has no Rust file to parse")
	}
	for _, source := range sources {
		_, err := toolchain.Run(ctx, dir, nil, rustfmtBinary, "--edition", edition, "--emit", "stdout", source)
		if err != nil {
			return err
		}
	}
	return nil
}

// TypeCheck checks every target of the crate, its tests included, with
// cargo check, which is the type check of Rust.
func (adapter) TypeCheck(ctx context.Context, dir string) error {
	_, err := cargo(ctx, dir, "check", "--all-targets")
	return err
}

// RunTests runs the tests of the crate with cargo test. It runs every
// failing test and reads the counts from the summary line of each test
// binary. A failure that the summaries do not report, such as a compile
// error, returns an error.
func (adapter) RunTests(ctx context.Context, dir string) (toolchain.TestReport, error) {
	out, err := cargo(ctx, dir, "test", "--no-fail-fast")
	report := tally(out)
	if err != nil && report.Failed == 0 {
		return report, err
	}
	return report, nil
}

// Satisfies asks rustc whether one type implements one trait. A probe
// module, which the crate root declares for the check, calls a function
// bounded by the trait with the type, and cargo checks the crate again. The
// probe imports the names of the crate root, so a root type is spelled by
// its name and any other type by its path, such as crate::row::Row or
// std::fmt::Display.
//
// A probe that rustc refuses with unsatisfied trait bounds alone reports
// false. Satisfies returns an error for any other refusal, with the source
// of the probe, and for a crate that does not check without the probe. It
// restores the crate root after the check.
func (a adapter) Satisfies(ctx context.Context, dir, typeName, contract string) (bool, error) {
	if err := a.TypeCheck(ctx, dir); err != nil {
		return false, fmt.Errorf("the crate does not check, so nothing can be asked of it: %w", err)
	}
	r, err := os.OpenRoot(dir)
	if err != nil {
		return false, fmt.Errorf("testing: %w", err)
	}
	defer r.Close()
	root, err := crateRoot(r)
	if err != nil {
		return false, err
	}
	original, err := r.ReadFile(root)
	if err != nil {
		return false, fmt.Errorf("testing: %w", err)
	}
	probe := path.Join(path.Dir(filepath.ToSlash(root)), probeModule+rust.Extension)
	source := "#![allow(dead_code, unused_imports)]\nuse super::*;\n\n" +
		"fn eidos_probe<T: ?Sized + " + contract + ">() {}\n\n" +
		"fn eidos_probe_check() {\n    eidos_probe::<" + typeName + ">();\n}\n"
	declared := slices.Concat(original, []byte("\nmod "+probeModule+";\n"))
	var (
		out      string
		checkErr error
	)
	err = toolchain.Probe(dir, probe, source, func() error {
		if declareErr := r.WriteFile(root, declared, rootMode); declareErr != nil {
			return fmt.Errorf("testing: %w", declareErr)
		}
		out, checkErr = cargo(ctx, dir, "check")
		if restoreErr := r.WriteFile(root, original, rootMode); restoreErr != nil {
			return fmt.Errorf("testing: %w", restoreErr)
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	if checkErr == nil {
		return true, nil
	}
	if refused := strings.Count(out, unsatisfied); refused > 0 && strings.Count(out, codedError) == refused {
		return false, nil
	}
	return false, fmt.Errorf("the probe does not check for a reason other than the trait: %w\nthe probe is:\n%s",
		checkErr, source)
}

// crateRoot returns the name of the file in r that declares the top-level
// modules of the crate. It is the library root, or the binary root where
// the crate has no library. A crate with neither returns an error.
func crateRoot(r *os.Root) (string, error) {
	for _, candidate := range []string{libRoot, binRoot} {
		name := filepath.FromSlash(candidate)
		if _, err := r.Stat(name); err == nil {
			return name, nil
		}
	}
	return "", errors.New("testing: the laid-out crate has no src/lib.rs or src/main.rs to probe from")
}

// cargo runs one cargo command over the crate under ctx. cargo runs offline
// and builds into [targetDir], so a check makes no network request and
// writes its build inside the scratch directory.
func cargo(ctx context.Context, dir, command string, args ...string) (string, error) {
	full := append([]string{command, "--offline", "--manifest-path", filepath.Join(dir, manifestFile)}, args...)
	env := []string{"CARGO_TARGET_DIR=" + filepath.Join(dir, targetDir), "CARGO_TERM_COLOR=never"}
	return toolchain.Run(ctx, dir, env, cargoBinary, full...)
}

// tally sums the counts of every summary line, such as "test result: ok. 1
// passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in
// 0.00s".
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

// number reads the count in front of the label of a summary field. The
// count is the last word before the label, after a status such as "ok." in
// the first field. A count that does not parse is zero.
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
