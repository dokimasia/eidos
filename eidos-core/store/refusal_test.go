// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package store_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

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

			for _, want := range []string{"store: ", store.FrozenWrite.String(), reason} {
				if !strings.Contains(got, want) {
					t.Fatalf("Error() = %q, want it to name %q", got, want)
				}
			}
		})

		t.Run("carries its code through a wrapping", func(t *testing.T) {
			t.Parallel()

			wrapped := fmt.Errorf("loading svc/store: %w",
				&store.RefusedError{Code: store.FrozenWrite, Msg: "refused"})

			var refused *store.RefusedError
			if !errors.As(wrapped, &refused) {
				t.Fatalf("errors.As(%v) = false, want the refusal", wrapped)
			}
			if refused.Code != store.FrozenWrite {
				t.Fatalf("Code = %v, want %v", refused.Code, store.FrozenWrite)
			}
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
					if !known || meaning == "" {
						t.Fatalf("Kernel().Meaning(%v) = %q, %t; want the registered meaning",
							code, meaning, known)
					}
					if code.Prefix != diag.KernelPrefix {
						t.Fatalf("%v is owned by %q, want the kernel's prefix",
							code, code.Prefix)
					}
				})
			}
		})

		t.Run("name distinct findings", func(t *testing.T) {
			t.Parallel()

			seen := make(map[diag.Code]string, len(storeCodes))
			for name, code := range storeCodes {
				if first, taken := seen[code]; taken {
					t.Fatalf("%s and %s both spell %v", first, name, code)
				}
				seen[code] = name
			}
		})
	})
}
