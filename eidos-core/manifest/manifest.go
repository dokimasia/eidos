// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/plugin"
)

// Version is the format this package writes and reads.
const Version = 1

// The digest every entry's hash states: its name, the hex width of a
// sha256 sum, and the digits it is spelled in.
const (
	hashPrefix = "sha256:"
	hashLen    = 64
	hexDigits  = "0123456789abcdef"
)

// indent is what Encode indents each nesting level by.
const indent = "  "

// ErrUnsupported reports a manifest this package cannot read: another
// version, bytes that are not JSON, and a record that breaks the
// format's invariants.
var ErrUnsupported = errors.New("manifest: unsupported format")

// The empty lists Encode writes where an entry's slice is nil, shared
// so a nil slice costs no allocation.
var (
	noPlugins = []plugin.ID{}
	noSources = []string{}
)

// Manifest is the record of every generated file of one workspace.
type Manifest struct {
	Version   int    `json:"version"`
	Workspace string `json:"workspace"`
	// Files is sorted by path, one entry per path.
	Files []Entry `json:"files"`
}

// Equal reports whether two manifests record one version, one
// workspace name and the same files. A nil list and an empty one are
// equal, which is how Encode writes both. Equal allocates nothing.
func (m Manifest) Equal(o Manifest) bool {
	return m.Version == o.Version && m.Workspace == o.Workspace &&
		slices.EqualFunc(m.Files, o.Files, Entry.equal)
}

// Entry is one generated file.
type Entry struct {
	// Path is workspace-relative and slash-separated.
	Path string `json:"path"`
	// Plan is the plan that wrote the file.
	Plan string `json:"plan"`
	// Hash is "sha256:" and the hex digest of the file's bytes, frame
	// included: the value [go.dokimi.dev/eidos/core/output.Written]
	// records.
	Hash string `json:"hash"`
	// Plugins are the emitters whose units assembled the file and the
	// plugins that appended into the units' slots, distinct and sorted:
	// the names the file's frame attributes it to.
	Plugins []plugin.ID `json:"plugins"`
	// Sources are the canonical identities of the declarations the file
	// derives from, sorted, in the canonical spelling that symbol.Parse
	// reads back.
	Sources []string `json:"sources"`
}

// equal reports whether two entries record one file alike.
func (e Entry) equal(o Entry) bool {
	return e.Path == o.Path && e.Plan == o.Plan && e.Hash == o.Hash &&
		slices.Equal(e.Plugins, o.Plugins) && slices.Equal(e.Sources, o.Sources)
}

// Encode returns the manifest's bytes: JSON indented by two spaces,
// with a final newline, and [] wherever a list is nil. Two equal
// manifests encode to equal bytes. It refuses a manifest that breaks
// the package's invariants, naming the first entry that breaks one,
// and copies the file list once, so the caller's manifest is not
// changed.
func Encode(m Manifest) ([]byte, error) {
	if err := check(m); err != nil {
		return nil, fmt.Errorf("manifest: encode: %w", err)
	}
	out := m
	out.Files = make([]Entry, len(m.Files))
	for i, e := range m.Files {
		if e.Plugins == nil {
			e.Plugins = noPlugins
		}
		if e.Sources == nil {
			e.Sources = noSources
		}
		out.Files[i] = e
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", indent)
	if err := enc.Encode(out); err != nil {
		return nil, fmt.Errorf("manifest: encode: %w", err)
	}
	return buf.Bytes(), nil
}

// Decode reads a manifest. It returns an error wrapping
// [ErrUnsupported] for another version, for bytes that are not a JSON
// object of the format, and for a record that breaks the package's
// invariants. A key the format does not know is skipped.
func Decode(b []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return Manifest{}, fmt.Errorf("%w: %w", ErrUnsupported, err)
	}
	if err := check(m); err != nil {
		return Manifest{}, fmt.Errorf("%w: %w", ErrUnsupported, err)
	}
	return m, nil
}

// check returns the first invariant a manifest breaks, nil where it
// breaks none.
func check(m Manifest) error {
	if m.Version != Version {
		return fmt.Errorf("version %d is not version %d", m.Version, Version)
	}
	for i, e := range m.Files {
		switch {
		case !fs.ValidPath(e.Path) || e.Path == "." || strings.ContainsRune(e.Path, '\\'):
			return fmt.Errorf("file %d names %q, which is no workspace-relative, slash-separated path", i, e.Path)
		case i > 0 && e.Path <= m.Files[i-1].Path:
			return fmt.Errorf("file %q does not sort after %q, and the files are sorted by path, one entry per path",
				e.Path, m.Files[i-1].Path)
		case e.Plan == "":
			return fmt.Errorf("file %q names no plan", e.Path)
		case !hashed(e.Hash):
			return fmt.Errorf("file %q states the hash %q, which is not %s and %d lowercase hex digits",
				e.Path, e.Hash, hashPrefix, hashLen)
		case !ascending(e.Plugins):
			return fmt.Errorf("file %q lists a plugin out of order or twice", e.Path)
		case !ascending(e.Sources):
			return fmt.Errorf("file %q lists a source out of order or twice", e.Path)
		}
	}
	return nil
}

// hashed reports whether a hash is "sha256:" and 64 lowercase hex
// digits, the spelling every digest of the kernel takes.
func hashed(h string) bool {
	digits, named := strings.CutPrefix(h, hashPrefix)
	return named && len(digits) == hashLen && strings.Trim(digits, hexDigits) == ""
}

// ascending reports whether a list is sorted with no repeat.
func ascending[S ~[]E, E cmp.Ordered](s S) bool {
	for i := 1; i < len(s); i++ {
		if s[i] <= s[i-1] {
			return false
		}
	}
	return true
}
