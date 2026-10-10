// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package toolchain_test

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/symbol"
	"go.dokimi.dev/eidos/core/toolchain"
)

// scriptedLang is the language the fixture adapter answers for.
const scriptedLang symbol.Lang = "scripted"

// scripted is an adapter whose every answer the case states, so the
// assertions are exercised without a compiler. It records the
// directory it laid out, which is what the cleanup contract rests
// on. The gate, suite and assertion specs run against it.
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

// Lang returns the fixture's language.
func (scripted) Lang() symbol.Lang { return scriptedLang }

// Available reports the toolchain present, or absent for the reason
// the case states.
func (s scripted) Available() (bool, string) {
	if s.absent != "" {
		return false, s.absent
	}
	return true, ""
}

// Layout returns a fresh temporary directory, or the case's error. It
// records the directory and the files it was handed where the case
// asks for them.
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

// Parse returns the error of a context that ended, and the case's parse
// error otherwise.
func (s scripted) Parse(ctx context.Context, _ string) error { return cmp.Or(ctx.Err(), s.parseErr) }

// TypeCheck returns the error of a context that ended, and the case's
// type-check error otherwise.
func (s scripted) TypeCheck(ctx context.Context, _ string) error { return cmp.Or(ctx.Err(), s.typeErr) }

// RunTests returns the case's report, and the error of a context that
// ended or the case's error.
func (s scripted) RunTests(ctx context.Context, _ string) (toolchain.TestReport, error) {
	return s.report, cmp.Or(ctx.Err(), s.testsErr)
}

// Satisfies returns the error of a context that ended, the case's
// verdict or error, and an error for an empty question.
func (s scripted) Satisfies(ctx context.Context, _, typeName, contract string) (bool, error) {
	if err := cmp.Or(ctx.Err(), s.satisfyErr); err != nil {
		return false, err
	}
	if typeName == "" || contract == "" {
		return false, errors.New("the fixture was asked an empty question")
	}
	return s.satisfies, nil
}

// seat is the seat a case hands the gate: an [assert.Recorder], whose
// records of the gate's failures the case reads, that also records the
// skip the gate asks for.
type seat struct {
	*assert.Recorder
	skipped string
}

// Skip records the skip's reason and returns, so the case reads it.
func (s *seat) Skip(args ...any) {
	s.skipped = fmt.Sprint(args...)
}

// A run over nothing proves nothing, so the fixture and the report
// each state their own emptiness, and every assertion reads it.
func TestAdapter(t *testing.T) {
	t.Parallel()

	t.Run("Generated", func(t *testing.T) {
		t.Parallel()

		t.Run("IsEmpty", func(t *testing.T) {
			t.Parallel()

			assert.True(t, toolchain.Generated{}.IsEmpty(), "a fixture carrying no file is empty")
			assert.False(t, toolchain.Generated{Files: map[string][]byte{"a.go": nil}}.IsEmpty(),
				"and one carrying a file is not, whatever the file holds")
		})
	})

	t.Run("TestReport", func(t *testing.T) {
		t.Parallel()

		t.Run("OK", func(t *testing.T) {
			t.Parallel()

			assert.False(t, toolchain.TestReport{}.OK(), "a report of no case is not a pass")
			assert.True(t, toolchain.TestReport{Passed: 1, Skipped: 2}.OK(),
				"one passing case and no failure is a pass")
			assert.False(t, toolchain.TestReport{Passed: 3, Failed: 1}.OK(), "and one failure fails it")
		})
	})
}

// output is a fixture carrying one generated file.
func output() toolchain.Generated {
	return toolchain.Generated{
		Files:  map[string][]byte{"gen/row.s": []byte("row\n")},
		Module: "example.test/generated",
	}
}
