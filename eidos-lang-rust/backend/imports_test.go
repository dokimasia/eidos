// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/rust/backend"
	"go.dokimi.dev/eidos/sdk/render"
)

// The use block is pinned byte for byte.
func TestImports(t *testing.T) {
	t.Parallel()

	t.Run("renders sorted use statements", func(t *testing.T) {
		t.Parallel()

		var set render.ImportSet
		set.Add("svc/store")
		set.Add("std/collections")
		assert.Equal(t, backend.Imports(&set),
			"use std::collections;\nuse svc::store;\n\n",
			"double colons for slashes, sorted, a blank line after")
	})

	t.Run("binds a named entry beside a bare one", func(t *testing.T) {
		t.Parallel()

		var set render.ImportSet
		set.AddNamed("std/collections", "HashMap")
		set.Add("svc/store")
		assert.Equal(t, backend.Imports(&set),
			"use std::collections::HashMap;\nuse svc::store;\n\n",
			"path::Name for the bound form, the path alone for the bare")
	})

	t.Run("a file importing nothing renders no block", func(t *testing.T) {
		t.Parallel()

		var set render.ImportSet
		assert.Equal(t, backend.Imports(&set), "", "no paths, no block")
	})
}
