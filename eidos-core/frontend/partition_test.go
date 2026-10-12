// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/frontend"
)

// The manifest that the probe cases look for, the manifest of the module
// at svc, and the directories that the cases probe from.
const (
	manifestName = "mod.cfg"
	svcManifest  = "svc/" + manifestName
	storeDir     = "svc/store/rows"
	apiDir       = "svc/store/api"
)

// probeTree is a workspace with one manifest, at svc.
var probeTree = fstest.MapFS{svcManifest: {Data: []byte("module svc\n")}}

// countingReader reads a tree and records the path of every read.
type countingReader struct {
	tree  fs.FS
	reads []string
}

// Read returns the bytes of a file of the tree, and records the read.
func (r *countingReader) Read(path string) ([]byte, error) {
	r.reads = append(r.reads, path)
	return fs.ReadFile(r.tree, path)
}

// TestPartition checks that Nearest finds the nearest manifest above a
// directory, and that the probes of a partition read each directory once.
func TestPartition(t *testing.T) {
	t.Parallel()

	t.Run("Nearest", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			dir  string
			want string
		}{
			{name: "returns the manifest of the directory itself", dir: "svc", want: svcManifest},
			{name: "returns the manifest of the nearest directory above", dir: storeDir, want: svcManifest},
			{name: "returns the empty string where no directory has a manifest", dir: "top/lonely"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				r := &countingReader{tree: probeTree}
				assert.Equal(t, frontend.Nearest(r, tt.dir, manifestName, map[string]string{}), tt.want,
					"the probe finds the nearest manifest")
			})
		}

		t.Run("stops at a directory that an earlier probe visited", func(t *testing.T) {
			t.Parallel()

			r := &countingReader{tree: probeTree}
			seen := map[string]string{}
			frontend.Nearest(r, storeDir, manifestName, seen)
			r.reads = nil
			assert.Equal(t, frontend.Nearest(r, apiDir, manifestName, seen), svcManifest,
				"the second probe finds the same manifest")
			assert.Equal(t, r.reads, []string{apiDir + "/" + manifestName},
				"the second probe reads only the directory that the first probe did not visit")
		})

		t.Run("records the result for every directory that the probe visited", func(t *testing.T) {
			t.Parallel()

			seen := map[string]string{}
			frontend.Nearest(&countingReader{tree: probeTree}, storeDir, manifestName, seen)
			assert.Equal(t, seen, map[string]string{
				storeDir: svcManifest, "svc/store": svcManifest, "svc": svcManifest,
			}, "each visited directory has the manifest that the probe found")
		})

		t.Run("records the empty string for the directories of a probe that finds nothing", func(t *testing.T) {
			t.Parallel()

			seen := map[string]string{}
			frontend.Nearest(&countingReader{tree: probeTree}, "top/lonely", manifestName, seen)
			assert.Equal(t, seen, map[string]string{"top/lonely": "", "top": "", ".": ""},
				"each visited directory up to the root has no manifest")
		})
	})
}

// TestPartitionAllocs checks that a probe from a directory that an earlier
// probe visited allocates nothing. The ordinary run runs no benchmark, so
// this test checks the ceiling. The check does not run in parallel, because
// the count includes the allocations of every goroutine.
func TestPartitionAllocs(t *testing.T) {
	r := &countingReader{tree: probeTree}
	seen := map[string]string{}
	frontend.Nearest(r, storeDir, manifestName, seen)
	var got string
	assert.MaxAllocs(t, func() { got = frontend.Nearest(r, storeDir, manifestName, seen) }, 0,
		"Nearest allocates nothing for a directory that seen has")
	assert.Equal(t, got, svcManifest, "Nearest returns the cached manifest")
}

// BenchmarkPartition measures a probe from a directory that an earlier
// probe visited, which a partition makes once for each file of a package.
func BenchmarkPartition(b *testing.B) {
	b.Run("Nearest", func(b *testing.B) {
		b.Run("a directory that seen has", func(b *testing.B) {
			r := &countingReader{tree: probeTree}
			seen := map[string]string{}
			frontend.Nearest(r, storeDir, manifestName, seen)
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var got string
			for c.Loop() {
				got = frontend.Nearest(r, storeDir, manifestName, seen)
			}
			assert.Equal(b, got, svcManifest, "Nearest returns the cached manifest")
		})
	})
}
