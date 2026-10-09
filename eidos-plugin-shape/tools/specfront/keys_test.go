// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/plugin/shape/tools/specfront"
	"go.dokimi.dev/eidos/sdk/meta"
)

// keyspace is the namespace of the keys of the spec frontend.
const keyspace = "shapespec"

// Keys registers every key that the spec frontend stamps, each with the
// type of its value.
func TestKeys(t *testing.T) {
	t.Parallel()

	t.Run("Keys", func(t *testing.T) {
		t.Parallel()

		t.Run("registers every key with the type of its value", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, specfront.Keys(r), "the keys register")
			tests := []struct {
				name       meta.KeyName
				registered func(*meta.Registry, meta.KeyName) bool
			}{
				{specfront.KeyForm, registeredAs[string]},
				{specfront.KeyResolve, registeredAs[string]},
				{specfront.KeyFrom, registeredAs[string]},
				{specfront.KeyDetected, registeredAs[bool]},
				{specfront.KeyDocumentary, registeredAs[bool]},
				{specfront.KeyRequired, registeredAs[bool]},
				{specfront.KeyCounterexample, registeredAs[bool]},
				{specfront.KeyYields, registeredAs[[]string]},
				{specfront.KeyRoles, registeredAs[[]string]},
				{specfront.KeyApplies, registeredAs[[]string]},
				{specfront.KeyExcludes, registeredAs[[]string]},
				{specfront.KeyAlsoOn, registeredAs[[]string]},
				{specfront.KeyMinimum, registeredAs[int64]},
				{specfront.KeyIndex, registeredAs[int64]},
			}
			for _, tt := range tests {
				expect.True(
					t,
					tt.registered(r, tt.name),
					"the registry contains "+string(tt.name)+" with the type of its value",
				)
				expect.Equal(
					t,
					tt.name.Namespace(),
					keyspace,
					"the key "+string(tt.name)+" is in the namespace of the frontend",
				)
			}
		})

		t.Run("returns the error of a namespace that another registrant claimed", func(t *testing.T) {
			t.Parallel()

			r := meta.NewRegistry()
			assert.NoError(t, r.ClaimNamespace(keyspace), "another registrant claims the namespace")
			assert.HasError(t, specfront.Keys(r), "the frontend cannot register into the namespace of another")
		})
	})
}

// registeredAs reports whether a registry contains a key of a name whose
// value has the type T.
func registeredAs[T meta.FactValue](r *meta.Registry, name meta.KeyName) bool {
	_, held := meta.Lookup[T](r, name)
	return held
}
