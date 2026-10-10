// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bytes"
	"errors"
	"fmt"
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
// any other file into a [Document]. Decode is strict. It returns an Error
// for an unknown key, for a value of the wrong type, and for a size or a
// policy that does not parse. Decode continues after such a fault, and
// joins the Errors of all faults with [errors.Join].
//
// Error modes:
//
//   - A file that is not a mapping, a file without a version, and a file
//     with a version other than [Current] return an *Error.
//   - A file that is not valid YAML returns an error that wraps one *Error.
//   - A file with unknown keys or with values that do not decode returns an
//     error that wraps one *Error for each such key and value.
//   - A list without workspaces returns an *Error, and a list with entries
//     without a root returns an error that wraps one *Error for each such
//     entry.
func Decode(name string, data []byte) (File, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return File{}, faults(name, err)
	}
	noVersion := fmt.Sprintf("the file has no version key: add version: %d", Current)
	if len(doc.Content) == 0 {
		return File{}, &Error{File: name, Line: 1, Msg: noVersion}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return File{}, &Error{File: name, Line: root.Line, Msg: "the file is not a mapping of keys to values"}
	}
	version := value(root, versionKey)
	if version == nil {
		return File{}, &Error{File: name, Line: root.Line, Msg: noVersion}
	}
	var v Version
	if err := version.Decode(&v); err != nil || v != Current {
		msg := fmt.Sprintf("the file has version %s, and this binary reads only version %d", version.Value, Current)
		return File{}, &Error{File: name, Line: version.Line, Msg: msg}
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if workspaces := value(root, workspacesKey); workspaces != nil {
		var l List
		if err := dec.Decode(&l); err != nil {
			return File{}, faults(name, err)
		}
		if err := l.check(name, workspaces); err != nil {
			return File{}, err
		}
		return File{List: &l}, nil
	}
	var d Document
	if err := dec.Decode(&d); err != nil {
		return File{}, faults(name, err)
	}
	return File{Document: &d}, nil
}

// value returns the value of key in the mapping m, or nil when m does not
// have the key.
func value(m *yaml.Node, key string) *yaml.Node {
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
