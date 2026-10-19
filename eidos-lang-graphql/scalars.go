// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package graphql

import (
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The built-in scalars of GraphQL. A reference to one has no target.
const (
	IntScalar     = "Int"
	FloatScalar   = "Float"
	StringScalar  = "String"
	BooleanScalar = "Boolean"
	IDScalar      = "ID"
)

// tableScalar is one scalar of the table of [Scalar]. name is its name, and
// arg is the argument of its annotation specifiedBy: url: and the URL of its
// specification on scalars.graphql.org as a GraphQL string. shape is the
// shape of the model that a scalar of that specification projects as.
type tableScalar struct {
	name  string
	arg   string
	shape rules.TypeShape
}

// scalarTable lists the scalars of scalars.graphql.org that have a shape of
// the model. A custom scalar whose specifiedBy URL is in the table projects
// as the shape of its entry.
var scalarTable = [...]tableScalar{
	{
		name:  string(DateTime),
		arg:   `url: "https://scalars.graphql.org/andimarek/date-time.html"`,
		shape: rules.Reference(string(DateTime), rules.WellKnownTimestamp),
	},
	{
		name:  string(Duration),
		arg:   `url: "https://scalars.graphql.org/chillicream/duration.html"`,
		shape: rules.Reference(string(Duration), rules.WellKnownDuration),
	},
	{
		name:  string(Long),
		arg:   `url: "https://scalars.graphql.org/chillicream/long.html"`,
		shape: rules.Scalar(string(Long), rules.ScalarInt, 64),
	},
	{
		name:  string(UnsignedInt),
		arg:   `url: "https://scalars.graphql.org/chillicream/unsigned-int.html"`,
		shape: rules.Scalar(string(UnsignedInt), rules.ScalarUint, 32),
	},
	{
		name:  string(UnsignedLong),
		arg:   `url: "https://scalars.graphql.org/chillicream/unsigned-long.html"`,
		shape: rules.Scalar(string(UnsignedLong), rules.ScalarUint, 64),
	},
	{
		name:  string(Base64String),
		arg:   `url: "https://scalars.graphql.org/chillicream/base64-string.html"`,
		shape: rules.Leaf(symbol.FormBytes, string(Base64String)),
	},
	{
		name:  string(Any),
		arg:   `url: "https://scalars.graphql.org/chillicream/any.html"`,
		shape: rules.Leaf(symbol.FormDynamic, string(Any)),
	},
}

// IsBuiltin reports whether spelling is the name of one of the five
// built-in scalars of GraphQL. It does not allocate.
func IsBuiltin(spelling string) bool {
	switch spelling {
	case IntScalar, FloatScalar, StringScalar, BooleanScalar, IDScalar:
		return true
	default:
		return false
	}
}

// ScalarShape returns the shape of a custom scalar with the annotations a.
// It returns the shape of the entry of the table whose URL is the argument
// of the annotation specifiedBy in a. ScalarShape reports false where a has
// no such annotation, and for a URL that the table does not contain. The
// frontend writes the argument of the annotation as url: and a GraphQL
// string in its canonical form, so ScalarShape compares the argument with
// the argument of each entry. ScalarShape does not allocate.
func ScalarShape(a symbol.Annotations) (rules.TypeShape, bool) {
	for i := range a {
		if a[i].Name != SpecifiedByDirective || len(a[i].Args) != 1 {
			continue
		}
		for j := range scalarTable {
			if scalarTable[j].arg == a[i].Args[0] {
				return scalarTable[j].shape, true
			}
		}
	}
	return rules.TypeShape{}, false
}

// Scalar returns the emit declaration of the scalar of the table named
// name. The declaration is a defined alias without a target, with the
// annotation specifiedBy and the URL of its specification. Scalar reports
// false for a name that the table does not contain. A generator appends the
// declaration once to the schema whose references spell the scalar, such as
// a schema whose plan chooses the scalar of a policy.
//
// # Allocation contract
//
// Scalar allocates the alias, its list of annotations and the list of
// arguments: three allocations. It does not allocate for a name outside the
// table.
func Scalar(name string) (*emit.Alias, bool) {
	for i := range scalarTable {
		if s := &scalarTable[i]; s.name == name {
			return &emit.Alias{
				Name: s.name, Defined: true,
				Annotations: symbol.Annotations{{Name: SpecifiedByDirective, Args: []string{s.arg}}},
			}, true
		}
	}
	return nil, false
}
