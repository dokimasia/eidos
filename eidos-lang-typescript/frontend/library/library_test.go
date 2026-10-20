// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package library_test

import (
	"io/fs"
	"path"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/typescript/frontend/library"
)

// The facts of the release that the embed pins: the number of its
// declaration files, the suffix of each, and a line of the licence in the
// header of each.
const (
	releaseFiles   = 108
	declarationExt = ".d.ts"
	licenceLine    = `Licensed under the Apache License, Version 2.0 (the "License"); you may not use`
)

// The embed is the library of one release, each file with the licence
// header of its author.
func TestLibrary(t *testing.T) {
	t.Parallel()

	t.Run("Files", func(t *testing.T) {
		t.Parallel()

		entries, err := fs.ReadDir(library.Files, library.Dir)
		assert.NoError(t, err, "the embed lists its directory")

		t.Run("embeds every declaration file of the release", func(t *testing.T) {
			t.Parallel()

			assert.Length(t, entries, releaseFiles, "the embed has one file for each library of the release")
		})

		t.Run("embeds each file with the licence header of Microsoft", func(t *testing.T) {
			t.Parallel()

			for _, e := range entries {
				data, err := fs.ReadFile(library.Files, path.Join(library.Dir, e.Name()))
				assert.NoError(t, err, "the embed reads "+e.Name())
				expect.HasSuffix(t, e.Name(), declarationExt, e.Name()+" is a declaration file")
				expect.Contains(t, string(data), licenceLine, e.Name()+" has the header of the Apache License 2.0")
			}
		})
	})
}
