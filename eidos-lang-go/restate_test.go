// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The spellings of the children that the cases restate, as the file
// writes them.
const (
	rowSpelling   = "rows.Row"
	keySpelling   = "rows.Key"
	errorSpelling = "error"
)

// The allocations of the restatements that the allocation test and the
// benchmark measure.
const (
	// restateAllocs is a slice: its spelling.
	restateAllocs = 1
	// restateFuncAllocs is a function type of one parameter and two
	// results: the joined parameters, the joined results and the
	// spelling.
	restateFuncAllocs = 3
)

// restateCall is one restatement that the allocation test and the
// benchmark measure: the reference, the spellings of its children, its
// ceiling and the spelling that it returns.
type restateCall struct {
	name   string
	give   *emit.TypeRef
	elems  []string
	allocs uint64
	want   string
}

// TestRestate checks the Go spelling of each composite form from the
// spellings of its children, and the error for each reference whose
// structure does not determine its spelling.
func TestRestate(t *testing.T) {
	t.Parallel()

	t.Run("Restate", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			give  *emit.TypeRef
			elems []string
			want  string
		}{
			{
				name:  "writes a pointer for an optional",
				give:  &emit.TypeRef{Form: symbol.FormOptional, Spelling: "*r.Row"},
				elems: []string{rowSpelling},
				want:  "*rows.Row",
			},
			{
				name: "writes a slice for a list", give: &emit.TypeRef{Form: symbol.FormList, Spelling: "[]r.Row"},
				elems: []string{rowSpelling}, want: "[]rows.Row",
			},
			{
				name:  "writes a variadic parameter for a list spelled with three dots",
				give:  &emit.TypeRef{Form: symbol.FormList, Spelling: "...r.Row"},
				elems: []string{rowSpelling}, want: "...rows.Row",
			},
			{
				name:  "writes an array of its length",
				give:  &emit.TypeRef{Form: symbol.FormArray, Spelling: "[4]r.Row", Length: 4},
				elems: []string{rowSpelling}, want: "[4]rows.Row",
			},
			{
				name:  "writes a map of its key and its value",
				give:  &emit.TypeRef{Form: symbol.FormMap, Spelling: "map[r.Key]r.Row"},
				elems: []string{keySpelling, rowSpelling},
				want:  "map[rows.Key]rows.Row",
			},
			{
				name:  "writes a receive-only channel for a spelling with spaces",
				give:  &emit.TypeRef{Form: symbol.FormStream, Spelling: "<- chan r.Row"},
				elems: []string{rowSpelling}, want: "<-chan rows.Row",
			},
			{
				name:  "writes a send-only channel for a spelling with spaces",
				give:  &emit.TypeRef{Form: symbol.FormStream, Spelling: "chan <- r.Row"},
				elems: []string{rowSpelling}, want: "chan<- rows.Row",
			},
			{
				name:  "writes a bidirectional channel",
				give:  &emit.TypeRef{Form: symbol.FormStream, Spelling: "chan r.Row"},
				elems: []string{rowSpelling}, want: "chan rows.Row",
			},
			{
				name:  "writes a function type without results",
				give:  &emit.TypeRef{Form: symbol.FormFunc, Spelling: "func(row r.Row)", Split: 1},
				elems: []string{rowSpelling}, want: "func(rows.Row)",
			},
			{
				name:  "writes a function type of one result after a space",
				give:  &emit.TypeRef{Form: symbol.FormFunc, Spelling: "func(r.Row) error", Split: 1},
				elems: []string{rowSpelling, errorSpelling}, want: "func(rows.Row) error",
			},
			{
				name:  "writes a function type of two results in parentheses",
				give:  &emit.TypeRef{Form: symbol.FormFunc, Spelling: "func() (r.Row, error)"},
				elems: []string{rowSpelling, errorSpelling}, want: "func() (rows.Row, error)",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := golang.Restate(tt.give, tt.elems)
				assert.NoError(t, err, "Go writes the form from its structure")
				assert.Equal(t, got, tt.want, "the spelling has the spellings of the children")
			})
		}

		refusals := []struct {
			name  string
			give  *emit.TypeRef
			elems []string
			want  string
		}{
			{
				name:  "returns an error for a map of one child",
				give:  &emit.TypeRef{Form: symbol.FormMap, Spelling: "map[r.Key]r.Row"},
				elems: []string{keySpelling},
				want:  "golang: map[r.Key]r.Row has 1 of the 2 types that its form takes",
			},
			{
				name:  "returns an error for a function type split past its children",
				give:  &emit.TypeRef{Form: symbol.FormFunc, Spelling: "func(r.Row, r.Key)", Split: 2},
				elems: []string{rowSpelling},
				want:  "golang: func(r.Row, r.Key) has 1 of the 2 types that its form takes",
			},
			{
				name:  "returns an error for an array whose length is an expression",
				give:  &emit.TypeRef{Form: symbol.FormArray, Spelling: "[n]r.Row"},
				elems: []string{rowSpelling},
				want:  "golang: [n]r.Row has a length that is an expression, and the reference records a literal length alone",
			},
			{
				name:  "returns an error for a union",
				give:  &emit.TypeRef{Form: symbol.FormUnion, Spelling: "~int | r.Key"},
				elems: []string{"int", keySpelling},
				want:  "golang: ~int | r.Key has a form without a Go spelling that follows from its structure",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := golang.Restate(tt.give, tt.elems)
				assert.HasError(t, err, "Go does not write the reference from its structure")
				assert.Equal(t, err.Error(), tt.want, "the error has the spelling of the reference")
				assert.Equal(t, got, "", "a refusal returns no spelling")
			})
		}
	})
}

// Restate allocates the spelling that it returns, and a function type
// allocates its joined lists too. The ordinary run, which runs no
// benchmark, checks those ceilings here.
func TestRestateAllocs(t *testing.T) {
	for _, tt := range restateCalls() {
		var (
			got string
			err error
		)
		assert.MaxAllocs(t, func() { got, err = golang.Restate(tt.give, tt.elems) }, tt.allocs,
			"Restate of "+tt.name+" allocates within its ceiling")
		assert.NoError(t, err, "Restate writes "+tt.name)
		assert.Equal(t, got, tt.want, "Restate writes "+tt.name+" in Go")
	}
}

// BenchmarkRestate measures the restatement that the Go backend writes
// for a composite whose child spells otherwise, and that the Go spoke
// writes for every composite of a translation.
func BenchmarkRestate(b *testing.B) {
	b.Run("Restate", func(b *testing.B) {
		for _, tt := range restateCalls() {
			b.Run(tt.name, func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var (
					got string
					err error
				)
				for c.Loop() {
					got, err = golang.Restate(tt.give, tt.elems)
				}
				assert.NoError(b, err, "Restate writes "+tt.name)
				assert.Equal(b, got, tt.want, "Restate writes "+tt.name+" in Go")
			})
		}
	})
}

// restateCalls returns the restatements that the allocation test and the
// benchmark measure: a slice and a function type of two results.
func restateCalls() []restateCall {
	return []restateCall{
		{
			name: "a slice", give: &emit.TypeRef{Form: symbol.FormList, Spelling: "[]r.Row"},
			elems: []string{rowSpelling}, allocs: restateAllocs, want: "[]rows.Row",
		},
		{
			name:  "a function type of two results",
			give:  &emit.TypeRef{Form: symbol.FormFunc, Spelling: "func(r.Key) (r.Row, error)", Split: 1},
			elems: []string{keySpelling, rowSpelling, errorSpelling}, allocs: restateFuncAllocs,
			want: "func(rows.Key) (rows.Row, error)",
		},
	}
}
