// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"io/fs"
	"os"
	"testing"

	golang "go.dokimi.dev/eidos/conformance/lang/go"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/core/workspace/pipelinetest"
	"go.dokimi.dev/eidos/core/workspace/workspacetest"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
)

// The end-to-end fixture's trees, the workspace and the one package of
// the standard library it reads, and the directory of the goldens of the
// files that the plan generates.
const (
	pipelineTree = "testdata/pipeline/tree"
	pipelineRoot = "testdata/pipeline/goroot"
	pipelineWant = "testdata/pipeline/want"
)

// The stub half of the one-declaration document runs end to end: the Go
// frontend loads an interface under //+acme:stub tag=test, stubgen
// mirrors its methods onto a double on pointer receivers, the audit
// weaver calls audit first in each, and the Go backend writes the double
// beside its source as svc/store_stub_test.go.
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
	})
}

// pipelineFixture returns the end-to-end fixture: the tree, the store of
// the standard library, the composition and the plan of the package, and
// the goldens under testdata/pipeline/want.
func pipelineFixture(t *testing.T) workspacetest.Fixture {
	t.Helper()

	return workspacetest.Fixture{
		Tree:    os.DirFS(pipelineTree),
		Stores:  map[string]fs.FS{gofrontend.GoRootStore: os.DirFS(pipelineRoot)},
		Compose: func(root string) (*workspace.Builder, error) { return golang.ComposePipeline(root), nil },
		Plans:   golang.PipelinePlans,
		Want:    workspacetest.Want(t, pipelineWant),
	}
}
