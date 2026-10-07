// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package toolchain

import (
	"context"
	"strconv"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
)

// AssertParses checks the generated output against its language's
// grammar: it lays the fixture out and asks the adapter, under ctx, to
// read the syntax and nothing more, so a syntax error is told apart
// from a type error. A refusal records a failure whose detail states
// the language's own error.
func AssertParses(ctx context.Context, tb assert.TB, a Adapter, g Generated) {
	tb.Helper()

	dir, done := Prepare(tb, a, g)
	if dir == "" {
		return
	}
	defer done()

	expect.NoError(tb, a.Parse(ctx, dir), string(a.Lang())+": the generated output parses")
}

// AssertTypeChecks checks the generated output against its language's
// type rules under ctx, which is the check that catches a reference the
// render qualified wrongly or an import it never recorded. A refusal
// records a failure whose detail states the language's own error.
func AssertTypeChecks(ctx context.Context, tb assert.TB, a Adapter, g Generated) {
	tb.Helper()

	dir, done := Prepare(tb, a, g)
	if dir == "" {
		return
	}
	defer done()

	expect.NoError(tb, a.TypeCheck(ctx, dir), string(a.Lang())+": the generated output type-checks")
}

// AssertTestsPass runs the generated project's own tests under ctx and
// requires every case that ran to pass. A run that does not report a
// case fails: generated tests that execute nothing pass while proving
// nothing, which is the failure this assertion exists to catch. Each
// failure's contract ends with the tool's own output.
func AssertTestsPass(ctx context.Context, tb assert.TB, a Adapter, g Generated) {
	tb.Helper()

	dir, done := Prepare(tb, a, g)
	if dir == "" {
		return
	}
	defer done()

	lang := string(a.Lang())
	report, err := a.RunTests(ctx, dir)
	expect.NoError(tb, err, lang+": the generated tests run\n"+report.Output)
	if err != nil {
		return
	}
	expect.NotEqual(tb, report.Passed+report.Failed, 0, lang+": the generated tests report a case, and "+
		strconv.Itoa(report.Skipped)+" skipped: a suite that executes nothing passes while proving nothing\n"+
		report.Output)
	expect.Equal(tb, report.Failed, 0, lang+": every generated test passes, of "+
		strconv.Itoa(report.Passed+report.Failed)+"\n"+report.Output)
}

// AssertSatisfies checks one generated type against one contract under
// ctx: the interface, trait or protocol a doubling generator promised
// its output would meet.
func AssertSatisfies(ctx context.Context, tb assert.TB, a Adapter, g Generated, typeName, contract string) {
	tb.Helper()
	satisfies(ctx, tb, a, g, typeName, contract, true)
}

// AssertDoesNotSatisfy is the inverse, for a generator that
// narrows: a type the output must not accidentally meet, so a
// contract widened by mistake is caught rather than welcomed.
func AssertDoesNotSatisfy(ctx context.Context, tb assert.TB, a Adapter, g Generated, typeName, contract string) {
	tb.Helper()
	satisfies(ctx, tb, a, g, typeName, contract, false)
}

// satisfies runs one satisfaction question under ctx and compares the
// answer with what the caller expected. A question without a type or a
// contract records a failure and does not run the toolchain.
func satisfies(ctx context.Context, tb assert.TB, a Adapter, g Generated, typeName, contract string, want bool) {
	tb.Helper()

	expect.NotEmpty(tb, typeName, "toolchain: a satisfaction check names a type")
	expect.NotEmpty(tb, contract, "toolchain: a satisfaction check names a contract")
	if typeName == "" || contract == "" {
		return
	}
	dir, done := Prepare(tb, a, g)
	if dir == "" {
		return
	}
	defer done()

	lang := string(a.Lang())
	got, err := a.Satisfies(ctx, dir, typeName, contract)
	expect.NoError(tb, err, lang+": the toolchain answers whether "+typeName+" satisfies "+contract)
	if err != nil {
		return
	}
	if want {
		expect.True(tb, got, lang+": "+typeName+" satisfies "+contract+", as the generated output promised")
		return
	}
	expect.False(tb, got, lang+": "+typeName+" does not satisfy "+contract+
		", which the generated output was not to widen to")
}
