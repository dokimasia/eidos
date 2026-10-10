// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package testing_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
	"go.dokimi.dev/assert/files"

	"go.dokimi.dev/eidos/lang/rust"
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

			files.HasContent(t, filepath.Join(laidOut(t, healthy()), filepath.FromSlash(libFile)), libSource,
				"the generated file is on disk with its own bytes")
		})

		t.Run("writes a manifest naming the default crate for a fixture stating none", func(t *testing.T) {
			t.Parallel()

			assert.Contains(t, files.Read(t, filepath.Join(laidOut(t, healthy()), manifestFile)),
				`name = "eidos_generated"`, "tests import the crate by that name")
		})

		t.Run("writes a manifest naming the fixture's module", func(t *testing.T) {
			t.Parallel()

			g := healthy()
			g.Module = "harness"
			assert.Contains(t, files.Read(t, filepath.Join(laidOut(t, g), manifestFile)), `name = "harness"`,
				"the module names the crate")
		})

		t.Run("keeps a manifest the output wrote", func(t *testing.T) {
			t.Parallel()

			own := "[package]\nname = \"own\"\nversion = \"1.0.0\"\nedition = \"2024\"\n"
			files.HasContent(t, filepath.Join(laidOut(t, with(manifestFile, own)), manifestFile), own,
				"the harness states nothing the output already did")
		})

		t.Run("returns an error for a path climbing out of the scratch project", func(t *testing.T) {
			t.Parallel()

			_, err := adapter().Layout(only("../escape.rs", "pub fn f() {}\n"))
			assert.HasError(t, err, "a fixture is not a place to write from")
			assert.Contains(t, err.Error(), "climbs out", "the refusal names the escape")
		})
	})

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the syntax of a healthy crate", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			assert.NoError(t, adapter().Parse(t.Context(), laidOut(t, healthy())), "the healthy output parses")
		})

		t.Run("returns an error naming the file of a syntax error", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			err := adapter().Parse(t.Context(), laidOut(t, only(libFile, "pub fn broken( {}\n")))
			assert.HasError(t, err, "a syntax error refuses")
			assert.Contains(t, err.Error(), "lib.rs", "the refusal names the file")
		})

		t.Run("reads the syntax of a crate whose only error is a type error", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			err := adapter().Parse(t.Context(), laidOut(t, only(libFile, typeErrorSource)))
			assert.NoError(t, err, "a type error is no syntax error")
		})

		t.Run("returns an error for a crate with no Rust file", func(t *testing.T) {
			t.Parallel()

			err := adapter().Parse(t.Context(), laidOut(t, only("src/lib.txt", "pub fn f() {}\n")))
			assert.HasError(t, err, "a parse of nothing proves nothing")
			assert.Contains(t, err.Error(), "no Rust file", "the refusal names why")
		})

		t.Run("returns the context's error for a context that ended", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			dir := laidOut(t, healthy())
			assert.HonoursCancellation(t, func(ctx context.Context) error { return adapter().Parse(ctx, dir) },
				"an ended context starts no rustfmt")
		})
	})

	t.Run("TypeCheck", func(t *testing.T) {
		t.Parallel()

		t.Run("passes a healthy crate", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			assert.NoError(t, adapter().TypeCheck(t.Context(), laidOut(t, healthy())), "the output checks")
		})

		t.Run("returns an error naming rustc's code", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			err := adapter().TypeCheck(t.Context(), laidOut(t, only(libFile, typeErrorSource)))
			assert.HasError(t, err, "a type error refuses")
			assert.Contains(t, err.Error(), "E0308", "the compiler's own words follow")
		})

		t.Run("returns an error for a type error in a test", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			dir := laidOut(t, with(rowTestFile, "#[test]\nfn broken() {\n    let _: i32 = \"text\";\n}\n"))
			assert.HasError(t, adapter().TypeCheck(t.Context(), dir), "every target is checked, the tests included")
		})

		t.Run("returns the context's error for a context that ended", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			dir := laidOut(t, healthy())
			assert.HonoursCancellation(t, func(ctx context.Context) error { return adapter().TypeCheck(ctx, dir) },
				"an ended context starts no cargo")
		})
	})

	t.Run("RunTests", func(t *testing.T) {
		t.Parallel()

		t.Run("counts a passing test", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			report, err := adapter().RunTests(t.Context(), laidOut(t, healthy()))
			assert.NoError(t, err, "the run completes")
			expect.Equal(t, report.Passed, 1, "one test passes")
			expect.Equal(t, report.Failed, 0, "no test fails")
		})

		t.Run("counts a failing test", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			report, err := adapter().RunTests(t.Context(), laidOut(t, with(rowTestFile, failingTestSource)))
			assert.NoError(t, err, "the run completes")
			assert.Equal(t, report.Failed, 1, "the test's failure counts")
		})

		t.Run("returns the output of a failing test", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			report, err := adapter().RunTests(t.Context(), laidOut(t, with(rowTestFile, failingTestSource)))
			assert.NoError(t, err, "the run completes")
			assert.Contains(t, report.Output, "the generated check disagrees", "the case's own words follow")
		})

		t.Run("counts an ignored test as skipped", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			report, err := adapter().RunTests(t.Context(),
				laidOut(t, with(rowTestFile, "#[test]\n#[ignore]\nfn later() {}\n")))
			assert.NoError(t, err, "the run completes")
			assert.Equal(t, report.Skipped, 1, "the ignored test counts as skipped")
		})

		t.Run("returns an empty report for a crate without a test", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			report, err := adapter().RunTests(t.Context(), laidOut(t, only(libFile, "pub struct Row;\n")))
			assert.NoError(t, err, "the run completes")
			assert.False(t, report.OK(), "a run of nothing is not a pass")
		})

		t.Run("returns an error for a crate that does not compile", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			_, err := adapter().RunTests(t.Context(), laidOut(t, only(libFile, typeErrorSource)))
			assert.HasError(t, err, "a crate that does not compile runs no test")
			assert.Contains(t, err.Error(), "E0308", "the compiler's refusal returns")
		})

		t.Run("returns the context's error for a context that ended", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			dir := laidOut(t, healthy())
			assert.HonoursCancellation(t, func(ctx context.Context) error {
				_, err := adapter().RunTests(ctx, dir)
				return err
			}, "an ended context runs no test")
		})
	})

	t.Run("Satisfies", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for a type implementing the trait", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			got, err := adapter().Satisfies(t.Context(), laidOut(t, healthy()), rowType, readerTrait)
			assert.NoError(t, err, "the probe runs")
			assert.True(t, got, "a Row implements Reader")
		})

		t.Run("reports false for a type not implementing the trait", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			got, err := adapter().Satisfies(t.Context(), laidOut(t, healthy()), plainType, readerTrait)
			assert.NoError(t, err, "the probe runs")
			assert.False(t, got, "a Plain does not implement Reader")
		})

		t.Run("restores the crate root after the probe", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			dir := laidOut(t, healthy())
			_, err := adapter().Satisfies(t.Context(), dir, rowType, readerTrait)
			assert.NoError(t, err, "the probe runs")
			files.HasContent(t, filepath.Join(dir, filepath.FromSlash(libFile)), libSource,
				"the root declares no probe module afterwards")
		})

		t.Run("returns an error for a trait the crate does not declare", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			_, err := adapter().Satisfies(t.Context(), laidOut(t, healthy()), rowType, nowhereTrait)
			assert.HasError(t, err, "an undefined trait is a broken question")
			assert.Contains(t, err.Error(), "other than the trait", "the refusal names why")
		})

		t.Run("returns an error for a crate that does not check", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			_, err := adapter().Satisfies(t.Context(), laidOut(t, with(libFile, typeErrorSource)), rowType, readerTrait)
			assert.HasError(t, err, "a broken crate returns an error, not false")
			assert.Contains(t, err.Error(), "the crate does not check", "the refusal names the crate")
		})

		t.Run("probes from the binary root of a crate without a library", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			g := only("src/main.rs", libSource+"\nfn main() {}\n")
			got, err := adapter().Satisfies(t.Context(), laidOut(t, g), rowType, readerTrait)
			assert.NoError(t, err, "the probe runs")
			assert.True(t, got, "the binary root declares the probe")
		})

		t.Run("returns an error for a crate without a root", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			g := only("src/bin/tool.rs", "fn main() {}\n")
			_, err := adapter().Satisfies(t.Context(), laidOut(t, g), rowType, readerTrait)
			assert.HasError(t, err, "the probe needs a root to declare it from")
			assert.Contains(t, err.Error(), "no src/lib.rs", "the refusal names the missing root")
		})

		t.Run("returns the context's error for a context that ended", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			dir := laidOut(t, healthy())
			assert.HonoursCancellation(t, func(ctx context.Context) error {
				_, err := adapter().Satisfies(ctx, dir, rowType, readerTrait)
				return err
			}, "an ended context asks the compiler nothing")
		})
	})
}

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

// laidOut lays a fixture out and removes it when the case ends.
func laidOut(t *testing.T, g toolchain.Generated) string {
	t.Helper()

	dir, err := adapter().Layout(g)
	assert.NoError(t, err, "the fixture lays out")
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}
