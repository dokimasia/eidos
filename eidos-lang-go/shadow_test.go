// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/emit"
)

// The names of the signatures of the shadow cases: predeclared
// identifiers of Go, the names that they take, and a name that the
// universe scope does not declare.
const (
	appendName = "append"
	lenName    = "len"
	errorName  = "error"
	trueName   = "true"
	ctxName    = "ctx"
)

// renameAllocs is the ceiling of a call of RenamePredeclared that renames
// one name: the set of the names of the method and the new name.
const renameAllocs = 2

// TestShadow pins the names that RenamePredeclared gives the parameters
// and the results that would hide a predeclared identifier of Go.
func TestShadow(t *testing.T) {
	t.Parallel()

	t.Run("RenamePredeclared", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name        string
			giveParams  []string
			giveResults []string
			wantParams  []string
			wantResults []string
		}{
			{
				name:       "renames a parameter that has the name of a predeclared function",
				giveParams: []string{appendName}, wantParams: []string{appendName + "0"},
			},
			{
				name:        "renames a result that has the name of a predeclared type",
				giveResults: []string{errorName}, wantResults: []string{errorName + "0"},
			},
			{
				name:       "renames a parameter that has the name of a predeclared constant",
				giveParams: []string{trueName}, wantParams: []string{trueName + "0"},
			},
			{
				name:       "numbers the new name past a name that the method has",
				giveParams: []string{lenName, lenName + "0"}, wantParams: []string{lenName + "1", lenName + "0"},
			},
			{
				name:       "keeps a name that the universe scope does not declare",
				giveParams: []string{ctxName}, wantParams: []string{ctxName},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				m := golang.RenamePredeclared(signature(tt.giveParams, tt.giveResults))
				var params, results []string
				for _, p := range m.Params.Items() {
					params = append(params, p.Name)
				}
				for _, r := range m.Returns.Items() {
					results = append(results, r.Name)
				}
				expect.Equal(
					t,
					params,
					tt.wantParams,
					"every parameter has a name that hides no predeclared identifier",
				)
				expect.Equal(t, results, tt.wantResults, "every result has a name that hides no predeclared identifier")
			})
		}

		t.Run("returns the method that it renames", func(t *testing.T) {
			t.Parallel()

			m := signature([]string{appendName}, nil)
			assert.Equal(t, golang.RenamePredeclared(m), m, "the call chains after Mirror", assert.ByIdentity())
		})

		t.Run("returns nil for a nil method", func(t *testing.T) {
			t.Parallel()

			assert.Nil(t, golang.RenamePredeclared(nil), "a nil method has no names")
		})
	})
}

// TestShadowAllocs checks the allocation ceilings of RenamePredeclared in
// a run without benchmarks: nothing for a method without a predeclared
// name, and the set and the new name for one predeclared name.
func TestShadowAllocs(t *testing.T) {
	plain, shadowing := signature([]string{ctxName}, nil), signature([]string{appendName}, nil)
	param := shadowing.Params.Items()[0]
	var got *emit.Method
	assert.MaxAllocs(t, func() { got = golang.RenamePredeclared(plain) }, 0,
		"RenamePredeclared allocates nothing for a method without a predeclared name")
	assert.Equal(t, got.Params.Items()[0].Name, ctxName, "RenamePredeclared keeps the name")
	assert.MaxAllocs(t, func() {
		param.Name = appendName
		got = golang.RenamePredeclared(shadowing)
	}, renameAllocs, "RenamePredeclared allocates the set of the names and the new name")
	assert.Equal(t, got.Params.Items()[0].Name, appendName+"0", "RenamePredeclared numbers the name")
}

// BenchmarkShadow measures RenamePredeclared over a method whose one
// parameter has the name of a predeclared function, which a generator of
// doubles renames for each mirrored method.
func BenchmarkShadow(b *testing.B) {
	b.Run("RenamePredeclared", func(b *testing.B) {
		m := signature([]string{appendName}, nil)
		param := m.Params.Items()[0]
		c := bench.Start(b).MaxAllocs(renameAllocs)
		defer c.End()
		for c.Loop() {
			param.Name = appendName
			m = golang.RenamePredeclared(m)
		}
		assert.Equal(b, param.Name, appendName+"0", "RenamePredeclared numbers the name")
	})
}

// signature returns a method with a parameter for each name of params and
// a result for each name of results.
func signature(params, results []string) *emit.Method {
	m := &emit.Method{Name: putMethod}
	for _, name := range params {
		m.Params.Append(&emit.Param{Name: name, Type: &emit.TypeRef{Spelling: sessionTy}})
	}
	for _, name := range results {
		m.Returns.Append(&emit.Return{Name: name, Type: &emit.TypeRef{Spelling: sessionTy}})
	}
	return m
}
