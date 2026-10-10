// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/lang/typescript/spell"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// stubPath is the routed file every case names the module of.
const stubPath = "svc/store.stub.ts"

// A TypeScript file is a module of its own. A routed file's package is
// its own path, whatever its directory contains.
func TestPackage(t *testing.T) {
	t.Parallel()

	t.Run("Package", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give plugin.Placement
		}{
			{name: "returns the file's module path", give: plugin.Placement{Path: stubPath}},
			{
				name: "returns the file's module path beside a loaded module",
				give: plugin.Placement{
					Path: stubPath,
					Residents: []plugin.Resident{{
						File: "svc/store.ts",
						Pkg:  symbol.Identity{Lang: typescript.Lang, Package: "svc/store", Kind: symbol.KindPackage},
					}},
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spell.Package(tt.give)
				assert.NoError(t, err, "every path derives a package")
				assert.Equal(t, got, stubModule(), "the module of the file")
			})
		}
	})
}

// Package returns the module of a path without allocating in the
// ordinary run, which runs no benchmark.
func TestPackageAllocs(t *testing.T) {
	placement := plugin.Placement{Path: stubPath}
	var (
		got symbol.Identity
		err error
	)
	assert.MaxAllocs(t, func() { got, err = spell.Package(placement) }, 0, "Package allocates nothing")
	assert.NoError(t, err, "Package derives the module")
	assert.Equal(t, got, stubModule(), "Package returns the file's module")
}

// BenchmarkPackage measures the module a backend names once per routed
// file.
func BenchmarkPackage(b *testing.B) {
	b.Run("Package", func(b *testing.B) {
		placement := plugin.Placement{Path: stubPath}
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var (
			got symbol.Identity
			err error
		)
		for c.Loop() {
			got, err = spell.Package(placement)
		}
		assert.NoError(b, err, "Package derives the module")
		assert.Equal(b, got, stubModule(), "Package returns the file's module")
	})
}

// stubModule returns the module of the routed stub file.
func stubModule() symbol.Identity {
	return symbol.Identity{
		Lang: typescript.Lang, Package: "svc/store.stub", Name: "store.stub", Kind: symbol.KindPackage,
	}
}
