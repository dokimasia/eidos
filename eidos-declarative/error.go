// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The YAML decoder starts a syntax error with "yaml: line N: ", and each
// entry of a *yaml.TypeError with "line N: ". These constants are the parts
// of the two prefixes.
const (
	syntaxPrefix = "yaml: "
	linePrefix   = "line "
	lineEnd      = ": "
)

// Error is a fault in a plugin directory.
type Error struct {
	// File is the path of the file with the fault. It is the path of the
	// plugin directory for a fault of the directory as a whole.
	File string
	// Line is the line of the fault, counted from 1. It is 0 for a fault of
	// a file as a whole.
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

// faults converts an error of the YAML decoder into one Error for each
// fault in file. A *yaml.TypeError has one fault for each entry, and any
// other error is one fault. When a message starts with a line number,
// faults moves the number into the Line field.
func faults(file string, err error) []*Error {
	entries := []string{strings.TrimPrefix(err.Error(), syntaxPrefix)}
	if typed, is := errors.AsType[*yaml.TypeError](err); is {
		entries = typed.Errors
	}
	out := make([]*Error, 0, len(entries))
	for _, entry := range entries {
		fault := &Error{File: file, Msg: entry}
		if rest, numbered := strings.CutPrefix(entry, linePrefix); numbered {
			digits, msg, _ := strings.Cut(rest, lineEnd)
			if line, perr := strconv.Atoi(digits); perr == nil {
				fault.Line, fault.Msg = line, msg
			}
		}
		out = append(out, fault)
	}
	return out
}

// lineFault returns a *yaml.TypeError whose one entry has the line of n and
// the formatted message. An UnmarshalYAML method returns this error, and the
// decoder then records the fault and continues with the next value.
func lineFault(n *yaml.Node, format string, args ...any) error {
	entry := linePrefix + strconv.Itoa(n.Line) + lineEnd + fmt.Sprintf(format, args...)
	return &yaml.TypeError{Errors: []string{entry}}
}

// joined sorts the faults by their files, and the faults of one file by
// their lines, and returns them joined with [errors.Join]. The sort is
// stable, so two faults at one line keep the order of the checks. Load
// calls it with one fault or more, so the error is not nil.
func joined(fs []*Error) error {
	slices.SortStableFunc(fs, func(a, b *Error) int {
		return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line))
	})
	errs := make([]error, 0, len(fs))
	for _, f := range fs {
		errs = append(errs, f)
	}
	return errors.Join(errs...)
}
