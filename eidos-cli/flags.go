// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"flag"
	"fmt"
	"strconv"

	gocli "go.stealthscale.io/cli"
)

// The messages of the commands name these flags.
const (
	formatFlag  = "format"
	quietFlag   = "quiet"
	verboseFlag = "verbose"
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

// Set sets f to the format with the name s. It implements [flag.Value], and
// go-cli converts each value of --format through it.
//
// Error modes: Set returns an error that wraps [ErrFormat] when s is neither
// text nor json. go-cli puts the text of the error into the message of its
// usage error, which does not wrap ErrFormat.
func (f *Format) Set(s string) error {
	for g := FormatText; g.Valid(); g++ {
		if g.String() == s {
			*f = g
			return nil
		}
	}
	return fmt.Errorf("%w %q: use %s or %s", ErrFormat, s, FormatText, FormatJSON)
}

// Global are the global options of a binary built with eidos. Every command
// of the app accepts them before and after its name. go-cli reads each
// option from its flag, its environment variable and its key in the config
// file, such as --strict, ACME_STRICT and strict for the brand acme.
type Global struct {
	// Config is the path of the config file. A run that sets neither the
	// flag nor the variable takes the path that the search of Prepare finds,
	// and the empty path when the search does not find a file. go-cli reads
	// a relative path against the working directory of the process, also
	// under Chdir.
	Config gocli.ConfigFile `help:"Read this config file instead of the one that the search finds."`
	// Chdir is the working directory of the command, and empty for the
	// working directory of the process. A relative path is relative to the
	// working directory of the process. The search for the config file
	// starts at it, and the patterns of a command are relative to it.
	Chdir string `help:"Run in this directory instead of the working directory." short:"C" key:"-"`
	// Format is the output format, FormatText or FormatJSON.
	Format Format `help:"Write the output as text or json." default:"text"`
	// Strict reports every Warning as an Error.
	Strict bool `help:"Report every warning as an error."`
	// Verbose renders the Info findings.
	Verbose bool `help:"Render info findings."`
	// Quiet renders the Error findings alone. A command refuses Quiet with
	// Verbose.
	Quiet bool `help:"Render error findings alone."`
	// NoColor renders text without colour.
	NoColor bool `help:"Render text without colour."`
}
