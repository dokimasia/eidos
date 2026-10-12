// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The spellings of the children that the restate cases pass.
const (
	stringType = "string"
	rowType    = "Row"
	keyType    = "Key"
)

// restateAllocs is the spelling that Restate returns, sized once.
const restateAllocs = 1

// Each composite form of TypeScript restates from the spellings of its
// children, and every other form returns an error.
func TestRestate(t *testing.T) {
	t.Parallel()

	t.Run("Restate", func(t *testing.T) {
		t.Parallel()

		named := &emit.TypeRef{}
		union := &emit.TypeRef{Form: symbol.FormUnion}
		fn := &emit.TypeRef{Form: symbol.FormFunc}
		frozen := &emit.TypeRef{Form: symbol.FormList, Mutability: symbol.MutabilityImmutable}
		tests := []struct {
			name      string
			giveRef   *emit.TypeRef
			giveElems []string
			want      string
		}{
			{
				name: "writes a list behind its element",
				giveRef: &emit.TypeRef{
					Form:  symbol.FormList,
					Elems: []*emit.TypeRef{named},
				},
				giveElems: []string{stringType},
				want:      "string[]",
			},
			{
				name: "writes readonly before an immutable list",
				giveRef: &emit.TypeRef{
					Form: symbol.FormList, Mutability: symbol.MutabilityImmutable, Elems: []*emit.TypeRef{named},
				},
				giveElems: []string{stringType}, want: "readonly string[]",
			},
			{
				name: "writes a union inside a list in parentheses",
				giveRef: &emit.TypeRef{
					Form:  symbol.FormList,
					Elems: []*emit.TypeRef{union},
				},
				giveElems: []string{"Row | Key"},
				want:      "(Row | Key)[]",
			},
			{
				name:      "writes an immutable list inside a list in parentheses",
				giveRef:   &emit.TypeRef{Form: symbol.FormList, Elems: []*emit.TypeRef{frozen}},
				giveElems: []string{"readonly Row[]"}, want: "(readonly Row[])[]",
			},
			{
				name:      "writes a tuple of its elements",
				giveRef:   &emit.TypeRef{Form: symbol.FormTuple, Elems: []*emit.TypeRef{named, named}},
				giveElems: []string{rowType, keyType}, want: "[Row, Key]",
			},
			{
				name: "writes readonly before an immutable tuple",
				giveRef: &emit.TypeRef{
					Form: symbol.FormTuple, Mutability: symbol.MutabilityImmutable, Elems: []*emit.TypeRef{named},
				},
				giveElems: []string{rowType}, want: "readonly [Row]",
			},
			{
				name:      "writes a map as an object type of one index signature",
				giveRef:   &emit.TypeRef{Form: symbol.FormMap, Elems: []*emit.TypeRef{named, named}},
				giveElems: []string{stringType, rowType}, want: "{ [key: string]: Row }",
			},
			{
				name: "writes readonly before the index signature of an immutable map",
				giveRef: &emit.TypeRef{
					Form: symbol.FormMap, Mutability: symbol.MutabilityImmutable, Elems: []*emit.TypeRef{named, named},
				},
				giveElems: []string{stringType, rowType}, want: "{ readonly [key: string]: Row }",
			},
			{
				name:      "writes a function type inside a union in parentheses",
				giveRef:   &emit.TypeRef{Form: symbol.FormUnion, Elems: []*emit.TypeRef{fn, named}},
				giveElems: []string{"(p0: Row) => Key", stringType}, want: "((p0: Row) => Key) | string",
			},
			{
				name:      "writes a union inside an intersection in parentheses",
				giveRef:   &emit.TypeRef{Form: symbol.FormIntersection, Elems: []*emit.TypeRef{union, named}},
				giveElems: []string{"Row | Key", stringType}, want: "(Row | Key) & string",
			},
			{
				name:      "writes a function type with its parameters and its result",
				giveRef:   &emit.TypeRef{Form: symbol.FormFunc, Split: 2, Elems: []*emit.TypeRef{named, named, named}},
				giveElems: []string{rowType, keyType, stringType}, want: "(p0: Row, p1: Key) => string",
			},
			{
				name:      "writes void for a function type without a result",
				giveRef:   &emit.TypeRef{Form: symbol.FormFunc, Split: 1, Elems: []*emit.TypeRef{named}},
				giveElems: []string{rowType}, want: "(p0: Row) => void",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := typescript.Restate(tt.giveRef, tt.giveElems)
				assert.NoError(t, err, "the form has a TypeScript spelling")
				assert.Equal(t, got, tt.want, "Restate writes the composite from its children")
			})
		}

		list := &emit.TypeRef{Form: symbol.FormList}
		refusals := []struct {
			name      string
			giveRef   *emit.TypeRef
			giveElems []string
		}{
			{
				name: "returns an error for an optional",
				giveRef: &emit.TypeRef{
					Form:  symbol.FormOptional,
					Elems: []*emit.TypeRef{named},
				},
				giveElems: []string{rowType},
			},
			{
				name: "returns an error for an array",
				giveRef: &emit.TypeRef{
					Form:  symbol.FormArray,
					Elems: []*emit.TypeRef{named},
				},
				giveElems: []string{rowType},
			},
			{
				name:      "returns an error for a function type whose last parameter is a list",
				giveRef:   &emit.TypeRef{Form: symbol.FormFunc, Split: 1, Elems: []*emit.TypeRef{list}},
				giveElems: []string{"Row[]"},
			},
			{
				name:      "returns an error for a function type of two results",
				giveRef:   &emit.TypeRef{Form: symbol.FormFunc, Split: 0, Elems: []*emit.TypeRef{named, named}},
				giveElems: []string{rowType, keyType},
			},
			{
				name:      "returns an error for fewer spellings than children",
				giveRef:   &emit.TypeRef{Form: symbol.FormMap, Elems: []*emit.TypeRef{named, named}},
				giveElems: []string{stringType},
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := typescript.Restate(tt.giveRef, tt.giveElems)
				assert.HasError(t, err, "the form has no TypeScript spelling that follows from its children")
			})
		}
	})
}

// A restatement allocates the spelling that it returns. The ordinary
// run, which runs no benchmark, checks the ceiling here.
func TestRestateAllocs(t *testing.T) {
	ref, elems := restatedList()
	var got string
	assert.MaxAllocs(t, func() { got, _ = typescript.Restate(ref, elems) }, restateAllocs,
		"Restate allocates the spelling")
	assert.Equal(t, got, "readonly string[]", "Restate writes the list")
}

// BenchmarkRestate measures the restatement of a composite that the
// backend writes for each substituted reference.
func BenchmarkRestate(b *testing.B) {
	ref, elems := restatedList()
	c := bench.Start(b).MaxAllocs(restateAllocs)
	defer c.End()
	var got string
	for c.Loop() {
		got, _ = typescript.Restate(ref, elems)
	}
	assert.Equal(b, got, "readonly string[]", "Restate writes the list")
}

// restatedList returns an immutable list of strings and the spelling of
// its element.
func restatedList() (*emit.TypeRef, []string) {
	ref := &emit.TypeRef{
		Form: symbol.FormList, Mutability: symbol.MutabilityImmutable, Elems: []*emit.TypeRef{{}},
	}
	return ref, []string{stringType}
}
