// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront

import (
	"regexp"

	"go.yaml.in/yaml/v3"

	"go.dokimi.dev/eidos/sdk/jsonschema"
)

// namePattern is the pattern of a name: lowercase words of letters and
// digits, each opening with a letter, joined by single hyphens.
const namePattern = `^[a-z][a-z0-9]*(-[a-z][a-z0-9]*)*$`

// nameRule matches [namePattern].
var nameRule = regexp.MustCompile(namePattern)

// Name is the name of a spec, of a param, of a binding or of a role, such
// as batch-writer. The generator joins its words in Pascal case into a Go
// identifier, so each word opens with a letter.
type Name string

var _ jsonschema.Schemer = Name("")

// UnmarshalYAML decodes a name, and returns an error at the value's line
// for a value that breaks [namePattern].
func (nm *Name) UnmarshalYAML(n *yaml.Node) error {
	var s string
	if err := n.Decode(&s); err != nil {
		return err
	}
	if !nameRule.MatchString(s) {
		return lineFault(n, "%q is no name: write lowercase words that open with a letter, joined by hyphens", s)
	}
	*nm = Name(s)
	return nil
}

// JSONSchema returns the JSON Schema of a name, which is a string that
// matches [namePattern]. Each call returns a new map.
func (Name) JSONSchema() map[string]any {
	return map[string]any{keywordType: typeString, keywordPattern: namePattern}
}
