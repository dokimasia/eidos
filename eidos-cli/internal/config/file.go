// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"go.dokimi.dev/eidos/core/jsonschema"
)

// The YAML decoder starts a syntax error with "yaml: line N: ", and each
// entry of a *yaml.TypeError with "line N: ". These constants are the parts
// of the two prefixes.
const (
	syntaxPrefix = "yaml: "
	linePrefix   = "line "
	lineEnd      = ": "
)

// Decode reads these keys before it decodes a file into its type.
const (
	versionKey    = "version"
	workspacesKey = "workspaces"
)

// Version is the version of the file format.
type Version int

// Current is the version of the file format that this package decodes.
const Current Version = 1

var _ jsonschema.Schemer = Current

// JSONSchema returns the JSON Schema of the version field, a new map on
// each call. The schema allows only [Current].
func (Version) JSONSchema() map[string]any {
	return map[string]any{keywordType: typeInteger, keywordConst: int(Current)}
}

// Error is a fault in a config file. It includes the line of the fault.
type Error struct {
	// File is the file name that the caller passed to Decode.
	File string
	// Line is the line number of the fault, and the first line is 1. Line is
	// 0 when the decoder reports the fault without a line.
	Line int
	// Msg describes the fault.
	Msg string
}

// Error returns the fault in the form FILE:LINE: MSG, or in the form
// FILE: MSG when Line is 0.
func (e *Error) Error() string {
	if e.Line == 0 {
		return e.File + lineEnd + e.Msg
	}
	return e.File + ":" + strconv.Itoa(e.Line) + lineEnd + e.Msg
}

// File is a decoded config file. Decode sets exactly one field. It sets
// Document when the file configures one workspace, and List when the file
// lists workspaces.
type File struct {
	Document *Document
	List     *List
}

// Decode decodes the config file data. Each [Error] includes name as the
// file name.
//
// Decode decodes a file with the key workspaces into a [List]. It decodes
// any other file into a [Document]. Decode skips each top-level key that is
// not a key of a Document or of a List, because the command line reads the
// values of its options from those keys and checks them itself. Below the
// top level, Decode is strict. It returns an Error for an unknown key, for
// a value of the wrong type, and for a size or a policy that does not parse.
// Decode continues after such a fault, and joins the Errors of all faults
// with [errors.Join].
//
// Error modes:
//
//   - A file that is not a mapping, a file without a version, and a file
//     with a version other than [Current] return an *Error.
//   - A file that is not valid YAML returns an error that wraps one *Error.
//   - A file with unknown keys or with values that do not decode returns an
//     error that wraps one *Error for each such key and value.
//   - A list with a key of a document returns an error that wraps one
//     *Error for each such key.
//   - A list without workspaces returns an *Error, and a list with entries
//     without a root returns an error that wraps one *Error for each such
//     entry.
func Decode(name string, data []byte) (File, error) {
	root, err := parse(name, data)
	if err != nil {
		return File{}, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if workspaces := value(root, workspacesKey); workspaces != nil {
		var l struct {
			List `yaml:",inline"`
			Rest map[string]yaml.Node `yaml:",inline"`
		}
		if err := dec.Decode(&l); err != nil {
			return File{}, faults(name, err)
		}
		if err := listed(name, root); err != nil {
			return File{}, err
		}
		if err := l.check(name, workspaces); err != nil {
			return File{}, err
		}
		return File{List: &l.List}, nil
	}
	var d struct {
		Document `yaml:",inline"`
		Rest     map[string]yaml.Node `yaml:",inline"`
	}
	if err := dec.Decode(&d); err != nil {
		return File{}, faults(name, err)
	}
	return File{Document: &d.Document}, nil
}

// DecodeMember decodes the config file data of a member of a list into a
// [Document], as [Decode] decodes a document. The command line reads the
// values of its options from the config file of the run alone, so
// DecodeMember returns an Error for each top-level key that is not a key of
// a Document. Each [Error] includes name as the file name.
//
// Error modes:
//
//   - the error modes of Decode for a document.
//   - an *Error at the key workspaces for a file that lists workspaces.
//   - an error that wraps one *Error for each top-level key that is not a
//     key of a Document.
func DecodeMember(name string, data []byte) (*Document, error) {
	root, err := parse(name, data)
	if err != nil {
		return nil, err
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if key := root.Content[i]; key.Value == workspacesKey {
			return nil, &Error{
				File: name,
				Line: key.Line,
				Msg:  "the config file of a member of a list lists workspaces",
			}
		}
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var d Document
	if err := dec.Decode(&d); err != nil {
		return nil, faults(name, err)
	}
	return &d, nil
}

// Keys returns the top-level keys of a [Document] and of a [List] in
// order. The command line skips these keys, because Decode reads them.
func Keys() []string {
	keys := slices.Concat(keysOf(reflect.TypeFor[Document]()), keysOf(reflect.TypeFor[List]()))
	slices.Sort(keys)
	return slices.Compact(keys)
}

// parse parses data and returns the mapping at the top of the file, after
// it checks the version of the file.
//
// Error modes: parse returns an error that wraps one *Error for data that
// is not valid YAML, and an *Error for a file that is not a mapping, a file
// without a version, and a file with a version other than [Current].
func parse(name string, data []byte) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, faults(name, err)
	}
	noVersion := fmt.Sprintf("the file has no version key: add version: %d", Current)
	if len(doc.Content) == 0 {
		return nil, &Error{File: name, Line: 1, Msg: noVersion}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, &Error{File: name, Line: root.Line, Msg: "the file is not a mapping of keys to values"}
	}
	version := value(root, versionKey)
	if version == nil {
		return nil, &Error{File: name, Line: root.Line, Msg: noVersion}
	}
	var v Version
	if err := version.Decode(&v); err != nil || v != Current {
		msg := fmt.Sprintf("the file has version %s, and this binary reads only version %d", version.Value, Current)
		return nil, &Error{File: name, Line: version.Line, Msg: msg}
	}
	return root, nil
}

// listed returns an Error for each top-level key of the list root that is
// a key of a [Document] and not a key of a [List]. A list configures no
// workspace itself.
func listed(name string, root *yaml.Node) error {
	document, list := keysOf(reflect.TypeFor[Document]()), keysOf(reflect.TypeFor[List]())
	var errs []error
	for i := 0; i+1 < len(root.Content); i += 2 {
		key := root.Content[i]
		if slices.Contains(document, key.Value) && !slices.Contains(list, key.Value) {
			errs = append(errs, &Error{File: name, Line: key.Line, Msg: "a list does not have the key " + key.Value})
		}
	}
	return errors.Join(errs...)
}

// keysOf returns the top-level keys of the format type t in order: the
// properties of the JSON Schema of t, so the keys and the schema agree.
func keysOf(t reflect.Type) []string {
	properties, _ := jsonschema.Of(t, jsonschema.TagYAML)[keywordProperties].(map[string]any)
	return slices.Sorted(maps.Keys(properties))
}

// value returns the value of key in the mapping m, or nil when m does not
// have the key. It returns nil for an m that is nil or not a mapping.
func value(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// faults converts an error of the YAML decoder into one [Error] for each
// fault, and joins the Errors with [errors.Join]. A *yaml.TypeError has one
// fault for each entry, and any other error is one fault. When a message
// starts with a line number, faults moves the number into the Line field.
func faults(name string, err error) error {
	entries := []string{strings.TrimPrefix(err.Error(), syntaxPrefix)}
	if typed, is := errors.AsType[*yaml.TypeError](err); is {
		entries = typed.Errors
	}
	errs := make([]error, 0, len(entries))
	for _, entry := range entries {
		fault := &Error{File: name, Msg: entry}
		if rest, numbered := strings.CutPrefix(entry, linePrefix); numbered {
			digits, msg, _ := strings.Cut(rest, lineEnd)
			if line, perr := strconv.Atoi(digits); perr == nil {
				fault.Line, fault.Msg = line, msg
			}
		}
		errs = append(errs, fault)
	}
	return errors.Join(errs...)
}

// lineFault returns a *yaml.TypeError whose one entry has the line of n and
// the formatted message. An UnmarshalYAML method returns this error, and the
// decoder then records the fault and continues with the next value.
func lineFault(n *yaml.Node, format string, args ...any) error {
	entry := linePrefix + strconv.Itoa(n.Line) + lineEnd + fmt.Sprintf(format, args...)
	return &yaml.TypeError{Errors: []string{entry}}
}
