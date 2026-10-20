// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"golang.org/x/mod/semver"
)

// The directory of the changesets of a checkout, the extension of a
// changeset, the separator of its lines, the line that opens and closes the
// frontmatter of a changeset, and the separator of a module and its bump in
// that frontmatter.
const (
	changesetDir = ".changeset"
	changesetExt = ".md"
	lineSep      = "\n"
	frontmatter  = "---"
	bumpSep      = ":"
)

// The prefix of a semantic version, the separator of its pre-release, the
// layout of the three numbers of a release, and the major version without a
// promise of compatibility.
const (
	versionPrefix = "v"
	prereleaseSep = "-"
	numbersLayout = "%d.%d.%d"
	unstableMajor = "v0"
)

// bump is the kind of change to the version of a module. The frontmatter of a
// changeset has one bump for each module that the changeset releases. A
// higher bump is a larger change. The next version of a module follows from
// the highest bump among its changesets.
type bump uint8

const (
	// bumpNone keeps the version.
	bumpNone bump = iota
	// bumpPatch raises the patch.
	bumpPatch
	// bumpMinor raises the minor and clears the patch.
	bumpMinor
	// bumpMajor raises the major and clears the minor and the patch.
	bumpMajor
)

// bumpWords maps the word of each bump in the frontmatter of a changeset to
// the bump.
var bumpWords = map[string]bump{"patch": bumpPatch, "minor": bumpMinor, "major": bumpMajor}

// pending returns the highest bump of each module across the changesets of
// fsys. A changeset is a file of the directory changesetDir with the
// extension changesetExt whose first line opens a frontmatter. Each line of
// the frontmatter is a module path, a colon and a bump. Double quotes around
// the module path are optional. An empty changeset has an empty frontmatter
// and no bump. pending skips a file without a frontmatter, such as a README.
// It returns an empty map for a file system without the directory.
//
// Error modes:
//   - the error of a directory or a file that does not read
//   - an error for a frontmatter that does not close
//   - an error for a line of a frontmatter that is not a module path, a colon
//     and a known bump
func pending(fsys fs.FS) (map[string]bump, error) {
	entries, err := fs.ReadDir(fsys, changesetDir)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]bump{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", commandName, err)
	}
	out := map[string]bump{}
	for _, e := range entries {
		if e.IsDir() || path.Ext(e.Name()) != changesetExt {
			continue
		}
		name := path.Join(changesetDir, e.Name())
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", commandName, err)
		}
		lines := strings.Split(string(data), lineSep)
		if strings.TrimSpace(lines[0]) != frontmatter {
			continue
		}
		end := slices.IndexFunc(lines[1:], func(line string) bool { return strings.TrimSpace(line) == frontmatter })
		if end < 0 {
			return nil, fmt.Errorf("%s: the frontmatter of %s does not close", commandName, name)
		}
		for _, line := range lines[1 : end+1] {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			mod, word, cut := strings.Cut(line, bumpSep)
			mod = strings.Trim(strings.TrimSpace(mod), `"`)
			b, known := bumpWords[strings.TrimSpace(word)]
			if !cut || mod == "" || !known {
				return nil, fmt.Errorf("%s: the frontmatter of %s states %q instead of a module and a bump",
					commandName, name, line)
			}
			out[mod] = max(out[mod], b)
		}
	}
	return out, nil
}

// raise returns the version that follows version under the bump b. raise
// drops the pre-release of version, so a patch of v0.3.0-rc.1 is v0.3.1. It
// returns version unchanged for bumpNone.
//
// Error modes: an error for a version that is not a semantic version, and an
// error for a version with a number that does not fit an int.
func raise(version string, b bump) (string, error) {
	if !semver.IsValid(version) {
		return "", fmt.Errorf("%s: %q is not a semantic version", commandName, version)
	}
	if b == bumpNone {
		return version, nil
	}
	release, _, _ := strings.Cut(strings.TrimPrefix(semver.Canonical(version), versionPrefix), prereleaseSep)
	var major, minor, patch int
	if _, err := fmt.Sscanf(release, numbersLayout, &major, &minor, &patch); err != nil {
		return "", fmt.Errorf("%s: the version %s: %w", commandName, version, err)
	}
	switch b {
	case bumpMajor:
		major, minor, patch = major+1, 0, 0
	case bumpMinor:
		minor, patch = minor+1, 0
	default:
		patch++
	}
	return versionPrefix + fmt.Sprintf(numbersLayout, major, minor, patch), nil
}

// lineOf returns the compatibility line of a valid version. The line of a
// version in v0 is its major and its minor, such as v0.3. The line of a later
// version is its major, such as v2. A version of a later line can break the
// API of an earlier line.
func lineOf(version string) string {
	if semver.Major(version) == unstableMajor {
		return semver.MajorMinor(version)
	}
	return semver.Major(version)
}
