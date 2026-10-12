// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing_test

import (
	"context"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	golang "go.dokimi.dev/eidos/lang/go"
	gotesting "go.dokimi.dev/eidos/lang/go/testing"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// These constants are the test files of the parallel cases: a suite whose
// tests and subtests call Parallel, a test that does not, a subtest that
// does not, a test that calls Parallel inside a nested function literal
// alone, and a file that does not parse.
const (
	parallelSuite = `package harness

import "testing"

func TestRow(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"a", "b"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
		})
	}
	t.Run("reads the name", func(t *testing.T) {
		t.Parallel()
	})
}

func TestMain(m *testing.M) { m.Run() }
`
	serialTest = `package harness

import "testing"

func TestRow(t *testing.T) {
	_ = t
}
`
	serialSubtest = `package harness

import "testing"

func TestRow(t *testing.T) {
	t.Parallel()

	t.Run("reads the name", func(t *testing.T) {
		_ = t
	})
}
`
	deferredParallel = `package harness

import "testing"

func TestRow(t *testing.T) {
	defer func() { t.Parallel() }()
}
`
	brokenTest = "package harness\n\nfunc TestRow(t *testing.T) {\n"
)

// These constants are the contracts of the parallel cases.
const (
	serialRow   = lang + ": the generated test TestRow at row_test.go:5:28 calls t.Parallel"
	serialName  = lang + ": the generated test TestRow/reads the name at row_test.go:8:45 calls t.Parallel"
	declaresOne = lang + ": the generated output declares a test"
)

// Vet and the parallel suite are the assertions of Go alone. What each
// passes and what each reports are contract for every generator that the
// harness checks.
func TestAssert(t *testing.T) {
	t.Parallel()

	t.Run("AssertVets", func(t *testing.T) {
		t.Parallel()

		t.Run("fails a fixture without output", func(t *testing.T) {
			t.Parallel()

			failure := rejection(t, func(tb assert.TB) {
				gotesting.AssertVets(t.Context(), tb, toolchain.Generated{})
			})
			want := string(golang.Lang) +
				": the fixture has generated output, without which a toolchain run proves nothing"
			assert.Equal(t, failure.Contract, want, "a vet over nothing passes while proving nothing")
		})

		t.Run("passes output that vets clean", func(t *testing.T) {
			t.Parallel()
			requireGo(t)

			gotesting.AssertVets(t.Context(), t, healthy())
		})

		t.Run("fails compiling output that go vet reports on", func(t *testing.T) {
			t.Parallel()
			requireGo(t)

			failure := rejection(t, func(tb assert.TB) {
				gotesting.AssertVets(t.Context(), tb, only(rowFile,
					"package harness\n\nimport \"fmt\"\n\n"+
						"// Print misuses its verb.\nfunc Print() string { return fmt.Sprintf(\"%d\", \"text\") }\n"))
			})
			assert.Equal(t, failure.Contract, string(golang.Lang)+": go vet accepts the generated output",
				"the wrong verb is caught")
			assert.Contains(t, reason(t, failure), "wrong type", "the reason is the text of go vet")
		})

		t.Run("fails the run of a context that ended", func(t *testing.T) {
			t.Parallel()

			ended, cancel := context.WithCancel(t.Context())
			cancel()
			failure := rejection(t, func(tb assert.TB) { gotesting.AssertVets(ended, tb, healthy()) })
			got, stated := failure.Got()
			assert.True(t, stated, "the record has the error of the run")
			err, isErr := got.(error)
			assert.True(t, isErr, "the value is an error")
			assert.ErrorIs(t, err, context.Canceled, "the error wraps the error of the context")
		})
	})

	t.Run("AssertParallel", func(t *testing.T) {
		t.Parallel()

		t.Run("passes tests and subtests that call Parallel", func(t *testing.T) {
			t.Parallel()

			gotesting.AssertParallel(t, with(rowTestFile, parallelSuite))
		})

		tests := []struct {
			name string
			give string
			want string
		}{
			{name: "fails a test that does not call Parallel", give: serialTest, want: serialRow},
			{name: "fails a subtest that does not call Parallel", give: serialSubtest, want: serialName},
			{
				name: "fails a test that calls Parallel inside a nested function literal",
				give: deferredParallel, want: serialRow,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				failure := rejection(t, func(tb assert.TB) {
					gotesting.AssertParallel(tb, with(rowTestFile, tt.give))
				})
				assert.Equal(t, failure.Contract, tt.want, "the check names the test and its position")
			})
		}

		t.Run("fails a fixture without a test", func(t *testing.T) {
			t.Parallel()

			failure := rejection(t, func(tb assert.TB) {
				gotesting.AssertParallel(tb, only(rowFile, "package harness\n"))
			})
			assert.Equal(t, failure.Contract, declaresOne, "a check over no test proves nothing")
		})

		t.Run("fails a test file that does not parse", func(t *testing.T) {
			t.Parallel()

			got := assert.Rejects(t, "a test file that does not parse fails the check", func(tb assert.TB) {
				gotesting.AssertParallel(tb, only(rowTestFile, brokenTest))
			})
			assert.Length(t, got, 2, "the check fails twice")
			expect.Equal(t, got[0].Contract, lang+": the generated test file row_test.go parses",
				"the first failure names the file")
			expect.Equal(t, got[1].Contract, declaresOne, "the second failure finds no test")
		})
	})
}
