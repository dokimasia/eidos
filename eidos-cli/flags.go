// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"flag"
	"fmt"
	"strconv"
)

// Every command accepts the flags with these names.
const (
	formatFlag  = "format"
	configFlag  = "config"
	strictFlag  = "strict"
	verboseFlag = "verbose"
	quietFlag   = "quiet"
	noColorFlag = "no-color"
)

// ErrFormat reports a value of --format that is neither text nor json.
// [Format.Set] returns an error that wraps it.
var ErrFormat = errors.New("cli: unknown output format")

// Format is the output format that --format selects. The zero Format is
// FormatText.
type Format uint8

const (
	// FormatText renders text for a person.
	FormatText Format = 0
	// FormatJSON renders one JSON event per line under the versioned schema.
	FormatJSON Format = 1
)

var _ flag.Value = (*Format)(nil)

// Valid reports whether f is one of the two formats.
func (f Format) Valid() bool { return f <= FormatJSON }

// String returns the name of the format: text for FormatText and json for
// FormatJSON. For a value outside the formats, it returns the number in
// the form Format(n).
func (f Format) String() string {
	switch f {
	case FormatText:
		return "text"
	case FormatJSON:
		return "json"
	default:
		return "Format(" + strconv.Itoa(int(f)) + ")"
	}
}

// Set sets f to the format with the name s. It implements [flag.Value].
//
// Error modes: Set returns an error that wraps [ErrFormat] when s is neither
// text nor json. The flag package formats the error of a value with the verb
// %v, so the error of a parse of the flags contains the text of the error and
// does not wrap ErrFormat.
func (f *Format) Set(s string) error {
	for g := FormatText; g.Valid(); g++ {
		if g.String() == s {
			*f = g
			return nil
		}
	}
	return fmt.Errorf("%w %q: use %s or %s", ErrFormat, s, FormatText, FormatJSON)
}

// Flags are the flags that every command accepts.
type Flags struct {
	// Config is the path that --config sets, and empty when the flag is not
	// set. A relative path is relative to the working directory.
	Config string
	// Format is the output format that --format selects.
	Format Format
	// Strict is true under --strict, which reports every Warning as an
	// Error.
	Strict bool
	// Verbose is true under --verbose, which renders Info findings.
	Verbose bool
	// Quiet is true under --quiet, which renders the Error findings alone.
	// A kernel command refuses --quiet with --verbose.
	Quiet bool
	// NoColor is true under --no-color, which renders text without colour.
	NoColor bool
}

// Register defines the flags on fs. Each flag sets its field of f.
func (f *Flags) Register(fs *flag.FlagSet) {
	fs.Var(&f.Format, formatFlag, "the output `format`, text or json")
	fs.StringVar(&f.Config, configFlag, "", "the config `file`, which replaces the search for one")
	fs.BoolVar(&f.Strict, strictFlag, false, "report every warning as an error")
	fs.BoolVar(&f.Verbose, verboseFlag, false, "render info findings")
	fs.BoolVar(&f.Quiet, quietFlag, false, "render error findings alone")
	fs.BoolVar(&f.NoColor, noColorFlag, false, "render text without colour")
}
