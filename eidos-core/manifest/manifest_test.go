// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package manifest_test

import (
	"fmt"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/plugin"
)

// The fixture's record: a workspace name, a plan, two plugins and a
// digest spelled the way every digest of the kernel is.
const (
	workspaceName = "platform"
	planName      = "go-services"
	stubgen       = plugin.ID("stubgen")
	audit         = plugin.ID("acme-audit")
	storeSource   = "golang:svc/store.Store"
	stubPath      = "svc/store_stub.go"
)

// benchFiles is the canonical scale's file count: 1,000 packages of 10
// generated files.
const benchFiles = 10_000

// hash returns a well-formed digest of one repeated byte pair.
func hash(pair string) string { return "sha256:" + strings.Repeat(pair, 32) }

// entry returns a well-formed entry for one path.
func entry(path string) manifest.Entry {
	return manifest.Entry{
		Path: path, Plan: planName, Hash: hash("ab"),
		Plugins: []plugin.ID{audit, stubgen}, Sources: []string{storeSource},
	}
}

// record returns a well-formed manifest of the given entries.
func record(files ...manifest.Entry) manifest.Manifest {
	return manifest.Manifest{Version: manifest.Version, Workspace: workspaceName, Files: files}
}

// scaled returns a manifest of benchFiles entries, sorted by path.
func scaled() manifest.Manifest {
	m := record()
	for i := range benchFiles {
		m.Files = append(m.Files, entry(fmt.Sprintf("p%04d/f%d_stub.go", i/10, i%10)))
	}
	return m
}

// The record is versioned public API: its bytes are canonical, it
// reads back what it wrote, and it refuses a record that breaks its
// invariants.
func TestManifest(t *testing.T) {
	t.Parallel()

	t.Run("Encode", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the record as JSON indented by two spaces with a final newline", func(t *testing.T) {
			t.Parallel()

			got, err := manifest.Encode(record(entry(stubPath)))
			assert.NoError(t, err, "the record encodes")
			assert.Equal(t, string(got), `{
  "version": 1,
  "workspace": "platform",
  "files": [
    {
      "path": "svc/store_stub.go",
      "plan": "go-services",
      "hash": "`+hash("ab")+`",
      "plugins": [
        "acme-audit",
        "stubgen"
      ],
      "sources": [
        "golang:svc/store.Store"
      ]
    }
  ]
}
`, "the canonical bytes")
		})

		t.Run("returns an empty list for every nil list", func(t *testing.T) {
			t.Parallel()

			bare := entry(stubPath)
			bare.Plugins, bare.Sources = nil, nil
			got, err := manifest.Encode(record(bare))
			assert.NoError(t, err, "the record encodes")
			assert.Contains(t, string(got), `"plugins": [],`, "the plugins are an empty list")
			assert.Contains(t, string(got), `"sources": []`, "the sources are an empty list")

			empty, err := manifest.Encode(record())
			assert.NoError(t, err, "the empty record encodes")
			assert.Contains(t, string(empty), `"files": []`, "the files are an empty list")
		})

		t.Run("returns equal bytes for a nil list and an empty one", func(t *testing.T) {
			t.Parallel()

			nilFiles, err := manifest.Encode(record())
			assert.NoError(t, err, "the nil list encodes")
			emptyFiles, err := manifest.Encode(record([]manifest.Entry{}...))
			assert.NoError(t, err, "the empty list encodes")
			assert.Equal(t, nilFiles, emptyFiles, "one record, one encoding")
		})

		t.Run("returns a path without escaping its HTML characters", func(t *testing.T) {
			t.Parallel()

			got, err := manifest.Encode(record(entry("svc/a&b.go")))
			assert.NoError(t, err, "the record encodes")
			assert.Contains(t, string(got), `"path": "svc/a&b.go"`, "the path is as written")
		})

		t.Run("leaves the caller's nil lists nil", func(t *testing.T) {
			t.Parallel()

			bare := entry(stubPath)
			bare.Plugins = nil
			m := record(bare)
			_, err := manifest.Encode(m)
			assert.NoError(t, err, "the record encodes")
			assert.Nil(t, m.Files[0].Plugins, "the caller's entry is unchanged")
		})

		tests := []struct {
			name string
			give manifest.Manifest
		}{
			{
				name: "returns an error for a manifest of another version",
				give: manifest.Manifest{Version: manifest.Version + 1, Workspace: workspaceName},
			},
			{
				name: "returns an error for a manifest without a version",
				give: manifest.Manifest{Workspace: workspaceName},
			},
			{
				name: "returns an error for files out of path order",
				give: record(entry("svc/b.go"), entry("svc/a.go")),
			},
			{
				name: "returns an error for a path listed twice",
				give: record(entry(stubPath), entry(stubPath)),
			},
			{name: "returns an error for an empty path", give: record(entry(""))},
			{name: "returns an error for an absolute path", give: record(entry("/svc/a.go"))},
			{name: "returns an error for a path that climbs out of the root", give: record(entry("../a.go"))},
			{name: "returns an error for the root's own path", give: record(entry("."))},
			{name: "returns an error for a path with a backslash", give: record(entry(`svc\a.go`))},
			{
				name: "returns an error for an entry without a plan",
				give: record(func() manifest.Entry { e := entry(stubPath); e.Plan = ""; return e }()),
			},
			{
				name: "returns an error for a hash without its digest's name",
				give: record(
					func() manifest.Entry { e := entry(stubPath); e.Hash = strings.Repeat("ab", 32); return e }(),
				),
			},
			{
				name: "returns an error for a hash of the wrong width",
				give: record(func() manifest.Entry { e := entry(stubPath); e.Hash = "sha256:abab"; return e }()),
			},
			{
				name: "returns an error for a hash in uppercase digits",
				give: record(func() manifest.Entry { e := entry(stubPath); e.Hash = hash("AB"); return e }()),
			},
			{
				name: "returns an error for a hash with a digit outside hex",
				give: record(func() manifest.Entry { e := entry(stubPath); e.Hash = hash("zz"); return e }()),
			},
			{
				name: "returns an error for plugins out of order",
				give: record(func() manifest.Entry {
					e := entry(stubPath)
					e.Plugins = []plugin.ID{stubgen, audit}
					return e
				}()),
			},
			{
				name: "returns an error for a plugin listed twice",
				give: record(func() manifest.Entry {
					e := entry(stubPath)
					e.Plugins = []plugin.ID{stubgen, stubgen}
					return e
				}()),
			},
			{
				name: "returns an error for sources out of order",
				give: record(func() manifest.Entry {
					e := entry(stubPath)
					e.Sources = []string{"golang:svc/store.Z", "golang:svc/store.A"}
					return e
				}()),
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := manifest.Encode(tt.give)
				assert.HasError(t, err, "the record is refused")
				assert.Contains(t, err.Error(), "manifest: encode:", "the error names the package and the step")
			})
		}
	})

	t.Run("Decode", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the manifest Encode wrote", func(t *testing.T) {
			t.Parallel()

			want := record(entry("svc/a.go"), entry(stubPath))
			encoded, err := manifest.Encode(want)
			assert.NoError(t, err, "the record encodes")
			got, err := manifest.Decode(encoded)
			assert.NoError(t, err, "the bytes decode")
			assert.True(t, got.Equal(want), "the record reads back")
		})

		t.Run("returns the manifest without a key the format does not know", func(t *testing.T) {
			t.Parallel()

			got, err := manifest.Decode([]byte(`{"version":1,"workspace":"platform","files":[],"cache":{}}`))
			assert.NoError(t, err, "an unknown key is skipped")
			assert.True(t, got.Equal(record()), "the record is the known keys")
		})

		tests := []struct {
			name string
			give string
		}{
			{name: "returns ErrUnsupported for a manifest of another version", give: `{"version":2,"files":[]}`},
			{name: "returns ErrUnsupported for bytes that are not JSON", give: `version: 1`},
			{name: "returns ErrUnsupported for a JSON list", give: `[]`},
			{
				name: "returns ErrUnsupported for files out of path order",
				give: `{"version":1,"files":[` +
					`{"path":"b.go","plan":"p","hash":"` + hash("ab") + `"},` +
					`{"path":"a.go","plan":"p","hash":"` + hash("ab") + `"}]}`,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := manifest.Decode([]byte(tt.give))
				assert.ErrorIs(t, err, manifest.ErrUnsupported, "the record is not read")
			})
		}
	})

	t.Run("Equal", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for a nil list and an empty one", func(t *testing.T) {
			t.Parallel()

			bare, empty := entry(stubPath), entry(stubPath)
			bare.Plugins, bare.Sources = nil, nil
			empty.Plugins, empty.Sources = []plugin.ID{}, []string{}
			assert.True(t, record(bare).Equal(record(empty)), "nil and empty record one list")
			assert.True(t, record().Equal(record([]manifest.Entry{}...)), "and one file list")
		})

		tests := []struct {
			name   string
			change func(*manifest.Manifest)
		}{
			{name: "reports false for another version", change: func(m *manifest.Manifest) { m.Version++ }},
			{name: "reports false for another workspace", change: func(m *manifest.Manifest) { m.Workspace = "other" }},
			{
				name:   "reports false for another file count",
				change: func(m *manifest.Manifest) { m.Files = m.Files[:1] },
			},
			{name: "reports false for another path", change: func(m *manifest.Manifest) { m.Files[0].Path = "x.go" }},
			{name: "reports false for another plan", change: func(m *manifest.Manifest) { m.Files[0].Plan = "other" }},
			{
				name:   "reports false for another hash",
				change: func(m *manifest.Manifest) { m.Files[0].Hash = hash("cd") },
			},
			{
				name:   "reports false for other plugins",
				change: func(m *manifest.Manifest) { m.Files[0].Plugins = []plugin.ID{stubgen} },
			},
			{
				name:   "reports false for other sources",
				change: func(m *manifest.Manifest) { m.Files[0].Sources = nil },
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				changed := record(entry("svc/a.go"), entry(stubPath))
				tt.change(&changed)
				assert.False(t, record(entry("svc/a.go"), entry(stubPath)).Equal(changed), "the records differ")
			})
		}
	})
}

// The comparison a ledger runs on every commit allocates nothing. The
// check runs alone, because AllocsPerRun counts every goroutine's
// allocations and refuses to run beside parallel tests.
func TestManifestAllocs(t *testing.T) {
	m, other := scaled(), scaled()
	allocs := testing.AllocsPerRun(10, func() {
		if !m.Equal(other) {
			t.Fatal("Equal reports two equal records apart")
		}
	})
	assert.Equal(t, allocs, 0.0, "Equal allocates nothing")
}

// BenchmarkManifest measures the record at the canonical scale of
// 10,000 generated files: one encoding and one decoding, each under
// its allocation ceiling, and one comparison of equal records. Encode
// allocates once per growth step of its buffers, so its count grows
// with the logarithm of the record's size: 19 to 93 at this scale, the
// higher count where a collection emptied the JSON encoder's pooled
// state, under a ceiling of 128. Decode measures 40,166, four per
// entry, under a ceiling of 44,000. Equal reports its allocations, and
// [TestManifestAllocs] pins them at zero.
func BenchmarkManifest(b *testing.B) {
	m := scaled()
	encoded, err := manifest.Encode(m)
	if err != nil {
		b.Fatalf("Encode: unexpected error: %v", err)
	}

	b.Run("Encode", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(128)
		defer c.End()
		var got []byte
		for c.Loop() {
			got, err = manifest.Encode(m)
		}
		if err != nil || len(got) != len(encoded) {
			b.Fatalf("Encode returned %d bytes and %v", len(got), err)
		}
	})

	b.Run("Decode", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(44_000)
		defer c.End()
		var got manifest.Manifest
		for c.Loop() {
			got, err = manifest.Decode(encoded)
		}
		if err != nil || len(got.Files) != benchFiles {
			b.Fatalf("Decode returned %d files and %v", len(got.Files), err)
		}
	})

	b.Run("Equal", func(b *testing.B) {
		other := scaled()
		b.ReportAllocs()
		equal := false
		for b.Loop() {
			equal = m.Equal(other)
		}
		if !equal {
			b.Fatal("Equal reports two equal records apart")
		}
	})
}
