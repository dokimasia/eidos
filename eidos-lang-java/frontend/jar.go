// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
)

// The parts of a JAR the frontend reads (the JAR File Specification): the
// separator of an entry's path, the directory of the metadata, its
// versioned class files and its manifest, the main section's attribute
// that marks a multi-release JAR, and that attribute's value, and the
// manifest's separator of a name and a value, the opening of a continued
// line and the bytes that end a line.
const (
	entrySeparator    = "/"
	metaInfDir        = "META-INF/"
	versionsDir       = "META-INF/versions/"
	manifestEntry     = "META-INF/MANIFEST.MF"
	multiReleaseName  = "Multi-Release"
	multiReleaseValue = "true"
	headerSeparator   = ":"
	continuationMark  = " "
	lineEnd           = "\r\n"
)

// maxEntrySize caps the uncompressed size of one JAR entry the frontend
// reads, 64 MiB, so a crafted JAR cannot exhaust memory.
const maxEntrySize = 67108864

// errEntryTooLarge reports a JAR entry past maxEntrySize.
var errEntryTooLarge = errors.New("frontend: the entry is larger than 64 MiB")

// jarClasses decodes the class files a JAR has for a release: each
// class's entry under the highest META-INF/versions/N with N at most the
// release where the manifest marks the JAR multi-release, and its root
// entry otherwise. Release 0 takes the highest version. A JAR that does
// not open, and an entry that does not read or decode, reports under
// [BadClassFile], and loads nothing. A manifest that does not read
// reports the same way, and the JAR's root entries load.
func jarClasses(u *plugin.SourceUnit, p string, data []byte, release int) []classFile {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		u.Errorf(BadClassFile, classPos(p), "%v", err)
		return nil
	}
	multi, err := multiRelease(zr)
	if err != nil {
		u.Errorf(BadClassFile, classPos(p), "%s: %v", manifestEntry, err)
	}
	chosen := map[string]*zip.File{}
	versions := map[string]int{}
	for _, f := range zr.File {
		name, version, ok := classEntry(f.Name, multi, release)
		if !ok {
			continue
		}
		if old, met := versions[name]; met && old >= version {
			continue
		}
		chosen[name], versions[name] = f, version
	}
	var out []classFile
	for _, name := range slices.Sorted(maps.Keys(chosen)) {
		f := chosen[name]
		data, err := readEntry(f)
		if err != nil {
			u.Errorf(BadClassFile, classPos(p), "%s: %v", f.Name, err)
			continue
		}
		if cf, ok := decodeClass(u, p, f.Name, data); ok {
			out = append(out, cf)
		}
	}
	return out
}

// classEntry returns the class path a JAR entry provides and the version
// it provides it for, 0 for a root entry, and reports false for an entry
// that is no class file the release reads: a file of another kind, an
// entry under META-INF other than a versioned class, a versioned class
// of a JAR that is not multi-release, and one of a version past the
// release.
func classEntry(name string, multi bool, release int) (string, int, bool) {
	if !strings.HasSuffix(name, classExt) {
		return "", 0, false
	}
	rest, versioned := strings.CutPrefix(name, versionsDir)
	if !versioned {
		return name, 0, !strings.HasPrefix(name, metaInfDir)
	}
	// Without a slash, number is the entry's name, which ends in .class
	// and is no number.
	number, class, _ := strings.Cut(rest, entrySeparator)
	version, err := strconv.Atoi(number)
	if !multi || err != nil || release > 0 && version > release {
		return "", 0, false
	}
	return class, version, true
}

// multiRelease reports whether a JAR's manifest marks it multi-release:
// its main section, up to the first blank line, states the attribute
// Multi-Release with the value true, both compared without case. A
// line that opens with a space continues the line before it. A JAR
// without a manifest is not multi-release, and a manifest that does not
// read returns readEntry's error.
func multiRelease(zr *zip.Reader) (bool, error) {
	i := slices.IndexFunc(zr.File, func(f *zip.File) bool { return f.Name == manifestEntry })
	if i < 0 {
		return false, nil
	}
	data, err := readEntry(zr.File[i])
	if err != nil {
		return false, err
	}
	var lines []string
	for line := range strings.Lines(string(data)) {
		line = strings.TrimRight(line, lineEnd)
		if line == "" {
			break
		}
		if rest, continued := strings.CutPrefix(line, continuationMark); continued && len(lines) > 0 {
			lines[len(lines)-1] += rest
			continue
		}
		lines = append(lines, line)
	}
	for _, line := range lines {
		name, value, _ := strings.Cut(line, headerSeparator)
		if strings.EqualFold(strings.TrimSpace(name), multiReleaseName) &&
			strings.EqualFold(strings.TrimSpace(value), multiReleaseValue) {
			return true, nil
		}
	}
	return false, nil
}

// readEntry returns one JAR entry's bytes, and refuses an entry larger
// than maxEntrySize.
func readEntry(f *zip.File) ([]byte, error) {
	if f.UncompressedSize64 > maxEntrySize {
		return nil, errEntryTooLarge
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// classPos returns the position findings about a class file, a JAR or one
// of its entries name: the start of the file.
func classPos(p string) position.Pos {
	return position.Pos{File: p, Line: 1, Col: 1}
}
