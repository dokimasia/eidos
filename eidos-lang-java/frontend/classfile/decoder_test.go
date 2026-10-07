// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package classfile_test

import (
	"fmt"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/java/frontend/classfile"
)

// The prefix every reader error starts with, which the package's name
// opens.
const errorPrefix = "classfile: "

// The decoder reads every item in bounds, so a class file cut short at
// any byte is malformed and never a panic.
func TestDecoder(t *testing.T) {
	t.Parallel()

	t.Run("take", func(t *testing.T) {
		t.Parallel()

		t.Run("returns ErrMalformed for a class file cut short at any byte", func(t *testing.T) {
			t.Parallel()

			data := fixtureBytes(t, classesDir, boxFile)
			for n := range len(data) {
				_, err := classfile.Parse(data[:n])
				assert.ErrorIs(t, err, classfile.ErrMalformed,
					fmt.Sprintf("the first %d of %d bytes are malformed", n, len(data)))
			}
		})
	})

	t.Run("fail", func(t *testing.T) {
		t.Parallel()

		t.Run("reports the first fault a class file has", func(t *testing.T) {
			t.Parallel()

			_, err := classfile.Parse([]byte{0, 0})
			assert.HasError(t, err, "two bytes are no class file")
			assert.Contains(t, err.Error(), "2 bytes are left where 4 are read",
				"the short magic number, and not the faults after it")
		})

		t.Run("opens its error with the package's name", func(t *testing.T) {
			t.Parallel()

			_, err := classfile.Parse(nil)
			assert.HasError(t, err, "no bytes are no class file")
			assert.HasPrefix(t, err.Error(), errorPrefix, "the error names its package")
		})
	})
}
