// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The YAML decoder starts each entry of a *yaml.TypeError with "line N: ".
// These constants are the parts of the prefix.
const (
	linePrefix = "line "
	lineEnd    = ": "
)

// The keywords and the type names of JSON Schema in the schemas that the
// JSONSchema methods of the vocabulary types return.
const (
	keywordType    = "type"
	keywordEnum    = "enum"
	keywordConst   = "const"
	keywordPattern = "pattern"
	typeString     = "string"
)

// spelling is a string type whose values come from a closed list.
type spelling interface{ ~string }

// decodeSpelling decodes a scalar into out where the scalar is one of the
// spellings. For any other value, it returns an error at the line of the
// scalar whose message lists the spellings. For a node that is no string,
// it returns the error of the decoder. what is the noun phrase of the
// value in the message, such as "a param type".
func decodeSpelling[T spelling](n *yaml.Node, out *T, spellings []T, what string) error {
	var s string
	if err := n.Decode(&s); err != nil {
		return err
	}
	if !slices.Contains(spellings, T(s)) {
		quoted := make([]string, 0, len(spellings))
		for _, sp := range spellings {
			quoted = append(quoted, strconv.Quote(string(sp)))
		}
		return lineFault(n, "%q is not %s: write %s", s, what, strings.Join(quoted, " or "))
	}
	*out = T(s)
	return nil
}

// spellingSchema returns the JSON Schema of a closed list of spellings.
// The schema is a string with a constant value for one spelling, and a
// string enum for two or more. Each call returns a new map.
func spellingSchema[T spelling](spellings []T) map[string]any {
	if len(spellings) == 1 {
		return map[string]any{keywordType: typeString, keywordConst: string(spellings[0])}
	}
	enum := make([]any, 0, len(spellings))
	for _, s := range spellings {
		enum = append(enum, string(s))
	}
	return map[string]any{keywordType: typeString, keywordEnum: enum}
}

// lineFault returns a *yaml.TypeError whose one entry has the line of n
// and the formatted message. An UnmarshalYAML method returns it, and the
// decoder records the fault and continues with the next value.
func lineFault(n *yaml.Node, format string, args ...any) error {
	entry := linePrefix + strconv.Itoa(n.Line) + lineEnd + fmt.Sprintf(format, args...)
	return &yaml.TypeError{Errors: []string{entry}}
}
