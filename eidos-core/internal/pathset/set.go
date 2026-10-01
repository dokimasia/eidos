// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package pathset

import (
	"path"
	"strconv"
	"strings"
)

// Clash is how a path collides with a path a [Set] contains.
type Clash uint8

const (
	// ClashNone reports a path that fits beside every path of the set.
	ClashNone Clash = 0
	// ClashCase reports a path that differs from a path of the set only
	// in case.
	ClashCase Clash = 1
	// ClashDirectory reports a path that a path of the set needs as a
	// directory.
	ClashDirectory Clash = 2
	// ClashFile reports a path that needs a path of the set as a
	// directory.
	ClashFile Clash = 3
)

// String returns the clash's name in a diagnostic. A clash outside the
// four returns its number.
func (c Clash) String() string {
	switch c {
	case ClashNone:
		return "none"
	case ClashCase:
		return "case"
	case ClashDirectory:
		return "directory"
	case ClashFile:
		return "file"
	default:
		return "Clash(" + strconv.Itoa(int(c)) + ")"
	}
}

// Set is a set of file paths that coexist in one directory tree on
// every filesystem: no two differ only in case, and no path is a
// directory another path needs. Paths are slash-separated, relative and
// clean, as [io/fs.ValidPath] admits them, and Set does not validate
// them.
//
// The zero Set is empty and ready to use. A Set is not safe for
// concurrent use.
type Set struct {
	// files maps the folded spelling of each path to the path.
	files map[string]string
	// dirs maps the folded spelling of each directory a path needs to
	// the first path added that needs it.
	dirs map[string]string
}

// Clash returns how p collides with the set's paths, and the path of
// the set it collides with. It reports [ClashNone] with the empty path
// where p fits, and for a path the set contains in the same spelling.
// The checks run in a fixed order, so a path that clashes twice reports
// the case clash first, then the directory clash, then the file clash
// nearest p.
func (s *Set) Clash(p string) (Clash, string) {
	key := strings.ToLower(p)
	if other, held := s.files[key]; held && other != p {
		return ClashCase, other
	}
	if other, held := s.dirs[key]; held {
		return ClashDirectory, other
	}
	for dir := path.Dir(key); dir != "." && dir != "/"; dir = path.Dir(dir) {
		if other, held := s.files[dir]; held {
			return ClashFile, other
		}
	}
	return ClashNone, ""
}

// Add records p and every directory p needs. It records the path
// whatever [Set.Clash] reports, so a caller that refuses a clash checks
// before it adds. A path added twice, or added after a path that
// differs from it only in case, keeps the first spelling.
func (s *Set) Add(p string) {
	if s.files == nil {
		s.files = map[string]string{}
		s.dirs = map[string]string{}
	}
	key := strings.ToLower(p)
	if _, held := s.files[key]; !held {
		s.files[key] = p
	}
	for dir := path.Dir(key); dir != "." && dir != "/"; dir = path.Dir(dir) {
		if _, held := s.dirs[dir]; held {
			// Add records a directory with every directory above it, so
			// the rest of the chain is recorded too.
			return
		}
		s.dirs[dir] = p
	}
}
