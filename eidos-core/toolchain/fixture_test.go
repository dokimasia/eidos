// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package toolchain_test

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/toolchain"
)

// scriptedLang is the language the fixture adapter answers for.
const scriptedLang symbol.Lang = "scripted"

// scripted is an adapter whose every answer the case states, so the
// assertions are exercised without a compiler. It records the
// directory it laid out, which is what the cleanup contract rests
// on.
type scripted struct {
	absent     string
	layoutErr  error
	parseErr   error
	typeErr    error
	report     toolchain.TestReport
	testsErr   error
	satisfies  bool
	satisfyErr error
	laidOut    *string
	sawFiles   *map[string]string
}

func (scripted) Lang() symbol.Lang { return scriptedLang }

func (s scripted) Available() (bool, string) {
	if s.absent != "" {
		return false, s.absent
	}
	return true, ""
}

func (s scripted) Layout(g toolchain.Generated) (string, error) {
	if s.layoutErr != nil {
		return "", s.layoutErr
	}
	dir, err := os.MkdirTemp("", "scripted-*")
	if err != nil {
		return "", err
	}
	if s.sawFiles != nil {
		seen := map[string]string{}
		for path, body := range g.Files {
			seen[path] = string(body)
		}
		*s.sawFiles = seen
	}
	if s.laidOut != nil {
		*s.laidOut = dir
	}
	return dir, nil
}

func (s scripted) Parse(string) error     { return s.parseErr }
func (s scripted) TypeCheck(string) error { return s.typeErr }

func (s scripted) RunTests(string) (toolchain.TestReport, error) {
	return s.report, s.testsErr
}

func (s scripted) Satisfies(_, typeName, contract string) (bool, error) {
	if s.satisfyErr != nil {
		return false, s.satisfyErr
	}
	if typeName == "" || contract == "" {
		return false, errors.New("the fixture was asked an empty question")
	}
	return s.satisfies, nil
}

// output is a fixture carrying one generated file.
func output() toolchain.Generated {
	return toolchain.Generated{
		Files:  map[string][]byte{"gen/row.s": []byte("row\n")},
		Module: "example.test/generated",
	}
}

// recorder is a test handle collecting what an assertion reported
// and whether it skipped, so a case reads the outcome rather than
// failing the run.
type recorder struct {
	failures []string
	skipped  string
	fatal    bool
}

func (*recorder) Helper() {}

func (r *recorder) Errorf(format string, args ...any) {
	r.failures = append(r.failures, fmt.Sprintf(format, args...))
}

func (r *recorder) Fatalf(format string, args ...any) {
	r.fatal = true
	r.failures = append(r.failures, fmt.Sprintf(format, args...))
}

func (r *recorder) Skip(args ...any) {
	r.skipped = fmt.Sprint(args...)
}

// failed reports whether anything was recorded.
func (r *recorder) failed() bool { return len(r.failures) > 0 }

// says reports whether any recorded failure mentions text.
func (r *recorder) says(text string) bool {
	for _, f := range r.failures {
		if strings.Contains(f, text) {
			return true
		}
	}
	return false
}

// exists reports whether a path is on disk.
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
