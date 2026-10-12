// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	golang "go.dokimi.dev/eidos/lang/go"
)

// The fixture module.
const acmeModule = "example.com/acme"

// joinedAllocs is the import path of a directory below its module's
// root: the joined path.
const joinedAllocs = 1

// The frontend that loads a package and the backend that writes into
// one agree on the path the go command gives a directory. Each way a
// directory relates to its module is pinned.
func TestImportPath(t *testing.T) {
	t.Parallel()

	t.Run("ImportPath", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name   string
			module string
			root   string
			dir    string
			want   string
		}{
			{
				name:   "returns the module path for the module's root directory",
				module: acmeModule, root: "svc", dir: "svc", want: acmeModule,
			},
			{
				name:   "returns the module path joined with a directory below the root",
				module: acmeModule, root: "svc", dir: "svc/store", want: acmeModule + "/store",
			},
			{
				name:   "returns the module path joined with a directory of a module at the tree's root",
				module: acmeModule, root: ".", dir: "svc/store", want: acmeModule + "/svc/store",
			},
			{
				name:   "returns the module path for the tree's root of a module at the tree's root",
				module: acmeModule, root: ".", dir: ".", want: acmeModule,
			},
			{
				name:   "returns the directory for an empty module path",
				module: "", root: ".", dir: "svc/store", want: "svc/store",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, golang.ImportPath(tt.module, tt.root, tt.dir), tt.want, "the import path")
			})
		}
	})

	t.Run("Standard", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want bool
		}{
			{name: "reports true for a path of one element", give: "fmt", want: true},
			{name: "reports true for a nested path of the standard library", give: "net/http", want: true},
			{name: "reports false for a module path", give: acmeModule + "/store", want: false},
			{name: "reports false for a module root of one element with a dot", give: "gopkg.in", want: false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, golang.Standard(tt.give), tt.want, "the go command's rule for "+tt.give)
			})
		}
	})
}

// ImportPath returns a module's root without allocating, and allocates
// the joined path of a directory below it. Standard allocates nothing.
// The ordinary run, which runs no benchmark, checks those ceilings here.
func TestImportPathAllocs(t *testing.T) {
	var got string
	assert.MaxAllocs(t, func() { got = golang.ImportPath(acmeModule, "svc", "svc") }, 0,
		"ImportPath allocates nothing for the module's root")
	assert.Equal(t, got, acmeModule, "ImportPath returns the module path")
	assert.MaxAllocs(t, func() { got = golang.ImportPath(acmeModule, "svc", "svc/store") }, joinedAllocs,
		"ImportPath allocates the joined path")
	assert.Equal(t, got, acmeModule+"/store", "ImportPath joins the path")
	var standard bool
	assert.MaxAllocs(t, func() { standard = golang.Standard(acmeModule) }, 0, "Standard allocates nothing")
	assert.False(t, standard, "Standard reports false for a module path")
}

// BenchmarkImportPath measures the import path the frontend and the
// backends derive once per directory, and the test of the standard
// library that they make once per import.
func BenchmarkImportPath(b *testing.B) {
	paths := []struct {
		name   string
		dir    string
		allocs uint64
		want   string
	}{
		{
			name:   "a directory below the module's root",
			dir:    "svc/store",
			allocs: joinedAllocs,
			want:   acmeModule + "/store",
		},
		{name: "the module's root", dir: "svc", want: acmeModule},
	}
	b.Run("ImportPath", func(b *testing.B) {
		for _, tt := range paths {
			b.Run(tt.name, func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var got string
				for c.Loop() {
					got = golang.ImportPath(acmeModule, "svc", tt.dir)
				}
				assert.Equal(b, got, tt.want, "ImportPath returns the import path")
			})
		}
	})
	b.Run("Standard", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got bool
		for c.Loop() {
			got = golang.Standard(acmeModule)
		}
		assert.False(b, got, "Standard reports false for a module path")
	})
}
