// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript_test

import (
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/conformance/lang/typescript"
	"go.dokimi.dev/eidos/core/workspace"
	"go.dokimi.dev/eidos/core/workspace/pipelinetest"
	tstesting "go.dokimi.dev/eidos/lang/typescript/testing"
	"go.dokimi.dev/eidos/sdk/toolchain"
)

// The TypeScript pipeline fixture's tree, the module that it stubs, and
// the stub that the plan generates with its golden.
const (
	pipelineTree = "testdata/pipeline/tree"
	storeModule  = "svc/store.ts"
	stubModule   = "svc/store.stub.test.ts"
	stubWant     = "testdata/pipeline/want/" + stubModule
)

// TestPipeline checks that the TypeScript satellite runs end to end: the
// frontend loads an interface under // +acme:stub tag=test, stubgen
// doubles it as a class, and the backend writes the class beside its
// source with an import of the interface from './store'.
func TestPipeline(t *testing.T) {
	t.Parallel()

	t.Run("ComposeStubs", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the pipeline suite over the stub fixture", func(t *testing.T) {
			t.Parallel()

			want, err := os.ReadFile(filepath.FromSlash(stubWant))
			assert.NoError(t, err, "the golden of the stub reads")
			pipelinetest.RunPipelineSuite(t, pipelinetest.Fixture{
				Tree:    os.DirFS(pipelineTree),
				Compose: typescript.ComposeStubs,
				Want:    map[string][]byte{stubModule: want},
			})
		})

		t.Run("writes a stub that tsc type-checks beside its source", func(t *testing.T) {
			t.Parallel()

			adapter := tstesting.New()
			if !toolchain.Require(t, adapter) {
				return
			}
			root := t.TempDir()
			assert.NoError(t, os.CopyFS(root, os.DirFS(pipelineTree)), "the tree copies")
			w, err := typescript.ComposeStubs(root)
			assert.NoError(t, err, "the composition builds")
			_, err = w.Run(t.Context(), workspace.Input{Tree: os.DirFS(root)})
			assert.NoError(t, err, "the run is clean")
			files := map[string][]byte{}
			for _, path := range []string{storeModule, stubModule} {
				b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
				assert.NoError(t, err, path+" reads")
				files[path] = b
			}
			toolchain.AssertTypeChecks(t.Context(), t, adapter, toolchain.Generated{Files: files})
		})
	})
}
