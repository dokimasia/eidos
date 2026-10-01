// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
)

// The fixture module.
const acmeModule = "example.com/acme"

// A directory's import path is the one the go command gives it, so the
// frontend that loads a package and the backend that writes into one
// agree on the path.
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
}
