// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package toolchain_test

import (
	"errors"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/toolchain"
)

// The assertion set is written once here and every language gets
// it, so what each one passes on and what it refuses is contract
// for all of them.
func TestAssert(t *testing.T) {
	t.Parallel()

	t.Run("AssertParses", func(t *testing.T) {
		t.Parallel()

		t.Run("passes output the language reads", func(t *testing.T) {
			t.Parallel()

			var r recorder
			toolchain.AssertParses(&r, scripted{}, output())
			assert.False(t, r.failed(), "the fixture parses")
		})

		t.Run("reports a syntax refusal with the language's reason", func(t *testing.T) {
			t.Parallel()

			var r recorder
			toolchain.AssertParses(&r, scripted{parseErr: errors.New("row.s:1: bad token")}, output())
			assert.True(t, r.says("does not parse"), "the assertion names the class")
			assert.True(t, r.says("bad token"), "and carries the language's own reason")
		})
	})

	t.Run("AssertTypeChecks", func(t *testing.T) {
		t.Parallel()

		var r recorder
		toolchain.AssertTypeChecks(&r, scripted{}, output())
		assert.False(t, r.failed(), "the fixture type-checks")

		r = recorder{}
		toolchain.AssertTypeChecks(&r, scripted{typeErr: errors.New("undefined: Row")}, output())
		assert.True(t, r.says("does not type-check"), "the assertion names the class")
		assert.True(t, r.says("undefined: Row"), "and carries the reason, which is what catches a bad import")
	})

	t.Run("AssertTestsPass", func(t *testing.T) {
		t.Parallel()

		t.Run("passes a run whose every case passed", func(t *testing.T) {
			t.Parallel()

			var r recorder
			toolchain.AssertTestsPass(&r, scripted{report: toolchain.TestReport{Passed: 3}}, output())
			assert.False(t, r.failed(), "three passed and none failed")
		})

		t.Run("counts the failures and names them", func(t *testing.T) {
			t.Parallel()

			var r recorder
			toolchain.AssertTestsPass(&r, scripted{report: toolchain.TestReport{
				Passed: 2, Failed: 1, Output: "--- FAIL: TestRow",
			}}, output())
			assert.True(t, r.says("1 of 3 generated tests failed"), "the count reads off the report")
			assert.True(t, r.says("--- FAIL: TestRow"), "and the tool's own output follows")
		})

		t.Run("refuses a run that executed nothing", func(t *testing.T) {
			t.Parallel()

			var r recorder
			toolchain.AssertTestsPass(&r, scripted{report: toolchain.TestReport{Skipped: 4}}, output())
			assert.True(t, r.says("reported no case"),
				"a suite that executes nothing passes while proving nothing")
			assert.True(t, r.says("4 skipped"), "and the assertion says how many stood aside")
		})

		t.Run("reports a run the language could not start", func(t *testing.T) {
			t.Parallel()

			var r recorder
			toolchain.AssertTestsPass(&r, scripted{
				testsErr: errors.New("build failed"),
				report:   toolchain.TestReport{Output: "no packages"},
			}, output())
			assert.True(t, r.says("running the generated tests"), "the failure is the run's, not a case's")
			assert.True(t, r.says("no packages"), "with whatever the tool said")
		})
	})

	t.Run("satisfaction", func(t *testing.T) {
		t.Parallel()

		t.Run("holds a promise the output made", func(t *testing.T) {
			t.Parallel()

			var r recorder
			toolchain.AssertSatisfies(&r, scripted{satisfies: true}, output(), "Row", "Reader")
			assert.False(t, r.failed(), "the type satisfies the contract")

			r = recorder{}
			toolchain.AssertSatisfies(&r, scripted{}, output(), "Row", "Reader")
			assert.True(t, r.says("does not satisfy"), "and a broken promise reports")
			assert.True(t, r.says("promised it would"), "naming what the output claimed")
		})

		t.Run("holds a contract the output was not to widen to", func(t *testing.T) {
			t.Parallel()

			var r recorder
			toolchain.AssertDoesNotSatisfy(&r, scripted{}, output(), "Row", "Writer")
			assert.False(t, r.failed(), "the type does not satisfy the contract")

			r = recorder{}
			toolchain.AssertDoesNotSatisfy(&r, scripted{satisfies: true}, output(), "Row", "Writer")
			assert.True(t, r.says("was not to widen to"), "a widened contract reports")
		})

		t.Run("refuses an empty question and reports a broken one", func(t *testing.T) {
			t.Parallel()

			var r recorder
			toolchain.AssertSatisfies(&r, scripted{satisfies: true}, output(), "", "Reader")
			assert.True(t, r.says("names a type and a contract"), "a nameless type asks nothing")

			r = recorder{}
			toolchain.AssertSatisfies(&r, scripted{satisfies: true}, output(), "Row", "")
			assert.True(t, r.says("names a type and a contract"), "and neither does a nameless contract")

			r = recorder{}
			toolchain.AssertSatisfies(&r, scripted{satisfyErr: errors.New("no such type")}, output(), "Row", "Reader")
			assert.True(t, r.says("asking whether"), "a question the language could not answer reports as one")
		})
	})

	t.Run("every assertion refuses a fixture carrying nothing", func(t *testing.T) {
		t.Parallel()

		empty := toolchain.Generated{}
		for _, run := range []func(tb toolchain.TB){
			func(tb toolchain.TB) { toolchain.AssertParses(tb, scripted{}, empty) },
			func(tb toolchain.TB) { toolchain.AssertTypeChecks(tb, scripted{}, empty) },
			func(tb toolchain.TB) { toolchain.AssertTestsPass(tb, scripted{}, empty) },
			func(tb toolchain.TB) { toolchain.AssertSatisfies(tb, scripted{}, empty, "Row", "Reader") },
		} {
			var r recorder
			run(&r)
			assert.True(t, r.says("carries no generated output"),
				"a toolchain run over nothing passes while proving nothing")
		}
	})
}
