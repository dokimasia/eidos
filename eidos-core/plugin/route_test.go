// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/plugin"
)

// The route of a unit records the inputs of the layout, and a report spells
// each input by its name.
func TestRoute(t *testing.T) {
	t.Parallel()

	t.Run("Route", func(t *testing.T) {
		t.Parallel()

		t.Run("has no input in its zero value", func(t *testing.T) {
			t.Parallel()

			var r plugin.Route
			assert.Equal(t, r, plugin.Route{Family: 0, Policy: 0, Dir: 0, Name: 0},
				"a unit that the layout has not placed has no input in any part")
		})
	})

	t.Run("RouteInput", func(t *testing.T) {
		t.Parallel()

		t.Run("String", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give plugin.RouteInput
				want string
			}{
				{name: "returns default for RouteDefault", give: plugin.RouteDefault, want: "default"},
				{name: "returns handler for RouteHandler", give: plugin.RouteHandler, want: "handler"},
				{name: "returns source for RouteSource", give: plugin.RouteSource, want: "source"},
				{name: "returns target for RouteTarget", give: plugin.RouteTarget, want: "target"},
				{name: "returns plan for RoutePlan", give: plugin.RoutePlan, want: "plan"},
				{name: "returns generator for RouteGenerator", give: plugin.RouteGenerator, want: "generator"},
				{name: "returns family for RouteFamily", give: plugin.RouteFamily, want: "family"},
				{name: "returns package-out for RoutePackage", give: plugin.RoutePackage, want: "package-out"},
				{name: "returns out for RouteOut", give: plugin.RouteOut, want: "out"},
				{name: "returns directive for RouteDirective", give: plugin.RouteDirective, want: "directive"},
				{name: "returns the number of the zero input", give: 0, want: "RouteInput(0)"},
				{name: "returns every digit of the number of an undeclared input", give: 11, want: "RouteInput(11)"},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, tt.give.String(), tt.want, "the spelling is pinned")
				})
			}
		})

		t.Run("Valid", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give plugin.RouteInput
				want bool
			}{
				{name: "reports false for the zero input", give: 0, want: false},
				{name: "reports true for the first declared input", give: plugin.RouteDefault, want: true},
				{name: "reports true for the last declared input", give: plugin.RouteDirective, want: true},
				{name: "reports false for the input after the last declared one", give: 11, want: false},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, tt.give.Valid(), tt.want, "the input is declared or it is not")
				})
			}
		})
	})
}
