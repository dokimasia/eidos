// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"fmt"
	"testing"

	"go.dokimi.dev/assert"
	"go.yaml.in/yaml/v3"

	"go.dokimi.dev/eidos/cli/internal/config"
)

// sizeFault is the format of the fault for a value that is not a size.
const sizeFault = `line 1: %q is not a size: use a number of bytes, or a number followed by KiB, MiB or GiB`

// A size is a number of bytes, or a number followed by a binary unit.
func TestBytes(t *testing.T) {
	t.Parallel()

	t.Run("UnmarshalYAML", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want config.Bytes
		}{
			{name: "decodes a number of bytes", give: "1024", want: 1024},
			{name: "decodes 0", give: "0", want: 0},
			{name: "decodes a number of KiB", give: "4KiB", want: 4096},
			{name: "decodes a number of MiB", give: "512MiB", want: 536870912},
			{name: "decodes a number of GiB", give: "2GiB", want: 2147483648},
			{name: "decodes the largest number of GiB", give: "8589934591GiB", want: 9223372035781033984},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var got config.Bytes
				assert.NoError(t, yaml.Unmarshal([]byte(tt.give), &got), "the size decodes")
				assert.Equal(t, got, tt.want, "the size has the number of bytes")
			})
		}

		faults := []struct {
			name string
			give string
			want string
		}{
			{name: "returns a TypeError for a negative number", give: "-1", want: "-1"},
			{
				name: "returns a TypeError for a number above the largest int64",
				give: "9223372036854775808",
				want: "9223372036854775808",
			},
			{name: "returns a TypeError for an unknown unit", give: "12XB", want: "12XB"},
			{name: "returns a TypeError for a unit without a number", give: "KiB", want: "KiB"},
			{name: "returns a TypeError for a negative number of a unit", give: "-1KiB", want: "-1KiB"},
			{
				name: "returns a TypeError for a size above the largest int64",
				give: "8589934592GiB",
				want: "8589934592GiB",
			},
			{name: "returns a TypeError for a fraction", give: "1.5", want: "1.5"},
			{name: "returns a TypeError for a list", give: "[1]", want: ""},
		}
		for _, tt := range faults {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var got config.Bytes
				typed := assert.ErrorAs[*yaml.TypeError](t, yaml.Unmarshal([]byte(tt.give), &got),
					"the decoder reports the fault as a type error")
				assert.Equal(t, typed.Errors, []string{fmt.Sprintf(sizeFault, tt.want)},
					"the error quotes the value")
			})
		}
	})
}
