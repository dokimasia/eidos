// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package naming_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/naming"
)

// The shared filename shape is pinned once for the snake-cased
// languages.
func TestSnakeFilename(t *testing.T) {
	t.Parallel()

	assert.Equal(t, naming.SnakeFilename("svc/store/row.go", "stub", "", ".rs"),
		"row_stub.rs", "the stem drops its extension and joins the word")
	assert.Equal(t, naming.SnakeFilename("", "HTTPClient", "test", ".go"),
		"http_client_test.go", "a plan unit is the word and tag, snake-cased as one")
}
