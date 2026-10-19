// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protobuf_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/protobuf"
)

// TestMarkers pins the key of the tag entry of a wire number and the
// spelling of each marker, because a generator outside this module states
// them in the emit model.
func TestMarkers(t *testing.T) {
	t.Parallel()

	t.Run("NumberTag", func(t *testing.T) {
		t.Parallel()

		t.Run("spells number", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, protobuf.NumberTag, "number", "the key of the tag entry of a wire number")
		})
	})

	t.Run("Marker", func(t *testing.T) {
		t.Parallel()

		spellings := []struct {
			name string
			give protobuf.Marker
			want string
		}{
			{name: "spells the syntax marker", give: protobuf.SyntaxMarker, want: "protobuf:syntax"},
			{name: "spells the package marker", give: protobuf.PackageMarker, want: "protobuf:package"},
			{name: "spells the import marker", give: protobuf.ImportMarker, want: "protobuf:import"},
			{name: "spells the required marker", give: protobuf.RequiredMarker, want: "protobuf:required"},
			{name: "spells the delimited marker", give: protobuf.DelimitedMarker, want: "protobuf:delimited"},
			{name: "spells the number marker", give: protobuf.NumberMarker, want: "protobuf:number"},
			{name: "spells the reserved marker", give: protobuf.ReservedMarker, want: "protobuf:reserved"},
			{name: "spells the extensions marker", give: protobuf.ExtensionsMarker, want: "protobuf:extensions"},
			{name: "spells the closed marker", give: protobuf.ClosedMarker, want: "protobuf:closed"},
			{name: "spells the local marker", give: protobuf.LocalMarker, want: "protobuf:local"},
		}
		for _, tt := range spellings {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, string(tt.give), tt.want, "the name of the annotation of the marker")
			})
		}

		t.Run("Valid", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give protobuf.Marker
				want bool
			}{
				{name: "reports true for the syntax marker", give: protobuf.SyntaxMarker, want: true},
				{name: "reports true for the package marker", give: protobuf.PackageMarker, want: true},
				{name: "reports true for the import marker", give: protobuf.ImportMarker, want: true},
				{name: "reports true for the required marker", give: protobuf.RequiredMarker, want: true},
				{name: "reports true for the delimited marker", give: protobuf.DelimitedMarker, want: true},
				{name: "reports true for the number marker", give: protobuf.NumberMarker, want: true},
				{name: "reports true for the reserved marker", give: protobuf.ReservedMarker, want: true},
				{name: "reports true for the extensions marker", give: protobuf.ExtensionsMarker, want: true},
				{name: "reports true for the closed marker", give: protobuf.ClosedMarker, want: true},
				{name: "reports true for the local marker", give: protobuf.LocalMarker, want: true},
				{name: "reports false for a name with the prefix that no marker has", give: "protobuf:weight"},
				{name: "reports false for the prefix alone", give: protobuf.MarkerPrefix},
				{name: "reports false for an option name", give: "deprecated"},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, tt.give.Valid(), tt.want, "Valid reports whether the satellite declares the marker")
				})
			}
		})
	})
}

// TestMarkersAllocs checks that Valid allocates nothing, in the ordinary
// run, which runs no benchmark.
func TestMarkersAllocs(t *testing.T) {
	m := protobuf.LocalMarker
	valid := false
	assert.MaxAllocs(t, func() { valid = m.Valid() }, 0, "Valid allocates nothing")
	assert.True(t, valid, "the local marker is valid")
}

// BenchmarkMarkers measures the check of the last marker of the switch.
func BenchmarkMarkers(b *testing.B) {
	b.Run("Marker", func(b *testing.B) {
		b.Run("Valid", func(b *testing.B) {
			m := protobuf.LocalMarker
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			valid := false
			for c.Loop() {
				valid = m.Valid()
			}
			assert.True(b, valid, "the local marker is valid")
		})
	})
}
