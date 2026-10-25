// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	golang "go.dokimi.dev/eidos/lang/go"
)

// These constants are the packages and the names of the references that
// the cases check.
const (
	corePath     = "example.com/shop/core"
	coreTests    = corePath + golang.TestSuffix
	coreAPI      = corePath + "/api"
	coreInternal = corePath + "/internal/store"
	webPath      = "example.com/shop/web"
	rowName      = "Row"
	rowsName     = "rows"
)

// The backend, the layout and the rules of Go decide one way whether Go
// code of one package can refer to a declaration, so each rule of the go
// command and the compiler is pinned, with the packages that pass it.
func TestReference(t *testing.T) {
	t.Parallel()

	t.Run("CheckReference", func(t *testing.T) {
		t.Parallel()

		admitted := []struct {
			name     string
			giveFrom string
			givePkg  string
			giveName string
		}{
			{name: "returns nil for a declaration of no package", giveFrom: webPath, giveName: "int"},
			{
				name:     "returns nil for an unexported declaration of the package of the code",
				giveFrom: corePath, givePkg: corePath, giveName: rowsName,
			},
			{
				name:     "returns nil for an exported declaration of another package",
				giveFrom: webPath, givePkg: corePath, giveName: rowName,
			},
			{
				name:     "returns nil for the import of another package without a name",
				giveFrom: webPath, givePkg: corePath,
			},
			{
				name:     "returns nil for a package of an internal tree from the parent of the tree",
				giveFrom: corePath, givePkg: coreInternal, giveName: rowName,
			},
			{
				name:     "returns nil for a package of an internal tree from a package below its parent",
				giveFrom: coreAPI, givePkg: coreInternal, giveName: rowName,
			},
			{
				name:     "returns nil for a package of an internal tree from the external tests of its parent",
				giveFrom: coreTests, givePkg: coreInternal, giveName: rowName,
			},
			{
				name:     "returns nil for an unexported declaration of an external test package from its own code",
				giveFrom: coreTests, givePkg: coreTests, giveName: rowsName,
			},
		}
		for _, tt := range admitted {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.NoError(t, golang.CheckReference(tt.giveFrom, tt.givePkg, tt.giveName),
					"Go code of "+tt.giveFrom+" refers to the declaration")
			})
		}

		refused := []struct {
			name     string
			giveFrom string
			givePkg  string
			giveName string
			want     string
		}{
			{
				name:     "returns an error for a package whose path is not an import path",
				giveFrom: webPath, givePkg: "example.com/bad store", giveName: rowName,
				want: "malformed import path",
			},
			{
				name:     "returns an error for the external test package of another directory",
				giveFrom: webPath, givePkg: coreTests, giveName: rowName,
				want: coreTests + " is an external test package",
			},
			{
				name:     "returns an error for the external test package of the package of the code",
				giveFrom: corePath, givePkg: coreTests, giveName: rowName,
				want: coreTests + " is an external test package",
			},
			{
				name:     "returns an error for a package of an internal tree that does not contain the code",
				giveFrom: webPath, givePkg: coreInternal, giveName: rowName,
				want: "internal to " + corePath,
			},
			{
				name:     "returns an error for an internal package of another tree",
				giveFrom: webPath, givePkg: corePath + "/internal", giveName: rowName,
				want: "internal to " + corePath,
			},
			{
				name:     "returns an error for a package of the internal tree of the standard library",
				giveFrom: webPath, givePkg: "internal/abi", giveName: "Type",
				want: "standard library",
			},
			{
				name:     "returns an error for an unexported declaration of another package",
				giveFrom: webPath, givePkg: corePath, giveName: rowsName,
				want: rowsName + " is not exported",
			},
			{
				name:     "returns an error for an unexported declaration of a package from its external tests",
				giveFrom: coreTests, givePkg: corePath, giveName: rowsName,
				want: rowsName + " is not exported",
			},
		}
		for _, tt := range refused {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				err := golang.CheckReference(tt.giveFrom, tt.givePkg, tt.giveName)
				assert.HasError(t, err, "Go code of "+tt.giveFrom+" cannot refer to the declaration")
				assert.Contains(t, err.Error(), tt.want, "the error names the rule")
			})
		}
	})
}

// CheckReference allocates nothing for a declaration that the code can refer
// to. The ordinary run, which runs no benchmark, checks that ceiling here.
func TestReferenceAllocs(t *testing.T) {
	var err error
	assert.MaxAllocs(t, func() { err = golang.CheckReference(coreAPI, coreInternal, rowName) }, 0,
		"CheckReference allocates nothing for a declaration that the code can refer to")
	assert.NoError(t, err, "the package below the parent of the tree refers to the declaration")
}

// BenchmarkReference measures the check that the layout and the backend
// make for each reference of a generated declaration.
func BenchmarkReference(b *testing.B) {
	b.Run("CheckReference", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var err error
		for c.Loop() {
			err = golang.CheckReference(coreAPI, coreInternal, rowName)
		}
		assert.NoError(b, err, "the package below the parent of the tree refers to the declaration")
	})
}
