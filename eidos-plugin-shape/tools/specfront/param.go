// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront

import (
	"go.yaml.in/yaml/v3"

	"go.dokimi.dev/eidos/sdk/jsonschema"
)

// Param is one param of a classification's variant of its directive. The
// generator declares one constant for its key, one field of the spec's
// params struct, and one fact key.
type Param struct {
	Key            Name       `yaml:"key"            schema:"required" doc:"The key of the param in the directive, in lowercase words joined by hyphens."`
	Type           ParamType  `yaml:"type"           schema:"required" doc:"The type of the value: string, int or reference."`
	Resolve        Resolution `yaml:"resolve"                          doc:"The kind of declaration that a reference refers to. A reference requires it."`
	Required       bool       `yaml:"required"                         doc:"When true, an instance of the directive writes the param."`
	Counterexample bool       `yaml:"counterexample"                   doc:"When true, the value is an input that no derivation could invent."`
	Roles          []Name     `yaml:"roles"                            doc:"The roles of a contract under which the param applies."`
	Minimum        *int64     `yaml:"minimum"                          doc:"The least value of an int param."`
	Excludes       []Name     `yaml:"excludes"                         doc:"The params of the spec that an instance does not write beside this one."`
	AlsoOn         []Name     `yaml:"also_on"                          doc:"The callable params of the spec whose callables also declare the parameter that this host-param reference refers to."`
	Doc            string     `yaml:"doc"            schema:"required" doc:"The meaning of the param."`
}

// ParamType is the type of a param's value.
type ParamType string

// The param types. A reference refers to a declaration of its resolution
// kind. No classification has a bool or a list param, so the catalog has
// neither type.
const (
	TypeString    ParamType = "string"
	TypeInt       ParamType = "int"
	TypeReference ParamType = "reference"
)

// paramTypes lists every param type, in the order of a message.
var paramTypes = []ParamType{TypeString, TypeInt, TypeReference}

var _ jsonschema.Schemer = TypeString

// UnmarshalYAML decodes a param type, and returns an error at the value's
// line for any other spelling.
func (t *ParamType) UnmarshalYAML(n *yaml.Node) error {
	return decodeSpelling(n, t, paramTypes, "a param type")
}

// JSONSchema returns the JSON Schema of a param type, which is an enum of
// the spellings. Each call returns a new map.
func (ParamType) JSONSchema() map[string]any { return spellingSchema(paramTypes) }

// Resolution is the kind of declaration that a reference param refers to.
// Each spelling matches a resolution kind of the directive layer.
type Resolution string

// The resolution kinds.
const (
	// ResolveCallableInScope refers to a callable in the scope of the
	// subject. For a method, a method of its type comes before a function.
	ResolveCallableInScope Resolution = "callable-in-scope"
	// ResolvePackageVar refers to a package-level variable or constant.
	ResolvePackageVar Resolution = "package-var"
	// ResolveValueField refers to a field of the subject's value.
	ResolveValueField Resolution = "value-field"
	// ResolveHostParam refers to a parameter of the subject.
	ResolveHostParam Resolution = "host-param"
	// ResolveMemberOnHandle refers to a field or a method of the handle
	// that the subject returns.
	ResolveMemberOnHandle Resolution = "member-on-handle"
	// ResolveTypeInScope refers to a type in the scope of the file of the
	// subject.
	ResolveTypeInScope Resolution = "type-in-scope"
)

// resolutions lists every resolution kind, in the order of a message.
var resolutions = []Resolution{
	ResolveCallableInScope, ResolvePackageVar, ResolveValueField,
	ResolveHostParam, ResolveMemberOnHandle, ResolveTypeInScope,
}

var _ jsonschema.Schemer = ResolveCallableInScope

// UnmarshalYAML decodes a resolution kind, and returns an error at the
// value's line for any other spelling.
func (r *Resolution) UnmarshalYAML(n *yaml.Node) error {
	return decodeSpelling(n, r, resolutions, "a resolution kind")
}

// JSONSchema returns the JSON Schema of a resolution kind, which is an
// enum of the spellings. Each call returns a new map.
func (Resolution) JSONSchema() map[string]any { return spellingSchema(resolutions) }
