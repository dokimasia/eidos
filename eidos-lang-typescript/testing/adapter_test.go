// Copyright ThesmOS B.V. 2026
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

	typescript "go.dokimi.dev/eidos/lang/typescript"
	tstesting "go.dokimi.dev/eidos/lang/typescript/testing"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The files a healthy fixture contains, the configuration tsc reads,
// and the type expressions the satisfaction cases pass to Satisfies.
const (
	rowFile     = "row.ts"
	rowTestFile = "row.test.ts"
	configFile  = "tsconfig.json"
	rowType     = `import("./row").Row`
	plainType   = `import("./row").Plain`
	readerType  = `import("./row").Reader`
	nowhereType = `import("./row").Nowhere`
)

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

		t.Run("returns TypeScript", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, adapter().Lang(), typescript.Lang, "the language the assertions name")
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

			assert.Contains(t, files.Read(t, filepath.Join(laidOut(t, healthy()), rowFile)), "export class Row",
				"the generated file is on disk with its own bytes")
		})

		t.Run("writes a strict tsconfig.json for a fixture stating none", func(t *testing.T) {
			t.Parallel()

			assert.Contains(t, files.Read(t, filepath.Join(laidOut(t, healthy()), configFile)), `"strict": true`,
				"the project checks strictly")
		})

		t.Run("keeps a tsconfig.json the output wrote", func(t *testing.T) {
			t.Parallel()

			own := `{"compilerOptions": {"strict": false}}`
			files.HasContent(t, filepath.Join(laidOut(t, with(configFile, own)), configFile), own,
				"the harness states nothing the output already did")
		})

		t.Run("writes a nested path", func(t *testing.T) {
			t.Parallel()

			dir := laidOut(t, only("gen/deep/row.ts", "export {};\n"))
			files.IsFile(t, filepath.Join(dir, "gen", "deep", rowFile), "the nested path is created")
		})

		t.Run("returns an error for a path climbing out of the scratch project", func(t *testing.T) {
			t.Parallel()

			_, err := adapter().Layout(only("../escape.ts", "export {};\n"))
			assert.HasError(t, err, "a fixture is not a place to write from")
			assert.Contains(t, err.Error(), "climbs out", "the refusal names the escape")
		})
	})

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("reads the syntax of a healthy project", func(t *testing.T) {
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
			err := adapter().Parse(t.Context(), laidOut(t, only(rowFile, "export const x: number = ;\n")))
			assert.HasError(t, err, "a syntax error refuses")
			assert.Contains(t, err.Error(), rowFile, "the refusal names the file")
		})

		t.Run("reads the syntax of a project whose only error is a type error", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			err := adapter().Parse(t.Context(), laidOut(t, only(rowFile, "export const x: number = \"text\";\n")))
			assert.NoError(t, err, "a type error is no syntax error")
		})

		t.Run("returns an error for a configuration tsc cannot read", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			err := adapter().Parse(t.Context(),
				laidOut(t, with(configFile, `{"compilerOptions": {"eidosNoSuchOption": true}}`)))
			assert.HasError(t, err, "an unknown option leaves nothing parsed")
			assert.Contains(t, err.Error(), "TS5023", "the compiler's own code follows")
		})

		t.Run("returns an error for a project with no TypeScript file", func(t *testing.T) {
			t.Parallel()

			err := adapter().Parse(t.Context(), laidOut(t, only("row.js", "export {};\n")))
			assert.HasError(t, err, "a parse of nothing proves nothing")
			assert.Contains(t, err.Error(), "no TypeScript file", "the refusal names why")
		})

		t.Run("returns the context's error for a context that ended", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			dir := laidOut(t, healthy())
			assert.HonoursCancellation(t, func(ctx context.Context) error { return adapter().Parse(ctx, dir) },
				"an ended context starts no tsc")
		})
	})

	t.Run("TypeCheck", func(t *testing.T) {
		t.Parallel()

		t.Run("passes a healthy project", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			assert.NoError(t, adapter().TypeCheck(t.Context(), laidOut(t, healthy())), "the output is sound")
		})

		t.Run("returns an error naming the checker's code", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			err := adapter().TypeCheck(t.Context(), laidOut(t, only(rowFile, "export const x: number = \"text\";\n")))
			assert.HasError(t, err, "a type error refuses")
			assert.Contains(t, err.Error(), "TS2322", "the compiler's own words follow")
		})

		t.Run("returns an error for a type error in a test module", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			err := adapter().TypeCheck(t.Context(),
				laidOut(t, with(rowTestFile, "export const x: number = \"text\";\n")))
			assert.HasError(t, err, "a test module is checked too")
		})

		t.Run("returns the context's error for a context that ended", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			dir := laidOut(t, healthy())
			assert.HonoursCancellation(t, func(ctx context.Context) error { return adapter().TypeCheck(ctx, dir) },
				"an ended context starts no tsc")
		})
	})

	t.Run("RunTests", func(t *testing.T) {
		t.Parallel()

		t.Run("counts a passing test module", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			report, err := adapter().RunTests(t.Context(), laidOut(t, healthy()))
			assert.NoError(t, err, "the run completes")
			expect.Equal(t, report.Passed, 1, "one module passes")
			expect.Equal(t, report.Failed, 0, "no module fails")
		})

		t.Run("counts a failing test module", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			report, err := adapter().RunTests(t.Context(), laidOut(t, with(rowTestFile,
				"throw new Error(\"the generated check disagrees\");\n")))
			assert.NoError(t, err, "the run completes")
			assert.Equal(t, report.Failed, 1, "the module's failure counts")
		})

		t.Run("returns the output of a failing test module", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			report, err := adapter().RunTests(t.Context(), laidOut(t, with(rowTestFile,
				"throw new Error(\"the generated check disagrees\");\n")))
			assert.NoError(t, err, "the run completes")
			assert.Contains(t, report.Output, "the generated check disagrees", "the case's own words follow")
		})

		t.Run("returns an empty report for a project without a test module", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			report, err := adapter().RunTests(t.Context(), laidOut(t, only(rowFile, "export class Row {}\n")))
			assert.NoError(t, err, "the run completes")
			assert.False(t, report.OK(), "a run of nothing is not a pass")
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

		t.Run("reports true for a type assignable to the contract", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			got, err := adapter().Satisfies(t.Context(), laidOut(t, healthy()), rowType, readerType)
			assert.NoError(t, err, "the probe runs")
			assert.True(t, got, "a Row implements Reader")
		})

		t.Run("reports false for a type missing the contract's member", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			got, err := adapter().Satisfies(t.Context(), laidOut(t, healthy()), plainType, readerType)
			assert.NoError(t, err, "the probe runs")
			assert.False(t, got, "a Plain declares no read")
		})

		t.Run("returns an error for a contract no module declares", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			_, err := adapter().Satisfies(t.Context(), laidOut(t, healthy()), rowType, nowhereType)
			assert.HasError(t, err, "an undefined contract is a broken question")
			assert.Contains(t, err.Error(), "other than the contract", "the refusal names why")
		})

		t.Run("returns an error for a project that does not type-check", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			_, err := adapter().Satisfies(t.Context(),
				laidOut(t, with(rowTestFile, "export const x: number = \"text\";\n")), rowType, readerType)
			assert.HasError(t, err, "a broken project returns an error, not false")
			assert.Contains(t, err.Error(), "the project does not type-check", "the refusal names the project")
		})

		t.Run("returns the context's error for a context that ended", func(t *testing.T) {
			t.Parallel()

			if !toolchain.Require(t, adapter()) {
				return
			}
			dir := laidOut(t, healthy())
			assert.HonoursCancellation(t, func(ctx context.Context) error {
				_, err := adapter().Satisfies(ctx, dir, rowType, readerType)
				return err
			}, "an ended context asks the checker nothing")
		})
	})
}

// healthy returns generated output that parses, type-checks and whose
// one test module passes.
func healthy() toolchain.Generated {
	return toolchain.Generated{Files: map[string][]byte{
		rowFile: []byte(`// Reader is what a Row satisfies.
export interface Reader {
  read(): string;
}

// Row is a generated record.
export class Row implements Reader {
  constructor(public readonly name: string) {}

  read(): string {
    return this.name;
  }
}

// Plain declares no read method.
export class Plain {
  constructor(public readonly id: number) {}
}
`),
		rowTestFile: []byte(`import { Row } from "./row";

if (new Row("a").read() !== "a") {
  throw new Error("read returns the name");
}
`),
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
// module.
func only(path, body string) toolchain.Generated {
	return toolchain.Generated{Files: map[string][]byte{path: []byte(body)}}
}

// adapter is the harness under test.
func adapter() toolchain.Adapter { return tstesting.New() }

// laidOut lays a fixture out and removes it when the case ends.
func laidOut(t *testing.T, g toolchain.Generated) string {
	t.Helper()

	dir, err := adapter().Layout(g)
	assert.NoError(t, err, "the fixture lays out")
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}
