// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"fmt"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/store"
)

// refusalAllocs is the text of a refusal: the code's spelling and the
// joined message.
const refusalAllocs = 2

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

		t.Run("returns its code with its reason", func(t *testing.T) {
			t.Parallel()

			const reason = "svc/store is added after Freeze"
			assert.That(t, (&store.RefusedError{Code: store.FrozenWrite, Msg: reason}).Error()).
				HasPrefix("store: ", "the error has the package prefix").
				Contains(store.FrozenWrite.String(), "names its code").
				Contains(reason, "and states its reason")
		})

		t.Run("returns its code to errors.As through a wrapping", func(t *testing.T) {
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

// A refusal's text allocates the code's spelling and the joined message
// in the ordinary run, which runs no benchmark. The check runs alone,
// because the count includes every goroutine's allocations.
func TestRefusalAllocs(t *testing.T) {
	refused := &store.RefusedError{Code: store.FrozenWrite, Msg: "svc/store is added after Freeze"}
	var got string
	assert.MaxAllocs(
		t,
		func() { got = refused.Error() },
		refusalAllocs,
		"Error allocates the code's spelling and the text",
	)
	assert.HasPrefix(t, got, "store: ", "Error returns the prefixed text")
}

// BenchmarkRefusal measures the text of a refusal, which a run spells
// once per refused write.
func BenchmarkRefusal(b *testing.B) {
	b.Run("Error", func(b *testing.B) {
		refused := &store.RefusedError{Code: store.FrozenWrite, Msg: "svc/store is added after Freeze"}
		c := bench.Start(b).MaxAllocs(refusalAllocs)
		defer c.End()
		var got string
		for c.Loop() {
			got = refused.Error()
		}
		assert.HasPrefix(b, got, "store: ", "Error returns the prefixed text")
	})
}
