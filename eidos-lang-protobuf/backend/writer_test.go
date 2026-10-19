// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/lang/protobuf/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The files of the writer cases: the file under render and the files that
// it refers into, by their import paths, and the clauses of their packages.
const (
	homePath     = "acme/svc/store.proto"
	homeClause   = "acme.svc"
	siblingPath  = "acme/svc/session.proto"
	otherPath    = "acme/other/user.proto"
	otherClause  = "acme.other"
	rootPath     = "root.proto"
	externalPath = "google/type/date.proto"
)

// The allocations of the spellings that the allocation test and the
// benchmark measure.
const (
	// namedAllocs is a reference written as it is: nothing.
	namedAllocs = 0
	// qualifiedAllocs is a reference into a file of another package: the
	// qualified name.
	qualifiedAllocs = 1
	// repeatedAllocs is a list of a reference: the spelling with its
	// label.
	repeatedAllocs = 1
)

// sourceUser is the declaration of another language that a translated
// reference refers to.
var sourceUser = symbol.Identity{Lang: "golang", Package: "example.com/other", Name: "User", Kind: symbol.KindStruct}

// TestWriter pins how a protobuf file writes each reference, and which
// import each reference records.
func TestWriter(t *testing.T) {
	t.Parallel()

	t.Run("Spell", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name        string
			give        *emit.TypeRef
			want        string
			wantImports []string
		}{
			{
				name: "returns a scalar as written",
				give: &emit.TypeRef{Spelling: "int64"}, want: "int64",
			},
			{
				name: "returns a well-known type as written and imports the file that declares it",
				give: &emit.TypeRef{Spelling: protobuf.TypeTimestamp},
				want: protobuf.TypeTimestamp, wantImports: []string{protobuf.TimestampFile},
			},
			{
				name: "returns a reference to a protobuf declaration as written and imports its file",
				give: &emit.TypeRef{
					Spelling: "other.User",
					Package:  otherPath,
					Target: symbol.Identity{
						Lang:    protobuf.Lang,
						Package: otherClause,
						Name:    "User",
						Kind:    symbol.KindStruct,
					},
				},
				want: "other.User", wantImports: []string{otherPath},
			},
			{
				name: "returns a reference into a file of another package under the clause of that file",
				give: &emit.TypeRef{Spelling: "User", Package: otherPath, Target: sourceUser},
				want: otherClause + ".User", wantImports: []string{otherPath},
			},
			{
				name: "returns a reference into a file of the same package bare",
				give: &emit.TypeRef{Spelling: "Session", Package: siblingPath},
				want: "Session", wantImports: []string{siblingPath},
			},
			{
				name: "returns a reference into a file without a package bare",
				give: &emit.TypeRef{Spelling: "Root", Package: rootPath},
				want: "Root", wantImports: []string{rootPath},
			},
			{
				name: "returns a reference into a file outside the plan as written",
				give: &emit.TypeRef{Spelling: "google.type.Date", Package: externalPath},
				want: "google.type.Date", wantImports: []string{externalPath},
			},
			{
				name: "returns a reference into the file itself bare and imports nothing",
				give: &emit.TypeRef{Spelling: "Row", Package: homePath, Target: sourceUser},
				want: "Row",
			},
			{
				name: "returns a reference without a package as written",
				give: &emit.TypeRef{Spelling: "Row"}, want: "Row",
			},
			{
				name: "returns a list under the label repeated",
				give: &emit.TypeRef{Form: symbol.FormList, Elems: []*emit.TypeRef{{Spelling: "string"}}},
				want: "repeated string",
			},
			{
				name: "returns a map of its key and its value",
				give: &emit.TypeRef{
					Form:  symbol.FormMap,
					Elems: []*emit.TypeRef{{Spelling: "string"}, {Spelling: protobuf.TypeTimestamp}},
				},
				want: "map<string, " + protobuf.TypeTimestamp + ">", wantImports: []string{protobuf.TimestampFile},
			},
			{
				name: "returns an optional as its type",
				give: &emit.TypeRef{Form: symbol.FormOptional, Elems: []*emit.TypeRef{{Spelling: "Row"}}},
				want: "Row",
			},
			{
				name: "returns a stream under the mark stream",
				give: &emit.TypeRef{Form: symbol.FormStream, Async: true, Elems: []*emit.TypeRef{{Spelling: "Row"}}},
				want: "stream Row",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				set := homeSet()
				got, err := backend.NewWriter(set).Spell(tt.give)
				assert.NoError(t, err, "protobuf writes the reference")
				assert.Equal(t, got, tt.want, "the spelling of the reference")
				assert.Equal(t, set.Paths(), tt.wantImports, "the imports that the reference records")
			})
		}

		refusals := []struct {
			name string
			give *emit.TypeRef
			want string
		}{
			{
				name: "returns an error for no reference",
				want: "protobuf: the declaration states no type",
			},
			{
				name: "returns an error for a reference without a spelling",
				give: &emit.TypeRef{},
				want: "protobuf: the declaration states no type",
			},
			{
				name: "returns an error for a reference with type arguments",
				give: &emit.TypeRef{Spelling: "Page", Args: []*emit.TypeRef{{Spelling: "int64"}}},
				want: "protobuf: Page states 1 type arguments, and protobuf has none",
			},
			{
				name: "returns an error for a list of a list",
				give: &emit.TypeRef{
					Spelling: "[][]string", Form: symbol.FormList,
					Elems: []*emit.TypeRef{{Form: symbol.FormList, Elems: []*emit.TypeRef{{Spelling: "string"}}}},
				},
				want: "protobuf: [][]string is a list of a list, and protobuf writes such a field through a wrapper message",
			},
			{
				name: "returns an error for a map to a map",
				give: &emit.TypeRef{
					Spelling: "map[string]map[string]int", Form: symbol.FormMap,
					Elems: []*emit.TypeRef{
						{Spelling: "string"},
						{Form: symbol.FormMap, Elems: []*emit.TypeRef{{Spelling: "string"}, {Spelling: "int32"}}},
					},
				},
				want: "protobuf: map[string]map[string]int is a map of a map, " +
					"and protobuf writes such a field through a wrapper message",
			},
			{
				name: "returns an error for a map of a list key",
				give: &emit.TypeRef{
					Spelling: "Map<string[], int>", Form: symbol.FormMap,
					Elems: []*emit.TypeRef{
						{Form: symbol.FormList, Elems: []*emit.TypeRef{{Spelling: "string"}}},
						{Spelling: "int32"},
					},
				},
				want: "protobuf: Map<string[], int> is a map of a list, " +
					"and protobuf writes such a field through a wrapper message",
			},
			{
				name: "returns an error for a map of one child",
				give: &emit.TypeRef{
					Spelling: "map[string]",
					Form:     symbol.FormMap,
					Elems:    []*emit.TypeRef{{Spelling: "string"}},
				},
				want: "protobuf: a map takes two children, and the shape has 1",
			},
			{
				name: "returns an error for a list without a child",
				give: &emit.TypeRef{Spelling: "[]", Form: symbol.FormList},
				want: "protobuf: a list takes one child, and the shape has 0",
			},
			{
				name: "returns an error for a tuple",
				give: &emit.TypeRef{
					Spelling: "(string, int)", Form: symbol.FormTuple,
					Elems: []*emit.TypeRef{{Spelling: "string"}, {Spelling: "int32"}},
				},
				want: "protobuf: (string, int) is a tuple, which protobuf does not spell",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := backend.NewWriter(homeSet()).Spell(tt.give)
				assert.HasError(t, err, "protobuf has no spelling of the reference")
				assert.Equal(t, err.Error(), tt.want, "the error states the reason")
				assert.Empty(t, got, "a refusal returns no spelling")
			})
		}
	})
}

// Spell writes a reference as it is without allocating, and allocates a
// qualified name and a label. The ordinary run, which runs no benchmark,
// checks those ceilings here.
func TestWriterAllocs(t *testing.T) {
	for _, tt := range writerCalls() {
		w := backend.NewWriter(grownHomeSet())
		var (
			got string
			err error
		)
		assert.MaxAllocs(t, func() { got, err = w.Spell(tt.give) }, tt.allocs,
			"Spell of "+tt.name+" allocates within its ceiling")
		assert.NoError(t, err, "Spell writes "+tt.name)
		assert.Equal(t, got, tt.want, "Spell writes "+tt.name+" in protobuf")
	}
}

// BenchmarkWriter measures the spelling of the references that every field
// of a protobuf file writes.
func BenchmarkWriter(b *testing.B) {
	b.Run("Spell", func(b *testing.B) {
		for _, tt := range writerCalls() {
			b.Run(tt.name, func(b *testing.B) {
				w := backend.NewWriter(grownHomeSet())
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var (
					got string
					err error
				)
				for c.Loop() {
					got, err = w.Spell(tt.give)
				}
				assert.NoError(b, err, "Spell writes "+tt.name)
				assert.Equal(b, got, tt.want, "Spell writes "+tt.name+" in protobuf")
			})
		}
	})
}

// writerCall is one reference that the allocation test and the benchmark
// measure: the reference, its ceiling and the spelling that it returns.
type writerCall struct {
	name   string
	give   *emit.TypeRef
	allocs uint64
	want   string
}

// writerCalls returns the references that the allocation test and the
// benchmark measure: a reference into a file of the same package, a
// reference into a file of another package, and a list.
func writerCalls() []writerCall {
	return []writerCall{
		{
			name: "a reference into a file of the same package", allocs: namedAllocs, want: "Session",
			give: &emit.TypeRef{Spelling: "Session", Package: siblingPath},
		},
		{
			name: "a reference into a file of another package", allocs: qualifiedAllocs, want: otherClause + ".User",
			give: &emit.TypeRef{Spelling: "User", Package: otherPath, Target: sourceUser},
		},
		{
			name: "a list", allocs: repeatedAllocs, want: "repeated string",
			give: &emit.TypeRef{Form: symbol.FormList, Elems: []*emit.TypeRef{{Spelling: "string"}}},
		},
	}
}

// homeSet returns the import set of the file at homePath, whose references
// refer into the files at otherPath, siblingPath and rootPath, in the order
// of their paths as the layout records them.
func homeSet() *render.ImportSet {
	set := &render.ImportSet{}
	set.SetPackage(filePackage(homePath, homeClause))
	set.SetReferents([]symbol.Identity{
		filePackage(otherPath, otherClause),
		filePackage(siblingPath, homeClause),
		filePackage(rootPath, ""),
	})
	return set
}

// grownHomeSet returns the set of [homeSet] after each reference of the
// measured calls was recorded once, so the set has the storage that a file
// of the render reuses.
func grownHomeSet() *render.ImportSet {
	set := homeSet()
	set.Add(siblingPath)
	set.Add(otherPath)
	return set
}

// filePackage returns the package of the protobuf file at an import path,
// with the clause of the file.
func filePackage(importPath, clause string) symbol.Identity {
	return symbol.Identity{Lang: protobuf.Lang, Package: importPath, Name: clause, Kind: symbol.KindPackage}
}
