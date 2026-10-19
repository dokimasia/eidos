// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package graphql

import (
	"errors"

	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// namespace is the namespace of metadata of every graphql key.
const namespace = "graphql"

// The classification and fact keys of the satellite. The frontend stamps
// them for the constructs of GraphQL that the model has no field for, such
// as the schema statement of a file and the type condition of a fragment.
const (
	// SchemaKey marks a file with a schema statement. The value is
	// [SchemaStatement] where the file has a schema definition, and
	// [ExtendStatement] where it has extensions of the schema alone.
	SchemaKey meta.KeyName = "graphql.schema"

	// RootsKey lists the root operation types of the schema statements of a
	// file as written, one entry for each, such as query: Query. An entry
	// joins the operation and the type with [NameSep].
	RootsKey meta.KeyName = "graphql.roots"

	// OperationKey is the kind of operation: query, mutation or
	// subscription. The frontend stamps it on the root type of the
	// operations of the kind and on the function of an operation.
	OperationKey meta.KeyName = "graphql.operation"

	// InputKey marks the struct of an input object. The value is input.
	InputKey meta.KeyName = "graphql.input"

	// DirectiveKey lists the locations of the definition of a directive as
	// written. The frontend stamps it on the interface of the definition.
	DirectiveKey meta.KeyName = "graphql.directive"

	// RepeatableKey marks the definition of a repeatable directive. The
	// value is repeatable.
	RepeatableKey meta.KeyName = "graphql.repeatable"

	// ExtensionKey marks the declaration of an extension whose type its
	// schema does not declare. A subgraph of a federated graph extends a
	// type of another subgraph this way. The value is extend.
	ExtensionKey meta.KeyName = "graphql.extension"

	// AnonymousKey marks the function of an operation without a name. The
	// function has the base name of its file. The value is anonymous.
	AnonymousKey meta.KeyName = "graphql.anonymous"

	// FragmentKey is the type condition of a fragment. The frontend stamps
	// it on the function of the fragment.
	FragmentKey meta.KeyName = "graphql.fragment"

	// SelectionKey is the selection set of an operation or a fragment as
	// written. The text starts at the opening brace and ends at the closing
	// brace. A client sends that text with its operation.
	SelectionKey meta.KeyName = "graphql.selection"

	// CoordinateKey is the schema coordinate of the field that a field of a
	// selection selects, such as User.name.
	CoordinateKey meta.KeyName = "graphql.coordinate"

	// SpreadsKey lists the fragments that a selection spreads, in source
	// order. The frontend stamps it on the struct or the sum variant of the
	// selection.
	SpreadsKey meta.KeyName = "graphql.spreads"
)

// The values of [SchemaKey]. SchemaStatement marks a file with a schema
// definition, and ExtendStatement a file with extensions of the schema alone.
const (
	SchemaStatement = "schema"
	ExtendStatement = "extend"
)

// Keys claims the graphql namespace of metadata for the registrant of the
// handle, and registers every key in it through the same handle. The
// frontend stamps the roots, the directive and the spreads keys as lists of
// text, and every other key as text.
//
// A composition passes Keys to the workspace builder, and a corpus fixture
// passes it to the conformance runner, so both register one set. After a
// failed claim of the namespace, Keys does not register a key. Otherwise it
// registers every key that it can, and returns the errors of the others
// joined. A second call through the same registrant repeats the
// registration and returns nil. A call through another registrant fails,
// because a namespace has one registrant.
//
// # Allocation contract
//
// Keys allocates the nine kind lists of the keys, and the registry
// allocates its namespace claim and the growth of its lists and maps to
// twelve keys: 24 allocations into a fresh registry.
func Keys(r *meta.Registry) error {
	if err := r.ClaimNamespace(namespace); err != nil {
		return err
	}
	var errs []error
	file := []symbol.Kind{symbol.KindFile}
	function := []symbol.Kind{symbol.KindFunction}
	for _, spec := range []meta.KeySpec{
		{
			Name: SchemaKey, Kinds: file,
			Doc: "marks a file with a schema statement: schema, or extend for extensions alone",
		},
		{
			Name: OperationKey, Kinds: []symbol.Kind{symbol.KindStruct, symbol.KindFunction},
			Doc: "the kind of operation of a root type or of the function of an operation",
		},
		{
			Name: InputKey, Kinds: []symbol.Kind{symbol.KindStruct},
			Doc: "marks a struct that loads from an input object",
		},
		{
			Name: RepeatableKey, Kinds: []symbol.Kind{symbol.KindInterface},
			Doc: "marks the definition of a repeatable directive",
		},
		{
			Name: ExtensionKey,
			Kinds: []symbol.Kind{
				symbol.KindStruct, symbol.KindInterface, symbol.KindEnum, symbol.KindAlias,
			},
			Doc: "marks a declaration that loads from an extension of a type outside its schema",
		},
		{
			Name: AnonymousKey, Kinds: function,
			Doc: "marks the function of an operation without a name",
		},
		{
			Name: FragmentKey, Kinds: function,
			Doc: "the type condition of a fragment",
		},
		{
			Name: SelectionKey, Kinds: function,
			Doc: "the selection set of an operation or a fragment, as written",
		},
		{
			Name: CoordinateKey, Kinds: []symbol.Kind{symbol.KindField},
			Doc: "the schema coordinate of the schema field that a field of a selection selects",
		},
	} {
		if _, err := meta.Register[string](r, spec); err != nil {
			errs = append(errs, err)
		}
	}
	for _, spec := range []meta.KeySpec{
		{
			Name: RootsKey, Kinds: file,
			Doc: "the root operation types of the schema statements of a file, as written",
		},
		{
			Name: DirectiveKey, Kinds: []symbol.Kind{symbol.KindInterface},
			Doc: "the locations of the definition of a directive, as written",
		},
		{
			Name: SpreadsKey, Kinds: []symbol.Kind{symbol.KindStruct, symbol.KindSumVariant},
			Doc: "the fragments that a selection spreads, in source order",
		},
	} {
		if _, err := meta.Register[[]string](r, spec); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
