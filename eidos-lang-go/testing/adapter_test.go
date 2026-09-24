// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing_test

import (
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	gotesting "go.dokimi.dev/eidos/lang/go/testing"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The adapter is what the kernel's assertions drive, so what it
// lays out and what each answer means is contract. Parsing needs
// no toolchain; everything below it gates on one.
func TestAdapter(t *testing.T) {
	t.Parallel()

	t.Run("names the language it answers for", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, adapter().Lang(), golang.Lang, "the Go adapter speaks Go")
	})

	t.Run("Layout", func(t *testing.T) {
		t.Parallel()

		t.Run("writes every file and a module the fixture named", func(t *testing.T) {
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

		t.Run("writes a nested path and declares a module for a fixture naming none", func(t *testing.T) {
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

		t.Run("keeps a go.mod the output carried", func(t *testing.T) {
			t.Parallel()

			g := with("go.mod", "module carried.test/own\n\ngo 1.27.0\n")
			dir, err := adapter().Layout(g)
			assert.NoError(t, err, "the fixture lays out")
			t.Cleanup(func() { _ = os.RemoveAll(dir) })

			mod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
			assert.NoError(t, err, "the carried file is on disk")
			assert.Contains(t, string(mod), "carried.test/own",
				"the output's own module stands, because the harness states nothing the output already did")
		})

		t.Run("refuses a path climbing out of the scratch project", func(t *testing.T) {
			t.Parallel()

			_, err := adapter().Layout(only("../escape.go", "package escape\n"))
			assert.HasError(t, err, "a fixture is not a place to write from")
			assert.Contains(t, err.Error(), "climbs out", "and the refusal says so")
		})
	})

	t.Run("Parse reads syntax without a toolchain", func(t *testing.T) {
		t.Parallel()

		a := adapter()
		dir, err := a.Layout(healthy())
		assert.NoError(t, err, "the fixture lays out")
		t.Cleanup(func() { _ = os.RemoveAll(dir) })
		assert.NoError(t, a.Parse(dir), "the healthy output parses")

		broken := adapter()
		dir, err = broken.Layout(only(rowFile, "package harness\n\nfunc F( {}\n"))
		assert.NoError(t, err, "the broken fixture lays out")
		t.Cleanup(func() { _ = os.RemoveAll(dir) })
		err = broken.Parse(dir)
		assert.HasError(t, err, "a syntax error refuses")
		assert.Contains(t, err.Error(), rowFile, "naming the file, relative to the project")
	})

	t.Run("Available says which binary it looked for", func(t *testing.T) {
		t.Parallel()

		held, why := adapter().Available()
		if held {
			assert.Equal(t, why, "", "a toolchain that is there states no reason")
			return
		}
		assert.Contains(t, why, "go is not on PATH", "and one that is not names the binary")
	})

	t.Run("drives the toolchain over generated output", func(t *testing.T) {
		t.Parallel()

		var probe recorder
		if !toolchain.Require(&probe, adapter()) {
			t.Skip("the Go toolchain is absent, so the assertions below are skipped here " +
				"and required in CI: " + probe.skipped)
		}

		t.Run("a healthy project type-checks, tests and vets", func(t *testing.T) {
			t.Parallel()

			var r recorder
			toolchain.AssertTypeChecks(&r, adapter(), healthy())
			toolchain.AssertTestsPass(&r, adapter(), healthy())
			gotesting.AssertVets(&r, adapter(), healthy())
			assert.False(t, r.failed(), "the generated output is sound")
		})

		t.Run("a type error reports as one", func(t *testing.T) {
			t.Parallel()

			var r recorder
			toolchain.AssertTypeChecks(&r, adapter(),
				only(rowFile, "package harness\n\nvar X int = \"text\"\n"))
			assert.True(t, r.says("does not type-check"), "the class is named")
			assert.True(t, r.says("cannot use"), "and the compiler's own words follow")
		})

		t.Run("a failing generated test reports its count and its output", func(t *testing.T) {
			t.Parallel()

			var r recorder
			toolchain.AssertTestsPass(&r, adapter(), with(rowTestFile,
				"package harness\n\nimport \"testing\"\n\n"+
					"func TestRow(t *testing.T) { t.Fatal(\"the generated check disagrees\") }\n"))
			assert.True(t, r.says("1 of 1 generated tests failed"), "the count reads off the run")
			assert.True(t, r.says("the generated check disagrees"), "and the case's own words follow")
		})

		t.Run("a project whose tests execute nothing refuses", func(t *testing.T) {
			t.Parallel()

			var r recorder
			toolchain.AssertTestsPass(&r, adapter(), only(rowFile, "package harness\n\ntype Row struct{}\n"))
			assert.True(t, r.says("reported no case"),
				"generated tests that run nothing pass while proving nothing")
		})

		t.Run("satisfaction answers both ways and survives the probe", func(t *testing.T) {
			t.Parallel()

			var r recorder
			toolchain.AssertSatisfies(&r, adapter(), healthy(), "*Row", "Reader")
			assert.False(t, r.failed(), "a *Row satisfies the Reader it was generated against")

			r = recorder{}
			toolchain.AssertDoesNotSatisfy(&r, adapter(), healthy(), "Row", "Reader")
			assert.False(t, r.failed(), "and a Row does not, because Read has a pointer receiver")

			r = recorder{}
			toolchain.AssertDoesNotSatisfy(&r, adapter(), healthy(), "*Row", "error")
			assert.False(t, r.failed(), "and satisfies no error, which nothing widened it to")

			r = recorder{}
			toolchain.AssertSatisfies(&r, adapter(), healthy(), "*Row", "error")
			assert.True(t, r.says("does not satisfy"), "a promise the output never met reports")
		})

		t.Run("satisfaction imports a qualified contract", func(t *testing.T) {
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

		t.Run("satisfaction refuses a probe that fails for another reason", func(t *testing.T) {
			t.Parallel()

			var r recorder
			toolchain.AssertDoesNotSatisfy(&r, adapter(), healthy(), "Row", "Nowhere")
			assert.True(t, r.says("for a reason other than the contract"),
				"an undefined contract is a broken question, not a false answer")
		})

		t.Run("satisfaction probes from the package, never its external tests", func(t *testing.T) {
			t.Parallel()

			var r recorder
			toolchain.AssertSatisfies(&r, adapter(),
				with("a_test.go", "package harness_test\n"), "*Row", "Reader")
			assert.False(t, r.failed(), "an external test file sorting first leaves the probe in the package")
		})

		t.Run("satisfaction refuses a project that does not build", func(t *testing.T) {
			t.Parallel()

			var r recorder
			toolchain.AssertSatisfies(&r, adapter(),
				only(rowFile, "package harness\n\nvar X int = \"text\"\n"), "Row", "Reader")
			assert.True(t, r.says("does not build"),
				"a false answer is told apart from a broken project")
		})

		t.Run("vet catches what compiles and is still wrong", func(t *testing.T) {
			t.Parallel()

			var r recorder
			gotesting.AssertVets(&r, adapter(), only(rowFile,
				"package harness\n\nimport \"fmt\"\n\n"+
					"// Print misuses its verb.\nfunc Print() string { return fmt.Sprintf(\"%d\", \"text\") }\n"))
			assert.True(t, r.says("go vet refused"), "the wrong verb is caught")
		})
	})
}
