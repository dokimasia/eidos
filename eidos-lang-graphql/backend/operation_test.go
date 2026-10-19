// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/lang/graphql/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// functionAllocs is the definition of an operation or a fragment: the text
// that the writer returns.
const functionAllocs = 1

// mutationOperation is the marker of a mutation.
var mutationOperation = symbol.Annotation{
	Name: string(graphql.OperationMarker), Args: []string{string(graphql.OperationMutation)},
}

// fieldsSelection is the selection set of a fragment on the type Row.
const fieldsSelection = "{\n  id\n}"

// TestOperation pins the executable definitions, byte for byte in the
// layout of graphql-js: an operation of each kind, an anonymous operation
// and a fragment.
func TestOperation(t *testing.T) {
	t.Parallel()

	t.Run("Vocabulary", func(t *testing.T) {
		t.Parallel()

		t.Run("Function", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give *emit.Function
				want string
			}{
				{
					name: "returns a query with its name and its selection set",
					give: &emit.Function{
						Name: "GetRow", Annotations: symbol.Annotations{queryOperation},
						Body: emit.Body{Verbatim: selection},
					},
					want: "query GetRow {\n  __typename\n}\n",
				},
				{
					name: "returns the variables of an operation",
					give: &emit.Function{
						Name: "Rename", Annotations: symbol.Annotations{mutationOperation},
						Params: emit.SlotOf(
							&emit.Param{Name: idField, Type: idRef},
							&emit.Param{Name: nameField, Type: stringRef, Optional: true},
						),
						Body: emit.Body{Verbatim: selection},
					},
					want: "mutation Rename($id: ID!, $name: String) {\n  __typename\n}\n",
				},
				{
					name: "returns the default and the directives of a variable",
					give: &emit.Function{
						Name: "Rows", Annotations: symbol.Annotations{queryOperation},
						Params: emit.SlotOf(&emit.Param{
							Name: "first", Type: intRef, Default: "10", Annotations: symbol.Annotations{deprecated},
						}),
						Body: emit.Body{Verbatim: selection},
					},
					want: "query Rows($first: Int! = 10 @deprecated) {\n  __typename\n}\n",
				},
				{
					name: "returns the variables on lines of their own where a variable has a description",
					give: &emit.Function{
						Name: "GetRow", Annotations: symbol.Annotations{queryOperation},
						Params: emit.SlotOf(&emit.Param{Name: idField, Type: idRef, Doc: []string{"The key."}}),
						Body:   emit.Body{Verbatim: selection},
					},
					want: "query GetRow(\n  \"The key.\"\n  $id: ID!\n) {\n  __typename\n}\n",
				},
				{
					name: "returns an anonymous operation without its name",
					give: &emit.Function{
						Name: "Changed", Annotations: symbol.Annotations{subscriptionOperation, anonymous},
						Body: emit.Body{Verbatim: selection},
					},
					want: "subscription {\n  __typename\n}\n",
				},
				{
					name: "returns the selection set alone for an anonymous query without variables",
					give: &emit.Function{
						Name: "Anything", Annotations: symbol.Annotations{queryOperation, anonymous},
						Body: emit.Body{Verbatim: selection},
					},
					want: "{\n  __typename\n}\n",
				},
				{
					name: "returns the keyword of an anonymous query with a directive",
					give: &emit.Function{
						Name: "Anything", Annotations: symbol.Annotations{queryOperation, anonymous, shareable},
						Body: emit.Body{Verbatim: selection},
					},
					want: "query @shareable {\n  __typename\n}\n",
				},
				{
					name: "returns the keyword of an anonymous query with variables",
					give: &emit.Function{
						Name: "Anything", Annotations: symbol.Annotations{queryOperation, anonymous},
						Params: emit.SlotOf(&emit.Param{Name: idField, Type: idRef}),
						Body:   emit.Body{Verbatim: selection},
					},
					want: "query ($id: ID!) {\n  __typename\n}\n",
				},
				{
					name: "returns the keyword of an anonymous query with a description",
					give: &emit.Function{
						Name:        "Anything",
						Doc:         []string{"Reads anything."},
						Annotations: symbol.Annotations{queryOperation, anonymous},
						Body:        emit.Body{Verbatim: selection},
					},
					want: "\"Reads anything.\"\nquery {\n  __typename\n}\n",
				},
				{
					name: "returns the variables of an anonymous operation before its directives",
					give: &emit.Function{
						Name: "Changed", Annotations: symbol.Annotations{subscriptionOperation, anonymous, shareable},
						Params: emit.SlotOf(&emit.Param{Name: idField, Type: idRef}),
						Body:   emit.Body{Verbatim: selection},
					},
					want: "subscription ($id: ID!) @shareable {\n  __typename\n}\n",
				},
				{
					name: "returns a fragment with its type condition",
					give: &emit.Function{
						Name: "RowFields", Annotations: symbol.Annotations{rowFragment},
						Body: emit.Body{Verbatim: fieldsSelection},
					},
					want: "fragment RowFields on Row {\n  id\n}\n",
				},
				{
					name: "returns the variables of a fragment before its type condition",
					give: &emit.Function{
						Name: "RowFields", Annotations: symbol.Annotations{rowFragment},
						Params: emit.SlotOf(&emit.Param{Name: "first", Type: intRef}),
						Body:   emit.Body{Verbatim: fieldsSelection},
					},
					want: "fragment RowFields($first: Int!) on Row {\n  id\n}\n",
				},
				{
					name: "returns the directives of an operation before its selection set",
					give: &emit.Function{
						Name: "GetRow",
						Annotations: symbol.Annotations{
							queryOperation, {Name: "cached", Args: []string{"maxAge: 60"}},
						},
						Body: emit.Body{Verbatim: selection},
					},
					want: "query GetRow @cached(maxAge: 60) {\n  __typename\n}\n",
				},
				{
					name: "returns the trailing comment of an operation after the opening brace",
					give: &emit.Function{
						Name: "GetRow", Comment: "by key", Annotations: symbol.Annotations{queryOperation},
						Body: emit.Body{Verbatim: selection},
					},
					want: "query GetRow { # by key\n  __typename\n}\n",
				},
				{
					name: "returns the selection set on the next line where the brace has text after it",
					give: &emit.Function{
						Name: "GetRow", Comment: "by key", Annotations: symbol.Annotations{queryOperation},
						Body: emit.Body{Verbatim: "{ __typename }"},
					},
					want: "query GetRow { # by key\n __typename }\n",
				},
				{
					name: "returns the description of an operation",
					give: &emit.Function{
						Name:        "GetRow",
						Doc:         []string{"GetRow reads a row."},
						Annotations: symbol.Annotations{queryOperation},
						Body:        emit.Body{Verbatim: selection},
					},
					want: "\"GetRow reads a row.\"\nquery GetRow {\n  __typename\n}\n",
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, (&backend.Vocabulary{}).Function(tt.give), tt.want,
						"the executable definition")
				})
			}
		})
	})
}

// Function allocates the text of the definition, once. The ordinary run,
// which runs no benchmark, checks that ceiling here.
func TestOperationAllocs(t *testing.T) {
	v := &backend.Vocabulary{}
	query := &emit.Function{
		Name: "GetRow", Annotations: symbol.Annotations{queryOperation}, Body: emit.Body{Verbatim: selection},
	}
	var got string
	assert.MaxAllocs(t, func() { got = v.Function(query) }, functionAllocs,
		"Function allocates the text of the definition")
	assert.Equal(t, got, "query GetRow {\n  __typename\n}\n", "Function writes the query")
}

// BenchmarkOperation measures the definition that every operation and every
// fragment of a document write.
func BenchmarkOperation(b *testing.B) {
	b.Run("Vocabulary", func(b *testing.B) {
		b.Run("Function", func(b *testing.B) {
			v := &backend.Vocabulary{}
			query := &emit.Function{
				Name: "GetRow", Annotations: symbol.Annotations{queryOperation}, Body: emit.Body{Verbatim: selection},
			}
			c := bench.Start(b).Warmup(1).MaxAllocs(functionAllocs)
			defer c.End()
			var got string
			for c.Loop() {
				got = v.Function(query)
			}
			assert.Equal(b, got, "query GetRow {\n  __typename\n}\n", "Function writes the query")
		})
	})
}
