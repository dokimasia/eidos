// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront

import (
	"go.yaml.in/yaml/v3"

	"go.dokimi.dev/eidos/sdk/jsonschema"
)

// Binding states the position of the parameter or the return that fills
// one part of a shape, such as the parameter of the key of a reader. The
// plugin shape stamps the identity of the parameter or the return at the
// position, for a detected shape and for a declared one.
type Binding struct {
	From  Source `yaml:"from"  schema:"required" doc:"The list that the index counts in: input, the input parameters, or result, the returns that are values or streams."`
	Index int    `yaml:"index"                   doc:"The position in the list, from 0."`
	Doc   string `yaml:"doc"   schema:"required" doc:"The meaning of the binding."`
}

// Source is the list of a callable's positions that a binding counts in.
type Source string

// The sources of a binding.
const (
	// SourceInput counts the parameters with the input role, so a
	// context parameter is no position.
	SourceInput Source = "input"
	// SourceResult counts the returns with the value or the stream role,
	// so the error and the ok flag are no position.
	SourceResult Source = "result"
)

// sources lists every source, in the order of a message.
var sources = []Source{SourceInput, SourceResult}

var _ jsonschema.Schemer = SourceInput

// UnmarshalYAML decodes a source, and returns an error at the value's line
// for any other spelling.
func (s *Source) UnmarshalYAML(n *yaml.Node) error {
	return decodeSpelling(n, s, sources, "the source of a binding")
}

// JSONSchema returns the JSON Schema of a source, which is an enum of the
// spellings. Each call returns a new map.
func (Source) JSONSchema() map[string]any { return spellingSchema(sources) }
