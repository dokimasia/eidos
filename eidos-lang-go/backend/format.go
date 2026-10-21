// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"bytes"
	"os"
	"strconv"
	"unicode/utf8"

	"github.com/golangci/golines/shorten"
	gofumpt "mvdan.cc/gofumpt/format"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// formatVersion is the version of Go that gofumpt formats for. gofumpt
// writes an octal literal with the prefix 0o from Go 1.13 on, and no other
// rule of gofumpt v0.12.0 depends on the version.
const formatVersion = "go1.13"

// tabWidth is the number of columns that golines counts for a tab.
const tabWidth = 4

// The environment variable that turns on the split of long lines of
// gofumpt, and the value that turns it on.
const (
	splitVariable = "GOFUMPT_SPLIT_LONG_LINES"
	splitOn       = "on"
)

// lineBlanks are the characters before the first character of a line that
// golines skips when it looks for a comment.
const lineBlanks = " \t"

// The parts of a line that golines reads: the end of a line, a tab, and the
// opening of a line comment, which golines does not wrap.
var (
	lineEnd     = []byte("\n")
	lineTab     = []byte("\t")
	lineComment = []byte("//")
)

// Format formats one rendered Go file as gofumpt v0.12.0 and then golines
// v0.16.0 format it, so a project that runs gofmt, gofumpt or golines does
// not change the file.
//
//   - gofumpt formats with its three extra rules: grouped parameters,
//     clothed returns and balanced calls. It formats for Go 1.13 and later
//     and without a module path, as golangci-lint does by default.
//   - golines wraps each line of code that passes the width that the plan
//     selects through [golang.Width], with a tab of four columns. It keeps
//     each comment and each struct tag as gofumpt leaves it. A policy that
//     does not declare the width, such as the zero Policy, selects
//     [golang.Width100], and [golang.WidthNone] wraps no line. Format calls
//     golines only for a file with a line of code that passes the width.
//
// Error modes:
//   - an environment that sets GOFUMPT_SPLIT_LONG_LINES to on returns an
//     error that names the variable, because gofumpt then splits long lines
//     and the bytes of a file would depend on the environment;
//   - a width that is not a number of columns returns an error with the
//     width;
//   - a source that does not parse returns an error that wraps the error of
//     gofumpt, and a failure of golines returns an error that wraps its
//     error.
//
// Format is safe for concurrent use. It allocates what gofumpt allocates for
// the parse and the print of the file, and what golines allocates where it
// wraps a line.
func Format(src []byte, p plugin.Policy) ([]byte, error) {
	if os.Getenv(splitVariable) == splitOn {
		return nil, refuse("gofumpt splits long lines under %s=%s, so the format would depend on the environment",
			splitVariable, splitOn)
	}
	width, declared := p.Lookup(golang.Width)
	if !declared {
		width = golang.Width100
	}
	columns := 0
	if width != golang.WidthNone {
		n, err := strconv.Atoi(string(width))
		if err != nil {
			return nil, refuse("the width %q is not a number of columns", width)
		}
		columns = n
	}
	out, err := gofumpt.Source(src, gofumpt.Options{
		LangVersion: formatVersion,
		Extra:       gofumpt.Extra{GroupParams: true, ClotheReturns: true, BalanceCalls: true},
	})
	// gofumpt returns no output with its error, so no line of a source that
	// it refuses passes the width.
	if columns > 0 && passes(out, columns) {
		out, err = shorten.NewShortener(&shorten.Config{MaxLen: columns, TabLen: tabWidth, ChainSplitDots: true}).
			Process(out)
	}
	if err != nil {
		return nil, refuse("%w", err)
	}
	return out, nil
}

// passes reports whether a line of code of src has more columns than
// columns, as golines counts them: one column for each character and
// tabWidth for each tab. A line comment is no line of code. passes does not
// allocate.
func passes(src []byte, columns int) bool {
	for line := range bytes.Lines(src) {
		line = bytes.TrimSuffix(line, lineEnd)
		if bytes.HasPrefix(bytes.TrimLeft(line, lineBlanks), lineComment) {
			continue
		}
		if utf8.RuneCount(line)+(tabWidth-1)*bytes.Count(line, lineTab) > columns {
			return true
		}
	}
	return false
}
