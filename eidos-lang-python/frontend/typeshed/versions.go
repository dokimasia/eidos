// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typeshed

import (
	"fmt"
	"path"
	"strconv"
	"strings"
)

// The marks of a line of VERSIONS: the start of a comment, the separator of
// the module and its range, the separator of the two versions of a range,
// the separator of the names of a module, and the major version of every
// range.
const (
	commentMark  = "#"
	rangeMark    = ":"
	toMark       = "-"
	moduleSep    = "."
	majorVersion = "3."
)

// OpenEnd is the last minor version of a module that exists in every later
// version of Python.
const OpenEnd = -1

// Range is the minor versions of Python 3 from First through Last in which
// a module exists. Last is [OpenEnd] for a module that exists in every
// version from First on.
type Range struct {
	First int
	Last  int
}

// Versions is the table of VERSIONS: the range of the minor versions of
// Python 3 in which each listed module exists, by the slash path of the
// module, as asyncio/taskgroups for asyncio.taskgroups. A submodule that
// the table does not list has the range of its nearest listed parent.
type Versions map[string]Range

// ParseVersions parses the bytes of a VERSIONS file. A line has a module, a
// colon and a range, as asyncio.taskgroups: 3.11- or distutils: 3.0-3.11.
// A blank line, and the text from a # to the end of its line, are left out.
//
// Error modes: ParseVersions returns an error that contains the number of
// the line for a line without a colon, and for a range that is not 3.X- or
// 3.X-3.Y.
//
// # Allocation contract
//
// ParseVersions allocates the text of the file, the table and its growth,
// and the slash path of each module whose name has a dot.
func ParseVersions(b []byte) (Versions, error) {
	v := Versions{}
	n := 0
	for line := range strings.Lines(string(b)) {
		n++
		text, _, _ := strings.Cut(line, commentMark)
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		module, span, found := strings.Cut(text, rangeMark)
		first, last, ranged := strings.Cut(strings.TrimSpace(span), toMark)
		r := Range{Last: OpenEnd}
		read := found && ranged
		if read {
			r.First, read = minor(first)
		}
		if read && last != "" {
			r.Last, read = minor(last)
		}
		if !read {
			return nil, fmt.Errorf("typeshed: VERSIONS:%d: %q is not a module, a colon and a range of Python 3",
				n, text)
		}
		v[strings.ReplaceAll(strings.TrimSpace(module), moduleSep, "/")] = r
	}
	return v, nil
}

// Includes reports whether the module at the slash path module exists in
// the minor version minor of Python 3. The range of the module decides
// where the table lists it, and the range of its nearest listed parent
// decides otherwise. A module that neither the table nor a parent in it
// lists is no module of the standard library. Includes does not allocate.
func (v Versions) Includes(module string, minor int) bool {
	for m := module; m != "." && m != ""; m = path.Dir(m) {
		if r, listed := v[m]; listed {
			return minor >= r.First && (r.Last == OpenEnd || minor <= r.Last)
		}
	}
	return false
}

// minor returns the minor version of a version of Python 3, as 11 for
// 3.11, and reports false for a text that is no version of Python 3.
func minor(version string) (int, bool) {
	rest, three := strings.CutPrefix(version, majorVersion)
	n, err := strconv.ParseUint(rest, 10, 8)
	if !three || err != nil {
		return 0, false
	}
	return int(n), true
}
