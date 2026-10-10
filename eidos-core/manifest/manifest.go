// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/diag"
)

// Version is the format this package writes and reads: the record split
// into 256 documents by the first byte of each path's SHA-256.
const Version = 2

// The digest every entry's hash states: its name, the hex width of a
// sha256 sum, and the digits it is spelled in.
const (
	hashPrefix = "sha256:"
	hashLen    = 64
	hexDigits  = "0123456789abcdef"
)

// ErrUnsupported reports a document this package cannot read: another
// version, bytes that are not JSON, and a record that breaks the
// format's invariants.
var ErrUnsupported = errors.New("manifest: unsupported format")

// Manifest is the record of every generated file of one workspace, as
// the documents of its buckets state it joined.
type Manifest struct {
	// Version is [Version].
	Version int
	// Workspace names the workspace the record belongs to.
	Workspace string
	// Files is sorted by path, one entry per path.
	Files []Entry
}

// Equal reports whether two manifests record one version, one
// workspace name and the same files. A nil list and an empty one are
// equal, which is how the documents encode both. Equal allocates
// nothing.
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
	// the names the file's frame attributes it to. A plugin's name is
	// the origin its findings report under, so the type is
	// [diag.Origin], which a plugin's ID is an alias of.
	Plugins []diag.Origin `json:"plugins"`
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

// checkFiles returns the first invariant a list of entries breaks, nil
// where it breaks none: sorted by path with one entry per path, each
// path workspace-relative and slash-separated, each entry naming its
// plan, each hash spelled as "sha256:" and 64 lowercase hex digits, and
// each entry's plugins and sources sorted without a repeat.
func checkFiles(files []Entry) error {
	for i, e := range files {
		switch {
		case !fs.ValidPath(e.Path) || e.Path == "." || strings.ContainsRune(e.Path, '\\'):
			return fmt.Errorf("file %d names %q, which is no workspace-relative, slash-separated path", i, e.Path)
		case i > 0 && e.Path <= files[i-1].Path:
			return fmt.Errorf("file %q does not sort after %q, and the files are sorted by path, one entry per path",
				e.Path, files[i-1].Path)
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
