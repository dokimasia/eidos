// Copyright Dokimasia B.V. 2026
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

// These constants are the contracts of the assertions, each under the
// language of the fixture.
const (
	parses     = "scripted: the generated output parses"
	typeChecks = "scripted: the generated output type-checks"
	hasOutput  = "scripted: the fixture has generated output, without which a toolchain run proves nothing"
)

// listed is the listing of the fixture's one generated file that ends the
// contract of a failed toolchain run.
const listed = "\n" + generatedFile + ":\n1 | row\n"

// The kit declares the assertion set once for every language, so what
// each assertion passes and refuses is contract for all of them.
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
			assert.Equal(t, coretest.Contracts(got), []string{parses + listed}, "the check fails for the syntax")
			reason, stated := got[0].Got()
			assert.True(t, stated, "the record has the error")
			assert.Equal(t, reason, any(refusal), "the error is the reason of the language")
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
			assert.Equal(t, coretest.Contracts(got), []string{typeChecks + listed}, "the check fails for the types")
			reason, stated := got[0].Got()
			assert.True(t, stated, "the record has the error")
			assert.Equal(t, reason, any(refusal), "the error is the reason of the language")
		})

		t.Run("ends the contract with each generated file behind its line numbers", func(t *testing.T) {
			t.Parallel()

			g := toolchain.Generated{Files: map[string][]byte{
				"b.s": []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"),
				"a.s": []byte("only"),
			}}
			got := assert.Rejects(t, "output the language does not type-check fails the check", func(tb assert.TB) {
				toolchain.AssertTypeChecks(t.Context(), tb, scripted{typeErr: errors.New("undefined: Row")}, g)
			})
			want := typeChecks + "\na.s:\n1 | only\n" +
				"\nb.s:\n 1 | 1\n 2 | 2\n 3 | 3\n 4 | 4\n 5 | 5\n 6 | 6\n 7 | 7\n 8 | 8\n 9 | 9\n10 | 10\n"
			assert.Equal(t, coretest.Contracts(got), []string{want},
				"the files are in path order, and each line number has the width of the largest")
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
				[]string{"scripted: every generated test passes, of 3\n--- FAIL: TestRow" + listed},
				"the contract contains the count of the cases and the output of the tool")
			failed, stated := got[0].Got()
			assert.True(t, stated, "the record has the count")
			assert.Equal(t, failed, any(1), "the count is the number of failed cases")
		})

		t.Run("records a run that executed nothing", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "a run of no case fails the check", func(tb assert.TB) {
				toolchain.AssertTestsPass(t.Context(), tb, scripted{report: toolchain.TestReport{Skipped: 4}}, output())
			})
			assert.Equal(t, coretest.Contracts(got),
				[]string{"scripted: the generated tests report a case, and 4 skipped: " +
					"a suite that executes nothing passes while proving nothing\n" + listed},
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
			assert.Equal(t, coretest.Contracts(got),
				[]string{"scripted: the generated tests run\nno packages" + listed},
				"the failure is the failure of the run, with the output of the tool")
			reason, stated := got[0].Got()
			assert.True(t, stated, "the record has the error")
			assert.Equal(t, reason, any(refusal), "the error is the reason of the language")
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
				[]string{"scripted: Row satisfies Reader, as the generated output promised" + listed},
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
				want: "scripted: the toolchain answers whether Row satisfies Reader" + listed,
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

		t.Run("passes a type that does not satisfy the contract", func(t *testing.T) {
			t.Parallel()

			toolchain.AssertDoesNotSatisfy(t.Context(), t, scripted{}, output(), "Row", "Writer")
		})

		t.Run("records a type widened to the contract", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "a widened contract fails the check", func(tb assert.TB) {
				toolchain.AssertDoesNotSatisfy(t.Context(), tb, scripted{satisfies: true}, output(), "Row", "Writer")
			})
			want := "scripted: Row does not satisfy Writer, which the generated output was not to widen to" + listed
			assert.Equal(t, coretest.Contracts(got), []string{want}, "the contract names the widening")
		})
	})

	t.Run("records an empty fixture in every assertion", func(t *testing.T) {
		t.Parallel()

		empty := toolchain.Generated{}
		for _, run := range []func(tb assert.TB){
			func(tb assert.TB) { toolchain.AssertParses(t.Context(), tb, scripted{}, empty) },
			func(tb assert.TB) { toolchain.AssertTypeChecks(t.Context(), tb, scripted{}, empty) },
			func(tb assert.TB) { toolchain.AssertTestsPass(t.Context(), tb, scripted{}, empty) },
			func(tb assert.TB) { toolchain.AssertSatisfies(t.Context(), tb, scripted{}, empty, "Row", "Reader") },
		} {
			got := assert.Rejects(t, "an empty fixture fails the check", run)
			expect.Equal(t, coretest.Contracts(got), []string{hasOutput},
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
			expect.True(t, stated, "the record has the error")
			expect.Equal(t, reason, any(context.Canceled),
				"the error is the error of the context, so the adapter ran under it")
		}
	})
}
