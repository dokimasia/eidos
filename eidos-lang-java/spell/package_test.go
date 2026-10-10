// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spell_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/java"
	"go.dokimi.dev/eidos/lang/java/spell"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// A Java file declares its package in its clause. A routed file keeps
// the package its declarations derive from wherever it is written.
func TestPackage(t *testing.T) {
	t.Parallel()

	t.Run("Package", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the origin's package for a file in another directory", func(t *testing.T) {
			t.Parallel()

			got, err := spell.Package(stubPlacement())
			assert.NoError(t, err, "the origin names the package")
			assert.Equal(t, got, storePackage(), "the declarations' package")
		})

		t.Run("returns an error for a plan file", func(t *testing.T) {
			t.Parallel()

			_, err := spell.Package(plugin.Placement{Path: "gen/Registry.java"})
			assert.HasError(t, err, "a plan file derives from no package")
		})
	})
}

// Package returns the origin's package without allocating in the
// ordinary run, which runs no benchmark.
func TestPackageAllocs(t *testing.T) {
	placement := stubPlacement()
	var (
		got symbol.Identity
		err error
	)
	assert.MaxAllocs(t, func() { got, err = spell.Package(placement) }, 0, "Package allocates nothing")
	assert.NoError(t, err, "Package derives the package")
	assert.Equal(t, got, storePackage(), "Package returns the origin's package")
}

// BenchmarkPackage measures the package a backend names once per routed
// file.
func BenchmarkPackage(b *testing.B) {
	b.Run("Package", func(b *testing.B) {
		placement := stubPlacement()
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var (
			got symbol.Identity
			err error
		)
		for c.Loop() {
			got, err = spell.Package(placement)
		}
		assert.NoError(b, err, "Package derives the package")
		assert.Equal(b, got, storePackage(), "Package returns the origin's package")
	})
}

// storePackage returns the package com.acme.store.
func storePackage() symbol.Identity {
	return symbol.Identity{Lang: java.Lang, Package: "com.acme.store", Kind: symbol.KindPackage}
}

// stubPlacement returns a stub file of com.acme.store routed under gen.
func stubPlacement() plugin.Placement {
	return plugin.Placement{Path: "gen/com/acme/store/RowStub.java", Origin: storePackage()}
}
