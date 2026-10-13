// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The key of the extensions of a document, and the parts of the yaml tags
// of the type that [Extension] decodes a file into.
const (
	extensionsKey = "extensions"
	// inlineTag puts each other key of a mapping into a map of nodes. A
	// decoder with known fields accepts every such key.
	inlineTag reflect.StructTag = `yaml:",inline"`
	// The yaml tag - leaves a field out, and a comma ends the key of a yaml
	// tag. So neither the key - nor a key with a comma is an extension name.
	skipKey = "-"
	keyEnd  = ","
)

// nodes is the type of a mapping whose values Extension does not decode.
var nodes = reflect.TypeFor[map[string]yaml.Node]()

// Extension decodes the value of the extension name of the config file data
// into v. file is the file name that each [Error] includes, and v is a
// pointer that is not nil. Extension decodes into the value of v, so a field
// that the extension does not set keeps its value, and an extension of null
// leaves v as it is. Extension reports false and leaves v as it is when the
// file has no such extension.
//
// Extension decodes the whole file again with the strict rules of [Decode].
// The type of the decode has the type of v at the extension and a node at
// each other key, so each fault has its line in the file. A fault outside
// the extension is not a fault of Extension.
//
// Extension panics for a v that is nil or not a pointer, and for a name that
// is empty, is - or contains a comma. Both are defects of the binary.
//
// Error modes: Extension returns an error that wraps one *Error for each key
// that v does not declare and for each value that does not decode, and one
// *Error for a file that is not valid YAML. v can then contain the values
// that decoded.
func Extension(file string, data []byte, name string, v any) (bool, error) {
	target := reflect.ValueOf(v)
	if target.Kind() != reflect.Pointer || target.IsNil() {
		panic(fmt.Sprintf("config: Extension decodes into a pointer that is not nil, and v is %#v", v))
	}
	if name == "" || name == skipKey || strings.Contains(name, keyEnd) {
		panic(fmt.Sprintf("config: %q is not an extension name: a name is not empty or -, and has no comma", name))
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false, faults(file, err)
	}
	if len(doc.Content) == 0 || value(value(doc.Content[0], extensionsKey), name) == nil {
		return false, nil
	}
	extension := reflect.StructOf([]reflect.StructField{
		{Name: "Value", Type: target.Type(), Tag: reflect.StructTag("yaml:" + strconv.Quote(name))},
		{Name: "Rest", Type: nodes, Tag: inlineTag},
	})
	out := reflect.New(reflect.StructOf([]reflect.StructField{
		{Name: "Extensions", Type: extension, Tag: reflect.StructTag("yaml:" + strconv.Quote(extensionsKey))},
		{Name: "Rest", Type: nodes, Tag: inlineTag},
	}))
	out.Elem().Field(0).Field(0).Set(target)
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(out.Interface()); err != nil {
		return true, faults(file, err)
	}
	return true, nil
}
