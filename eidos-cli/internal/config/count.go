// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"go.yaml.in/yaml/v3"

	"go.dokimi.dev/eidos/core/jsonschema"
)

// Count is a whole number of 0 or more, such as the number of workers.
type Count int

var _ jsonschema.Schemer = Count(0)

// UnmarshalYAML decodes a count from n.
//
// Error modes: UnmarshalYAML returns a *yaml.TypeError at the line of n when
// n is not a whole number of 0 or more.
func (c *Count) UnmarshalYAML(n *yaml.Node) error {
	refused := lineFault(n, "%q is not a count: use a whole number of 0 or more", n.Value)
	if n.ShortTag() != intTag {
		return refused
	}
	var v int
	if err := n.Decode(&v); err != nil || v < 0 {
		return refused
	}
	*c = Count(v)
	return nil
}

// JSONSchema returns the JSON Schema of a count, a new map on each call.
// The schema allows an integer of 0 or more.
func (Count) JSONSchema() map[string]any {
	return map[string]any{"type": "integer", "minimum": 0}
}
