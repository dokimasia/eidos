// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/graphql/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The allocations of the extend blocks: the list of the blocks of a unit,
// the list of a block of two members, and the text of a block.
const (
	clusterAllocs   = 1
	pairAllocs      = 1 + 1
	extensionAllocs = 1
)

// The types that the extension members of the cluster cases receive: the
// object type Row and the interface Node.
var (
	rowReceiver  = &emit.TypeRef{Spelling: rowName, Target: symbol.Identity{Name: rowName, Kind: symbol.KindStruct}}
	nodeReceiver = &emit.TypeRef{
		Spelling: nodeName, Target: symbol.Identity{Name: nodeName, Kind: symbol.KindInterface},
	}
)

// TestCluster pins the extend blocks: which members a block gathers, in
// which order, and the text of a block.
func TestCluster(t *testing.T) {
	t.Parallel()

	t.Run("Cluster", func(t *testing.T) {
		t.Parallel()

		track := &emit.Method{Name: "track", Receives: rowReceiver}
		label := &emit.Field{Name: "label", Receives: rowReceiver}
		peer := &emit.Field{Name: "peer", Receives: nodeReceiver}
		row := &emit.Struct{Name: rowName}
		tests := []struct {
			name string
			give []symbol.Symbol
			want []render.Clustered
		}{
			{
				name: "returns no block for a unit without an extension member",
				give: []symbol.Symbol{row, &emit.Enum{Name: phaseName}},
			},
			{
				name: "returns one block for the members that receive one type",
				give: []symbol.Symbol{track, label},
				want: []render.Clustered{{Group: backend.ExtendGroup, Decls: []symbol.Symbol{track, label}}},
			},
			{
				name: "returns one block for each received type in the order of its first member",
				give: []symbol.Symbol{track, peer, label},
				want: []render.Clustered{
					{Group: backend.ExtendGroup, Decls: []symbol.Symbol{track, label}},
					{Group: backend.ExtendGroup, Decls: []symbol.Symbol{peer}},
				},
			},
			{
				name: "returns no block for a declaration that receives no type",
				give: []symbol.Symbol{row, track},
				want: []render.Clustered{{Group: backend.ExtendGroup, Decls: []symbol.Symbol{track}}},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, backend.Cluster(tt.give), tt.want, "the extend blocks of the unit")
			})
		}

		t.Run("returns a block that leaves the unit unchanged where a second member joins it", func(t *testing.T) {
			t.Parallel()

			decls := []symbol.Symbol{track, row, label}
			got := backend.Cluster(decls)
			assert.Equal(t, decls, []symbol.Symbol{track, row, label}, "the unit lists its declarations")
			assert.Length(t, got, 1, "the unit has one extend block")
		})
	})

	t.Run("Groups", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the extend template under the extend group", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.Groups(), map[render.GroupName]string{backend.ExtendGroup: backend.ExtendTemplate},
				"the group templates")
		})

		t.Run("returns a map of the caller's own", func(t *testing.T) {
			t.Parallel()

			delete(backend.Groups(), backend.ExtendGroup)
			assert.Length(t, backend.Groups(), 1, "a change of one map leaves the next map unchanged")
		})
	})

	t.Run("Vocabulary", func(t *testing.T) {
		t.Parallel()

		t.Run("Extension", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				give render.Clustered
				want string
			}{
				{
					name: "returns an extension of an object type with its members in their order",
					give: render.Clustered{Group: backend.ExtendGroup, Decls: []symbol.Symbol{
						&emit.Method{
							Name: "track", Receives: rowReceiver, Comment: "detached",
							Params:  emit.SlotOf(&emit.Param{Name: "since", Type: intRef}),
							Returns: emit.SlotOf(&emit.Return{Type: stringRef}),
						},
						&emit.Field{Name: "label", Receives: rowReceiver, Type: stringRef, Doc: []string{"A label."}},
					}},
					want: "extend type Row {\n  track(since: Int!): String! # detached\n  \"A label.\"\n  label: String!\n}\n",
				},
				{
					name: "returns an extension of an interface where its first member receives an interface",
					give: render.Clustered{Group: backend.ExtendGroup, Decls: []symbol.Symbol{
						&emit.Field{Name: "label", Receives: nodeReceiver, Type: stringRef},
					}},
					want: "extend interface Node {\n  label: String!\n}\n",
				},
				{
					name: "returns nothing for a block without members",
					give: render.Clustered{Group: backend.ExtendGroup},
				},
				{
					name: "returns nothing for a block whose first member receives no type",
					give: render.Clustered{Group: backend.ExtendGroup, Decls: []symbol.Symbol{
						&emit.Field{Name: "label", Type: stringRef},
					}},
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					assert.Equal(t, (&backend.Vocabulary{}).Extension(tt.give), tt.want, "the extend block")
				})
			}
		})
	})
}

// Cluster allocates the list of the blocks once, and a block of two members
// its own list. Extension allocates the text of the block. The ordinary run,
// which runs no benchmark, checks those ceilings here.
func TestClusterAllocs(t *testing.T) {
	track := &emit.Method{Name: "track", Receives: rowReceiver, Returns: emit.SlotOf(&emit.Return{Type: stringRef})}
	label := &emit.Field{Name: "label", Receives: rowReceiver, Type: stringRef}
	peer := &emit.Field{Name: "peer", Receives: nodeReceiver, Type: stringRef}
	singles := []symbol.Symbol{track, peer}
	pair := []symbol.Symbol{track, label}
	plain := []symbol.Symbol{&emit.Struct{Name: rowName}}
	var got []render.Clustered
	assert.MaxAllocs(t, func() { got = backend.Cluster(singles) }, clusterAllocs,
		"Cluster allocates the list of the blocks")
	assert.Length(t, got, 2, "Cluster returns a block for each received type")
	assert.MaxAllocs(t, func() { got = backend.Cluster(pair) }, pairAllocs,
		"Cluster allocates the list of the blocks and the list of a block of two members")
	assert.Length(t, got, 1, "Cluster returns one block for one received type")
	assert.MaxAllocs(t, func() { got = backend.Cluster(plain) }, 0,
		"Cluster allocates nothing for a unit without an extension member")
	assert.Empty(t, got, "Cluster returns no block")

	v := &backend.Vocabulary{}
	block := render.Clustered{Group: backend.ExtendGroup, Decls: pair}
	var text string
	assert.MaxAllocs(t, func() { text = v.Extension(block) }, extensionAllocs,
		"Extension allocates the text of the block")
	assert.Equal(t, text, "extend type Row {\n  track: String!\n  label: String!\n}\n", "Extension writes the block")
}

// BenchmarkCluster measures the clustering of the declarations of every
// unit, and the extend block of every type that a unit extends.
func BenchmarkCluster(b *testing.B) {
	track := &emit.Method{Name: "track", Receives: rowReceiver, Returns: emit.SlotOf(&emit.Return{Type: stringRef})}
	label := &emit.Field{Name: "label", Receives: rowReceiver, Type: stringRef}
	peer := &emit.Field{Name: "peer", Receives: nodeReceiver, Type: stringRef}
	units := []struct {
		name   string
		give   []symbol.Symbol
		want   int
		allocs uint64
	}{
		{name: "blocks of one member", give: []symbol.Symbol{track, peer}, want: 2, allocs: clusterAllocs},
		{name: "a block of two members", give: []symbol.Symbol{track, label}, want: 1, allocs: pairAllocs},
		{name: "a unit without an extension member", give: []symbol.Symbol{&emit.Struct{Name: rowName}}},
	}
	b.Run("Cluster", func(b *testing.B) {
		for _, tt := range units {
			b.Run(tt.name, func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var got []render.Clustered
				for c.Loop() {
					got = backend.Cluster(tt.give)
				}
				assert.Length(b, got, tt.want, "Cluster returns the blocks of the unit")
			})
		}
	})

	b.Run("Vocabulary", func(b *testing.B) {
		b.Run("Extension", func(b *testing.B) {
			v := &backend.Vocabulary{}
			block := render.Clustered{Group: backend.ExtendGroup, Decls: []symbol.Symbol{track, label}}
			c := bench.Start(b).Warmup(1).MaxAllocs(extensionAllocs)
			defer c.End()
			var got string
			for c.Loop() {
				got = v.Extension(block)
			}
			assert.Equal(b, got, "extend type Row {\n  track: String!\n  label: String!\n}\n",
				"Extension writes the block")
		})
	})
}
