// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"path"
	"strconv"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"
)

// The texts of the changesets of the cases. kernelMinor and kernelPatch have
// a minor and a patch bump of the kernel, with the module path in double
// quotes. facadeMajor has a major bump of the facade, with the module path
// without quotes, and an empty line. emptyChangeset has an empty
// frontmatter. readmeText has no frontmatter. The frontmatter of unclosedText
// does not close. The error for it contains unclosedFault. unknownBump,
// withoutBump and withoutModule are lines of a frontmatter that do not parse.
// frontmatterOpen and frontmatterEnd enclose such a line.
const (
	kernelMinor     = "---\n\"" + kernelModule + "\": minor\n---\n\nAdd a fact.\n"
	kernelPatch     = "---\n\"" + kernelModule + "\": patch\n---\n\nFix a fact.\n"
	facadeMajor     = "---\n" + facadeModule + ": major\n\n---\n\nRemove a facade.\n"
	emptyChangeset  = "---\n---\n\nRelease nothing.\n"
	readmeText      = "# Changesets\n"
	unclosedText    = "---\n\"" + kernelModule + "\": minor\n"
	unclosedFault   = "does not close"
	unknownBump     = "\"" + kernelModule + "\": huge"
	withoutBump     = "\"" + kernelModule + "\""
	withoutModule   = ": minor"
	frontmatterOpen = "---\n"
	frontmatterEnd  = "\n---\n"
)

// The names of the files of the directory of the changesets in the cases.
// nestedFile is a file in a directory whose name has the extension of a
// changeset.
const (
	minorFile    = "add-a-fact.md"
	patchFile    = "fix-a-fact.md"
	majorFile    = "remove-a-facade.md"
	emptyFile    = "release-nothing.md"
	readmeFile   = "README.md"
	configFile   = "config.json"
	nestedFile   = "nested.md/inner.md"
	unclosedFile = "unclosed.md"
)

// The versions of the cases of raise and lineOf. kernelVersion and
// stableVersion are releases in v0 and in v1. rcVersion is a pre-release.
// The minor of hugeVersion does not fit an int. invalidText is not a semantic
// version. nextPatch, nextMinor and nextMajor follow kernelVersion and
// stableVersion. rcPatch follows rcVersion. kernelLine, stableLine and
// pseudoLine are the compatibility lines of kernelVersion, stableVersion and
// pseudoVersion.
const (
	kernelVersion = "v0.2.3"
	stableVersion = "v1.2.3"
	rcVersion     = "v0.3.0-rc.1"
	hugeVersion   = "v0.99999999999999999999.0"
	invalidText   = "0.2.0"
	nextPatch     = "v0.2.4"
	nextMinor     = "v0.3.0"
	nextMajor     = "v2.0.0"
	rcPatch       = "v0.3.1"
	kernelLine    = "v0.2"
	stableLine    = "v1"
	pseudoLine    = "v0.0"
)

// errUnreadable is the error of the file that unreadableFS does not read.
var errUnreadable = errors.New("the file does not read")

// unreadableFS is a map file system whose ReadFile returns errUnreadable for
// the name failing. It reads every other file from the map.
type unreadableFS struct {
	fstest.MapFS
	failing string
}

// ReadFile returns errUnreadable for the name failing and the file of the
// map for every other name.
func (f unreadableFS) ReadFile(name string) ([]byte, error) {
	if name == f.failing {
		return nil, errUnreadable
	}
	return f.MapFS.ReadFile(name)
}

// The ring reads the pending bumps of the changesets and raises a version by
// a bump. It computes the compatibility line of a version.
func TestLine(t *testing.T) {
	t.Parallel()

	t.Run("pending", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			give    fstest.MapFS
			want    map[string]bump
			wantErr string
		}{
			{
				name: "returns no bump for a file system without changesets",
				give: fstest.MapFS{},
				want: map[string]bump{},
			},
			{
				name: "returns the highest bump of each module",
				give: changesets(map[string]string{
					minorFile: kernelMinor, patchFile: kernelPatch, majorFile: facadeMajor,
				}),
				want: map[string]bump{kernelModule: bumpMinor, facadeModule: bumpMajor},
			},
			{
				name: "returns no bump for an empty changeset",
				give: changesets(map[string]string{emptyFile: emptyChangeset}),
				want: map[string]bump{},
			},
			{
				name: "skips a file without a frontmatter",
				give: changesets(map[string]string{readmeFile: readmeText}),
				want: map[string]bump{},
			},
			{
				name: "skips a file without the extension of a changeset",
				give: changesets(map[string]string{configFile: kernelMinor}),
				want: map[string]bump{},
			},
			{
				name: "skips a directory with the extension of a changeset",
				give: changesets(map[string]string{nestedFile: kernelMinor}),
				want: map[string]bump{},
			},
			{
				name:    "returns an error for a frontmatter that does not close",
				give:    changesets(map[string]string{unclosedFile: unclosedText}),
				wantErr: unclosedFault,
			},
			{
				name:    "returns an error for a line with a bump that the changesets do not define",
				give:    changesets(map[string]string{unclosedFile: frontmatterOpen + unknownBump + frontmatterEnd}),
				wantErr: strconv.Quote(unknownBump),
			},
			{
				name:    "returns an error for a line without a bump",
				give:    changesets(map[string]string{unclosedFile: frontmatterOpen + withoutBump + frontmatterEnd}),
				wantErr: strconv.Quote(withoutBump),
			},
			{
				name:    "returns an error for a line without a module",
				give:    changesets(map[string]string{unclosedFile: frontmatterOpen + withoutModule + frontmatterEnd}),
				wantErr: strconv.Quote(withoutModule),
			},
			{
				name:    "returns the error of a directory of the changesets that does not read",
				give:    fstest.MapFS{changesetDir: {Data: []byte(readmeText)}},
				wantErr: changesetDir,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := pending(tt.give)
				if tt.wantErr != "" {
					assert.HasError(t, err, "the changesets do not read")
					expect.That(t, err.Error()).
						HasPrefix(commandName+": ", "the error starts with the name of the command").
						Contains(tt.wantErr, "the error contains the fault")
					return
				}
				assert.NoError(t, err, "the changesets read")
				assert.Equal(t, got, tt.want, "pending returns the highest bump of each module")
			})
		}

		t.Run("returns the error of a changeset that does not read", func(t *testing.T) {
			t.Parallel()

			fsys := unreadableFS{
				MapFS:   changesets(map[string]string{minorFile: kernelMinor}),
				failing: path.Join(changesetDir, minorFile),
			}
			_, err := pending(fsys)
			assert.ErrorIs(t, err, errUnreadable, "the error wraps the error of the file")
		})
	})

	t.Run("raise", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			give     string
			giveBump bump
			want     string
		}{
			{name: "returns the version for no bump", give: kernelVersion, giveBump: bumpNone, want: kernelVersion},
			{name: "returns the next patch for a patch", give: kernelVersion, giveBump: bumpPatch, want: nextPatch},
			{name: "returns the next minor for a minor", give: kernelVersion, giveBump: bumpMinor, want: nextMinor},
			{name: "returns the next major for a major", give: stableVersion, giveBump: bumpMajor, want: nextMajor},
			{name: "drops the pre-release of a version", give: rcVersion, giveBump: bumpPatch, want: rcPatch},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := raise(tt.give, tt.giveBump)
				assert.NoError(t, err, "the version raises")
				assert.Equal(t, got, tt.want, "raise returns the next version")
			})
		}

		t.Run("returns an error for a version that is not a semantic version", func(t *testing.T) {
			t.Parallel()

			_, err := raise(invalidText, bumpMinor)
			assert.HasError(t, err, "the version does not raise")
			expect.That(t, err.Error()).
				HasPrefix(commandName+": ", "the error starts with the name of the command").
				Contains(strconv.Quote(invalidText), "the error contains the version")
		})

		t.Run("returns an error for a version with a number that does not fit an int", func(t *testing.T) {
			t.Parallel()

			_, err := raise(hugeVersion, bumpPatch)
			assert.ErrorIs(t, err, strconv.ErrRange, "the error wraps the error of the number")
		})
	})

	t.Run("lineOf", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want string
		}{
			{name: "returns the minor line of a version in v0", give: kernelVersion, want: kernelLine},
			{name: "returns the major line of a version from v1", give: stableVersion, want: stableLine},
			{name: "returns the minor line of a pseudo-version in v0", give: pseudoVersion, want: pseudoLine},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, lineOf(tt.give), tt.want, "lineOf returns the compatibility line")
			})
		}
	})
}

// changesets returns a map file system whose directory of changesets has a
// file for each entry of texts. The key of an entry is the name of the file,
// and the value is its text.
func changesets(texts map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for name, text := range texts {
		fsys[path.Join(changesetDir, name)] = &fstest.MapFile{Data: []byte(text)}
	}
	return fsys
}
