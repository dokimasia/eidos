// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/go/frontend"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// useFile is the one file of the use cases.
const useFile = "p/a.go"

// The names that the use cases refer to in the imported packages.
const (
	typeName  = "T"
	rowName   = "Row"
	firstName = "A"
	lastName  = "B"
)

// A unit uses each declaration outside its package that a reference of its
// declarations or its directives can resolve to, so a dependency round
// places the packages that the load needs.
func TestUse(t *testing.T) {
	t.Parallel()

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			giveDepth plugin.Depth
			giveSrc   string
			want      []symbol.Identity
		}{
			{
				name:      "records a use of a type that a qualifier names",
				giveDepth: plugin.DepthFull,
				giveSrc: "package p\n\nimport \"" + signatureImport + "\"\n\n" +
					"// T has a sig.T.\ntype T struct{ F sig.T }\n",
				want: []symbol.Identity{at(signatureImport, typeName)},
			},
			{
				name:      "records a use of each unaliased import for a qualifier that no import binds",
				giveDepth: plugin.DepthFull,
				giveSrc: "package p\n\nimport (\n\t\"" + signatureImport + "\"\n\tal \"" + aliasedImport + "\"\n)\n\n" +
					"// F takes a type whose qualifier no import path assumes.\nfunc F(v other.T) {}\n\nvar _ = al.X\n",
				want: []symbol.Identity{at(signatureImport, typeName)},
			},
			{
				name:      "records a use of a dot import for an exported bare name",
				giveDepth: plugin.DepthFull,
				giveSrc: "package p\n\nimport . \"" + dotImport + "\"\n\n" +
					"// F takes a dot-imported T.\nfunc F(v T) {}\n",
				want: []symbol.Identity{at(dotImport, typeName)},
			},
			{
				name:      "records a use of a type that an argument of a directive names",
				giveDepth: plugin.DepthFull,
				giveSrc: "package p\n\nimport \"" + signatureImport + "\"\n\n" +
					"// T is a table.\n//+fixture:gen:table of=sig.Row\ntype T struct{}\n",
				want: []symbol.Identity{at(signatureImport, rowName)},
			},
			{
				name:      "records a use of each element of a list argument of a directive",
				giveDepth: plugin.DepthFull,
				giveSrc: "package p\n\nimport \"" + signatureImport + "\"\n\n" +
					"// T is a table.\n//+fixture:gen:table of=[sig.A,sig.B]\ntype T struct{}\n",
				want: []symbol.Identity{at(signatureImport, firstName), at(signatureImport, lastName)},
			},
			{
				name:      "records a use of an exported declaration at signature depth",
				giveDepth: plugin.DepthSignatures,
				giveSrc: "package p\n\nimport \"" + signatureImport + "\"\n\n" +
					"// T has a sig.T.\ntype T struct{ F sig.T }\n",
				want: []symbol.Identity{at(signatureImport, typeName)},
			},
			{
				name:      "does not record a use of an unexported declaration at signature depth",
				giveDepth: plugin.DepthSignatures,
				giveSrc:   "package p\n\nimport \"" + signatureImport + "\"\n\ntype t struct{ F sig.T }\n",
			},
			{
				name:      "does not record a use of a package that a function body names",
				giveDepth: plugin.DepthFull,
				giveSrc:   "package p\n\nimport \"" + bodyImport + "\"\n\n// F calls body.Do.\nfunc F() { body.Do() }\n",
			},
			{
				name:      "does not record a use of a package that a value names",
				giveDepth: plugin.DepthFull,
				giveSrc:   "package p\n\nimport \"runtime\"\n\n// System is the system of a build.\nvar System = runtime.GOOS\n",
			},
			{
				name:      "does not record a use of a declaration of the package of the file",
				giveDepth: plugin.DepthFull,
				giveSrc:   "package p\n\n// A has a B.\ntype A struct{ F B }\n\n// B is empty.\ntype B struct{}\n",
			},
			{
				name:      "does not record a use of a predeclared type",
				giveDepth: plugin.DepthFull,
				giveSrc:   "package p\n\n// A has a text.\ntype A struct{ S string }\n",
			},
			{
				name:      "does not record a use of a file that the build excludes",
				giveDepth: plugin.DepthFull,
				giveSrc: "//go:build ignore\n\npackage p\n\nimport \"" + signatureImport + "\"\n\n" +
					"// T has a sig.T.\ntype T struct{ F sig.T }\n",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, usesOf(t, tt.giveDepth, tt.giveSrc), tt.want, "the uses of the unit",
					assert.EquateEmpty())
			})
		}
	})
}

// usesOf parses the one file of a unit at a depth and returns the uses of
// the unit.
func usesOf(tb testing.TB, depth plugin.Depth, src string) []symbol.Identity {
	tb.Helper()

	tree := fstest.MapFS{useFile: {Data: []byte(src)}}
	f := frontend.New(nil)
	u := plugin.NewSourceUnit(
		[]plugin.SourceRef{{Path: useFile}}, tree, depth, f.Syntax(), brand, diag.NewSink(), f.Name(),
	)
	assert.NoError(tb, f.Parse(tb.Context(), u), "the file parses")
	return u.Uses()
}
