// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript_test

import (
	"os"
	"testing"

	"go.dokimi.dev/eidos/conformance/lang/typescript"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/core/workspace/pipelinetest"
	"go.dokimi.dev/eidos/core/workspace/workspacetest"
	tstesting "go.dokimi.dev/eidos/lang/typescript/testing"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The TypeScript pipeline fixture's tree, the directory of the goldens of
// the files that its plan generates, and the extension of a TypeScript
// file.
const (
	pipelineTree = "testdata/pipeline/tree"
	pipelineWant = "testdata/pipeline/want"
	tsExt        = ".ts"
)

// TestPipeline checks that the TypeScript satellite runs end to end: the
// frontend loads an interface under // +acme:stub tag=test, stubgen
// doubles it as a class, and the backend writes the class beside its
// source with an import of the interface from './store'.
func TestPipeline(t *testing.T) {
	t.Parallel()

	f := pipelineFixture(t)

	t.Run("ComposePipeline", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the pipeline suite over the stub fixture", func(t *testing.T) {
			t.Parallel()

			pipelinetest.RunPipelineSuite(t, f)
		})

		t.Run("writes the goldens of the stub fixture", func(t *testing.T) {
			t.Parallel()

			workspacetest.AssertGolden(t, f, t.TempDir(), pipelineWant)
		})

		t.Run("writes a stub that tsc type-checks beside its source", func(t *testing.T) {
			t.Parallel()

			adapter := tstesting.New()
			if !toolchain.Require(t, adapter) {
				return
			}
			root := t.TempDir()
			pipelinetest.AssertClean(t, f, root)
			toolchain.AssertTypeChecks(t.Context(), t, adapter,
				workspacetest.Generated(t, root, typescript.Brand, f.Tree, tsExt))
		})
	})
}

// pipelineFixture returns the TypeScript pipeline fixture: the tree, the
// composition and the plan of the package, and the goldens under
// testdata/pipeline/want.
func pipelineFixture(t *testing.T) workspacetest.Fixture {
	t.Helper()

	return workspacetest.Fixture{
		Tree:    os.DirFS(pipelineTree),
		Compose: func(root string) (*workspace.Builder, error) { return typescript.ComposePipeline(root), nil },
		Plans:   typescript.PipelinePlans,
		Want:    workspacetest.Want(t, pipelineWant),
	}
}
