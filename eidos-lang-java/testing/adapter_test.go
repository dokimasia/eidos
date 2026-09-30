// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing_test

import (
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"

	java "go.dokimi.dev/eidos/lang/java"
	javatesting "go.dokimi.dev/eidos/lang/java/testing"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The files a healthy fixture contains, and the type names the
// satisfaction cases pass to Satisfies.
const (
	readerFile  = "demo/Reader.java"
	rowFile     = "demo/Row.java"
	plainFile   = "demo/Plain.java"
	rowTestFile = "demo/RowTest.java"
	rowType     = "demo.Row"
	plainType   = "demo.Plain"
	readerType  = "demo.Reader"
	nowhereType = "demo.Nowhere"
)

// The sources the fixtures are built from.
const (
	readerSource = "package demo;\n\n/** Reader is what a Row satisfies. */\n" +
		"public interface Reader {\n    String read();\n}\n"
	rowSource = "package demo;\n\n/** Row is a generated record. */\n" +
		"public final class Row implements Reader {\n    private final String name;\n\n" +
		"    public Row(String name) {\n        this.name = name;\n    }\n\n" +
		"    @Override\n    public String read() {\n        return name;\n    }\n}\n"
	plainSource = "package demo;\n\n/** Plain declares no read method. */\n" +
		"public final class Plain {\n}\n"
	rowTestSource = "package demo;\n\n/** RowTest checks a Row. */\n" +
		"public final class RowTest {\n    public static void main(String[] args) {\n" +
		"        if (!new Row(\"a\").read().equals(\"a\")) {\n" +
		"            throw new AssertionError(\"read returns the name\");\n        }\n    }\n}\n"
	failingTestSource = "package demo;\n\npublic final class RowTest {\n" +
		"    public static void main(String[] args) {\n" +
		"        throw new AssertionError(\"the generated check disagrees\");\n    }\n}\n"
	typeErrorSource = "package demo;\n\npublic final class Row {\n    int x = \"text\";\n}\n"
)

// healthy returns generated output that parses, compiles and whose
// one test class passes.
func healthy() toolchain.Generated {
	return toolchain.Generated{Files: map[string][]byte{
		readerFile:  []byte(readerSource),
		rowFile:     []byte(rowSource),
		plainFile:   []byte(plainSource),
		rowTestFile: []byte(rowTestSource),
	}}
}

// with returns the healthy fixture with one file replaced or added,
// for a case that breaks one thing.
func with(path, body string) toolchain.Generated {
	g := healthy()
	g.Files[path] = []byte(body)
	return g
}

// only returns a fixture with one file, for a case that needs no test
// class.
func only(path, body string) toolchain.Generated {
	return toolchain.Generated{Files: map[string][]byte{path: []byte(body)}}
}

// adapter is the harness under test.
func adapter() toolchain.Adapter { return javatesting.New() }

// errText returns an error's text, and empty for no error, so a case
// asserting on the text fails and does not panic.
func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// laidOut lays a fixture out and removes it when the case ends.
func laidOut(t *testing.T, g toolchain.Generated) string {
	t.Helper()

	dir, err := adapter().Layout(g)
	assert.NoError(t, err, "the fixture lays out")
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// The kernel's assertions drive the adapter, so the project it lays
// out and the meaning of each result are its contract. The layout
// needs no toolchain, and every case that runs one skips locally
// where it is absent and fails in CI.
func TestAdapter(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an adapter that passes the kernel's toolchain suite", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			toolchain.RunToolchainSuite(t, func(toolchain.TB) (toolchain.Adapter, toolchain.Generated) {
				return adapter(), healthy()
			})
		})
	})

	t.Run("Lang", func(t *testing.T) {
		t.Parallel()

		t.Run("returns Java", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, adapter().Lang(), java.Lang, "the language the assertions name")
		})
	})

	t.Run("Available", func(t *testing.T) {
		t.Parallel()

		t.Run("names the binary it did not find", func(t *testing.T) {
			t.Parallel()

			present, why := adapter().Available()
			if present {
				assert.Equal(t, why, "", "a toolchain that is there states no reason")
				return
			}
			assert.Contains(t, why, "is not on PATH", "the reason names the absent binary")
		})
	})

	t.Run("Layout", func(t *testing.T) {
		t.Parallel()

		t.Run("writes every generated file", func(t *testing.T) {
			t.Parallel()

			body, err := os.ReadFile(filepath.Join(laidOut(t, healthy()), filepath.FromSlash(rowFile)))
			assert.NoError(t, err, "the generated file is on disk")
			assert.Equal(t, string(body), rowSource, "with its own bytes")
		})

		t.Run("returns an error for a path climbing out of the scratch project", func(t *testing.T) {
			t.Parallel()

			_, err := adapter().Layout(only("../Escape.java", "final class Escape {}\n"))
			assert.Contains(t, errText(err), "climbs out", "a fixture is not a place to write from")
		})
	})

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the syntax of a healthy project", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			assert.NoError(t, adapter().Parse(laidOut(t, healthy())), "the healthy output parses")
		})

		t.Run("returns an error naming the file of a syntax error", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			err := adapter().Parse(laidOut(t, only(rowFile, "package demo;\n\nfinal class Row { int x = ; }\n")))
			assert.Contains(t, errText(err), "Row.java", "the refusal names the file")
		})

		t.Run("reads the syntax of a project whose only error is a type error", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			err := adapter().Parse(laidOut(t, only(rowFile, typeErrorSource)))
			assert.NoError(t, err, "a type error is no syntax error")
		})

		t.Run("returns an error for a project with no Java file", func(t *testing.T) {
			t.Parallel()

			err := adapter().Parse(laidOut(t, only("Row.txt", "final class Row {}\n")))
			assert.Contains(t, errText(err), "no Java file", "a parse of nothing proves nothing")
		})
	})

	t.Run("TypeCheck", func(t *testing.T) {
		t.Parallel()

		t.Run("passes a healthy project", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			assert.NoError(t, adapter().TypeCheck(laidOut(t, healthy())), "the output compiles")
		})

		t.Run("returns an error naming the compiler's refusal", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			err := adapter().TypeCheck(laidOut(t, only(rowFile, typeErrorSource)))
			assert.Contains(t, errText(err), "Row.java", "the compiler's own words follow")
		})

		t.Run("returns an error for a project with no Java file", func(t *testing.T) {
			t.Parallel()

			err := adapter().TypeCheck(laidOut(t, only("Row.txt", "final class Row {}\n")))
			assert.Contains(t, errText(err), "no Java file", "a compile of nothing proves nothing")
		})
	})

	t.Run("RunTests", func(t *testing.T) {
		t.Parallel()

		t.Run("counts a passing test class", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			report, err := adapter().RunTests(laidOut(t, healthy()))
			assert.NoError(t, err, "the run completes")
			assert.Equal(t, []int{report.Passed, report.Failed}, []int{1, 0}, "one class passes")
		})

		t.Run("counts a failing test class", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			report, err := adapter().RunTests(laidOut(t, with(rowTestFile, failingTestSource)))
			assert.NoError(t, err, "the run completes")
			assert.Equal(t, report.Failed, 1, "the class's failure counts")
		})

		t.Run("returns the output of a failing test class", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			report, _ := adapter().RunTests(laidOut(t, with(rowTestFile, failingTestSource)))
			assert.Contains(t, report.Output, "the generated check disagrees", "the case's own words follow")
		})

		t.Run("counts a test class without a main method as a failure", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			report, err := adapter().RunTests(laidOut(t, with(rowTestFile,
				"package demo;\n\npublic final class RowTest {\n}\n")))
			assert.NoError(t, err, "the run completes")
			assert.Equal(t, report.Failed, 1, "a class the convention cannot run fails")
		})

		t.Run("runs no nested class whose name ends in Test", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			nested := "package demo;\n\npublic final class RowTest {\n" +
				"    static final class ReadTest {\n    }\n\n" +
				"    public static void main(String[] args) {\n    }\n}\n"
			report, err := adapter().RunTests(laidOut(t, with(rowTestFile, nested)))
			assert.NoError(t, err, "the run completes")
			assert.Equal(t, []int{report.Passed, report.Failed}, []int{1, 0}, "the nested class is no test class")
		})

		t.Run("returns an empty report for a project without a test class", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			dir := laidOut(t, only(rowFile, "package demo;\n\npublic final class Row {\n}\n"))
			report, err := adapter().RunTests(dir)
			assert.NoError(t, err, "the run completes")
			assert.False(t, report.OK(), "a run of nothing is not a pass")
		})

		t.Run("returns an error for a project that does not compile", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			_, err := adapter().RunTests(laidOut(t, only(rowFile, typeErrorSource)))
			assert.Contains(t, errText(err), "Row.java", "the compiler's refusal returns")
		})
	})

	t.Run("Satisfies", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for a type assignable to the contract", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			got, err := adapter().Satisfies(laidOut(t, healthy()), rowType, readerType)
			assert.NoError(t, err, "the probe runs")
			assert.True(t, got, "a Row implements Reader")
		})

		t.Run("reports false for a type the contract does not admit", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			got, err := adapter().Satisfies(laidOut(t, healthy()), plainType, readerType)
			assert.NoError(t, err, "the probe runs")
			assert.False(t, got, "a Plain does not implement Reader")
		})

		t.Run("returns an error for a contract no class declares", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			_, err := adapter().Satisfies(laidOut(t, healthy()), rowType, nowhereType)
			assert.Contains(t, errText(err), "other than the contract", "an undefined contract is a broken question")
		})

		t.Run("returns an error for a project that does not compile", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			_, err := adapter().Satisfies(laidOut(t, with(rowFile, typeErrorSource)), rowType, readerType)
			assert.Contains(t, errText(err), "the project does not compile",
				"a broken project returns an error, not false")
		})
	})
}
