// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/protobuf/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// clusterAllocs is the cluster of a unit without an extension: nothing.
const clusterAllocs = 0

// TestCluster pins the extend blocks at file level: one cluster for each
// extended message, with the message of each group after its field.
func TestCluster(t *testing.T) {
	t.Parallel()

	t.Run("Cluster", func(t *testing.T) {
		t.Parallel()

		t.Run("returns one cluster for each extended message in the order of its first extension", func(t *testing.T) {
			t.Parallel()

			first := &emit.Field{Name: "first", Receives: &emit.TypeRef{Spelling: otherName}}
			another := &emit.Field{Name: "another", Receives: &emit.TypeRef{Spelling: anotherName}}
			second := &emit.Field{Name: "second", Receives: &emit.TypeRef{Spelling: otherName}}
			row := &emit.Struct{Name: "Row"}
			got := backend.Cluster([]symbol.Symbol{first, row, another, second, &emit.Field{Name: "count"}})
			assert.Equal(t, got, []render.Clustered{
				{Group: backend.ExtendGroup, Decls: []symbol.Symbol{first, second}},
				{Group: backend.ExtendGroup, Decls: []symbol.Symbol{another}},
			}, "the extensions of each extended message")
		})

		t.Run("returns the message of a delimited extension after its field", func(t *testing.T) {
			t.Parallel()

			group := &emit.Struct{Name: resultName}
			result := &emit.Field{
				Name:        "result",
				Receives:    &emit.TypeRef{Spelling: otherName},
				Annotations: symbol.Annotations{delimitedMarker},
				Type: &emit.TypeRef{
					Spelling: "repeated Result",
					Form:     symbol.FormList,
					Elems:    []*emit.TypeRef{{Spelling: resultName}},
				},
			}
			got := backend.Cluster([]symbol.Symbol{group, result})
			assert.Equal(t, got, []render.Clustered{
				{Group: backend.ExtendGroup, Decls: []symbol.Symbol{result, group}},
			}, "the extension and the message of its group")
		})

		t.Run("returns a delimited extension without the message of a group alone", func(t *testing.T) {
			t.Parallel()

			result := &emit.Field{
				Name:        "result",
				Receives:    &emit.TypeRef{Spelling: otherName},
				Annotations: symbol.Annotations{delimitedMarker},
				Type:        &emit.TypeRef{Spelling: resultName},
			}
			got := backend.Cluster([]symbol.Symbol{&emit.Struct{Name: "Other"}, result, &emit.Field{
				Name:     "untyped",
				Receives: &emit.TypeRef{Spelling: otherName}, Annotations: symbol.Annotations{delimitedMarker},
			}})
			assert.Length(t, got, 1, "one message is extended")
			assert.Length(t, got[0].Decls, 2, "the cluster contains the two extensions alone")
		})

		t.Run("returns a delimited extension with an upper-case name without a group", func(t *testing.T) {
			t.Parallel()

			result := &emit.Field{
				Name: resultName, Receives: &emit.TypeRef{Spelling: otherName},
				Annotations: symbol.Annotations{delimitedMarker}, Type: &emit.TypeRef{Spelling: resultName},
			}
			got := backend.Cluster([]symbol.Symbol{&emit.Struct{Name: resultName}, result})
			assert.Equal(t, got, []render.Clustered{
				{Group: backend.ExtendGroup, Decls: []symbol.Symbol{result}},
			}, "the field of a group has the name of its message in lower case")
		})

		t.Run("returns no cluster for a unit without an extension", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, backend.Cluster([]symbol.Symbol{&emit.Struct{Name: "Row"}}), "no extend block")
		})
	})

	t.Run("Groups", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the extend template under the extend group", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.Groups(), map[render.GroupName]string{backend.ExtendGroup: backend.ExtendTemplate},
				"the group templates")
		})
	})
}

// Cluster allocates nothing for a unit without an extension. The ordinary
// run, which runs no benchmark, checks that ceiling here.
func TestClusterAllocs(t *testing.T) {
	decls := []symbol.Symbol{&emit.Struct{Name: "Row"}, &emit.Enum{Name: "Kind"}}
	var got []render.Clustered
	assert.MaxAllocs(t, func() { got = backend.Cluster(decls) }, clusterAllocs, "Cluster allocates nothing")
	assert.Empty(t, got, "Cluster gathers nothing")
}

// BenchmarkCluster measures the cluster that every unit of a render runs.
func BenchmarkCluster(b *testing.B) {
	b.Run("Cluster", func(b *testing.B) {
		decls := []symbol.Symbol{&emit.Struct{Name: "Row"}, &emit.Enum{Name: "Kind"}}
		c := bench.Start(b).MaxAllocs(clusterAllocs)
		defer c.End()
		var got []render.Clustered
		for c.Loop() {
			got = backend.Cluster(decls)
		}
		assert.Empty(b, got, "Cluster gathers nothing")
	})
}
