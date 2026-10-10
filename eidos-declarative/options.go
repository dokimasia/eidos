// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"go.dokimi.dev/eidos/core/jsonschema"
)

// The struct tags of a field of the options struct. The workspace reads
// the opt tag and the doc tag to validate and populate the options, and
// encodes them under the json tag into the fingerprint.
const (
	optTag  = "opt"
	jsonTag = "json"
	docTag  = "doc"
)

// The YAML tags of the scalars that the default of an option can have.
const (
	strTag  = "!!str"
	intTag  = "!!int"
	boolTag = "!!bool"
)

// optionDecl is one option of the manifest. The loader lowers the options
// to one struct, whose fields have the options' keys and types.
type optionDecl struct {
	Key     string       `yaml:"key"     schema:"required" doc:"The key of the option. A config file sets the option under this key, in the section of the plugin. A key is a lower-case letter followed by letters and digits."`
	Type    optionType   `yaml:"type"    schema:"required" doc:"The type of the option: string, int, bool, or list for a list of strings."`
	Default defaultValue `yaml:"default"                   doc:"The default value of the option, a value of its type. An option without a default has the zero value of its type."`
	Doc     string       `yaml:"doc"     schema:"required" doc:"The meaning of the option."`

	// line is the line of the option's mapping.
	line int
}

// optionType is the type of an option.
type optionType uint8

const (
	// optionString is a string, a field of type string.
	optionString optionType = 1
	// optionInt is an integer, a field of type int64.
	optionInt optionType = 2
	// optionBool is a boolean, a field of type bool.
	optionBool optionType = 3
	// optionList is a list of strings, a field of type []string.
	optionList optionType = 4
)

// optionTypes are the spellings of the type of an option.
var optionTypes = []word[optionType]{
	{spelling: spellString, value: optionString},
	{spelling: spellInt, value: optionInt},
	{spelling: spellBool, value: optionBool},
	{spelling: spellList, value: optionList},
}

var (
	_ yaml.Unmarshaler   = (*optionType)(nil)
	_ jsonschema.Schemer = optionType(0)
)

// UnmarshalYAML decodes the type of an option from n.
//
// Error modes: UnmarshalYAML returns a *yaml.TypeError at the line of n for
// a spelling that is not an option type.
func (t *optionType) UnmarshalYAML(n *yaml.Node) error {
	v, err := decodeWord(n, optionTypes, "an option type")
	*t = v
	return err
}

// JSONSchema returns the JSON Schema of the type of an option, a new map on
// each call.
func (optionType) JSONSchema() map[string]any { return wordsSchema(optionTypes) }

// goType returns the type of the field of an option of type t: string,
// int64, bool or []string. It returns string for the zero type, which
// checkOptions refuses.
func (t optionType) goType() reflect.Type {
	switch t {
	case optionInt:
		return reflect.TypeFor[int64]()
	case optionBool:
		return reflect.TypeFor[bool]()
	case optionList:
		return reflect.TypeFor[[]string]()
	default:
		return reflect.TypeFor[string]()
	}
}

// admits reports whether the YAML node n is a value of type t that decodes
// into the field of an option of type t: a string, an integer or a boolean
// scalar, or a sequence of string scalars. A scalar that YAML resolves to
// another type, such as 3 for a string or yes for a boolean, is not a value
// of t, and neither is an integer that int64 does not hold.
func (t optionType) admits(n *yaml.Node) bool {
	shaped := false
	switch t {
	case optionString:
		shaped = n.Kind == yaml.ScalarNode && n.ShortTag() == strTag
	case optionInt:
		shaped = n.Kind == yaml.ScalarNode && n.ShortTag() == intTag
	case optionBool:
		shaped = n.Kind == yaml.ScalarNode && n.ShortTag() == boolTag
	case optionList:
		shaped = n.Kind == yaml.SequenceNode
		for _, e := range n.Content {
			shaped = shaped && e.Kind == yaml.ScalarNode && e.ShortTag() == strTag
		}
	}
	return shaped && n.Decode(reflect.New(t.goType()).Interface()) == nil
}

// defaultValue is the default of an option as the manifest writes it: the
// YAML node, which the loader decodes into the field of the option once
// the type of the option is known. A null default leaves node nil, as an
// option without a default does.
type defaultValue struct {
	node *yaml.Node
}

var (
	_ yaml.Unmarshaler   = (*defaultValue)(nil)
	_ jsonschema.Schemer = defaultValue{}
)

// UnmarshalYAML keeps the node n of the default. It cannot fail.
func (d *defaultValue) UnmarshalYAML(n *yaml.Node) error {
	d.node = n
	return nil
}

// JSONSchema returns the JSON Schema of the default of an option, a new map
// on each call: a string, an integer, a boolean or an array of strings.
func (defaultValue) JSONSchema() map[string]any {
	return map[string]any{
		keywordType:  []any{typeString, typeInteger, typeBoolean, typeArray},
		keywordItems: map[string]any{keywordType: typeString},
	}
}

// checkOptions returns an Error at the line of each option whose key is
// not a key or is the key of an earlier option, that has no type or no
// doc, or whose default is not a value of its type. The options that pass
// lower to a struct that [reflect.StructOf] builds, and StructOf panics on
// a field name that two fields have.
func checkOptions(file string, opts []optionDecl) []*Error {
	var out []*Error
	keyed := make(map[string]int, len(opts))
	for _, o := range opts {
		if !validKey(o.Key) {
			out = append(out, &Error{
				File: file,
				Line: o.line,
				Msg: fmt.Sprintf(
					"%q is not an option key: use a lower-case letter followed by letters and digits",
					o.Key,
				),
			})
		} else if at, taken := keyed[o.Key]; taken {
			out = append(out, &Error{
				File: file, Line: o.line, Msg: fmt.Sprintf("the option %s is declared at line %d too", o.Key, at),
			})
		} else {
			keyed[o.Key] = o.line
		}
		if o.Type == 0 {
			out = append(out, &Error{
				File: file, Line: o.line, Msg: "the option " + o.Key + " has no type: use " + spellings(optionTypes),
			})
		} else if o.Default.node != nil && !o.Type.admits(o.Default.node) {
			out = append(out, &Error{
				File: file,
				Line: o.line,
				Msg: fmt.Sprintf(
					"the default of the option %s is not a value of the type %s",
					o.Key,
					spellingOf(optionTypes, o.Type),
				),
			})
		}
		if o.Doc == "" {
			out = append(out, &Error{File: file, Line: o.line, Msg: "the option " + o.Key + " has no doc"})
		}
	}
	return out
}

// validKey reports whether s is an option key: a lower-case ASCII letter
// followed by ASCII letters and digits. The field of an option has the key
// with its first letter in upper case as its name, so the field is
// exported, and its name is an identifier of Go.
func validKey(s string) bool {
	if s == "" || s[0] < 'a' || s[0] > 'z' {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

// optionsOf returns a pointer to a new options struct with one field for
// each option, in the order of opts, set to the option's default. It
// returns the zero Value for no option. Each field has the opt and json
// tags with the option's key, and the doc tag with the option's doc. The
// options passed checkOptions, so every key is valid and unique, and every
// default decodes into its field.
func optionsOf(opts []optionDecl) reflect.Value {
	if len(opts) == 0 {
		return reflect.Value{}
	}
	fields := make([]reflect.StructField, 0, len(opts))
	for _, o := range opts {
		tag := optTag + ":" + strconv.Quote(o.Key) + " " + jsonTag + ":" + strconv.Quote(o.Key) +
			" " + docTag + ":" + strconv.Quote(o.Doc)
		fields = append(fields, reflect.StructField{
			Name: strings.ToUpper(o.Key[:1]) + o.Key[1:],
			Type: o.Type.goType(),
			Tag:  reflect.StructTag(tag),
		})
	}
	v := reflect.New(reflect.StructOf(fields))
	for i, o := range opts {
		if o.Default.node != nil {
			// checkOptions decoded the default into a value of the field's
			// type, so this decode does not fail.
			_ = o.Default.node.Decode(v.Elem().Field(i).Addr().Interface())
		}
	}
	return v
}

// optionValues returns the value of each field of the options struct that
// the pointer v points to, by the key of its option. It returns nil for
// the zero Value, which a manifest without options has.
func optionValues(v reflect.Value, opts []optionDecl) map[string]any {
	if !v.IsValid() {
		return nil
	}
	out := make(map[string]any, len(opts))
	for i, o := range opts {
		out[o.Key] = v.Elem().Field(i).Interface()
	}
	return out
}
