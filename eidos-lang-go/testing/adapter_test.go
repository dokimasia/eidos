// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.dokimi.dev/assert"

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

// recorder is a test handle collecting what an assertion reported,
// so a case reads the outcome rather than failing the run.
type recorder struct {
	failures []string
	skipped  string
}

// Helper marks nothing: the recorder reports no line of its own.
func (*recorder) Helper() {}

// Errorf records one failure.
func (r *recorder) Errorf(format string, args ...any) {
	r.failures = append(r.failures, fmt.Sprintf(format, args...))
}

// Fatalf records one failure, and returns.
func (r *recorder) Fatalf(format string, args ...any) {
	r.failures = append(r.failures, fmt.Sprintf(format, args...))
}

// Skip records the skip's reason.
func (r *recorder) Skip(args ...any) { r.skipped = fmt.Sprint(args...) }

// failed reports whether anything was recorded.
func (r *recorder) failed() bool { return len(r.failures) > 0 }

// says reports whether any recorded failure mentions text.
func (r *recorder) says(text string) bool {
	for _, f := range r.failures {
		if strings.Contains(f, text) {
			return true
		}
	}
	return false
}

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

			dir, err := adapter().Layout(healthy())
			assert.NoError(t, err, "the fixture lays out")
			t.Cleanup(func() { _ = os.RemoveAll(dir) })

			body, err := os.ReadFile(filepath.Join(dir, rowFile))
			assert.NoError(t, err, "the generated file is on disk")
			assert.Contains(t, string(body), "type Row struct", "with its own bytes")
			mod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
			assert.NoError(t, err, "and a go.mod beside it")
			assert.Contains(t, string(mod), "module "+fixtureModule,
				"declaring the module the fixture named, so the output resolves its own imports")
		})

		t.Run("writes a fixture naming no module under the default module", func(t *testing.T) {
			t.Parallel()

			g := only("gen/deep/row.go", "package deep\n")
			g.Module = ""
			dir, err := adapter().Layout(g)
			assert.NoError(t, err, "the fixture lays out")
			t.Cleanup(func() { _ = os.RemoveAll(dir) })

			_, err = os.Stat(filepath.Join(dir, "gen", "deep", "row.go"))
			assert.NoError(t, err, "the nested path is created")
			mod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
			assert.NoError(t, err, "a module is declared anyway")
			assert.Contains(t, string(mod), "module eidos.test/generated",
				"because a project without one resolves nothing")
		})

		t.Run("keeps a go.mod the output wrote", func(t *testing.T) {
			t.Parallel()

			g := with("go.mod", "module carried.test/own\n\ngo 1.27.0\n")
			dir, err := adapter().Layout(g)
			assert.NoError(t, err, "the fixture lays out")
			t.Cleanup(func() { _ = os.RemoveAll(dir) })

			mod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
			assert.NoError(t, err, "the output's go.mod is on disk")
			assert.Contains(t, string(mod), "carried.test/own",
				"the output's own module is kept, because the harness states nothing the output already did")
		})

		t.Run("declares a root module beside a nested one", func(t *testing.T) {
			t.Parallel()

			g := with("api/go.mod", "module nested.test/api\n\ngo 1.27.0\n")
			dir, err := adapter().Layout(g)
			assert.NoError(t, err, "the fixture lays out")
			t.Cleanup(func() { _ = os.RemoveAll(dir) })

			mod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
			assert.NoError(t, err, "the root still declares a module")
			assert.Contains(t, string(mod), "module ", "because a nested go.mod declares only its own subtree")
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

			a := adapter()
			sound, err := a.Layout(healthy())
			assert.NoError(t, err, "the fixture lays out")
			t.Cleanup(func() { _ = os.RemoveAll(sound) })
			assert.NoError(t, a.Parse(sound), "the healthy output parses without a toolchain")
		})

		t.Run("returns an error naming the file of a syntax error", func(t *testing.T) {
			t.Parallel()

			a := adapter()
			unparsable, err := a.Layout(only(rowFile, "package harness\n\nfunc F( {}\n"))
			assert.NoError(t, err, "the broken fixture lays out")
			t.Cleanup(func() { _ = os.RemoveAll(unparsable) })
			err = a.Parse(unparsable)
			assert.HasError(t, err, "a syntax error refuses")
			assert.Contains(t, err.Error(), rowFile, "naming the file, relative to the project")
		})

		t.Run("returns an error for a project without a Go file", func(t *testing.T) {
			t.Parallel()

			a := adapter()
			bare, err := a.Layout(only("row.golang", "package harness\n"))
			assert.NoError(t, err, "a fixture with no Go file lays out")
			t.Cleanup(func() { _ = os.RemoveAll(bare) })
			err = a.Parse(bare)
			assert.HasError(t, err, "a parse of no Go file refuses, because it proves nothing")
			assert.Contains(t, err.Error(), "no Go file", "and the refusal names why")
		})
	})

	t.Run("TypeCheck", func(t *testing.T) {
		t.Parallel()
		requireGo(t)

		t.Run("returns nil for a healthy project", func(t *testing.T) {
			t.Parallel()

			var r recorder
			toolchain.AssertTypeChecks(&r, adapter(), healthy())
			assert.False(t, r.failed(), "the generated output type-checks")
		})

		t.Run("returns the compiler's error for a type error", func(t *testing.T) {
			t.Parallel()

			var r recorder
			toolchain.AssertTypeChecks(&r, adapter(),
				only(rowFile, "package harness\n\nvar X int = \"text\"\n"))
			assert.True(t, r.says("does not type-check"), "the class is named")
			assert.True(t, r.says("cannot use"), "and the compiler's own words follow")
		})

		t.Run("returns an error for a type error in a generated test", func(t *testing.T) {
			t.Parallel()

			var r recorder
			toolchain.AssertTypeChecks(&r, adapter(), with(rowTestFile,
				"package harness\n\nvar _ int = \"text\"\n"))
			assert.True(t, r.says("does not type-check"),
				"a test file is compiled too, where most generated checks are")
		})
	})

	t.Run("RunTests", func(t *testing.T) {
		t.Parallel()
		requireGo(t)

		t.Run("returns a passing report for a healthy project", func(t *testing.T) {
			t.Parallel()

			var r recorder
			toolchain.AssertTestsPass(&r, adapter(), healthy())
			assert.False(t, r.failed(), "the generated tests pass")
		})

		t.Run("returns a report of a failing generated test", func(t *testing.T) {
			t.Parallel()

			var r recorder
			toolchain.AssertTestsPass(&r, adapter(), with(rowTestFile,
				"package harness\n\nimport \"testing\"\n\n"+
					"func TestRow(t *testing.T) { t.Fatal(\"the generated check disagrees\") }\n"))
			assert.True(t, r.says("1 of 1 generated tests failed"), "the count reads off the run")
			assert.True(t, r.says("the generated check disagrees"), "and the case's own words follow")
		})

		t.Run("returns a report of no case for a project without tests", func(t *testing.T) {
			t.Parallel()

			var r recorder
			toolchain.AssertTestsPass(&r, adapter(), only(rowFile, "package harness\n\ntype Row struct{}\n"))
			assert.True(t, r.says("reported no case"),
				"generated tests that run nothing pass while proving nothing")
		})
	})

	t.Run("Satisfies", func(t *testing.T) {
		t.Parallel()
		requireGo(t)

		t.Run("reports true for a type that implements the contract", func(t *testing.T) {
			t.Parallel()

			var r recorder
			toolchain.AssertSatisfies(&r, adapter(), healthy(), "*Row", "Reader")
			assert.False(t, r.failed(), "a *Row satisfies the Reader the output declares")
		})

		t.Run("reports false for a value whose method has a pointer receiver", func(t *testing.T) {
			t.Parallel()

			var r recorder
			toolchain.AssertDoesNotSatisfy(&r, adapter(), healthy(), "Row", "Reader")
			assert.False(t, r.failed(), "a Row does not satisfy Reader, because Read has a pointer receiver")
		})

		t.Run("reports false for a contract the type does not implement", func(t *testing.T) {
			t.Parallel()

			var r recorder
			toolchain.AssertDoesNotSatisfy(&r, adapter(), healthy(), "*Row", "error")
			assert.False(t, r.failed(), "a *Row satisfies no error, which nothing widened it to")

			r = recorder{}
			toolchain.AssertSatisfies(&r, adapter(), healthy(), "*Row", "error")
			assert.True(t, r.says("does not satisfy"), "a contract the output does not satisfy reports")
		})

		t.Run("imports the package of a qualified contract", func(t *testing.T) {
			t.Parallel()

			g := with("stringer.go",
				"package harness\n\n// String names the row.\nfunc (r Row) String() string { return r.Name }\n")
			var r recorder
			toolchain.AssertSatisfies(&r, adapter(), g, "Row", "fmt.Stringer")
			assert.False(t, r.failed(), "a Row satisfies fmt.Stringer through its imported package")

			r = recorder{}
			toolchain.AssertDoesNotSatisfy(&r, adapter(), g, "Row", "io.Reader")
			assert.False(
				t,
				r.failed(),
				"and not io.Reader: the compiler reports the missing method, not a missing import",
			)
		})

		t.Run("returns an error for a contract the project does not declare", func(t *testing.T) {
			t.Parallel()

			var r recorder
			toolchain.AssertDoesNotSatisfy(&r, adapter(), healthy(), "Row", "Nowhere")
			assert.True(t, r.says("for a reason other than the contract"),
				"an undefined contract is a broken question, not a false answer")
		})

		t.Run("probes from the package beside an external test file", func(t *testing.T) {
			t.Parallel()

			var r recorder
			toolchain.AssertSatisfies(&r, adapter(),
				with("a_test.go", "package harness_test\n"), "*Row", "Reader")
			assert.False(t, r.failed(), "an external test file sorting first leaves the probe in the package")
		})

		t.Run("returns an error for a project that does not build", func(t *testing.T) {
			t.Parallel()

			var r recorder
			toolchain.AssertSatisfies(&r, adapter(),
				only(rowFile, "package harness\n\nvar X int = \"text\"\n"), "Row", "Reader")
			assert.True(t, r.says("does not build"),
				"a false answer is told apart from a broken project")
		})
	})
}

// requireGo skips the calling test where the Go toolchain is absent.
// CI requires the toolchain, so the skip happens only locally.
func requireGo(t *testing.T) {
	t.Helper()

	var probe recorder
	if !toolchain.Require(&probe, adapter()) {
		t.Skip("the Go toolchain is absent, so the toolchain cases are skipped here and required in CI: " +
			probe.skipped)
	}
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

// with returns the healthy fixture carrying one file replaced or
// added, for a case that breaks exactly one thing.
func with(path, body string) toolchain.Generated {
	g := healthy()
	g.Files[path] = []byte(body)
	return g
}

// only returns a fixture holding one file, for a case that needs no
// test file.
func only(path, body string) toolchain.Generated {
	return toolchain.Generated{
		Module: fixtureModule,
		Files:  map[string][]byte{path: []byte(body)},
	}
}

// adapter is the harness under test.
func adapter() toolchain.Adapter { return gotesting.New() }
