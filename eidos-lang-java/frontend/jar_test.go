// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"archive/zip"
	"bytes"
	"hash/crc32"
	"maps"
	"path"
	"slices"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/java/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// The entries the JAR cases write: the manifest, a class at its root
// path and under two versions, and the main sections that do and do not
// mark a JAR multi-release.
const (
	manifestName  = "META-INF/MANIFEST.MF"
	circlePath    = "com/acme/lib/Circle.class"
	versioned11   = "META-INF/versions/11/" + circlePath
	versioned21   = "META-INF/versions/21/" + circlePath
	multiRelease  = "Manifest-Version: 1.0\nMulti-Release: true\n"
	singleRelease = "Manifest-Version: 1.0\n"
	unknownMethod = 99
	release17     = 17
	release21     = 21
	release25     = 25
	circleName    = "Circle"
	squareName    = "Square"
	shapeName     = "Shape"
)

// The cap on an entry's size, 64 MiB, a size past it, and the ends of
// the messages that report an entry over the cap, an entry shorter than
// its header states, and an entry whose checksum is wrong.
const (
	entryCap       = 67108864
	tooLargeSize   = 134217728
	tooLargeReason = "the entry is larger than 64 MiB"
	shortReason    = "unexpected EOF"
	badEntryReason = "zip: checksum error"
)

// A JAR provides one class per path, the versioned one where the JAR is
// multi-release, so which entry a release reads, and what a broken JAR
// reports, is pinned.
func TestJar(t *testing.T) {
	t.Parallel()

	t.Run("jarClasses", func(t *testing.T) {
		t.Parallel()

		t.Run("lowers every class a JAR has", func(t *testing.T) {
			t.Parallel()

			gb, found := jarUnit(t, mustRead(t, libJAR), plugin.DepthSignatures)
			assert.Empty(t, found, "every class decodes")
			named[*node.Struct](t, packageDecls(t, gb, libPackage), boxClass)
		})

		t.Run("gives a JAR's classes of one package one File node", func(t *testing.T) {
			t.Parallel()

			gb, _ := jarUnit(t, mustRead(t, libJAR), plugin.DepthSignatures)
			assert.Length(t, packageIn(t, gb, libPackage).Files, 1, "the JAR's")
		})

		t.Run("reports BadClassFile for a JAR that does not open", func(t *testing.T) {
			t.Parallel()

			_, found := jarUnit(t, []byte("junk"), plugin.DepthSignatures)
			assert.Equal(t, codesOf(found), []diag.Code{frontend.BadClassFile}, "no ZIP file")
		})

		t.Run("reports BadClassFile for an entry that does not decode", func(t *testing.T) {
			t.Parallel()

			_, found := jarUnit(t, jarOf(t, "", map[string][]byte{circlePath: []byte("junk")}), plugin.DepthSignatures)
			assert.Equal(t, codesOf(found), []diag.Code{frontend.BadClassFile}, "no class file")
		})

		t.Run("names the entry that does not decode", func(t *testing.T) {
			t.Parallel()

			_, found := jarUnit(t, jarOf(t, "", map[string][]byte{circlePath: []byte("junk")}), plugin.DepthSignatures)
			assert.NotEmpty(t, found, "the entry reports")
			assert.HasPrefix(t, found[0].Msg, circlePath, "the entry opens the message")
		})

		t.Run("reports BadClassFile for an entry whose bytes do not read", func(t *testing.T) {
			t.Parallel()

			data := fixtureClass(t, circleName)
			h := zip.FileHeader{
				Name: circlePath, CRC32: crc32.ChecksumIEEE(data) + 1, UncompressedSize64: uint64(len(data)),
			}
			_, found := jarUnit(t, rawJAR(t, h, data, nil), plugin.DepthSignatures)
			assert.NotEmpty(t, found, "the entry reports")
			assert.HasSuffix(t, found[0].Msg, badEntryReason, "its checksum is wrong")
		})

		t.Run("reports BadClassFile for an entry larger than 64 MiB", func(t *testing.T) {
			t.Parallel()

			_, found := jarUnit(t, sizedJAR(t, tooLargeSize), plugin.DepthSignatures)
			assert.Equal(t, codesOf(found), []diag.Code{frontend.BadClassFile}, "the size its header states")
		})

		t.Run("names the cap an entry larger than 64 MiB is over", func(t *testing.T) {
			t.Parallel()

			_, found := jarUnit(t, sizedJAR(t, tooLargeSize), plugin.DepthSignatures)
			assert.NotEmpty(t, found, "the entry reports")
			assert.HasSuffix(t, found[0].Msg, tooLargeReason, "before any byte reads")
		})

		t.Run("reads an entry whose header states 64 MiB", func(t *testing.T) {
			t.Parallel()

			_, found := jarUnit(t, sizedJAR(t, entryCap), plugin.DepthSignatures)
			assert.NotEmpty(t, found, "the entry reports")
			assert.HasSuffix(t, found[0].Msg, shortReason, "its bytes end before 64 MiB")
		})

		t.Run("reports BadClassFile for an entry of an unknown compression method", func(t *testing.T) {
			t.Parallel()

			data := fixtureClass(t, circleName)
			h := zip.FileHeader{
				Name: circlePath, Method: unknownMethod, CRC32: crc32.ChecksumIEEE(data),
				UncompressedSize64: uint64(len(data)),
			}
			_, found := jarUnit(t, rawJAR(t, h, data, nil), plugin.DepthSignatures)
			assert.Equal(t, codesOf(found), []diag.Code{frontend.BadClassFile}, "no decompressor opens it")
		})

		t.Run("reports BadClassFile for a manifest whose bytes do not read", func(t *testing.T) {
			t.Parallel()

			_, found := jarUnit(t, brokenManifestJAR(t), plugin.DepthSignatures)
			assert.Equal(t, codesOf(found), []diag.Code{frontend.BadClassFile}, "its checksum is wrong")
		})

		t.Run("names the manifest that does not read", func(t *testing.T) {
			t.Parallel()

			_, found := jarUnit(t, brokenManifestJAR(t), plugin.DepthSignatures)
			assert.NotEmpty(t, found, "the manifest reports")
			assert.HasPrefix(t, found[0].Msg, manifestName, "the manifest opens the message")
		})

		t.Run("loads the root classes of a JAR whose manifest does not read", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, declNames(jarDecls(t, brokenManifestJAR(t), 0)), []string{circleName},
				"the root class, and no versioned one")
		})
	})

	t.Run("classEntry", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			manifest string
			entries  map[string]string
			release  int
			want     []string
		}{
			{
				name:     "takes a versioned class of a multi-release JAR for a release at least its version",
				manifest: multiRelease, entries: map[string]string{circlePath: circleName, versioned21: squareName},
				release: release25, want: []string{squareName},
			},
			{
				name:     "takes a versioned class of a multi-release JAR for a release equal to its version",
				manifest: multiRelease, entries: map[string]string{circlePath: circleName, versioned21: squareName},
				release: release21, want: []string{squareName},
			},
			{
				name:     "takes the root class for a release older than the versioned one",
				manifest: multiRelease, entries: map[string]string{circlePath: circleName, versioned21: squareName},
				release: release17, want: []string{circleName},
			},
			{
				name:     "takes the highest version at most the release",
				manifest: multiRelease,
				entries:  map[string]string{circlePath: circleName, versioned11: squareName, versioned21: shapeName},
				release:  release17, want: []string{squareName},
			},
			{
				name:     "takes the highest version for release 0",
				manifest: multiRelease,
				entries:  map[string]string{circlePath: circleName, versioned11: squareName, versioned21: shapeName},
				want:     []string{shapeName},
			},
			{
				name:     "ignores the versioned classes of a JAR that is not multi-release",
				manifest: singleRelease, entries: map[string]string{circlePath: circleName, versioned21: squareName},
				want: []string{circleName},
			},
			{
				name:    "ignores a class under META-INF that is not versioned",
				entries: map[string]string{"META-INF/" + circlePath: circleName},
			},
			{
				name:     "ignores a versioned class whose version is no number",
				manifest: multiRelease, entries: map[string]string{"META-INF/versions/x/" + circlePath: circleName},
			},
			{
				name:     "ignores a versioned class without a class path",
				manifest: multiRelease, entries: map[string]string{"META-INF/versions/21.class": circleName},
			},
			{
				name:    "ignores an entry that is no class file",
				entries: map[string]string{"com/acme/lib/Circle.txt": circleName},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				entries := map[string][]byte{}
				for name, class := range tt.entries {
					entries[name] = fixtureClass(t, class)
				}
				data := jarOf(t, tt.manifest, entries)
				assert.Equal(t, declNames(jarDecls(t, data, tt.release)), tt.want, "the classes the release reads")
			})
		}
	})

	t.Run("multiRelease", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			manifest string
			want     []string
		}{
			{
				name: "reads the attribute's name and value without case", manifest: "multi-release: TRUE\n",
				want: []string{squareName},
			},
			{
				name: "reads an attribute continued on the next line", manifest: "Multi-Release: tr\n ue\n",
				want: []string{squareName},
			},
			{
				name: "reads a continued line that opens the manifest as a line", manifest: " Multi-Release: true\n",
				want: []string{squareName},
			},
			{
				name: "reads a manifest whose lines end in CRLF", manifest: "Multi-Release: true\r\n",
				want: []string{squareName},
			},
			{
				name:     "reads the main section alone",
				manifest: "Manifest-Version: 1.0\n\nName: x\nMulti-Release: true\n",
				want:     []string{circleName},
			},
			{
				name:     "reads the main section of a manifest whose lines end in CRLF alone",
				manifest: "Manifest-Version: 1.0\r\n\r\nName: x\r\nMulti-Release: true\r\n",
				want:     []string{circleName},
			},
			{
				name: "reads another value as not multi-release", manifest: "Multi-Release: false\n",
				want: []string{circleName},
			},
			{
				name: "reads a line without a colon as no attribute", manifest: "Multi-Release true\n",
				want: []string{circleName},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				data := jarOf(t, tt.manifest, map[string][]byte{
					circlePath: fixtureClass(t, circleName), versioned21: fixtureClass(t, squareName),
				})
				assert.Equal(t, declNames(jarDecls(t, data, 0)), tt.want, "the class the JAR provides")
			})
		}

		t.Run("reads a JAR without a manifest as not multi-release", func(t *testing.T) {
			t.Parallel()

			data := jarOf(t, "", map[string][]byte{
				circlePath: fixtureClass(t, circleName), versioned21: fixtureClass(t, squareName),
			})
			assert.Equal(t, declNames(jarDecls(t, data, 0)), []string{circleName}, "the root class")
		})
	})
}

// fixtureClass returns the class file of the fixture library's class of
// a simple name.
func fixtureClass(tb assert.TB, name string) []byte {
	tb.Helper()

	return mustRead(tb, path.Join(classesDir, libDir, name+classSuffix))
}

// jarOf returns a JAR of entries, each name to its bytes, written in
// name order after a manifest of a main section, and without a manifest
// where it is empty.
func jarOf(tb testing.TB, manifest string, entries map[string][]byte) []byte {
	tb.Helper()

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	if manifest != "" {
		entries = maps.Clone(entries)
		entries[manifestName] = []byte(manifest)
	}
	for _, name := range slices.Sorted(maps.Keys(entries)) {
		f, err := w.Create(name)
		assert.NoError(tb, err, "the entry is written")
		_, err = f.Write(entries[name])
		assert.NoError(tb, err, "its bytes are written")
	}
	assert.NoError(tb, w.Close(), "the JAR is closed")
	return buf.Bytes()
}

// rawJAR returns a JAR whose first entry is bytes stored under a header,
// whatever compression method, size and checksum the header states, and
// whose other entries are each name to its bytes, written in name order.
func rawJAR(tb testing.TB, h zip.FileHeader, data []byte, entries map[string][]byte) []byte {
	tb.Helper()

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	h.CompressedSize64 = uint64(len(data))
	f, err := w.CreateRaw(&h)
	assert.NoError(tb, err, "the raw entry is written")
	_, err = f.Write(data)
	assert.NoError(tb, err, "its bytes are written")
	for _, name := range slices.Sorted(maps.Keys(entries)) {
		f, err = w.Create(name)
		assert.NoError(tb, err, "the entry is written")
		_, err = f.Write(entries[name])
		assert.NoError(tb, err, "its bytes are written")
	}
	assert.NoError(tb, w.Close(), "the JAR is closed")
	return buf.Bytes()
}

// sizedJAR returns a JAR of Circle's class file under a header that
// states a size, whatever the size of its bytes.
func sizedJAR(tb testing.TB, size uint64) []byte {
	tb.Helper()

	data := fixtureClass(tb, circleName)
	return rawJAR(tb, zip.FileHeader{Name: circlePath, CRC32: crc32.ChecksumIEEE(data), UncompressedSize64: size}, data,
		nil)
}

// brokenManifestJAR returns a JAR whose manifest marks it multi-release
// under a wrong checksum, with Circle at its root and Square as its
// version 21.
func brokenManifestJAR(tb testing.TB) []byte {
	tb.Helper()

	manifest := []byte(multiRelease)
	h := zip.FileHeader{
		Name: manifestName, CRC32: crc32.ChecksumIEEE(manifest) + 1, UncompressedSize64: uint64(len(manifest)),
	}
	return rawJAR(tb, h, manifest, map[string][]byte{
		circlePath: fixtureClass(tb, circleName), versioned21: fixtureClass(tb, squareName),
	})
}

// jarDecls parses a unit of one JAR through a frontend of a release, at
// signature depth, and returns the declarations of the fixture package,
// none where the unit declares no such package.
func jarDecls(tb testing.TB, data []byte, release int) node.Symbols {
	tb.Helper()

	f := frontend.New(&frontend.Options{Release: release})
	tree := fstest.MapFS{jarMember: {Data: data}}
	u := plugin.NewSourceUnit([]plugin.SourceRef{{Path: jarMember}}, tree, plugin.DepthSignatures, f.Syntax(), brand,
		diag.NewSink(), f.Name())
	assert.NoError(tb, f.Parse(tb.Context(), u), "the unit parses")
	var out node.Symbols
	for _, p := range u.Graph().Packages() {
		if p.ID.Package == libPackage {
			for _, file := range p.Files {
				out = append(out, file.Decls...)
			}
		}
	}
	return out
}

// declNames returns the names of declarations, in order, and nil for
// none.
func declNames(decls node.Symbols) []string {
	var out []string
	for _, d := range decls {
		out = append(out, nameOf(d))
	}
	return out
}
