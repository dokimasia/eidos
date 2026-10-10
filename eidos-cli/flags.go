// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli

import (
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
	noColorFlag = "no-color"
)

// Format is the output format that --format selects. The zero Format is
// text.
type Format uint8

const (
	// FormatText renders text for a person.
	FormatText Format = 1
	// FormatJSON renders one JSON event per line under the versioned schema.
	FormatJSON Format = 2
)

var _ flag.Value = (*Format)(nil)

// Valid reports whether f is the zero Format or one of the two formats.
func (f Format) Valid() bool { return f <= FormatJSON }

// String returns the name of the format: text for the zero Format and for
// FormatText, and json for FormatJSON. For a value outside the formats, it
// returns the number in the form Format(n).
func (f Format) String() string {
	switch f {
	case 0, FormatText:
		return "text"
	case FormatJSON:
		return "json"
	default:
		return "Format(" + strconv.Itoa(int(f)) + ")"
	}
}

// Set sets f to the format with the name s. It implements [flag.Value].
//
// Error modes: Set returns an error when s is neither text nor json.
func (f *Format) Set(s string) error {
	for g := FormatText; g.Valid(); g++ {
		if g.String() == s {
			*f = g
			return nil
		}
	}
	return fmt.Errorf("cli: %q is not an output format: use %s or %s", s, FormatText, FormatJSON)
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
	// NoColor is true under --no-color, which renders text without colour.
	NoColor bool
}

// Register defines the flags on fs. Each flag sets its field of f.
func (f *Flags) Register(fs *flag.FlagSet) {
	fs.Var(&f.Format, formatFlag, "the output `format`, text or json")
	fs.StringVar(&f.Config, configFlag, "", "the config `file`, which replaces the search for one")
	fs.BoolVar(&f.Strict, strictFlag, false, "report every warning as an error")
	fs.BoolVar(&f.Verbose, verboseFlag, false, "render info findings")
	fs.BoolVar(&f.NoColor, noColorFlag, false, "render text without colour")
}
