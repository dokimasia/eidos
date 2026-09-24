// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package naming_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/naming"
)

// The filename parts and the snake-cased shape are pinned once for
// every target that builds a filename from them.
func TestFilename(t *testing.T) {
	t.Parallel()

	t.Run("FilenameParts", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, naming.FilenameParts("svc/store/row.go", "stub", "test"),
			[]string{"row", "stub", "test"},
			"the key's stem without its extension, then the word and the tag")
		assert.Equal(t, naming.FilenameParts("", "HTTPClient", ""),
			[]string{"HTTPClient"},
			"a plan unit has no key, and an empty part is left out")
		assert.Empty(t, naming.FilenameParts("", "", ""), "no parts, no filename")
	})

	t.Run("SnakeFilename", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, naming.SnakeFilename("svc/store/row.go", "stub", "", ".rs"),
			"row_stub.rs", "the stem drops its extension and joins the word")
		assert.Equal(t, naming.SnakeFilename("", "HTTPClient", "test", ".go"),
			"http_client_test.go", "a plan unit is the word and tag, snake-cased as one")
	})
}
