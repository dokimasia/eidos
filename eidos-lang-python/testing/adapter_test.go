// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/lang/python"
	pythontesting "go.dokimi.dev/eidos/lang/python/testing"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The files of the healthy fixture, the configuration that pyright reads,
// and the file that a case adds.
const (
	initFile    = "shop/__init__.py"
	modelsFile  = "shop/models.py"
	rowTestFile = "test_row.py"
	configFile  = "pyrightconfig.json"
	brokenFile  = "shop/broken.py"
	stubFile    = "shop/broken.pyi"
)

// The sources of the healthy fixture. The models declare a Protocol, a class
// that satisfies it, and a class without its member. The test module has a
// test that passes and a test that skips.
const (
	initSource   = "\"\"\"The shop package.\"\"\"\n"
	modelsSource = `"""Shop models."""

from typing import Protocol


class Reader(Protocol):
    """Reader reads a name."""

    def read(self) -> str: ...


class Row:
    """Row is a generated record."""

    def __init__(self, name: str) -> None:
        self.name = name

    def read(self) -> str:
        return self.name


class Plain:
    """Plain declares no read method."""

    def __init__(self, ident: int) -> None:
        self.ident = ident
`
	rowTestSource = `"""The tests of Row."""

import unittest

from shop.models import Row


class RowTest(unittest.TestCase):
    """RowTest checks Row."""

    def test_read(self) -> None:
        self.assertEqual(Row("a").read(), "a")

    @unittest.skip("a skipped case")
    def test_skipped(self) -> None:
        self.fail("a skipped case does not run")
`
)

// The sources that a case adds to break one check: a syntax error, a type
// statement, which Python 3.12 adds, a type error, a failing test, a test
// module that does not import, a test that ends the interpreter, and a test
// that prints counts that do not decode before it ends the interpreter.
const (
	syntaxSource     = "def load(:\n    return 1\n"
	typeStmtSource   = "type Id = int\n"
	typeErrorSource  = "count: int = \"text\"\n"
	failingSource    = "import unittest\n\n\nclass FailTest(unittest.TestCase):\n    def test_fail(self) -> None:\n        self.fail(\"the generated check disagrees\")\n"
	unimportedSource = "import shop.nowhere\n"
	exitSource       = "import os\n\nos._exit(0)\n"
	countsSource     = "import os\n\nprint('{\"run\": \"many\"}', flush=True)\nos._exit(0)\n"
)

// The class names that the satisfaction cases pass to Satisfies.
const (
	rowType       = "shop.models.Row"
	plainType     = "shop.models.Plain"
	readerType    = "shop.models.Reader"
	rowColonType  = "shop.models:Row"
	readerColon   = "shop.models:Reader"
	nowhereType   = "shop.models.Nowhere"
	boolType      = "bool"
	intType       = "int"
	defaultConfig = `"pythonVersion": "3.11"`
	strictConfig  = `"typeCheckingMode": "strict"`
)

// fakeOutput is the variable that turns the test binary into a toolchain
// binary that prints the value of the variable and exits with success.
const fakeOutput = "EIDOS_PYTHON_FAKE_OUTPUT"

// TestMain runs the test binary as the fake toolchain binary where the
// environment sets [fakeOutput], and runs the tests otherwise. The fake
// exits with 1 where its output does not write.
func TestMain(m *testing.M) {
	if out, fake := os.LookupEnv(fakeOutput); fake {
		if _, err := os.Stdout.WriteString(out); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// TestAdapter checks the adapter through the assertions of the kernel and
// through each of its methods. The layout needs no toolchain. Every case
// that runs one skips locally where it is absent, and fails in CI.
func TestAdapter(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an adapter that passes the toolchain suite of the kernel", func(t *testing.T) {
			t.Parallel()

			toolchain.RunToolchainSuite(t, pythontesting.New(python.Version311), healthy())
		})

		t.Run("returns an adapter of the default version of the policy for the empty choice", func(t *testing.T) {
			t.Parallel()

			dir, err := pythontesting.New("").Layout(healthy())
			assert.NoError(t, err, "the fixture lays out")
			t.Cleanup(func() { _ = os.RemoveAll(dir) })
			assert.Contains(t, files.Read(t, filepath.Join(dir, configFile)), defaultConfig,
				"the configuration checks for the default version")
		})
	})

	t.Run("Lang", func(t *testing.T) {
		t.Parallel()

		t.Run("returns python", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, pythontesting.New(python.Version311).Lang(), python.Lang,
				"the language that the assertions name")
		})
	})

	t.Run("Available", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a reason that contains the missing binary", func(t *testing.T) {
			t.Parallel()

			present, why := pythontesting.New(python.Version311).Available()
			if present {
				assert.Empty(t, why, "a toolchain that is there states no reason")
				return
			}
			assert.Contains(t, why, "is not on PATH", "the reason names the absent binary")
		})
	})

	t.Run("Layout", func(t *testing.T) {
		t.Parallel()

		t.Run("writes every generated file", func(t *testing.T) {
			t.Parallel()

			files.HasContent(t, filepath.Join(laidOut(t, healthy()), filepath.FromSlash(modelsFile)), modelsSource,
				"the generated file is on disk with its own bytes")
		})

		t.Run("writes a strict pyrightconfig.json of the target version for a fixture without one", func(t *testing.T) {
			t.Parallel()

			dir, err := pythontesting.New(python.Version313).Layout(healthy())
			assert.NoError(t, err, "the fixture lays out")
			t.Cleanup(func() { _ = os.RemoveAll(dir) })
			expect.That(t, files.Read(t, filepath.Join(dir, configFile))).
				Contains(`"pythonVersion": "3.13"`, "the configuration checks for the target version").
				Contains(strictConfig, "the configuration checks strictly")
		})

		t.Run("keeps a pyrightconfig.json that the output wrote", func(t *testing.T) {
			t.Parallel()

			own := `{"typeCheckingMode": "basic"}`
			files.HasContent(t, filepath.Join(laidOut(t, with(configFile, own)), configFile), own,
				"the harness states nothing that the output stated")
		})

		t.Run("returns an error for a path that leaves the scratch directory", func(t *testing.T) {
			t.Parallel()

			_, err := pythontesting.New(python.Version311).Layout(only("../escape.py", initSource))
			assert.HasError(t, err, "a fixture writes only inside its project")
			assert.Contains(t, err.Error(), "leaves the directory", "the error contains the reason")
		})

		refusals := []struct {
			name        string
			giveVersion plugin.Choice
		}{
			{name: "returns an error for a version of Python that the policy does not declare", giveVersion: "3.10"},
			{name: "returns an error for a choice that is no version of Python", giveVersion: "python"},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := pythontesting.New(tt.giveVersion).Layout(healthy())
				assert.HasError(t, err, "the adapter refuses the version")
				assert.Equal(t, err.Error(), fmt.Sprintf("testing: the policy python.version has no such version: %q",
					tt.giveVersion), "the error names the version")
			})
		}
	})

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the syntax of a healthy project", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, pythontesting.New(python.Version311)) {
				return
			}
			assert.NoError(t, pythontesting.New(python.Version311).Parse(t.Context(), laidOut(t, healthy())),
				"the healthy output parses")
		})

		t.Run("returns an error that contains the position of a syntax error", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, pythontesting.New(python.Version311)) {
				return
			}
			dir := laidOut(t, with(brokenFile, syntaxSource))
			err := pythontesting.New(python.Version311).Parse(t.Context(), dir)
			assert.HasError(t, err, "a syntax error refuses")
			assert.Contains(t, err.Error(), filepath.Join(dir, filepath.FromSlash(brokenFile))+":1:10: invalid syntax",
				"the refusal names the file, the line and the column")
		})

		t.Run("returns an error for a syntax error of a stub file", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, pythontesting.New(python.Version311)) {
				return
			}
			err := pythontesting.New(python.Version311).Parse(t.Context(), laidOut(t, with(stubFile, syntaxSource)))
			assert.HasError(t, err, "the parse reads the stub files")
			assert.Contains(t, err.Error(), filepath.FromSlash(stubFile), "the refusal names the stub file")
		})

		t.Run("returns an error for a construct of a later version", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, pythontesting.New(python.Version311)) {
				return
			}
			err := pythontesting.New(python.Version311).Parse(t.Context(), laidOut(t, with(brokenFile, typeStmtSource)))
			assert.HasError(t, err, "Python 3.11 has no type statement")
			assert.Contains(t, err.Error(), "Type statement is only supported in Python 3.12 and greater",
				"the refusal is the error of ast")
		})

		t.Run("reads the syntax of the version that adds a construct", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, pythontesting.New(python.Version312)) {
				return
			}
			err := pythontesting.New(python.Version312).Parse(t.Context(), laidOut(t, with(brokenFile, typeStmtSource)))
			assert.NoError(t, err, "Python 3.12 has the type statement")
		})

		t.Run("reads the syntax of a project whose only error is a type error", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, pythontesting.New(python.Version311)) {
				return
			}
			err := pythontesting.New(python.Version311).
				Parse(t.Context(), laidOut(t, with(brokenFile, typeErrorSource)))
			assert.NoError(t, err, "a type error is no syntax error")
		})

		t.Run("returns an error for a project without a Python file", func(t *testing.T) {
			t.Parallel()

			err := pythontesting.New(python.Version311).Parse(t.Context(), laidOut(t, only("notes.txt", initSource)))
			assert.HasError(t, err, "a parse of nothing proves nothing")
			assert.Equal(t, err.Error(), "testing: the laid-out project has no Python file", "the error states why")
		})

		t.Run("returns an error for a directory that does not exist", func(t *testing.T) {
			t.Parallel()

			err := pythontesting.New(python.Version311).Parse(t.Context(), filepath.Join(t.TempDir(), "missing"))
			assert.ErrorIs(t, err, os.ErrNotExist, "the walk of a missing directory fails")
		})

		t.Run("returns an error for a version that the policy does not declare", func(t *testing.T) {
			t.Parallel()

			err := pythontesting.New("3.10").Parse(t.Context(), t.TempDir())
			assert.HasError(t, err, "the adapter refuses the version")
			assert.Contains(t, err.Error(), "has no such version", "the error states why")
		})

		t.Run("returns the error of the context for a context that ended", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, pythontesting.New(python.Version311)) {
				return
			}
			dir := laidOut(t, healthy())
			assert.HonoursCancellation(t, func(ctx context.Context) error {
				return pythontesting.New(python.Version311).Parse(ctx, dir)
			}, "an ended context starts no python3")
		})
	})

	t.Run("TypeCheck", func(t *testing.T) {
		t.Parallel()

		t.Run("returns nil for a healthy project", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, pythontesting.New(python.Version311)) {
				return
			}
			assert.NoError(t, pythontesting.New(python.Version311).TypeCheck(t.Context(), laidOut(t, healthy())),
				"the output is sound under the strict mode of pyright")
		})

		t.Run("returns an error that contains the position and the rule of a diagnostic", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, pythontesting.New(python.Version311)) {
				return
			}
			err := pythontesting.New(python.Version311).TypeCheck(t.Context(),
				laidOut(t, with(brokenFile, typeErrorSource)))
			assert.HasError(t, err, "a type error refuses")
			assert.Contains(t, err.Error(), filepath.FromSlash(brokenFile)+":1:14: ", "the refusal names the position")
			assert.Contains(t, err.Error(), "(reportAssignmentType)", "the refusal names the rule of pyright")
		})

		t.Run("returns the error of a configuration that pyright cannot read", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, pythontesting.New(python.Version311)) {
				return
			}
			err := pythontesting.New(python.Version311).TypeCheck(t.Context(), laidOut(t, with(configFile, "{")))
			assert.HasError(t, err, "a configuration that does not parse fails the run without a diagnostic")
			assert.Contains(t, err.Error(), "could not be parsed", "the error contains the words of pyright")
		})

		t.Run("returns an error for a version that the policy does not declare", func(t *testing.T) {
			t.Parallel()

			err := pythontesting.New("3.10").TypeCheck(t.Context(), t.TempDir())
			assert.HasError(t, err, "the adapter refuses the version")
			assert.Contains(t, err.Error(), "has no such version", "the error states why")
		})

		t.Run("returns the error of the context for a context that ended", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, pythontesting.New(python.Version311)) {
				return
			}
			dir := laidOut(t, healthy())
			assert.HonoursCancellation(t, func(ctx context.Context) error {
				return pythontesting.New(python.Version311).TypeCheck(ctx, dir)
			}, "an ended context starts no pyright")
		})
	})

	t.Run("RunTests", func(t *testing.T) {
		t.Parallel()

		t.Run("counts the passing and the skipped tests", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, pythontesting.New(python.Version311)) {
				return
			}
			report, err := pythontesting.New(python.Version311).RunTests(t.Context(), laidOut(t, healthy()))
			assert.NoError(t, err, "the run completes")
			expect.Equal(t, report.Passed, 1, "one test passes")
			expect.Equal(t, report.Skipped, 1, "one test skips")
			expect.Equal(t, report.Failed, 0, "no test fails")
		})

		t.Run("counts a failing test with its output", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, pythontesting.New(python.Version311)) {
				return
			}
			report, err := pythontesting.New(python.Version311).RunTests(t.Context(),
				laidOut(t, with("test_fail.py", failingSource)))
			assert.NoError(t, err, "the run completes")
			expect.Equal(t, report.Failed, 1, "the failing test counts")
			expect.Contains(t, report.Output, "the generated check disagrees", "the words of the test follow")
		})

		t.Run("counts a test module that does not import as failed", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, pythontesting.New(python.Version311)) {
				return
			}
			report, err := pythontesting.New(python.Version311).RunTests(t.Context(),
				laidOut(t, with("test_broken.py", unimportedSource)))
			assert.NoError(t, err, "the run completes")
			assert.Equal(t, report.Failed, 1, "the discovery reports the module as a failed test")
		})

		t.Run("returns an empty report for a project without a test module", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, pythontesting.New(python.Version311)) {
				return
			}
			report, err := pythontesting.New(python.Version311).RunTests(t.Context(),
				laidOut(t, only(modelsFile, modelsSource)))
			assert.NoError(t, err, "the run completes")
			assert.False(t, report.OK(), "a run of nothing is not a pass")
		})

		summaries := []struct {
			name       string
			giveSource string
		}{
			{name: "returns an error for a run that ends without its counts", giveSource: exitSource},
			{name: "returns an error for counts that do not decode", giveSource: countsSource},
		}
		for _, tt := range summaries {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				if !toolchain.Require(t, pythontesting.New(python.Version311)) {
					return
				}
				_, err := pythontesting.New(python.Version311).RunTests(t.Context(),
					laidOut(t, with("test_exit.py", tt.giveSource)))
				assert.HasError(t, err, "a run without its counts proves nothing")
				assert.Contains(t, err.Error(), "testing: the run of the tests ended without its counts",
					"the error states why")
			})
		}

		t.Run("returns an error for a version that the policy does not declare", func(t *testing.T) {
			t.Parallel()

			_, err := pythontesting.New("3.10").RunTests(t.Context(), t.TempDir())
			assert.HasError(t, err, "the adapter refuses the version")
			assert.Contains(t, err.Error(), "has no such version", "the error states why")
		})

		t.Run("returns the error of the context for a context that ended", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, pythontesting.New(python.Version311)) {
				return
			}
			dir := laidOut(t, healthy())
			assert.HonoursCancellation(t, func(ctx context.Context) error {
				_, err := pythontesting.New(python.Version311).RunTests(ctx, dir)
				return err
			}, "an ended context runs no test")
		})
	})

	t.Run("Satisfies", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name         string
			giveType     string
			giveContract string
			want         bool
		}{
			{
				name:     "reports true for a class with the members of a Protocol",
				giveType: rowType, giveContract: readerType, want: true,
			},
			{
				name:     "reports false for a class without a member of the Protocol",
				giveType: plainType, giveContract: readerType,
			},
			{
				name:     "reports true for the names of the classes in the form of importlib",
				giveType: rowColonType, giveContract: readerColon, want: true,
			},
			{
				name:     "reports true for a builtin that subclasses a builtin",
				giveType: boolType, giveContract: intType, want: true,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				if !toolchain.Require(t, pythontesting.New(python.Version311)) {
					return
				}
				got, err := pythontesting.New(python.Version311).Satisfies(t.Context(), laidOut(t, healthy()),
					tt.giveType, tt.giveContract)
				assert.NoError(t, err, "the probe runs")
				assert.Equal(t, got, tt.want, "pyright checks the return of the probe against the contract")
			})
		}

		t.Run("returns an error for a contract that no module declares", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, pythontesting.New(python.Version311)) {
				return
			}
			_, err := pythontesting.New(python.Version311).Satisfies(t.Context(), laidOut(t, healthy()), rowType,
				nowhereType)
			assert.HasError(t, err, "an undeclared contract is a broken question")
			assert.Contains(t, err.Error(), "for a reason other than the contract", "the refusal states why")
		})

		t.Run("returns an error for a project that does not type-check", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, pythontesting.New(python.Version311)) {
				return
			}
			_, err := pythontesting.New(python.Version311).Satisfies(t.Context(),
				laidOut(t, with(brokenFile, typeErrorSource)), rowType, readerType)
			assert.HasError(t, err, "a broken project returns an error and not false")
			assert.Contains(t, err.Error(), "the project does not type-check", "the refusal names the project")
		})

		t.Run("returns the error of the context for a context that ended", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, pythontesting.New(python.Version311)) {
				return
			}
			dir := laidOut(t, healthy())
			assert.HonoursCancellation(t, func(ctx context.Context) error {
				_, err := pythontesting.New(python.Version311).Satisfies(ctx, dir, rowType, readerType)
				return err
			}, "an ended context asks the checker nothing")
		})
	})
}

// TestAdapterEnv checks the cases that change the search path or the
// permissions of the process, which run apart from the parallel cases of
// TestAdapter.
func TestAdapterEnv(t *testing.T) {
	t.Run("Parse", func(t *testing.T) {
		t.Run("returns the error of a python3 that is not on PATH", func(t *testing.T) {
			dir := laidOut(t, healthy())
			t.Setenv("PATH", t.TempDir())
			err := pythontesting.New(python.Version311).Parse(t.Context(), dir)
			assert.ErrorIs(t, err, exec.ErrNotFound, "the parse returns the error of the missing python3")
		})
	})

	t.Run("TypeCheck", func(t *testing.T) {
		t.Run("returns the error of a pyright that is not on PATH", func(t *testing.T) {
			dir := laidOut(t, healthy())
			t.Setenv("PATH", t.TempDir())
			err := pythontesting.New(python.Version311).TypeCheck(t.Context(), dir)
			assert.ErrorIs(t, err, exec.ErrNotFound, "the check returns the error of the missing pyright")
		})

		t.Run("returns an error for a pyright that writes no report", func(t *testing.T) {
			dir := laidOut(t, healthy())
			self, err := os.Executable()
			assert.NoError(t, err, "the test binary has a path")
			name := "pyright"
			if runtime.GOOS == "windows" {
				name += ".exe"
			}
			bin := t.TempDir()
			source, err := os.Open(self)
			assert.NoError(t, err, "the test binary opens")
			defer func() { _ = source.Close() }()
			target, err := os.OpenFile(filepath.Join(bin, name), os.O_CREATE|os.O_WRONLY, 0o700)
			assert.NoError(t, err, "the copy of the test binary opens")
			_, err = io.Copy(target, source)
			assert.NoError(t, err, "the test binary copies")
			assert.NoError(t, target.Close(), "the copy of the test binary closes")
			t.Setenv("PATH", bin)
			t.Setenv(fakeOutput, "no report")
			err = pythontesting.New(python.Version311).TypeCheck(t.Context(), dir)
			assert.HasError(t, err, "an output without a report proves nothing")
			assert.Contains(t, err.Error(), "testing: pyright wrote no report", "the error states why")
		})
	})

	t.Run("RunTests", func(t *testing.T) {
		t.Run("returns the error of a python3 that is not on PATH", func(t *testing.T) {
			dir := laidOut(t, healthy())
			t.Setenv("PATH", t.TempDir())
			_, err := pythontesting.New(python.Version311).RunTests(t.Context(), dir)
			assert.ErrorIs(t, err, exec.ErrNotFound, "the run returns the error of the missing python3")
		})
	})

	t.Run("Satisfies", func(t *testing.T) {
		t.Run("returns the error of a probe that does not write", func(t *testing.T) {
			if runtime.GOOS == "windows" {
				t.Skip("Windows writes into a directory without the write permission")
			}
			if !toolchain.Require(t, pythontesting.New(python.Version311)) {
				return
			}
			dir := laidOut(t, healthy())
			assert.NoError(t, os.Chmod(dir, 0o500), "the project becomes read-only")
			t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
			_, err := pythontesting.New(python.Version311).Satisfies(t.Context(), dir, rowType, readerType)
			assert.ErrorIs(t, err, os.ErrPermission, "the probe does not write into a read-only project")
		})
	})
}

// healthy returns generated output that parses, type-checks in the strict
// mode of pyright and runs one test that passes and one that skips.
func healthy() toolchain.Generated {
	return toolchain.Generated{Files: map[string][]byte{
		initFile:    []byte(initSource),
		modelsFile:  []byte(modelsSource),
		rowTestFile: []byte(rowTestSource),
	}}
}

// with returns the healthy fixture with one file replaced or added, for a
// case that breaks one check.
func with(path, body string) toolchain.Generated {
	g := healthy()
	g.Files[path] = []byte(body)
	return g
}

// only returns a fixture with one file.
func only(path, body string) toolchain.Generated {
	return toolchain.Generated{Files: map[string][]byte{path: []byte(body)}}
}

// laidOut lays a fixture out for Python 3.11 and removes it when the case
// ends.
func laidOut(t *testing.T, g toolchain.Generated) string {
	t.Helper()

	dir, err := pythontesting.New(python.Version311).Layout(g)
	assert.NoError(t, err, "the fixture lays out")
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}
