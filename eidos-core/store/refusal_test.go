// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package store_test

import (
	"fmt"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/store"
)

// storeCodes are every code this package refuses under.
var storeCodes = map[string]diag.Code{
	"FrozenWrite":      store.FrozenWrite,
	"DuplicatePackage": store.DuplicatePackage,
	"UnfrozenRead":     store.UnfrozenRead,
}

// A refusal carries the code a consumer scripts against, so the code
// has to survive both the error text and a wrapping.
func TestRefusal(t *testing.T) {
	t.Parallel()

	t.Run("Error", func(t *testing.T) {
		t.Parallel()

		t.Run("names its code and its reason", func(t *testing.T) {
			t.Parallel()

			const reason = "svc/store is added after Freeze"
			got := (&store.RefusedError{Code: store.FrozenWrite, Msg: reason}).Error()

			assert.HasPrefix(t, got, "store: ", "the error carries the package prefix")
			assert.Contains(t, got, store.FrozenWrite.String(), "names its code")
			assert.Contains(t, got, reason, "and states its reason")
		})

		t.Run("carries its code through a wrapping", func(t *testing.T) {
			t.Parallel()

			wrapped := fmt.Errorf("loading svc/store: %w",
				&store.RefusedError{Code: store.FrozenWrite, Msg: "refused"})

			refused := assert.ErrorAs[*store.RefusedError](t, wrapped,
				"the refusal survives a wrapping")
			assert.Equal(t, refused.Code, store.FrozenWrite,
				"with the code consumers script against")
		})
	})

	t.Run("codes", func(t *testing.T) {
		t.Parallel()

		t.Run("are registered in the kernel registry", func(t *testing.T) {
			t.Parallel()

			for name, code := range storeCodes {
				t.Run(name, func(t *testing.T) {
					t.Parallel()

					meaning, known := diag.Kernel().Meaning(code)
					assert.True(t, known, "the code is registered in the kernel registry")
					assert.NotEqual(t, meaning, "", "with a meaning the index anchors to")
					assert.Equal(t, code.Prefix, diag.KernelPrefix,
						"under the kernel's own prefix")
				})
			}
		})

		t.Run("name distinct findings", func(t *testing.T) {
			t.Parallel()

			seen := make(map[diag.Code]struct{}, len(storeCodes))
			for _, code := range storeCodes {
				seen[code] = struct{}{}
			}
			assert.Length(t, seen, len(storeCodes),
				"every refusal reports under its own code")
		})
	})
}
