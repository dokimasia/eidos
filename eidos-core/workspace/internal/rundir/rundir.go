// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rundir

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/output"
)

// digestPrefix names the digest a record's entry states a file's hash
// in, ahead of the hex digits.
const digestPrefix = "sha256:"

// Files returns the slash-separated path of every file under root,
// relative to root, in lexical order. It stops the check where the
// directory does not walk.
func Files(tb assert.TB, root string) []string {
	tb.Helper()

	var out []string
	err := fs.WalkDir(os.DirFS(root), ".", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			out = append(out, path)
		}
		return err
	})
	assert.NoError(tb, err, "the run's directory walks")
	return out
}

// Path returns the operating system's spelling of a slash-separated
// path under root.
func Path(root, slash string) string {
	return filepath.Join(root, filepath.FromSlash(slash))
}

// Sealed reports whether a slash-separated path relative to root is in
// the brand's sealed state: the state directory outside the manifest's
// documents, which a run rewrites as it records what it read. A warm
// run and a cold run over one tree may leave different bytes there.
func Sealed(brand output.Brand, path string) bool {
	return strings.HasPrefix(path, ledger.StateDir(brand)+"/") &&
		!strings.HasPrefix(path, ledger.ManifestPath(brand)+"/")
}

// Framed returns the text of every file under root that has the
// brand's frame, keyed by its slash-separated path relative to root. A
// file another brand framed is not the run's, and it is absent.
func Framed(tb assert.TB, root string, brand output.Brand) map[string]string {
	tb.Helper()

	out := map[string]string{}
	for _, path := range Files(tb, root) {
		b, err := os.ReadFile(Path(root, path))
		assert.NoError(tb, err, "a file of the run's directory reads")
		if p, held := output.Read(b); held && p.Brand == brand {
			out[path] = string(b)
		}
	}
	return out
}

// Texts returns a fixture's wanted files as text, keyed as the fixture
// keys them, so a comparison with [Framed] shows a mismatch as a diff.
func Texts(want map[string][]byte) map[string]string {
	out := make(map[string]string, len(want))
	for path, b := range want {
		out[path] = string(b)
	}
	return out
}

// Digest returns the digest of the file at a slash-separated path under
// root, spelled the way a record's entry states it. It stops the check
// where the file does not read.
func Digest(tb assert.TB, root, path string) string {
	tb.Helper()

	b, err := os.ReadFile(Path(root, path))
	assert.NoError(tb, err, "a generated file reads")
	sum := sha256.Sum256(b)
	return digestPrefix + hex.EncodeToString(sum[:])
}

// Record returns the record in the brand's state directory under root:
// the documents of its manifest, joined, and the empty manifest where
// the directory contains no document, which is how an empty record is
// stored. It stops the check where root does not open or a document
// does not decode.
func Record(tb assert.TB, root string, brand output.Brand) manifest.Manifest {
	tb.Helper()

	l, err := ledger.OpenDir(root, brand)
	assert.NoError(tb, err, "the workspace root opens")
	if err != nil {
		return manifest.Manifest{}
	}
	m, _, err := state.ReadManifest(context.Background(), l)
	assert.NoError(tb, err, "the record's documents decode")
	return m
}
