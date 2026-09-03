// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package toolchain

// AssertParses holds the generated output to its language's
// grammar: it lays the fixture out and asks the adapter to read the
// syntax and nothing more, so a syntax error is told apart from a
// type error and reported as the one it is.
func AssertParses(tb TB, a Adapter, g Generated) {
	tb.Helper()

	dir, done := Prepare(tb, a, g)
	if dir == "" {
		return
	}
	defer done()

	if err := a.Parse(dir); err != nil {
		tb.Errorf("%s: the generated output does not parse: %v", a.Lang(), err)
	}
}

// AssertTypeChecks holds the generated output to its language's
// type rules, which is the check that catches a reference the
// render qualified wrongly or an import it never recorded.
func AssertTypeChecks(tb TB, a Adapter, g Generated) {
	tb.Helper()

	dir, done := Prepare(tb, a, g)
	if dir == "" {
		return
	}
	defer done()

	if err := a.TypeCheck(dir); err != nil {
		tb.Errorf("%s: the generated output does not type-check: %v", a.Lang(), err)
	}
}

// AssertTestsPass runs the generated project's own tests and holds
// every case that ran to passing. A run reporting no case at all
// fails: generated tests that execute nothing pass while proving
// nothing, which is the failure this assertion exists to catch.
func AssertTestsPass(tb TB, a Adapter, g Generated) {
	tb.Helper()

	dir, done := Prepare(tb, a, g)
	if dir == "" {
		return
	}
	defer done()

	report, err := a.RunTests(dir)
	if err != nil {
		tb.Errorf("%s: running the generated tests: %v\n%s", a.Lang(), err, report.Output)
		return
	}
	if report.OK() {
		return
	}
	if report.Passed == 0 && report.Failed == 0 {
		tb.Errorf("%s: the generated tests reported no case, and %d skipped: a suite that "+
			"executes nothing passes while proving nothing\n%s",
			a.Lang(), report.Skipped, report.Output)
		return
	}
	tb.Errorf("%s: %d of %d generated tests failed\n%s",
		a.Lang(), report.Failed, report.Passed+report.Failed, report.Output)
}

// AssertSatisfies holds one generated type to one contract: the
// interface, trait or protocol a doubling generator promised its
// output would meet.
func AssertSatisfies(tb TB, a Adapter, g Generated, typeName, contract string) {
	tb.Helper()
	satisfies(tb, a, g, typeName, contract, true)
}

// AssertDoesNotSatisfy is the inverse, for a generator that
// narrows: a type the output must not accidentally meet, so a
// contract widened by mistake is caught rather than welcomed.
func AssertDoesNotSatisfy(tb TB, a Adapter, g Generated, typeName, contract string) {
	tb.Helper()
	satisfies(tb, a, g, typeName, contract, false)
}

// satisfies runs one satisfaction question and holds the answer to
// what the caller expected.
func satisfies(tb TB, a Adapter, g Generated, typeName, contract string, want bool) {
	tb.Helper()

	if typeName == "" || contract == "" {
		tb.Errorf("%s: a satisfaction check names a type and a contract, got %q and %q",
			a.Lang(), typeName, contract)
		return
	}
	dir, done := Prepare(tb, a, g)
	if dir == "" {
		return
	}
	defer done()

	got, err := a.Satisfies(dir, typeName, contract)
	if err != nil {
		tb.Errorf("%s: asking whether %s satisfies %s: %v", a.Lang(), typeName, contract, err)
		return
	}
	switch {
	case got == want:
	case want:
		tb.Errorf("%s: %s does not satisfy %s, and the generated output promised it would",
			a.Lang(), typeName, contract)
	default:
		tb.Errorf("%s: %s satisfies %s, which the generated output was not to widen to",
			a.Lang(), typeName, contract)
	}
}
