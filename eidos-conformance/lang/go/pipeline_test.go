// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/conformance/lang/go"
	"go.dokimi.dev/eidos/core/workspace/pipelinetest"
	gofrontend "go.dokimi.dev/eidos/lang/go/frontend"
)

// The end-to-end fixture's trees, the workspace and the one package of
// the standard library it reads, and the file the plan generates with
// its golden.
const (
	pipelineTree = "testdata/pipeline/tree"
	pipelineRoot = "testdata/pipeline/goroot"
	stubFile     = "svc/store_stub_test.go"
	stubWant     = "testdata/pipeline/want/" + stubFile
)

// The stub half of the one-declaration document runs end to end: the Go
// frontend loads an interface under //+acme:stub tag=test, stubgen
// mirrors its methods onto a double on pointer receivers, the audit
// weaver calls audit first in each, and the Go backend writes the double
// beside its source as svc/store_stub_test.go.
func TestPipeline(t *testing.T) {
	t.Parallel()

	t.Run("ComposeStubs", func(t *testing.T) {
		t.Parallel()

		t.Run("passes the pipeline suite over the stub fixture", func(t *testing.T) {
			t.Parallel()

			want, err := os.ReadFile(filepath.FromSlash(stubWant))
			assert.NoError(t, err, "the generated file's golden reads")
			pipelinetest.RunPipelineSuite(t, pipelinetest.Fixture{
				Tree:    os.DirFS(pipelineTree),
				Stores:  map[string]fs.FS{gofrontend.GoRootStore: os.DirFS(pipelineRoot)},
				Compose: golang.ComposeStubs,
				Want:    map[string][]byte{stubFile: want},
			})
		})
	})
}
