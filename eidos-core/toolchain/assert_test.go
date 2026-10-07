// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package toolchain_test

import (
	"context"
	"errors"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/toolchain"
)

// The contracts the assertions state, each under the fixture's
// language.
const (
	parses     = "scripted: the generated output parses"
	typeChecks = "scripted: the generated output type-checks"
	carries    = "scripted: the fixture carries generated output, without which a toolchain run proves nothing"
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

			toolchain.AssertParses(t.Context(), t, scripted{}, output())
		})

		t.Run("records a syntax refusal with the language's reason", func(t *testing.T) {
			t.Parallel()

			refusal := errors.New("row.s:1: bad token")
			got := assert.Rejects(t, "output the language does not read fails the check", func(tb assert.TB) {
				toolchain.AssertParses(t.Context(), tb, scripted{parseErr: refusal}, output())
			})
			assert.Equal(t, coretest.Contracts(got), []string{parses}, "the check fails for the syntax")
			reason, stated := got[0].Got()
			assert.True(t, stated, "the record states the error")
			assert.Equal(t, reason, any(refusal), "which is the language's own reason")
		})
	})

	t.Run("AssertTypeChecks", func(t *testing.T) {
		t.Parallel()

		t.Run("passes output the language accepts", func(t *testing.T) {
			t.Parallel()

			toolchain.AssertTypeChecks(t.Context(), t, scripted{}, output())
		})

		t.Run("records a type error with the language's reason", func(t *testing.T) {
			t.Parallel()

			refusal := errors.New("undefined: Row")
			got := assert.Rejects(t, "output the language does not type-check fails the check", func(tb assert.TB) {
				toolchain.AssertTypeChecks(t.Context(), tb, scripted{typeErr: refusal}, output())
			})
			assert.Equal(t, coretest.Contracts(got), []string{typeChecks}, "the check fails for the types")
			reason, stated := got[0].Got()
			assert.True(t, stated, "the record states the error")
			assert.Equal(t, reason, any(refusal), "which is what catches a bad import")
		})
	})

	t.Run("AssertTestsPass", func(t *testing.T) {
		t.Parallel()

		t.Run("passes a run whose every case passed", func(t *testing.T) {
			t.Parallel()

			toolchain.AssertTestsPass(t.Context(), t, scripted{report: toolchain.TestReport{Passed: 3}}, output())
		})

		t.Run("records the failed count beside the tool's output", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "a run with a failed case fails the check", func(tb assert.TB) {
				toolchain.AssertTestsPass(t.Context(), tb, scripted{report: toolchain.TestReport{
					Passed: 2, Failed: 1, Output: "--- FAIL: TestRow",
				}}, output())
			})
			assert.Equal(t, coretest.Contracts(got),
				[]string{"scripted: every generated test passes, of 3\n--- FAIL: TestRow"},
				"the contract counts the cases and carries the tool's output")
			failed, stated := got[0].Got()
			assert.True(t, stated, "the record states the count")
			assert.Equal(t, failed, any(1), "of the failed cases")
		})

		t.Run("records a run that executed nothing", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "a run of no case fails the check", func(tb assert.TB) {
				toolchain.AssertTestsPass(t.Context(), tb, scripted{report: toolchain.TestReport{Skipped: 4}}, output())
			})
			assert.Equal(t, coretest.Contracts(got),
				[]string{"scripted: the generated tests report a case, and 4 skipped: " +
					"a suite that executes nothing passes while proving nothing\n"},
				"a suite that executes nothing passes while proving nothing, and the contract counts the skips")
		})

		t.Run("records a run the language could not start", func(t *testing.T) {
			t.Parallel()

			refusal := errors.New("build failed")
			got := assert.Rejects(t, "a run that does not start fails the check", func(tb assert.TB) {
				toolchain.AssertTestsPass(t.Context(), tb, scripted{
					testsErr: refusal,
					report:   toolchain.TestReport{Output: "no packages"},
				}, output())
			})
			assert.Equal(t, coretest.Contracts(got), []string{"scripted: the generated tests run\nno packages"},
				"the failure is the run's, not a case's, with whatever the tool said")
			reason, stated := got[0].Got()
			assert.True(t, stated, "the record states the error")
			assert.Equal(t, reason, any(refusal), "which is the language's own reason")
		})
	})

	t.Run("AssertSatisfies", func(t *testing.T) {
		t.Parallel()

		t.Run("passes a type that meets the promised contract", func(t *testing.T) {
			t.Parallel()

			toolchain.AssertSatisfies(t.Context(), t, scripted{satisfies: true}, output(), "Row", "Reader")
		})

		t.Run("records a type that breaks the promised contract", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "a broken promise fails the check", func(tb assert.TB) {
				toolchain.AssertSatisfies(t.Context(), tb, scripted{}, output(), "Row", "Reader")
			})
			assert.Equal(t, coretest.Contracts(got),
				[]string{"scripted: Row satisfies Reader, as the generated output promised"},
				"the contract names what the output claimed")
		})

		questions := []struct {
			name     string
			adapter  scripted
			typeName string
			contract string
			want     string
		}{
			{
				name:    "records a question without a type",
				adapter: scripted{satisfies: true}, contract: "Reader",
				want: "toolchain: a satisfaction check names a type",
			},
			{
				name:    "records a question without a contract",
				adapter: scripted{satisfies: true}, typeName: "Row",
				want: "toolchain: a satisfaction check names a contract",
			},
			{
				name:    "records a question the language could not answer",
				adapter: scripted{satisfyErr: errors.New("no such type")}, typeName: "Row", contract: "Reader",
				want: "scripted: the toolchain answers whether Row satisfies Reader",
			},
		}
		for _, tt := range questions {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got := assert.Rejects(t, "a question without an answer fails the check", func(tb assert.TB) {
					toolchain.AssertSatisfies(t.Context(), tb, tt.adapter, output(), tt.typeName, tt.contract)
				})
				assert.Equal(t, coretest.Contracts(got), []string{tt.want}, "the check names what the question lacks")
			})
		}
	})

	t.Run("AssertDoesNotSatisfy", func(t *testing.T) {
		t.Parallel()

		t.Run("passes a type that stays clear of the contract", func(t *testing.T) {
			t.Parallel()

			toolchain.AssertDoesNotSatisfy(t.Context(), t, scripted{}, output(), "Row", "Writer")
		})

		t.Run("records a type widened to the contract", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "a widened contract fails the check", func(tb assert.TB) {
				toolchain.AssertDoesNotSatisfy(t.Context(), tb, scripted{satisfies: true}, output(), "Row", "Writer")
			})
			assert.Equal(t, coretest.Contracts(got),
				[]string{"scripted: Row does not satisfy Writer, which the generated output was not to widen to"},
				"the contract names the widening")
		})
	})

	t.Run("records a fixture carrying nothing in every assertion", func(t *testing.T) {
		t.Parallel()

		empty := toolchain.Generated{}
		for _, run := range []func(tb assert.TB){
			func(tb assert.TB) { toolchain.AssertParses(t.Context(), tb, scripted{}, empty) },
			func(tb assert.TB) { toolchain.AssertTypeChecks(t.Context(), tb, scripted{}, empty) },
			func(tb assert.TB) { toolchain.AssertTestsPass(t.Context(), tb, scripted{}, empty) },
			func(tb assert.TB) { toolchain.AssertSatisfies(t.Context(), tb, scripted{}, empty, "Row", "Reader") },
		} {
			got := assert.Rejects(t, "a fixture carrying nothing fails the check", run)
			expect.Equal(t, coretest.Contracts(got), []string{carries},
				"a toolchain run over nothing passes while proving nothing")
		}
	})

	t.Run("records the error of a context that ended in every assertion", func(t *testing.T) {
		t.Parallel()

		ended, cancel := context.WithCancel(t.Context())
		cancel()
		for _, run := range []func(tb assert.TB){
			func(tb assert.TB) { toolchain.AssertParses(ended, tb, scripted{}, output()) },
			func(tb assert.TB) { toolchain.AssertTypeChecks(ended, tb, scripted{}, output()) },
			func(tb assert.TB) { toolchain.AssertTestsPass(ended, tb, scripted{}, output()) },
			func(tb assert.TB) { toolchain.AssertSatisfies(ended, tb, scripted{}, output(), "Row", "Reader") },
			func(tb assert.TB) { toolchain.AssertDoesNotSatisfy(ended, tb, scripted{}, output(), "Row", "Reader") },
		} {
			got := assert.Rejects(t, "a run under a context that ended fails the check", run)
			assert.Length(t, got, 1, "the run's error is the one failure")
			reason, stated := got[0].Got()
			expect.True(t, stated, "the record states the error")
			expect.Equal(t, reason, any(context.Canceled), "which is the context's own, so the adapter ran under it")
		}
	})
}
