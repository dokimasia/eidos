// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront

import (
	"go.yaml.in/yaml/v3"

	"go.dokimi.dev/eidos/sdk/jsonschema"
)

// RoleArity is one role of a contract.
type RoleArity struct {
	Arity Arity `yaml:"arity" schema:"required" doc:"The number of callables of the role in one instance: one, optional, many or any."`
}

// Arity is the number of callables that one role of one contract
// instance has.
type Arity string

// The arities.
const (
	// ArityOne requires exactly one callable.
	ArityOne Arity = "one"
	// ArityOptional admits one callable or none.
	ArityOptional Arity = "optional"
	// ArityMany requires one callable or more.
	ArityMany Arity = "many"
	// ArityAny admits any number of callables, none included.
	ArityAny Arity = "any"
)

// arities lists every arity, in the order of a message.
var arities = []Arity{ArityOne, ArityOptional, ArityMany, ArityAny}

var _ jsonschema.Schemer = ArityOne

// UnmarshalYAML decodes an arity, and returns an error at the value's line
// for any other spelling.
func (a *Arity) UnmarshalYAML(n *yaml.Node) error {
	return decodeSpelling(n, a, arities, "an arity")
}

// JSONSchema returns the JSON Schema of an arity, which is an enum of the
// spellings. Each call returns a new map.
func (Arity) JSONSchema() map[string]any { return spellingSchema(arities) }
