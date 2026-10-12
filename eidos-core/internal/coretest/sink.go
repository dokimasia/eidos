// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package coretest

import (
	"strings"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/diag"
)

// Codes returns the codes a sink collected, in report order.
//
// A case that compares the whole slice checks what was reported and how
// many times. A membership check does not check the count.
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
// A call without codes asserts a clean run, which is the common case.
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
// has a position that [diag.Diag.Positioned] accepts and a message,
// and reports every finding that lacks either.
//
// A reader cannot act on a finding without either, so every reporting
// path shares this contract.
func AssertPositioned(tb assert.TB, s *diag.Sink) {
	tb.Helper()

	for d := range s.All() {
		expect.True(tb, d.Positioned(),
			"the "+d.Code.String()+" finding has a position")
		expect.NotEmpty(tb, d.Msg,
			"the "+d.Code.String()+" finding has a message")
	}
}

// describe writes the findings of a sink for a failure message, so a
// mismatch shows the reported findings and not only their codes.
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
