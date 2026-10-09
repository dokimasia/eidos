// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront

import (
	"go.yaml.in/yaml/v3"

	"go.dokimi.dev/eidos/sdk/jsonschema"
)

// The forms of a classification. Each is the one spelling of its form
// type, and the name of the directory below spec/ that contains the specs
// of the form, in the plural.
const (
	// FormShape is the form of a shape spec.
	FormShape ShapeForm = "shape"
	// FormMixin is the form of a mixin spec.
	FormMixin MixinForm = "mixin"
	// FormContract is the form of a contract spec.
	FormContract ContractForm = "contract"
)

// ShapeForm is the form field of a shape spec. Its one spelling is
// [FormShape].
type ShapeForm string

var _ jsonschema.Schemer = FormShape

// UnmarshalYAML decodes the form of a shape spec, and returns an error at
// the value's line for any other spelling.
func (f *ShapeForm) UnmarshalYAML(n *yaml.Node) error {
	return decodeSpelling(n, f, []ShapeForm{FormShape}, "the form of a spec in spec/shapes")
}

// JSONSchema returns the JSON Schema of the field, which is the constant
// shape. Each call returns a new map.
func (ShapeForm) JSONSchema() map[string]any { return spellingSchema([]ShapeForm{FormShape}) }

// MixinForm is the form field of a mixin spec. Its one spelling is
// [FormMixin].
type MixinForm string

var _ jsonschema.Schemer = FormMixin

// UnmarshalYAML decodes the form of a mixin spec, and returns an error at
// the value's line for any other spelling.
func (f *MixinForm) UnmarshalYAML(n *yaml.Node) error {
	return decodeSpelling(n, f, []MixinForm{FormMixin}, "the form of a spec in spec/mixins")
}

// JSONSchema returns the JSON Schema of the field, which is the constant
// mixin. Each call returns a new map.
func (MixinForm) JSONSchema() map[string]any { return spellingSchema([]MixinForm{FormMixin}) }

// ContractForm is the form field of a contract spec. Its one spelling is
// [FormContract].
type ContractForm string

var _ jsonschema.Schemer = FormContract

// UnmarshalYAML decodes the form of a contract spec, and returns an error
// at the value's line for any other spelling.
func (f *ContractForm) UnmarshalYAML(n *yaml.Node) error {
	return decodeSpelling(n, f, []ContractForm{FormContract}, "the form of a spec in spec/contracts")
}

// JSONSchema returns the JSON Schema of the field, which is the constant
// contract. Each call returns a new map.
func (ContractForm) JSONSchema() map[string]any { return spellingSchema([]ContractForm{FormContract}) }
