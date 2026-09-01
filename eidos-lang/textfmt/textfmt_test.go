// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package textfmt_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/textfmt"
)

// The normalizer owns the whitespace a template leaks, and nothing
// else: content bytes pass through untouched.
func TestNormalize(t *testing.T) {
	t.Parallel()

	t.Run("strips trailing whitespace and collapses blank runs", func(t *testing.T) {
		t.Parallel()

		got, err := textfmt.Normalize([]byte("a; \t\n\n\n\nb;\n"))
		assert.NoError(t, err, "the text normalizes")
		assert.Equal(t, string(got), "a;\n\nb;\n",
			"trailing whitespace gone, one blank line where three were")
	})

	t.Run("drops leading blanks and ends with one newline", func(t *testing.T) {
		t.Parallel()

		got, err := textfmt.Normalize([]byte("\n\nclass A {}\n\n\n"))
		assert.NoError(t, err, "the text normalizes")
		assert.Equal(t, string(got), "class A {}\n",
			"no leading blank, exactly one final newline")
	})

	t.Run("passes empty input through", func(t *testing.T) {
		t.Parallel()

		got, err := textfmt.Normalize(nil)
		assert.NoError(t, err, "nothing normalizes")
		assert.Length(t, got, 0, "to nothing")
	})

	t.Run("never touches interior content", func(t *testing.T) {
		t.Parallel()

		got, err := textfmt.Normalize([]byte("  indented\ntext  here\n"))
		assert.NoError(t, err, "the text normalizes")
		assert.Equal(t, string(got), "  indented\ntext  here\n",
			"indentation and interior spacing stay the template's")
	})
}
