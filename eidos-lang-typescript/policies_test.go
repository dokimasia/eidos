// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// A config document and a target directive name the policies and their
// choices by these spellings, so each spelling is pinned.
func TestPolicies(t *testing.T) {
	t.Parallel()

	t.Run("Policies", func(t *testing.T) {
		t.Parallel()

		t.Run("returns specs that resolve into the target's policy", func(t *testing.T) {
			t.Parallel()

			_, err := plugin.NewPolicy(typescript.Target, typescript.Policies(), nil)
			assert.NoError(t, err, "every spec is one that the kit accepts")
		})

		tests := []struct {
			name     string
			key      plugin.PolicyKey
			spelling string
			choices  []plugin.Choice
			def      plugin.Choice
		}{
			{
				name: "returns int64 with bigint as its default", key: typescript.Int64, spelling: "typescript.int64",
				choices: []plugin.Choice{"bigint", "string", "number"}, def: "bigint",
			},
			{
				name: "returns absent with undefined as its default", key: typescript.Absent,
				spelling: "typescript.absent", choices: []plugin.Choice{"undefined", "null"}, def: "undefined",
			},
			{
				name: "returns timestamp with Date as its default", key: typescript.Timestamp,
				spelling: "typescript.timestamp", choices: []plugin.Choice{"Date", "string"}, def: "Date",
			},
			{
				name: "returns bytes with Uint8Array as its default", key: typescript.Bytes,
				spelling: "typescript.bytes", choices: []plugin.Choice{"Uint8Array", "string"}, def: "Uint8Array",
			},
			{
				name: "returns duration with string as its default", key: typescript.Duration,
				spelling: "typescript.duration", choices: []plugin.Choice{"string", "number"}, def: "string",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var spec plugin.PolicySpec
				for _, s := range typescript.Policies() {
					if s.Key == tt.key {
						spec = s
					}
				}
				expect.Equal(t, string(spec.Key), tt.spelling, "the key's spelling")
				expect.Equal(t, spec.Choices, tt.choices, "the choices, in the order they document")
				expect.Equal(t, spec.Default, tt.def, "the default is the spelling the policy documents")
				expect.NotEmpty(t, spec.Doc, "the spec documents its key")
			})
		}

		t.Run("returns five policies", func(t *testing.T) {
			t.Parallel()

			assert.Length(t, typescript.Policies(), 5, "int64, absent, timestamp, bytes and duration")
		})
	})
}
