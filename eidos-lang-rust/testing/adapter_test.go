// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing_test

import (
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"

	rust "go.dokimi.dev/eidos/lang/rust"
	rusttesting "go.dokimi.dev/eidos/lang/rust/testing"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The files a healthy fixture contains, the manifest cargo reads, and
// the names the satisfaction cases pass to Satisfies.
const (
	libFile      = "src/lib.rs"
	rowTestFile  = "tests/row.rs"
	manifestFile = "Cargo.toml"
	rowType      = "Row"
	plainType    = "Plain"
	readerTrait  = "Reader"
	nowhereTrait = "Nowhere"
)

// The sources the fixtures are built from.
const (
	libSource = `/// Reader is what a Row satisfies.
pub trait Reader {
    fn read(&self) -> String;
}

/// Row is a generated record.
pub struct Row {
    pub name: String,
}

impl Reader for Row {
    fn read(&self) -> String {
        self.name.clone()
    }
}

/// Plain implements no Reader.
pub struct Plain;
`
	rowTestSource = `use eidos_generated::{Reader, Row};

#[test]
fn reads_the_name() {
    assert_eq!(Row { name: "a".into() }.read(), "a");
}
`
	failingTestSource = `#[test]
fn disagrees() {
    panic!("the generated check disagrees");
}
`
	typeErrorSource = "pub fn value() -> i32 {\n    \"text\"\n}\n"
)

// healthy returns generated output that parses, checks and whose one
// test passes.
func healthy() toolchain.Generated {
	return toolchain.Generated{Files: map[string][]byte{
		libFile:     []byte(libSource),
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

// only returns a fixture with one file, for a case that needs no
// test.
func only(path, body string) toolchain.Generated {
	return toolchain.Generated{Files: map[string][]byte{path: []byte(body)}}
}

// adapter is the harness under test.
func adapter() toolchain.Adapter { return rusttesting.New() }

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

// The kernel's assertions drive the adapter, so the crate it lays out
// and the meaning of each result are its contract. The layout needs
// no toolchain, and every case that runs one skips locally where it
// is absent and fails in CI.
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

		t.Run("returns Rust", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, adapter().Lang(), rust.Lang, "the language the assertions name")
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

			body, err := os.ReadFile(filepath.Join(laidOut(t, healthy()), filepath.FromSlash(libFile)))
			assert.NoError(t, err, "the generated file is on disk")
			assert.Equal(t, string(body), libSource, "with its own bytes")
		})

		t.Run("writes a manifest naming the default crate for a fixture stating none", func(t *testing.T) {
			t.Parallel()

			body, err := os.ReadFile(filepath.Join(laidOut(t, healthy()), manifestFile))
			assert.NoError(t, err, "the manifest is on disk")
			assert.Contains(t, string(body), `name = "eidos_generated"`, "tests import the crate by that name")
		})

		t.Run("writes a manifest naming the fixture's module", func(t *testing.T) {
			t.Parallel()

			g := healthy()
			g.Module = "harness"
			body, err := os.ReadFile(filepath.Join(laidOut(t, g), manifestFile))
			assert.NoError(t, err, "the manifest is on disk")
			assert.Contains(t, string(body), `name = "harness"`, "the module names the crate")
		})

		t.Run("keeps a manifest the output wrote", func(t *testing.T) {
			t.Parallel()

			own := "[package]\nname = \"own\"\nversion = \"1.0.0\"\nedition = \"2024\"\n"
			body, err := os.ReadFile(filepath.Join(laidOut(t, with(manifestFile, own)), manifestFile))
			assert.NoError(t, err, "the output's manifest is on disk")
			assert.Equal(t, string(body), own, "the harness states nothing the output already did")
		})

		t.Run("returns an error for a path climbing out of the scratch project", func(t *testing.T) {
			t.Parallel()

			_, err := adapter().Layout(only("../escape.rs", "pub fn f() {}\n"))
			assert.Contains(t, errText(err), "climbs out", "a fixture is not a place to write from")
		})
	})

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the syntax of a healthy crate", func(t *testing.T) {
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
			err := adapter().Parse(laidOut(t, only(libFile, "pub fn broken( {}\n")))
			assert.Contains(t, errText(err), "lib.rs", "the refusal names the file")
		})

		t.Run("reads the syntax of a crate whose only error is a type error", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			err := adapter().Parse(laidOut(t, only(libFile, typeErrorSource)))
			assert.NoError(t, err, "a type error is no syntax error")
		})

		t.Run("returns an error for a crate with no Rust file", func(t *testing.T) {
			t.Parallel()

			err := adapter().Parse(laidOut(t, only("src/lib.txt", "pub fn f() {}\n")))
			assert.Contains(t, errText(err), "no Rust file", "a parse of nothing proves nothing")
		})
	})

	t.Run("TypeCheck", func(t *testing.T) {
		t.Parallel()

		t.Run("passes a healthy crate", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			assert.NoError(t, adapter().TypeCheck(laidOut(t, healthy())), "the output checks")
		})

		t.Run("returns an error naming rustc's code", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			err := adapter().TypeCheck(laidOut(t, only(libFile, typeErrorSource)))
			assert.Contains(t, errText(err), "E0308", "the compiler's own words follow")
		})

		t.Run("returns an error for a type error in a test", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			dir := laidOut(t, with(rowTestFile, "#[test]\nfn broken() {\n    let _: i32 = \"text\";\n}\n"))
			assert.HasError(t, adapter().TypeCheck(dir), "every target is checked, the tests included")
		})
	})

	t.Run("RunTests", func(t *testing.T) {
		t.Parallel()

		t.Run("counts a passing test", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			report, err := adapter().RunTests(laidOut(t, healthy()))
			assert.NoError(t, err, "the run completes")
			assert.Equal(t, []int{report.Passed, report.Failed}, []int{1, 0}, "one test passes")
		})

		t.Run("counts a failing test", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			report, err := adapter().RunTests(laidOut(t, with(rowTestFile, failingTestSource)))
			assert.NoError(t, err, "the run completes")
			assert.Equal(t, report.Failed, 1, "the test's failure counts")
		})

		t.Run("returns the output of a failing test", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			report, _ := adapter().RunTests(laidOut(t, with(rowTestFile, failingTestSource)))
			assert.Contains(t, report.Output, "the generated check disagrees", "the case's own words follow")
		})

		t.Run("counts an ignored test as skipped", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			report, err := adapter().RunTests(laidOut(t, with(rowTestFile, "#[test]\n#[ignore]\nfn later() {}\n")))
			assert.NoError(t, err, "the run completes")
			assert.Equal(t, report.Skipped, 1, "the ignored test counts as skipped")
		})

		t.Run("returns an empty report for a crate without a test", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			report, err := adapter().RunTests(laidOut(t, only(libFile, "pub struct Row;\n")))
			assert.NoError(t, err, "the run completes")
			assert.False(t, report.OK(), "a run of nothing is not a pass")
		})

		t.Run("returns an error for a crate that does not compile", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			_, err := adapter().RunTests(laidOut(t, only(libFile, typeErrorSource)))
			assert.Contains(t, errText(err), "E0308", "the compiler's refusal returns")
		})
	})

	t.Run("Satisfies", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for a type implementing the trait", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			got, err := adapter().Satisfies(laidOut(t, healthy()), rowType, readerTrait)
			assert.NoError(t, err, "the probe runs")
			assert.True(t, got, "a Row implements Reader")
		})

		t.Run("reports false for a type not implementing the trait", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			got, err := adapter().Satisfies(laidOut(t, healthy()), plainType, readerTrait)
			assert.NoError(t, err, "the probe runs")
			assert.False(t, got, "a Plain does not implement Reader")
		})

		t.Run("restores the crate root after the probe", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			dir := laidOut(t, healthy())
			_, err := adapter().Satisfies(dir, rowType, readerTrait)
			assert.NoError(t, err, "the probe runs")
			body, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(libFile)))
			assert.NoError(t, err, "the crate root is on disk")
			assert.Equal(t, string(body), libSource, "the root declares no probe module afterwards")
		})

		t.Run("returns an error for a trait the crate does not declare", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			_, err := adapter().Satisfies(laidOut(t, healthy()), rowType, nowhereTrait)
			assert.Contains(t, errText(err), "other than the trait", "an undefined trait is a broken question")
		})

		t.Run("returns an error for a crate that does not check", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			_, err := adapter().Satisfies(laidOut(t, with(libFile, typeErrorSource)), rowType, readerTrait)
			assert.Contains(t, errText(err), "the crate does not check", "a broken crate returns an error, not false")
		})

		t.Run("probes from the binary root of a crate without a library", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			g := only("src/main.rs", libSource+"\nfn main() {}\n")
			got, err := adapter().Satisfies(laidOut(t, g), rowType, readerTrait)
			assert.NoError(t, err, "the probe runs")
			assert.True(t, got, "the binary root declares the probe")
		})

		t.Run("returns an error for a crate without a root", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			g := only("src/bin/tool.rs", "fn main() {}\n")
			_, err := adapter().Satisfies(laidOut(t, g), rowType, readerTrait)
			assert.Contains(t, errText(err), "no src/lib.rs", "the probe needs a root to declare it from")
		})
	})
}
