// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspacetest

import (
	"io/fs"
	"os"
	"path"
	"slices"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/toolchain"
	"go.dokimi.dev/eidos/core/workspace/internal/rundir"
)

// Generated returns the output of the run in root as the input of a
// toolchain check. The output has every file that the record of brand in
// root lists with one of the extensions exts, read from root, and every
// listed file where exts is empty. Its sources are sources. Generated
// stops the test where the record lists no file, and where a listed file
// does not read.
func Generated(tb assert.TB, root string, brand output.Brand, sources fs.FS, exts ...string) toolchain.Generated {
	tb.Helper()

	record := rundir.Record(tb, root, brand)
	assert.NotEmpty(tb, record.Files, "the record of the run in "+root+" lists its files")
	out := toolchain.Generated{Files: map[string][]byte{}, Sources: sources}
	for _, e := range record.Files {
		if len(exts) > 0 && !slices.Contains(exts, path.Ext(e.Path)) {
			continue
		}
		b, err := os.ReadFile(rundir.Path(root, e.Path))
		assert.NoError(tb, err, "the run writes "+e.Path)
		out.Files[e.Path] = b
	}
	return out
}
