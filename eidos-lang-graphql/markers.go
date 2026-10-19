// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package graphql

// MarkerPrefix opens the name of every [Marker]. A GraphQL name cannot
// contain a colon, so no directive has the name of a marker.
const MarkerPrefix = "graphql:"

// Marker is the name of an annotation that states a keyword of GraphQL that
// the model has no field for. A generator states a marker as an annotation
// with the name of the marker, and the backend writes the keyword or the
// statement for it and no directive.
type Marker string

// The markers of the GraphQL target.
const (
	// InputMarker marks a struct as an input object, and a sum as an input
	// object with the directive oneOf.
	InputMarker Marker = MarkerPrefix + "input"
	// ExtendMarker marks a declaration as the extension of a type, and a
	// header as the extension of the schema.
	ExtendMarker Marker = MarkerPrefix + "extend"
	// OperationMarker marks the root type of an operation, and a function
	// as an operation. Its one argument is query, mutation or subscription.
	OperationMarker Marker = MarkerPrefix + "operation"
	// AnonymousMarker marks an operation that the backend writes without
	// its name.
	AnonymousMarker Marker = MarkerPrefix + "anonymous"
	// FragmentMarker marks a function as a fragment. Its one argument is
	// the type condition of the fragment.
	FragmentMarker Marker = MarkerPrefix + "fragment"
	// QueryMarker is a header marker. Its one argument is the name of the
	// root type of queries in the schema statement of the file.
	QueryMarker Marker = MarkerPrefix + "query"
	// MutationMarker is a header marker. Its one argument is the name of
	// the root type of mutations in the schema statement of the file.
	MutationMarker Marker = MarkerPrefix + "mutation"
	// SubscriptionMarker is a header marker. Its one argument is the name
	// of the root type of subscriptions in the schema statement of the file.
	SubscriptionMarker Marker = MarkerPrefix + "subscription"
	// DirectiveMarker marks an interface as the definition of a directive,
	// with the locations of the directive as its arguments, such as OBJECT
	// and FIELD_DEFINITION. The fields of the interface are the arguments
	// of the directive, and its other annotations are the directives that
	// apply to the definition. With [ExtendMarker] and no argument, the
	// marker marks the extension of a directive, whose annotations are the
	// directives that the extension adds.
	DirectiveMarker Marker = MarkerPrefix + "directive"
	// RepeatableMarker marks the definition of a repeatable directive.
	RepeatableMarker Marker = MarkerPrefix + "repeatable"
)

// Valid reports whether m is one of the markers that the satellite
// declares. The backend refuses an annotation whose name starts with
// [MarkerPrefix] and is not a valid marker. Valid does not allocate.
func (m Marker) Valid() bool {
	switch m {
	case InputMarker, ExtendMarker, OperationMarker, AnonymousMarker, FragmentMarker, QueryMarker,
		MutationMarker, SubscriptionMarker, DirectiveMarker, RepeatableMarker:
		return true
	default:
		return false
	}
}

// Operation is a kind of operation of GraphQL. It is the keyword of an
// operation, the argument of [OperationMarker] and the value of
// [OperationKey].
type Operation string

// The kinds of operation of GraphQL.
const (
	OperationQuery        Operation = "query"
	OperationMutation     Operation = "mutation"
	OperationSubscription Operation = "subscription"
)

// Valid reports whether o is one of the three kinds of operation. The
// backend refuses the marker of an operation whose argument is not valid.
// Valid does not allocate.
func (o Operation) Valid() bool {
	switch o {
	case OperationQuery, OperationMutation, OperationSubscription:
		return true
	default:
		return false
	}
}
