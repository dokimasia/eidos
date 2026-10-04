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

// Two records are equal when they record one version, one workspace and
// the same files, a nil list and an empty one alike.
func TestManifest(t *testing.T) {
	t.Parallel()

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

// The comparison a commit runs on every record allocates nothing. The
// check runs alone, because AllocsPerRun counts every goroutine's
// allocations and refuses to run beside parallel tests.
func TestManifestZeroAlloc(t *testing.T) {
	m, other := scaled(), scaled()
	assert.MaxAllocs(t, func() {
		if !m.Equal(other) {
			t.Fatal("Equal reports two equal records apart")
		}
	}, 0, "Equal allocates nothing")
}

// BenchmarkManifest measures one comparison of two equal records at the
// canonical scale of 10,000 generated files, which allocates nothing.
func BenchmarkManifest(b *testing.B) {
	m, other := scaled(), scaled()

	b.Run("Equal", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		equal := false
		for c.Loop() {
			equal = m.Equal(other)
		}
		if !equal {
			b.Fatal("Equal reports two equal records apart")
		}
	})
}

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
