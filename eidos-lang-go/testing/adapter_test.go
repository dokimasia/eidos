// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/files"

	golang "go.dokimi.dev/eidos/lang/go"
	gotesting "go.dokimi.dev/eidos/lang/go/testing"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The fixture module and the files a healthy project holds.
const (
	fixtureModule = "eidos.test/harness"
	rowFile       = "row.go"
	rowTestFile   = "row_test.go"
)

// lang is the prefix of every contract the toolchain assertions state
// for the Go adapter.
const lang = string(golang.Lang)

// seat is the toolchain gate's seat over a recorder: it records each
// assertion's failure and the reason of a skip, so a case reads both
// without failing the run.
type seat struct {
	*assert.Recorder
	skipped string
}

// Skip records the skip's reason.
func (s *seat) Skip(args ...any) { s.skipped = fmt.Sprint(args...) }

// The kernel's assertions drive the adapter, so the project it lays
// out and the meaning of each result are its contract. The layout
// and the parse need no toolchain, and the cases that run one skip
// locally where it is absent.
func TestAdapter(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an adapter that meets the toolchain contract over a healthy fixture", func(t *testing.T) {
			t.Parallel()

			toolchain.RunToolchainSuite(t, func(toolchain.TB) (toolchain.Adapter, toolchain.Generated) {
				return adapter(), healthy()
			})
		})
	})

	t.Run("Lang", func(t *testing.T) {
		t.Parallel()

		t.Run("returns Go", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, adapter().Lang(), golang.Lang, "the Go adapter names Go")
		})
	})

	t.Run("Available", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the reason the go binary is unavailable", func(t *testing.T) {
			t.Parallel()

			held, why := adapter().Available()
			if held {
				assert.Equal(t, why, "", "a toolchain that is there states no reason")
				return
			}
			assert.Contains(t, why, "go is not on PATH", "and one that is not names the binary")
		})
	})

	t.Run("Layout", func(t *testing.T) {
		t.Parallel()

		t.Run("writes every file beside a go.mod declaring the fixture's module", func(t *testing.T) {
			t.Parallel()

			dir := laidOut(t, healthy())
			assert.Contains(t, files.Read(t, filepath.Join(dir, rowFile)), "type Row struct", "with its own bytes")
			assert.Contains(t, files.Read(t, filepath.Join(dir, "go.mod")), "module "+fixtureModule,
				"declaring the module the fixture named, so the output resolves its own imports")
		})

		t.Run("writes a fixture naming no module under the default module", func(t *testing.T) {
			t.Parallel()

			g := only("gen/deep/row.go", "package deep\n")
			g.Module = ""
			dir := laidOut(t, g)
			files.IsFile(t, filepath.Join(dir, "gen", "deep", "row.go"), "the nested path is created")
			assert.Contains(t, files.Read(t, filepath.Join(dir, "go.mod")), "module eidos.test/generated",
				"because a project without one resolves nothing")
		})

		t.Run("keeps a go.mod the output wrote", func(t *testing.T) {
			t.Parallel()

			dir := laidOut(t, with("go.mod", "module carried.test/own\n\ngo 1.27.0\n"))
			files.HasContent(t, filepath.Join(dir, "go.mod"), "module carried.test/own\n\ngo 1.27.0\n",
				"the output's own module is kept, because the harness states nothing the output already did")
		})

		t.Run("declares a root module beside a nested one", func(t *testing.T) {
			t.Parallel()

			dir := laidOut(t, with("api/go.mod", "module nested.test/api\n\ngo 1.27.0\n"))
			assert.Contains(t, files.Read(t, filepath.Join(dir, "go.mod")), "module "+fixtureModule,
				"because a nested go.mod declares only its own subtree")
		})

		t.Run("returns an error for a path that climbs out of the scratch project", func(t *testing.T) {
			t.Parallel()

			_, err := adapter().Layout(only("../escape.go", "package escape\n"))
			assert.HasError(t, err, "a fixture is not a place to write from")
			assert.Contains(t, err.Error(), "climbs out", "and the refusal names the escape")
		})
	})

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nil for a healthy project", func(t *testing.T) {
			t.Parallel()

			assert.NoError(t, adapter().Parse(t.Context(), laidOut(t, healthy())),
				"the healthy output parses without a toolchain")
		})

		t.Run("returns an error naming the file of a syntax error", func(t *testing.T) {
			t.Parallel()

			err := adapter().Parse(t.Context(), laidOut(t, only(rowFile, "package harness\n\nfunc F( {}\n")))
			assert.HasError(t, err, "a syntax error refuses")
			assert.Contains(t, err.Error(), rowFile, "naming the file, relative to the project")
		})

		t.Run("returns an error for a project without a Go file", func(t *testing.T) {
			t.Parallel()

			err := adapter().Parse(t.Context(), laidOut(t, only("row.golang", "package harness\n")))
			assert.HasError(t, err, "a parse of no Go file refuses, because it proves nothing")
			assert.Contains(t, err.Error(), "no Go file", "and the refusal names why")
		})

		t.Run("returns the context's error for a context that ended", func(t *testing.T) {
			t.Parallel()

			dir := laidOut(t, healthy())
			assert.HonoursCancellation(t, func(ctx context.Context) error { return adapter().Parse(ctx, dir) },
				"an ended context parses no file")
		})
	})

	t.Run("TypeCheck", func(t *testing.T) {
		t.Parallel()
		requireGo(t)

		t.Run("returns nil for a healthy project", func(t *testing.T) {
			t.Parallel()

			toolchain.AssertTypeChecks(t.Context(), t, adapter(), healthy())
		})

		t.Run("returns the compiler's error for a type error", func(t *testing.T) {
			t.Parallel()

			failure := rejection(t, func(tb assert.TB) {
				toolchain.AssertTypeChecks(t.Context(), tb, adapter(),
					only(rowFile, "package harness\n\nvar X int = \"text\"\n"))
			})
			assert.Equal(t, failure.Contract, lang+": the generated output type-checks", "the class is named")
			assert.Contains(t, reason(t, failure), "cannot use", "in the compiler's own words")
		})

		t.Run("returns an error for a type error in a generated test", func(t *testing.T) {
			t.Parallel()

			failure := rejection(t, func(tb assert.TB) {
				toolchain.AssertTypeChecks(t.Context(), tb, adapter(),
					with(rowTestFile, "package harness\n\nvar _ int = \"text\"\n"))
			})
			assert.Equal(t, failure.Contract, lang+": the generated output type-checks",
				"a test file is compiled too, where most generated checks are")
		})

		t.Run("returns the context's error for a context that ended", func(t *testing.T) {
			t.Parallel()

			dir := laidOut(t, healthy())
			assert.HonoursCancellation(t, func(ctx context.Context) error { return adapter().TypeCheck(ctx, dir) },
				"an ended context starts no go command")
		})
	})

	t.Run("RunTests", func(t *testing.T) {
		t.Parallel()
		requireGo(t)

		t.Run("returns a passing report for a healthy project", func(t *testing.T) {
			t.Parallel()

			toolchain.AssertTestsPass(t.Context(), t, adapter(), healthy())
		})

		t.Run("returns a report of a failing generated test", func(t *testing.T) {
			t.Parallel()

			failure := rejection(t, func(tb assert.TB) {
				toolchain.AssertTestsPass(t.Context(), tb, adapter(), with(rowTestFile,
					"package harness\n\nimport \"testing\"\n\n"+
						"func TestRow(t *testing.T) { t.Fatal(\"the generated check disagrees\") }\n"))
			})
			assert.HasPrefix(t, failure.Contract, lang+": every generated test passes, of 1\n",
				"the count reads off the run")
			assert.Contains(t, failure.Contract, "the generated check disagrees", "and the case's own words follow")
		})

		t.Run("returns a report of no case for a project without tests", func(t *testing.T) {
			t.Parallel()

			failure := rejection(t, func(tb assert.TB) {
				toolchain.AssertTestsPass(t.Context(), tb, adapter(),
					only(rowFile, "package harness\n\ntype Row struct{}\n"))
			})
			assert.HasPrefix(t, failure.Contract, lang+": the generated tests report a case, and 0 skipped",
				"generated tests that run nothing pass while proving nothing")
		})

		t.Run("returns the context's error for a context that ended", func(t *testing.T) {
			t.Parallel()

			dir := laidOut(t, healthy())
			assert.HonoursCancellation(t, func(ctx context.Context) error {
				_, err := adapter().RunTests(ctx, dir)
				return err
			}, "an ended context runs no test")
		})
	})

	t.Run("Satisfies", func(t *testing.T) {
		t.Parallel()
		requireGo(t)

		t.Run("reports true for a type that implements the contract", func(t *testing.T) {
			t.Parallel()

			toolchain.AssertSatisfies(t.Context(), t, adapter(), healthy(), "*Row", "Reader")
		})

		t.Run("reports false for a value whose method has a pointer receiver", func(t *testing.T) {
			t.Parallel()

			toolchain.AssertDoesNotSatisfy(t.Context(), t, adapter(), healthy(), "Row", "Reader")
		})

		t.Run("reports false for a contract the type does not implement", func(t *testing.T) {
			t.Parallel()

			toolchain.AssertDoesNotSatisfy(t.Context(), t, adapter(), healthy(), "*Row", "error")
			failure := rejection(t, func(tb assert.TB) {
				toolchain.AssertSatisfies(t.Context(), tb, adapter(), healthy(), "*Row", "error")
			})
			assert.Equal(t, failure.Contract, lang+": *Row satisfies error, as the generated output promised",
				"a contract the output does not satisfy reports")
		})

		t.Run("imports the package of a qualified contract", func(t *testing.T) {
			t.Parallel()

			g := with("stringer.go",
				"package harness\n\n// String names the row.\nfunc (r Row) String() string { return r.Name }\n")
			toolchain.AssertSatisfies(t.Context(), t, adapter(), g, "Row", "fmt.Stringer")
			toolchain.AssertDoesNotSatisfy(t.Context(), t, adapter(), g, "Row", "io.Reader")
		})

		t.Run("returns an error for a contract the project does not declare", func(t *testing.T) {
			t.Parallel()

			failure := rejection(t, func(tb assert.TB) {
				toolchain.AssertDoesNotSatisfy(t.Context(), tb, adapter(), healthy(), "Row", "Nowhere")
			})
			assert.Equal(t, failure.Contract, lang+": the toolchain answers whether Row satisfies Nowhere",
				"an undefined contract is a broken question, not a false answer")
			assert.Contains(t, reason(t, failure), "for a reason other than the contract", "and the reason says so")
		})

		t.Run("probes from the package beside an external test file", func(t *testing.T) {
			t.Parallel()

			toolchain.AssertSatisfies(t.Context(), t, adapter(), with("a_test.go", "package harness_test\n"),
				"*Row", "Reader")
		})

		t.Run("returns an error for a project that does not build", func(t *testing.T) {
			t.Parallel()

			failure := rejection(t, func(tb assert.TB) {
				toolchain.AssertSatisfies(t.Context(), tb, adapter(),
					only(rowFile, "package harness\n\nvar X int = \"text\"\n"), "Row", "Reader")
			})
			assert.Equal(t, failure.Contract, lang+": the toolchain answers whether Row satisfies Reader",
				"a broken project leaves the question without an answer")
			assert.Contains(
				t,
				reason(t, failure),
				"does not build",
				"a false answer is told apart from a broken project",
			)
		})

		t.Run("returns the context's error for a context that ended", func(t *testing.T) {
			t.Parallel()

			dir := laidOut(t, healthy())
			assert.HonoursCancellation(t, func(ctx context.Context) error {
				_, err := adapter().Satisfies(ctx, dir, "*Row", "Reader")
				return err
			}, "an ended context asks the compiler nothing")
		})
	})
}

// requireGo skips the calling test where the Go toolchain is absent.
// CI requires the toolchain, so the skip happens only locally.
func requireGo(t *testing.T) {
	t.Helper()

	probe := &seat{Recorder: assert.NewRecorder()}
	if !toolchain.Require(probe, adapter()) {
		t.Skip("the Go toolchain is absent, so the toolchain cases are skipped here and required in CI: " +
			probe.skipped)
	}
}

// rejection runs check against a seat that must refuse it, and returns
// the one record of the refusal.
func rejection(t *testing.T, check func(tb assert.TB)) assert.Failure {
	t.Helper()

	got := assert.Rejects(t, "the check refuses the output", check)
	assert.Length(t, got, 1, "the check fails once")
	return got[0]
}

// reason returns the text of what a record states as got: the error
// the toolchain returned.
func reason(t *testing.T, failure assert.Failure) string {
	t.Helper()

	got, stated := failure.Got()
	assert.True(t, stated, "the record states what the toolchain returned")
	return fmt.Sprint(got)
}

// laidOut lays the fixture out through the adapter, and removes the
// project when the test ends.
func laidOut(t *testing.T, g toolchain.Generated) string {
	t.Helper()

	dir, err := adapter().Layout(g)
	assert.NoError(t, err, "the fixture lays out")
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// healthy returns generated output that parses, type-checks, vets
// and whose one test passes.
func healthy() toolchain.Generated {
	return toolchain.Generated{
		Module: fixtureModule,
		Files: map[string][]byte{
			rowFile: []byte(`package harness

// Row is a generated record.
type Row struct {
	ID   int
	Name string
}

// Reader is what a Row satisfies.
type Reader interface {
	Read() string
}

// Read returns the row's name.
func (r *Row) Read() string { return r.Name }
`),
			rowTestFile: []byte(`package harness

import "testing"

func TestRow(t *testing.T) {
	r := &Row{ID: 1, Name: "a"}
	if got := r.Read(); got != "a" {
		t.Fatalf("Read() = %q, want a", got)
	}
}
`),
		},
	}
}

// with returns the healthy fixture with one file replaced or added, for
// a case that breaks exactly one thing.
func with(path, body string) toolchain.Generated {
	g := healthy()
	g.Files[path] = []byte(body)
	return g
}

// only returns a fixture of one file, for a case that needs no test
// file.
func only(path, body string) toolchain.Generated {
	return toolchain.Generated{
		Module: fixtureModule,
		Files:  map[string][]byte{path: []byte(body)},
	}
}

// adapter is the harness under test.
func adapter() toolchain.Adapter { return gotesting.New() }
