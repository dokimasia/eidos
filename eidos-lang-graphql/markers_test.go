// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package graphql_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/graphql"
)

// TestMarkers pins the spelling of each marker and of each kind of
// operation, because a generator outside this module states them in the
// emit model.
func TestMarkers(t *testing.T) {
	t.Parallel()

	t.Run("Marker", func(t *testing.T) {
		t.Parallel()

		spellings := []struct {
			name string
			give graphql.Marker
			want string
		}{
			{name: "spells the input marker", give: graphql.InputMarker, want: "graphql:input"},
			{name: "spells the extend marker", give: graphql.ExtendMarker, want: "graphql:extend"},
			{name: "spells the operation marker", give: graphql.OperationMarker, want: "graphql:operation"},
			{name: "spells the anonymous marker", give: graphql.AnonymousMarker, want: "graphql:anonymous"},
			{name: "spells the fragment marker", give: graphql.FragmentMarker, want: "graphql:fragment"},
			{name: "spells the query marker", give: graphql.QueryMarker, want: "graphql:query"},
			{name: "spells the mutation marker", give: graphql.MutationMarker, want: "graphql:mutation"},
			{name: "spells the subscription marker", give: graphql.SubscriptionMarker, want: "graphql:subscription"},
			{name: "spells the directive marker", give: graphql.DirectiveMarker, want: "graphql:directive"},
			{name: "spells the repeatable marker", give: graphql.RepeatableMarker, want: "graphql:repeatable"},
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
				give graphql.Marker
				want bool
			}{
				{name: "reports true for the input marker", give: graphql.InputMarker, want: true},
				{name: "reports true for the extend marker", give: graphql.ExtendMarker, want: true},
				{name: "reports true for the operation marker", give: graphql.OperationMarker, want: true},
				{name: "reports true for the anonymous marker", give: graphql.AnonymousMarker, want: true},
				{name: "reports true for the fragment marker", give: graphql.FragmentMarker, want: true},
				{name: "reports true for the query marker", give: graphql.QueryMarker, want: true},
				{name: "reports true for the mutation marker", give: graphql.MutationMarker, want: true},
				{name: "reports true for the subscription marker", give: graphql.SubscriptionMarker, want: true},
				{name: "reports true for the directive marker", give: graphql.DirectiveMarker, want: true},
				{name: "reports true for the repeatable marker", give: graphql.RepeatableMarker, want: true},
				{name: "reports false for a name with the prefix that no marker has", give: "graphql:union"},
				{name: "reports false for the prefix alone", give: graphql.MarkerPrefix},
				{name: "reports false for a directive name", give: graphql.DeprecatedDirective},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, tt.give.Valid(), tt.want, "Valid reports whether the satellite declares the marker")
				})
			}
		})
	})

	t.Run("Operation", func(t *testing.T) {
		t.Parallel()

		spellings := []struct {
			name string
			give graphql.Operation
			want string
		}{
			{name: "spells query", give: graphql.OperationQuery, want: "query"},
			{name: "spells mutation", give: graphql.OperationMutation, want: "mutation"},
			{name: "spells subscription", give: graphql.OperationSubscription, want: "subscription"},
		}
		for _, tt := range spellings {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, string(tt.give), tt.want, "the keyword of the kind of operation")
			})
		}

		t.Run("Valid", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give graphql.Operation
				want bool
			}{
				{name: "reports true for a query", give: graphql.OperationQuery, want: true},
				{name: "reports true for a mutation", give: graphql.OperationMutation, want: true},
				{name: "reports true for a subscription", give: graphql.OperationSubscription, want: true},
				{name: "reports false for a keyword of another definition", give: "fragment"},
				{name: "reports false for the empty kind", give: ""},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, tt.give.Valid(), tt.want, "Valid reports whether GraphQL has the kind")
				})
			}
		})
	})
}

// TestMarkersAllocs checks that the two checks of validity do not allocate,
// in the ordinary run, which does not run benchmarks.
func TestMarkersAllocs(t *testing.T) {
	t.Run("Marker", func(t *testing.T) {
		t.Run("Valid", func(t *testing.T) {
			m := graphql.RepeatableMarker
			valid := false
			assert.MaxAllocs(t, func() { valid = m.Valid() }, 0, "Valid does not allocate")
			assert.True(t, valid, "the repeatable marker is valid")
		})
	})

	t.Run("Operation", func(t *testing.T) {
		t.Run("Valid", func(t *testing.T) {
			o := graphql.OperationSubscription
			valid := false
			assert.MaxAllocs(t, func() { valid = o.Valid() }, 0, "Valid does not allocate")
			assert.True(t, valid, "a subscription is valid")
		})
	})
}

// BenchmarkMarkers measures the check of the last marker and of the last
// kind of operation of each switch.
func BenchmarkMarkers(b *testing.B) {
	b.Run("Marker", func(b *testing.B) {
		b.Run("Valid", func(b *testing.B) {
			m := graphql.RepeatableMarker
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			valid := false
			for c.Loop() {
				valid = m.Valid()
			}
			assert.True(b, valid, "the repeatable marker is valid")
		})
	})

	b.Run("Operation", func(b *testing.B) {
		b.Run("Valid", func(b *testing.B) {
			o := graphql.OperationSubscription
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			valid := false
			for c.Loop() {
				valid = o.Valid()
			}
			assert.True(b, valid, "a subscription is valid")
		})
	})
}
