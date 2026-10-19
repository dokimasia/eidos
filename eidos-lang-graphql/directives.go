// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package graphql

// The built-in directives of GraphQL that the satellite reads. A custom
// scalar names its specification through SpecifiedByDirective. An input
// object with OneOfDirective takes exactly one of its fields.
// DeprecatedDirective deprecates a field, an argument, an input field or an
// enum value. A response can lack a field of a selection under
// IncludeDirective or SkipDirective. The first payload of a response lacks
// the fields of a fragment under DeferDirective.
const (
	SpecifiedByDirective = "specifiedBy"
	OneOfDirective       = "oneOf"
	DeprecatedDirective  = "deprecated"
	IncludeDirective     = "include"
	SkipDirective        = "skip"
	DeferDirective       = "defer"
)

// NameSep separates a name from its value or its type, as GraphQL writes an
// argument and a root operation type. The frontend writes each argument of an
// applied directive as the name of the argument, NameSep and the value, such
// as url: and a string. It writes each entry of [RootsKey] as an operation,
// NameSep and a type.
const NameSep = ": "

// DirectiveSigil opens the name of a directive where GraphQL writes it. The
// lowering of the backend gives the interface of a directive definition a
// name that starts with the sigil, so the respell spells the name of a
// directive in camel case and the name of an interface in Pascal case.
const DirectiveSigil = "@"
