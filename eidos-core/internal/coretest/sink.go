// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package coretest

import (
	"strings"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/position"
)

// Codes returns the codes a sink collected, in report order.
//
// Report order is the assertion order: a case comparing the whole
// slice holds both what was reported and how many times, which a
// membership check does not.
func Codes(s *diag.Sink) []diag.Code {
	out := []diag.Code{}
	for d := range s.All() {
		out = append(out, d.Code)
	}
	return out
}

// AssertCodes fails unless the sink collected exactly want, in
// order.
//
// Passing no codes asserts a clean run, which is the common case
// and reads better than comparing against an empty slice.
func AssertCodes(tb assert.TB, s *diag.Sink, want ...diag.Code) {
	tb.Helper()

	got := Codes(s)
	if len(want) == 0 {
		assert.Empty(tb, got, "the run reports nothing: "+describe(s))
		return
	}
	assert.Equal(tb, got, want,
		"the run reports exactly what it should: "+describe(s))
}

// AssertReports fails unless the sink collected want at least once.
//
// A case about one finding among several uses this rather than
// [AssertCodes], so an unrelated finding elsewhere does not break
// it. A case about the whole report still uses [AssertCodes].
func AssertReports(tb assert.TB, s *diag.Sink, want diag.Code) {
	tb.Helper()

	assert.Contains(tb, Codes(s), want, "the run reports "+want.String()+": "+describe(s))
}

// AssertPositioned fails unless every finding the sink collected
// has a position and a message, and reports every finding that
// lacks either.
//
// A finding without either cannot be acted on, so this states the
// contract every reporting path shares, not one path's own.
func AssertPositioned(tb assert.TB, s *diag.Sink) {
	tb.Helper()

	for d := range s.All() {
		expect.NotEqual(tb, d.Pos, position.Pos{},
			"the "+d.Code.String()+" finding has a position")
		expect.NotEmpty(tb, d.Msg,
			"the "+d.Code.String()+" finding has a message")
	}
}

// describe renders a sink's findings for a failure message, so a
// mismatch names what was reported rather than a bare code list.
func describe(s *diag.Sink) string {
	var out strings.Builder
	out.WriteString("reported [")
	first := true
	for d := range s.All() {
		if !first {
			out.WriteString("; ")
		}
		first = false
		out.WriteString(d.Code.String() + ": " + d.Msg)
	}
	out.WriteString("]")
	return out.String()
}
